package match_test

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	"zolik/server/internal/db"
	"zolik/server/internal/learn"
	learnmodels "zolik/server/internal/learn/models"
	"zolik/server/internal/match"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/ws"
	"zolik/server/internal/zolikmod"
)

// watchedZolik is the real Žolíky module, whose bot is asked for exactly the
// way the runtime asks — once per move, through module.BotFor — and whose
// every decision is written down next to what the heuristic and the shipped
// model would each have played in the same position.
type watchedZolik struct {
	*zolikmod.Module
	heuristic, net module.Bot
	mu             sync.Mutex
	seen           []decision
}

type decision struct {
	skill                 module.Skill
	switchOn, torn        bool
	played, heur, byModel module.Action
}

func (w *watchedZolik) Bot() module.Bot {
	// The switch is read either side of the call, so a flip that lands in
	// the middle marks the decision as unattributable rather than making the
	// test flaky.
	before := hardSwitchOn("zolik")
	inner := w.Module.Bot()
	return watchedBot{w: w, inner: inner, switchOn: before, torn: before != hardSwitchOn("zolik")}
}

type watchedBot struct {
	w        *watchedZolik
	inner    module.Bot
	switchOn bool
	torn     bool
}

func (b watchedBot) Act(s module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	played, ok := b.inner.Act(s, seat, offers)
	heur, _ := b.w.heuristic.Act(s, seat, offers)
	byModel, _ := b.w.net.Act(s, seat, offers)
	b.w.mu.Lock()
	b.w.seen = append(b.w.seen, decision{skill: seat.Skill, switchOn: b.switchOn, torn: b.torn, played: played, heur: heur, byModel: byModel})
	b.w.mu.Unlock()
	return played, ok
}

func hardSwitchOn(game string) bool {
	for _, s := range learn.HardModels() {
		if s.Game == game {
			return s.Enabled
		}
	}
	return false
}

// A Hard bot seat in a live Žolíky match plays the shipped model while the
// switch is on and the heuristic once it is off — flipped between two moves of
// the same match, through the real bot loop, with no restart. The Medium seat
// at the same table plays the heuristic throughout.
func TestHardSeatFollowsTheModelSwitchMidMatch(t *testing.T) {
	t.Setenv("ZOLIK_LEARNED_MODEL_ZOLIK", "")
	g, err := learn.LookupGame("zolik")
	if err != nil {
		t.Fatal(err)
	}
	b, _, _ := learnmodels.Hard("zolik")
	net := learn.HardBot(g, b, g.Heuristic())
	if _, ok := net.(learn.NetBot); !ok {
		t.Fatal("the shipped Žolíky model does not fit")
	}
	if _, err := learn.SetHardModel("zolik", true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = learn.SetHardModel("zolik", false) })

	k, err := db.OpenKDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = k.Close(context.Background()) })
	repo := match.NewKDBRepository(k)
	hub, err := ws.NewHub(ws.NewConnRegistry(), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = hub.Close() })

	mod := &watchedZolik{Module: zolikmod.New(), heuristic: g.Heuristic(), net: net}
	manager := match.NewManager(repo, module.NewRegistry(mod), hub)
	manager.SetBotPace(time.Millisecond, 2*time.Millisecond)

	refs := []module.PlayerRef{{ID: "bot:H", Name: "H", IsAI: true}, {ID: "bot:M", Name: "M", IsAI: true}}
	state, err := mod.NewMatch(module.MatchConfig{}, refs, 11)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}
	seeded, err := repo.Insert(context.Background(), models.Match{
		ModuleID: "zolik", Status: "active", HostID: "bot:H",
		Players: []models.Player{
			{ID: "bot:H", Name: "H", IsAI: true, AIDifficulty: "hard"},
			{ID: "bot:M", Name: "M", IsAI: true, AIDifficulty: "medium"},
		},
		TurnOrder: []string{"bot:H", "bot:M"},
		Seed:      11, JoinCode: "HARDNN", Snapshots: []int{0},
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

	// Telling decisions: a Hard seat, in a position where the model and the
	// heuristic disagree, under the given switch setting.
	telling := func(on bool) int {
		mod.mu.Lock()
		defer mod.mu.Unlock()
		n := 0
		for _, d := range mod.seen {
			if d.skill == module.SkillHard && !d.torn && d.switchOn == on && !reflect.DeepEqual(d.heur, d.byModel) {
				n++
			}
		}
		return n
	}
	waitFor := func(on bool, want int) {
		t.Helper()
		deadline := time.Now().Add(60 * time.Second)
		for telling(on) < want {
			if time.Now().After(deadline) {
				t.Fatalf("switch %v: only %d telling Hard decisions in time", on, telling(on))
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	const want = 3
	waitFor(true, want)
	if _, err := learn.SetHardModel("zolik", false); err != nil {
		t.Fatal(err)
	}
	waitFor(false, want)
	cancel()

	mod.mu.Lock()
	defer mod.mu.Unlock()
	var hardOn, hardOff, medium int
	for i, d := range mod.seen {
		switch {
		case d.skill != module.SkillHard:
			medium++
			if !reflect.DeepEqual(d.played, d.heur) {
				t.Errorf("decision %d: a %s seat did not play the heuristic", i, d.skill)
			}
		case d.torn, reflect.DeepEqual(d.heur, d.byModel):
			// Both would have played it; says nothing about which did.
		case d.switchOn:
			hardOn++
			if !reflect.DeepEqual(d.played, d.byModel) {
				t.Errorf("decision %d: switch on, Hard played %+v; the model plays %+v", i, d.played, d.byModel)
			}
		default:
			hardOff++
			if !reflect.DeepEqual(d.played, d.heur) {
				t.Errorf("decision %d: switch off, Hard played %+v; the heuristic plays %+v", i, d.played, d.heur)
			}
		}
	}
	if medium == 0 {
		t.Error("the Medium seat never moved")
	}
	t.Logf("%d decisions: %d telling with the model on, %d with it off, %d Medium", len(mod.seen), hardOn, hardOff, medium)
}
