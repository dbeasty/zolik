package admin

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// The Bots card: whether each game's AI seats play the trained model this
// binary ships. A change reaches every live table at its next bot move, so it
// is audited here — who, what, from, to — and persisted by the closure the app
// supplies before the switch moves, so a restart comes back to it.

// ErrUnknownGame is what Deps.SetHardModel returns for a game that ships no
// model. ErrModelDoesNotFit is what it returns for a model that would not
// play. Declared here so the app can translate its own errors into them
// without this package importing learn.
var (
	ErrUnknownGame     = errors.New("no shipped model for that game")
	ErrModelDoesNotFit = errors.New("the shipped model does not fit this game's encoder")
)

// HardModelRow is one game's line on the Bots card.
type HardModelRow struct {
	Game     string `json:"game"`
	Title    string `json:"title"`
	Enabled  bool   `json:"enabled"`
	Embedded bool   `json:"embedded"`
	Fits     bool   `json:"fits"`
	Problem  string `json:"problem,omitempty"`
	// EnvOverride is the model file the server's environment names for this
	// game. AI seats play it whatever the switch says.
	EnvOverride string `json:"envOverride,omitempty"`
	// Model is the shipped model's metadata: source run, training date,
	// headline benchmark, size and hash.
	Model any `json:"model"`
	// UpdatedBy and UpdatedAt are the last change. Empty for a game nobody
	// has changed, which is off.
	UpdatedBy string     `json:"updatedBy,omitempty"`
	UpdatedAt *time.Time `json:"updatedAt,omitempty"`
}

// HardModelChange is a request to move one game's switch.
type HardModelChange struct {
	Game    string
	Enabled bool
	// By is who asked: the allow-listed address, or the console username.
	By string
	At time.Time
}

func (h *Handlers) bots(w http.ResponseWriter, r *http.Request) {
	if h.deps.HardModels == nil {
		http.Error(w, "bot settings are not configured", http.StatusServiceUnavailable)
		return
	}
	rows, err := h.deps.HardModels(r.Context())
	if err != nil {
		slog.Warn("reading hard bot settings failed", "error", err)
		http.Error(w, "could not read the bot settings", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"games": rows})
}

func (h *Handlers) setBot(w http.ResponseWriter, r *http.Request) {
	if h.deps.SetHardModel == nil {
		http.Error(w, "bot settings are not configured", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil || body.Enabled == nil {
		http.Error(w, `body must be {"enabled": true|false}`, http.StatusBadRequest)
		return
	}

	caller, _ := Caller(r)
	by := caller.Email
	if by == "" {
		by = caller.Username
	}
	game := chi.URLParam(r, "game")
	change := HardModelChange{Game: game, Enabled: *body.Enabled, By: by, At: time.Now().UTC()}

	from, err := h.deps.SetHardModel(r.Context(), change)
	switch {
	case errors.Is(err, ErrUnknownGame):
		http.Error(w, "no shipped model for game "+game, http.StatusNotFound)
		return
	case errors.Is(err, ErrModelDoesNotFit):
		slog.Warn("admin refused: hard bot model does not fit",
			"game", game, "user", caller.Username, "email", caller.Email, "remote", r.RemoteAddr, "error", err)
		http.Error(w, err.Error(), http.StatusConflict)
		return
	case err != nil:
		slog.Warn("changing a hard bot setting failed", "game", game, "error", err)
		http.Error(w, "could not save the setting", http.StatusInternalServerError)
		return
	}

	// The audit line. Written for every accepted change, including one that
	// sets the switch to what it already was: somebody asked, and that is
	// worth knowing when reading back what happened to a table.
	slog.Info("admin changed hard bot",
		"game", game,
		"from", from,
		"to", change.Enabled,
		"user", caller.Username,
		"email", caller.Email,
		"viaPassword", caller.ViaPassword,
		"remote", r.RemoteAddr,
	)
	h.botsAfterChange(w, r.Context())
}

// botsAfterChange answers a change with the whole card, so the console
// redraws from what the server now holds rather than from what it asked for.
func (h *Handlers) botsAfterChange(w http.ResponseWriter, ctx context.Context) {
	rows, err := h.deps.HardModels(ctx)
	if err != nil {
		writeJSON(w, map[string]any{"games": nil})
		return
	}
	writeJSON(w, map[string]any{"games": rows})
}
