package match_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"zolik/server/internal/botgov"
	"zolik/server/internal/canasta"
	"zolik/server/internal/capacity"
	"zolik/server/internal/match"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/ws"
)

// actLog records which engine made each decision, in order.
type actLog struct {
	mu   sync.Mutex
	acts []loggedAct
}

type loggedAct struct {
	seat   string
	engine string
}

func (l *actLog) add(seat, engine string) {
	l.mu.Lock()
	l.acts = append(l.acts, loggedAct{seat, engine})
	l.mu.Unlock()
}

func (l *actLog) snapshot() []loggedAct {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]loggedAct(nil), l.acts...)
}

// taggedBot plays like the module's own bot and notes that it did.
type taggedBot struct {
	inner  module.Bot
	engine string
	log    *actLog
}

func (b taggedBot) Act(s module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	b.log.add(seat.PlayerID, b.engine)
	return b.inner.Act(s, seat, offers)
}

// layered is what learn.HardModel hands out when a game's model is switched
// on: one bot for Hard seats over the heuristic for the rest.
type layered struct{ net, rule module.Bot }

func (l layered) Heuristic() module.Bot { return l.rule }

func (l layered) Act(s module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	if seat.Skill == module.SkillHard {
		return l.net.Act(s, seat, offers)
	}
	return l.rule.Act(s, seat, offers)
}

// modelCanasta is Canasta with its Hard seats "on a model": the model is the
// real heuristic in disguise, so the game plays normally and the log can say
// which engine the runtime chose.
type modelCanasta struct {
	*canasta.Module
	bot module.Bot
}

func (m modelCanasta) Bot() module.Bot { return m.bot }

// With the governor enforcing, a level drop moves model seats onto the rule
// bot — and never in the middle of a turn: every run of consecutive decisions
// by one seat is played by one engine.
func TestTheGovernorDowngradesModelSeatsAtTurnBoundaries(t *testing.T) {
	repo := newTestRepository(t)
	hub, err := ws.NewHub(ws.NewConnRegistry(), "")
	if err != nil {
		t.Fatalf("building a hub: %v", err)
	}
	t.Cleanup(func() { _ = hub.Close() })

	log := &actLog{}
	base := canasta.New()
	heur := module.BotFor(base)
	mod := modelCanasta{Module: base, bot: layered{
		net:  taggedBot{heur, botgov.EngineNet, log},
		rule: taggedBot{heur, botgov.EngineRule, log},
	}}

	gov := botgov.New(botgov.Config{
		Mode:  botgov.Enforce,
		Cores: 16,
		// Canasta's model made dear enough to need a lease, so the
		// governor has something to take away.
		Costs: map[botgov.Class]time.Duration{{Module: "canasta", Skill: "hard", Engine: botgov.EngineNet}: 50 * time.Millisecond},
	})
	manager := match.NewManager(repo, module.NewRegistry(mod), hub)
	manager.SetBotPace(time.Millisecond, 2*time.Millisecond)
	manager.SetGovernor(gov)

	refs := []module.PlayerRef{{ID: "bot:A", Name: "A", IsAI: true}, {ID: "bot:B", Name: "B", IsAI: true}}
	state, err := mod.NewMatch(module.MatchConfig{Options: module.Options{"targetScore": 5000}}, refs, 11)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}
	seeded, err := repo.Insert(context.Background(), models.Match{
		ModuleID: "canasta", Status: "active", HostID: "bot:A",
		Players: []models.Player{
			{ID: "bot:A", Name: "A", IsAI: true, AIDifficulty: "hard"},
			{ID: "bot:B", Name: "B", IsAI: true, AIDifficulty: "hard"},
		},
		TurnOrder: []string{"bot:A", "bot:B"},
		Seed:      11, JoinCode: "GOVTST", Snapshots: []int{0},
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

	waitFor := func(n int) {
		deadline := time.Now().Add(20 * time.Second)
		for len(log.snapshot()) < n {
			if time.Now().After(deadline) {
				t.Fatalf("only %d decisions in 20s, want %d", len(log.snapshot()), n)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	waitFor(30)
	gov.SetLevel(capacity.Red)
	before := len(log.snapshot())
	waitFor(before + 60)
	cancel()

	acts := log.snapshot()
	sawNetBefore, sawRuleAfter := false, false
	for i, a := range acts {
		if i < before && a.engine == botgov.EngineNet {
			sawNetBefore = true
		}
		if i >= before+10 && a.engine == botgov.EngineRule {
			sawRuleAfter = true
		}
		if i >= before+10 && a.engine == botgov.EngineNet {
			// Allowed only while a turn begun before the drop finishes.
			if acts[i-1].seat != a.seat || acts[i-1].engine != botgov.EngineNet {
				t.Fatalf("decision %d: a model turn started after red", i)
			}
		}
		if i > 0 && acts[i-1].seat == a.seat && acts[i-1].engine != a.engine {
			// One seat's consecutive decisions are one turn: a change of
			// engine inside it is the bug this test exists for.
			t.Fatalf("decision %d: %s switched from %s to %s in the middle of a turn",
				i, a.seat, acts[i-1].engine, a.engine)
		}
	}
	if !sawNetBefore {
		t.Fatal("the model never played at green: the lease was not granted")
	}
	if !sawRuleAfter {
		t.Fatal("the seats never moved to the rule bot at red")
	}
	if !gov.Reduced(seeded.ID.Hex(), "bot:A") && !gov.Reduced(seeded.ID.Hex(), "bot:B") {
		t.Fatal("the governor reports no reduced seat at red")
	}
}
