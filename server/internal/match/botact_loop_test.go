package match_test

import (
	"context"
	"testing"
	"time"

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
