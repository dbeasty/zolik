package admin

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// The governor panel on the Bots card. The governor decides which engine plays
// each bot seat so bots never take more CPU than the box can spare
// (internal/botgov, docs/bot-compute-gating-plan.md): off, observe (decides
// and counts, changes no move) or enforce (a seat it cannot afford plays a
// cheaper class until there is room). The mode was an environment variable;
// here it can be changed without a restart, and is audited like the model
// switches.

// GovernorView is the panel.
type GovernorView struct {
	// Mode is what the governor is doing now: off, observe or enforce.
	Mode string `json:"mode"`
	// Source is where Mode came from: "environment" (BOT_GOVERNOR, never
	// changed here) or "console".
	Source string `json:"source"`
	// EnvDefault is BOT_GOVERNOR's value, which a console change outranks.
	EnvDefault string     `json:"envDefault"`
	UpdatedBy  string     `json:"updatedBy,omitempty"`
	UpdatedAt  *time.Time `json:"updatedAt,omitempty"`
	// Governor and Monitor are the live status of each, rendered as they
	// come. Untyped so this package imports neither.
	Governor any `json:"governor"`
	Monitor  any `json:"monitor"`
}

// GovernorChange is a request to change the mode.
type GovernorChange struct {
	Mode string
	By   string
	At   time.Time
}

// ErrBadMode is a mode that is not off, observe or enforce.
var ErrBadMode = errors.New(`mode must be "off", "observe" or "enforce"`)

func validMode(m string) bool { return m == "off" || m == "observe" || m == "enforce" }

// withGovernor adds the governor panel to a Bots card response, when there
// is one to add. A failure to read it is logged and leaves the panel out
// rather than failing the whole card.
func (h *Handlers) withGovernor(ctx context.Context, out map[string]any) map[string]any {
	if h.deps.Governor == nil {
		return out
	}
	v, err := h.deps.Governor(ctx)
	if err != nil {
		slog.Warn("reading the bot governor failed", "error", err)
		return out
	}
	out["governor"] = v
	return out
}

func (h *Handlers) setGovernor(w http.ResponseWriter, r *http.Request) {
	if h.deps.SetGovernor == nil {
		http.Error(w, "the bot governor is not configured", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		Mode string `json:"mode"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil || !validMode(strings.TrimSpace(body.Mode)) {
		http.Error(w, ErrBadMode.Error(), http.StatusBadRequest)
		return
	}

	caller, _ := Caller(r)
	by := caller.Email
	if by == "" {
		by = caller.Username
	}
	change := GovernorChange{Mode: strings.TrimSpace(body.Mode), By: by, At: time.Now().UTC()}
	from, err := h.deps.SetGovernor(r.Context(), change)
	if err != nil {
		slog.Warn("changing the bot governor failed", "mode", change.Mode, "error", err)
		http.Error(w, "could not save the setting", http.StatusInternalServerError)
		return
	}

	// The audit line, for every accepted change, as for the model switches.
	slog.Info("admin changed bot governor",
		"from", from,
		"to", change.Mode,
		"user", caller.Username,
		"email", caller.Email,
		"viaPassword", caller.ViaPassword,
		"remote", r.RemoteAddr,
	)
	h.botsAfterChange(w, r.Context())
}
