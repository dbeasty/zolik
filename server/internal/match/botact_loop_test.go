package match_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"zolik/server/internal/botstats"
	"zolik/server/internal/canasta"
	"zolik/server/internal/match"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/ws"
)

// hangingCanasta is canasta with a bot that never decides: the real engine,
// the real offer list, and an Act that blocks until the test ends.
type hangingCanasta struct {
	*canasta.Module
	bot module.Bot
}

func (h hangingCanasta) Bot() module.Bot { return h.bot }

type blockingBot struct{ release <-chan struct{} }

func (b blockingBot) Act(module.State, module.BotSeat, []module.ActionOffer) (module.Action, bool) {
	<-b.release
	return module.Action{}, false
}

// A bot whose decision hangs costs its table the budget, not the match.
//
// Before the budget, botLoop called Act inline: a decision that never came
// back held the loop forever, and because the seat it was waiting on was a
// bot's, nothing a human could do would ever start another one. Now the loop
// stops waiting and plays from the offer list, so the table keeps moving.
func TestAHungBotDecisionDoesNotWedgeTheMatch(t *testing.T) {
	repo := newTestRepository(t)
	hub, err := ws.NewHub(ws.NewConnRegistry(), "")
	if err != nil {
		t.Fatalf("building a hub: %v", err)
	}
	t.Cleanup(func() { _ = hub.Close() })

	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	mod := hangingCanasta{Module: canasta.New(), bot: blockingBot{release: release}}

	manager := match.NewManager(repo, module.NewRegistry(mod), hub)
	manager.SetBotPace(time.Millisecond, 2*time.Millisecond)
	const budget = 100 * time.Millisecond
	manager.SetBotActBudget(budget)

	refs := []module.PlayerRef{{ID: "bot:A", Name: "A", IsAI: true}, {ID: "bot:B", Name: "B", IsAI: true}}
	state, err := mod.NewMatch(module.MatchConfig{Options: module.Options{"targetScore": 500}}, refs, 7)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}
	seeded, err := repo.Insert(context.Background(), models.Match{
		ModuleID: "canasta", Status: "active", HostID: "bot:A",
		Players: []models.Player{
			{ID: "bot:A", Name: "A", IsAI: true},
			{ID: "bot:B", Name: "B", IsAI: true},
		},
		TurnOrder: []string{"bot:A", "bot:B"},
		Seed:      7, JoinCode: "HUNGBT", Snapshots: []int{0},
	})
	if err != nil {
		t.Fatalf("seeding the match: %v", err)
	}
	if err := repo.CommitSnapshot(context.Background(), seeded.ID, seeded.Version, seeded,
		nil, 0, models.JSONDoc(state)); err != nil {
		t.Fatalf("seeding the board: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	manager.RunBotsIfNeeded(ctx, seeded.ID.Hex())

	// Several moves, across both seats, well inside what the hung decisions
	// would have cost unbounded (forever) — one budget per turn plus slack.
	const want = 4
	deadline := time.Now().Add(want*budget + 5*time.Second)
	for time.Now().Before(deadline) {
		moves, err := repo.Moves(context.Background(), seeded.ID, 0, -1)
		if err != nil {
			t.Fatalf("reading the moves: %v", err)
		}
		if len(moves) >= want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Errorf("the match made fewer than %d moves: a hung bot decision wedged the loop", want)
}

// A hint is worked out with the table unlocked. Before, Hint ran the bot under
// the match lock, so a slow adviser held every other seat's move until it
// answered.
func TestASlowHintDoesNotHoldTheTable(t *testing.T) {
	repo := newTestRepository(t)
	hub, err := ws.NewHub(ws.NewConnRegistry(), "")
	if err != nil {
		t.Fatalf("building a hub: %v", err)
	}
	t.Cleanup(func() { _ = hub.Close() })

	release := make(chan struct{})
	var once sync.Once
	free := func() { once.Do(func() { close(release) }) }
	t.Cleanup(free)
	mod := hangingCanasta{Module: canasta.New(), bot: blockingBot{release: release}}
	manager := match.NewManager(repo, module.NewRegistry(mod), hub)
	rec := botstats.New()
	manager.SetBotStats(rec)

	refs := []module.PlayerRef{{ID: "p1", Name: "One"}, {ID: "p2", Name: "Two"}}
	state, err := mod.NewMatch(module.MatchConfig{Options: module.Options{"targetScore": 500}}, refs, 7)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}
	seeded, err := repo.Insert(context.Background(), models.Match{
		ModuleID: "canasta", Status: "active", HostID: "p1",
		Players:   []models.Player{{ID: "p1", Name: "One"}, {ID: "p2", Name: "Two"}},
		TurnOrder: []string{"p1", "p2"},
		Seed:      7, JoinCode: "SLOWHT", Snapshots: []int{0},
	})
	if err != nil {
		t.Fatalf("seeding the match: %v", err)
	}
	if err := repo.CommitSnapshot(context.Background(), seeded.ID, seeded.Version, seeded,
		nil, 0, models.JSONDoc(state)); err != nil {
		t.Fatalf("seeding the board: %v", err)
	}
	id := seeded.ID.Hex()

	// Whoever is on turn asks for a hint, and the adviser never answers.
	actor := module.AwaitedSeats(mod, state, "p1", refs)[0]
	offers, err := mod.LegalActions(state, actor)
	if err != nil {
		t.Fatalf("LegalActions: %v", err)
	}
	move, ok := module.ChooseAction(offers, []string{"draw"})
	if !ok {
		t.Fatalf("nothing to play:\n%s", module.DescribeOffers(offers))
	}
	hinted := make(chan error, 1)
	go func() {
		_, err := manager.Hint(context.Background(), id, actor)
		hinted <- err
	}()
	deadline := time.Now().Add(5 * time.Second)
	for rec.Snapshot().InFlight == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the hint never started thinking")
		}
		time.Sleep(5 * time.Millisecond)
	}

	// The table still takes a move while the hint is thinking.
	played := make(chan error, 1)
	go func() { played <- manager.HandleAction(context.Background(), id, actor, move) }()
	select {
	case err := <-played:
		if err != nil {
			t.Fatalf("the move was refused: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a move waited on a hint: the hint is holding the table")
	}

	free()
	select {
	case <-hinted:
	case <-time.After(2 * time.Second):
		t.Fatal("the hint never returned")
	}
}
