package match_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/match"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/prsi"
	"zolik/server/internal/ws"
)

// A "my games" list needs the raw repository beside the HTTP surface — these
// tests move a match between statuses directly (the same shortcut
// abandonedMatch takes in resume_test.go) rather than playing it there, so
// newInviteHarness's shape is reused but its repo is not hidden.
type storedHarness struct {
	*inviteHarness
	repo match.Repository
}

func newStoredHarness(t *testing.T) *storedHarness {
	t.Helper()
	repo := newTestRepository(t)

	hub, err := ws.NewHub(ws.NewConnRegistry(), "")
	if err != nil {
		t.Fatalf("building a hub: %v", err)
	}
	t.Cleanup(func() { _ = hub.Close() })

	manager := match.NewManager(repo, module.NewRegistry(prsi.New()), hub)
	r := chi.NewRouter()
	handlers := match.NewHandlers(manager, false)
	handlers.RegisterRoutes(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	return &storedHarness{inviteHarness: &inviteHarness{t: t, server: srv}, repo: repo}
}

func (h *storedHarness) tables(t *testing.T, bearer, query string) []map[string]any {
	t.Helper()
	res := h.do(http.MethodGet, "/users/me/tables"+query, bearer, nil)
	if res.status != http.StatusOK {
		t.Fatalf("list tables: status %d body %s", res.status, res.raw)
	}
	raw, _ := res.body["tables"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, r := range raw {
		out = append(out, r.(map[string]any))
	}
	return out
}

// A list must show a player exactly the tables they are seated at, and none
// of anyone else's — the whole point of keying it on players.id rather than
// listing the collection.
func TestMyTablesShowsOnlyTheCallersOwnUnfinishedTables(t *testing.T) {
	h := newStoredHarness(t)
	hostToken := token(t, "host-1", "Host", false)
	otherToken := token(t, "host-2", "Someone Else", false)

	matchID := h.createMatch(t, hostToken)

	mine := h.tables(t, hostToken, "")
	if len(mine) != 1 || mine[0]["matchId"] != matchID {
		t.Fatalf("host's own list = %+v, want exactly the one match they opened", mine)
	}

	theirs := h.tables(t, otherToken, "")
	for _, row := range theirs {
		if row["matchId"] == matchID {
			t.Fatalf("a table reached a player who was never seated at it: %+v", row)
		}
	}
}

// Unfinished is the default, and completed sits behind its own tab rather
// than being mixed in.
func TestMyTablesDefaultExcludesCompletedFinishedTabIncludesIt(t *testing.T) {
	h := newStoredHarness(t)
	hostToken := token(t, "host-1", "Host", false)
	matchID := h.createMatch(t, hostToken)

	oid, err := bson.ObjectIDFromHex(matchID)
	if err != nil {
		t.Fatalf("parsing match id: %v", err)
	}
	m, err := h.repo.FindByID(t.Context(), oid)
	if err != nil {
		t.Fatalf("loading match: %v", err)
	}
	m.Status = "completed"
	if err := h.repo.UpdateWithVersion(t.Context(), m.ID, m.Version, m); err != nil {
		t.Fatalf("marking completed: %v", err)
	}

	unfinished := h.tables(t, hostToken, "")
	for _, row := range unfinished {
		if row["matchId"] == matchID {
			t.Fatalf("a completed match reached the default (unfinished) list: %+v", row)
		}
	}

	finished := h.tables(t, hostToken, "?status=finished")
	found := false
	for _, row := range finished {
		if row["matchId"] == matchID {
			found = true
			if row["status"] != "completed" {
				t.Errorf("status = %v, want completed", row["status"])
			}
		}
	}
	if !found {
		t.Fatalf("the completed match never appeared in the finished tab: %+v", finished)
	}
}

// The projection is the point: a list row must never carry the module's
// state, the action log, or another seat's account/guest id.
func TestMyTablesRowNeverLeaksStateOrAccountIdentifiers(t *testing.T) {
	h := newStoredHarness(t)
	hostToken := token(t, "host-1", "Host", false)
	matchID := h.createMatch(t, hostToken)

	rows := h.tables(t, hostToken, "")
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	row := rows[0]
	if _, ok := row["state"]; ok {
		t.Errorf("row carries state: %+v", row)
	}
	if _, ok := row["actionLog"]; ok {
		t.Errorf("row carries actionLog: %+v", row)
	}
	players, _ := row["players"].([]any)
	if len(players) == 0 {
		t.Fatalf("row has no players: %+v", row)
	}
	for _, p := range players {
		pm := p.(map[string]any)
		if _, ok := pm["userId"]; ok {
			t.Errorf("a player entry carries userId: %+v", pm)
		}
		if _, ok := pm["guestId"]; ok {
			t.Errorf("a player entry carries guestId: %+v", pm)
		}
	}
	if matchID == "" {
		t.Fatal("matchId missing from the row")
	}
}

// canResume must be exactly ResumeAbandoned's own rule — a seated human, and
// every other seat a bot — so the button this list offers is never one the
// server would then refuse.
func TestMyTablesCanResumeMatchesTheResumeRule(t *testing.T) {
	h := newStoredHarness(t)

	// abandonedMatch always seats "p1" as host, so both rows are read back
	// under that identity.
	resumable := abandonedMatch(t, h.repo, bot("bot:a"))
	notResumable := abandonedMatch(t, h.repo, models.Player{ID: "someone-else", Name: "Bo"})

	rows := h.tables(t, token(t, "p1", "Ada", false), "")
	byID := map[string]map[string]any{}
	for _, row := range rows {
		byID[row["matchId"].(string)] = row
	}
	if got := byID[resumable.ID.Hex()]; got == nil || got["canResume"] != true {
		t.Errorf("bots-only abandoned table: canResume = %+v, want true", got)
	}
	if got := byID[notResumable.ID.Hex()]; got == nil || got["canResume"] != false {
		t.Errorf("abandoned table with another human: canResume = %+v, want false", got)
	}
}

// Only the host sees canDelete: true, and only the host's DELETE succeeds.
func TestMyTablesDeleteIsHostOnly(t *testing.T) {
	h := newStoredHarness(t)
	hostToken := token(t, "host-1", "Host", false)
	joinerToken := token(t, "joiner-1", "Joiner", false)
	matchID := h.createMatch(t, hostToken)

	if res := h.do(http.MethodPost, "/matches/"+matchID+"/join", joinerToken, nil); res.status != http.StatusOK {
		t.Fatalf("join: status %d body %s", res.status, res.raw)
	}

	hostRows := h.tables(t, hostToken, "")
	if len(hostRows) != 1 || hostRows[0]["canDelete"] != true {
		t.Fatalf("host row = %+v, want canDelete true", hostRows)
	}
	joinerRows := h.tables(t, joinerToken, "")
	if len(joinerRows) != 1 || joinerRows[0]["canDelete"] != false {
		t.Fatalf("joiner row = %+v, want canDelete false", joinerRows)
	}

	if res := h.do(http.MethodDelete, "/matches/"+matchID, joinerToken, nil); res.status != http.StatusForbidden {
		t.Fatalf("non-host delete: status %d body %s, want 403", res.status, res.raw)
	}
	if res := h.do(http.MethodDelete, "/matches/"+matchID, hostToken, nil); res.status != http.StatusOK {
		t.Fatalf("host delete: status %d body %s, want 200", res.status, res.raw)
	}
	if res := h.do(http.MethodGet, "/matches/"+matchID, "", nil); res.status != http.StatusNotFound {
		t.Fatalf("match still resolvable after delete: status %d", res.status)
	}
}
