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
}

func newFakeBots() *fakeBots {
	return &fakeBots{enabled: map[string]bool{"zolik": false, "canasta": false, "misfit": false}}
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
