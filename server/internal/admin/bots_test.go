package admin

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"
)

// fakeBots stands in for the app's closures: a switch per game, the changes
// it was asked to store, and one game whose model does not fit.
type fakeBots struct {
	enabled map[string]bool
	stored  []HardModelChange
	// mode and modes are the governor's: what it is, and every change asked.
	mode  string
	modes []GovernorChange
}

func newFakeBots() *fakeBots {
	return &fakeBots{enabled: map[string]bool{"zolik": false, "canasta": false, "misfit": false}, mode: "observe"}
}

func (f *fakeBots) rows(context.Context) ([]HardModelRow, error) {
	out := []HardModelRow{}
	for _, g := range []string{"canasta", "misfit", "zolik"} {
		row := HardModelRow{Game: g, Title: g, Enabled: f.enabled[g], Embedded: true, Fits: g != "misfit",
			Model: map[string]any{"benchmark": "+1 vs Hard"}}
		for _, c := range f.stored {
			if c.Game == g {
				at := c.At
				row.UpdatedBy, row.UpdatedAt = c.By, &at
			}
		}
		out = append(out, row)
	}
	return out, nil
}

func (f *fakeBots) set(_ context.Context, c HardModelChange) (bool, error) {
	was, ok := f.enabled[c.Game]
	if !ok {
		return false, ErrUnknownGame
	}
	if c.Game == "misfit" && c.Enabled {
		return was, ErrModelDoesNotFit
	}
	f.stored = append(f.stored, c)
	f.enabled[c.Game] = c.Enabled
	return was, nil
}

// captureLog routes slog to a buffer for the length of a test.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestBotRoutesRequireAuth(t *testing.T) {
	h, _ := newHarness(t)
	for _, r := range [][3]string{
		{"GET", "/admin/api/bots", ""},
		{"PUT", "/admin/api/bots/zolik", `{"enabled":true}`},
	} {
		if rec := h.withToken(t, "", r[0], r[1], r[2]); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without a token: got %d, want 401", r[0], r[1], rec.Code)
		}
	}
	// A signed-in player who is not an administrator is refused too.
	if rec := h.as(t, h.other, "PUT", "/admin/api/bots/zolik", `{"enabled":true}`); rec.Code != http.StatusForbidden {
		t.Errorf("a non-admin player: got %d, want 403", rec.Code)
	}
	if h.bots.enabled["zolik"] || len(h.bots.stored) != 0 {
		t.Error("an unauthenticated request changed the switch")
	}
}

func TestBotsListsEveryGame(t *testing.T) {
	h, login := newHarness(t)
	rec := h.withToken(t, signedIn(t, h, login), "GET", "/admin/api/bots", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
	}
	games, _ := decode(t, rec)["games"].([]any)
	if len(games) != 3 {
		t.Fatalf("games = %v", games)
	}
	row, _ := games[2].(map[string]any)
	for _, k := range []string{"game", "title", "enabled", "embedded", "fits", "model"} {
		if _, ok := row[k]; !ok {
			t.Errorf("row has no %q: %v", k, row)
		}
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("bot settings are cacheable")
	}
}

func TestSetBotPersistsAndAudits(t *testing.T) {
	h, login := newHarness(t)
	logs := captureLog(t)
	token := signedIn(t, h, login)

	rec := h.withToken(t, token, "PUT", "/admin/api/bots/zolik", `{"enabled":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
	}
	if !h.bots.enabled["zolik"] {
		t.Error("the switch did not move")
	}
	if len(h.bots.stored) != 1 {
		t.Fatalf("stored %d changes, want 1", len(h.bots.stored))
	}
	c := h.bots.stored[0]
	if c.By != "operator" || c.At.IsZero() || time.Since(c.At) > time.Minute || !c.Enabled {
		t.Errorf("stored change = %+v", c)
	}
	// The response is the card as it now stands.
	games, _ := decode(t, rec)["games"].([]any)
	if z, _ := games[2].(map[string]any); z["enabled"] != true || z["updatedBy"] != "operator" {
		t.Errorf("response row = %v", z)
	}

	line := logs.String()
	for _, want := range []string{`msg="admin changed hard bot"`, "game=zolik", "from=false", "to=true", "user=operator", "remote="} {
		if !strings.Contains(line, want) {
			t.Errorf("audit line lacks %q: %s", want, line)
		}
	}
}

func TestSetBotRecordsTheAllowListedAddress(t *testing.T) {
	h, _ := newHarness(t, "boss@example.com")
	logs := captureLog(t)
	if rec := h.as(t, h.admin, "PUT", "/admin/api/bots/canasta", `{"enabled":true}`); rec.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
	}
	if len(h.bots.stored) != 1 || h.bots.stored[0].By != "boss@example.com" {
		t.Errorf("stored = %+v", h.bots.stored)
	}
	if !strings.Contains(logs.String(), "email=boss@example.com") {
		t.Errorf("audit line lacks the address: %s", logs.String())
	}
}

func TestSetBotRefusals(t *testing.T) {
	h, login := newHarness(t)
	token := signedIn(t, h, login)
	for _, tc := range []struct {
		name, path, body string
		want             int
	}{
		{"unknown game", "/admin/api/bots/prsi", `{"enabled":true}`, http.StatusNotFound},
		{"model does not fit", "/admin/api/bots/misfit", `{"enabled":true}`, http.StatusConflict},
		{"no body", "/admin/api/bots/zolik", ``, http.StatusBadRequest},
		{"no enabled field", "/admin/api/bots/zolik", `{}`, http.StatusBadRequest},
		{"not a bool", "/admin/api/bots/zolik", `{"enabled":"yes"}`, http.StatusBadRequest},
		{"unknown field", "/admin/api/bots/zolik", `{"enabled":true,"skill":"easy"}`, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if rec := h.withToken(t, token, "PUT", tc.path, tc.body); rec.Code != tc.want {
				t.Errorf("got %d (%s), want %d", rec.Code, strings.TrimSpace(rec.Body.String()), tc.want)
			}
		})
	}
	if len(h.bots.stored) != 0 {
		t.Errorf("a refused change was stored: %+v", h.bots.stored)
	}
	for g, on := range h.bots.enabled {
		if on {
			t.Errorf("%s was switched on by a refused request", g)
		}
	}
}

func (f *fakeBots) governor(context.Context) (GovernorView, error) {
	v := GovernorView{Mode: f.mode, Source: "environment", EnvDefault: "observe",
		Governor: map[string]any{"leases": 3}, Monitor: map[string]any{"level": "green"}}
	if n := len(f.modes); n > 0 {
		at := f.modes[n-1].At
		v.Source, v.UpdatedBy, v.UpdatedAt = "console", f.modes[n-1].By, &at
	}
	return v, nil
}

func (f *fakeBots) setGovernor(_ context.Context, c GovernorChange) (string, error) {
	was := f.mode
	f.mode = c.Mode
	f.modes = append(f.modes, c)
	return was, nil
}

func TestGovernorIsOnTheBotsCard(t *testing.T) {
	h, login := newHarness(t)
	rec := h.withToken(t, signedIn(t, h, login), "GET", "/admin/api/bots", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
	}
	g, _ := decode(t, rec)["governor"].(map[string]any)
	if g["mode"] != "observe" || g["source"] != "environment" || g["envDefault"] != "observe" || g["governor"] == nil || g["monitor"] == nil {
		t.Fatalf("governor panel = %v", g)
	}
}

func TestSetGovernorPersistsAndAudits(t *testing.T) {
	h, login := newHarness(t)
	logs := captureLog(t)
	token := signedIn(t, h, login)

	rec := h.withToken(t, token, "PUT", "/admin/api/governor", `{"mode":"enforce"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
	}
	if h.bots.mode != "enforce" || len(h.bots.modes) != 1 || h.bots.modes[0].By != "operator" {
		t.Fatalf("change = %+v, mode %s", h.bots.modes, h.bots.mode)
	}
	// The response is the whole card, governor included, as it now stands.
	body := decode(t, rec)
	g, _ := body["governor"].(map[string]any)
	if g["mode"] != "enforce" || g["source"] != "console" || g["updatedBy"] != "operator" {
		t.Errorf("response panel = %v", g)
	}
	if games, _ := body["games"].([]any); len(games) != 3 {
		t.Errorf("response lost the games: %v", body["games"])
	}
	line := logs.String()
	for _, want := range []string{`msg="admin changed bot governor"`, "from=observe", "to=enforce", "user=operator", "remote="} {
		if !strings.Contains(line, want) {
			t.Errorf("audit line lacks %q: %s", want, line)
		}
	}
}

func TestSetGovernorRefusesWhatIsNotAMode(t *testing.T) {
	h, login := newHarness(t)
	token := signedIn(t, h, login)
	for _, body := range []string{`{"mode":"on"}`, `{"mode":""}`, `{}`, `{"mode":"enforce","extra":1}`, `not json`} {
		if rec := h.withToken(t, token, "PUT", "/admin/api/governor", body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: got %d, want 400", body, rec.Code)
		}
	}
	if len(h.bots.modes) != 0 {
		t.Fatalf("a refused request changed the mode: %+v", h.bots.modes)
	}
}

func TestGovernorRouteRequiresAnAdministrator(t *testing.T) {
	h, _ := newHarness(t)
	if rec := h.withToken(t, "", "PUT", "/admin/api/governor", `{"mode":"enforce"}`); rec.Code != http.StatusUnauthorized {
		t.Errorf("without a token: got %d, want 401", rec.Code)
	}
	if rec := h.as(t, h.other, "PUT", "/admin/api/governor", `{"mode":"enforce"}`); rec.Code != http.StatusForbidden {
		t.Errorf("a non-admin player: got %d, want 403", rec.Code)
	}
	if len(h.bots.modes) != 0 {
		t.Fatal("an unauthorised request changed the mode")
	}
}
