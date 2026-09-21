package match_test

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"zolik/server/internal/canasta"
	"zolik/server/internal/match"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/prsi"
	"zolik/server/internal/ws"
	"zolik/server/internal/zolikmod"
)

// Opening a table starts its bots, if it is waiting on one.
//
// Every other thing that starts the bot loop follows a change to the match: it
// started, somebody acted, a table suspended for a missing player came back.
// None of those happens to a table that is *already* stuck — the loop gave up,
// or the process restarted and it was never running here at all — and the seat
// it is waiting on is a bot's, so the one person who could restart it by acting
// is the one person the engine will not let act. Game 6aaa157d0079d0b3a6624b3a
// sat exactly there: a bot on turn, no loop behind it, and a human looking at a
// board where every control said NOT_YOUR_TURN.
//
// Seeded straight into the repository rather than played into position, because
// "no loop has ever run for this match" is the whole precondition and starting
// one through the API would destroy it.
func TestOpeningASocketStartsTheBotsOnAStuckTable(t *testing.T) {
	h := newBotRestartHarness(t)
	const human, bot = "human-1", "bot:WEDGED"

	mod := canasta.New()
	refs := []module.PlayerRef{{ID: human, Name: "Human"}, {ID: bot, Name: "Bot", IsAI: true}}
	state, err := mod.NewMatch(module.MatchConfig{Options: module.Options{"targetScore": 500}}, refs, 7)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}

	// Whoever the deal put on turn is the seat that has to be the bot's, so
	// this does not depend on which end of the table the module deals to.
	awaited := module.AwaitedSeats(mod, state, human, refs)
	if len(awaited) != 1 {
		t.Fatalf("awaited = %v, want exactly one seat to wait on", awaited)
	}
	players := []models.Player{
		{ID: human, Name: "Human"},
		{ID: bot, Name: "Bot", IsAI: true},
	}
	if awaited[0] == human {
		// Swap the names over: the seat on turn becomes the bot's.
		players = []models.Player{
			{ID: human, Name: "Human", IsAI: true},
			{ID: bot, Name: "Bot"},
		}
	}
	seeded, err := h.repo.Insert(context.Background(), models.Match{
		ModuleID: "canasta", Status: "active", HostID: human,
		Players: players, TurnOrder: []string{human, bot},
		State: models.JSONDoc(state), Seed: 7, JoinCode: "WEDGED",
	})
	if err != nil {
		t.Fatalf("seeding the match: %v", err)
	}
	matchID := seeded.ID.Hex()
	startVersion := seeded.Version

	// Nothing is driving it: the table sits exactly as it was written.
	time.Sleep(150 * time.Millisecond)
	if v := h.version(t, matchID); v != startVersion {
		t.Fatalf("the match moved on its own before anybody opened it (version %d → %d)",
			startVersion, v)
	}

	// A player opens the table. They cannot act — it is not their turn — so
	// this socket is the only thing that happens.
	whoWatches := human
	if players[0].IsAI {
		whoWatches = bot
	}
	conn := dialMatch(t, h.inviteHarness, matchID, token(t, whoWatches, "Watcher", false))
	defer func() { _ = conn.Close() }()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if h.version(t, matchID) > startVersion {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Error("the bot never moved: opening the table did not restart its loop")
}

// botRestartHarness is the invite harness with two differences it needs: the
// repository kept, so a match can be seeded behind the API, and a bot pace of
// nearly nothing, so the test waits on a decision rather than on a pause that
// exists only to be watchable.
type botRestartHarness struct {
	*inviteHarness
	repo match.Repository
}

func newBotRestartHarness(t *testing.T) *botRestartHarness {
	t.Helper()
	repo := newTestRepository(t)

	hub, err := ws.NewHub(ws.NewConnRegistry(), "")
	if err != nil {
		t.Fatalf("building a hub: %v", err)
	}
	t.Cleanup(func() { _ = hub.Close() })

	mods := module.NewRegistry(zolikmod.New(), prsi.New(), canasta.New())
	manager := match.NewManager(repo, mods, hub)
	manager.SetBotPace(time.Millisecond, 2*time.Millisecond)

	r := chi.NewRouter()
	handlers := match.NewHandlers(manager, false)
	handlers.RegisterRoutes(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	return &botRestartHarness{
		inviteHarness: &inviteHarness{t: t, server: srv, hub: hub},
		repo:          repo,
	}
}

// version is how this test sees a move happen: every accepted action goes
// through UpdateWithVersion, so the number going up is the runtime having
// written something.
func (h *botRestartHarness) version(t *testing.T, matchID string) int64 {
	t.Helper()
	m, err := h.repo.Resolve(context.Background(), matchID)
	if err != nil {
		t.Fatalf("resolving the match: %v", err)
	}
	return m.Version
}
