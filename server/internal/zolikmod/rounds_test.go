package zolikmod_test

import (
	"testing"

	"zolik/server/internal/module"
	"zolik/server/internal/rules"
	"zolik/server/internal/zolikmod"
)

// The pause between deals, for the one game an offer-driver cannot play.
//
// internal/module holds every game that can be driven from its offer list to
// the same rules; Žolíky cannot be, because going out needs a meld shape the
// offer protocol deliberately does not enumerate. So it is driven here by its
// own bot instead, and asked the same questions.

func TestARummyMatchStopsBetweenDeals(t *testing.T) {
	mod := zolikmod.New()
	players := []module.PlayerRef{
		{ID: "p1", Name: "p1", IsAI: true},
		{ID: "p2", Name: "p2", IsAI: true},
	}
	cfg := module.MatchConfig{Options: module.Options{
		module.OptPauseBetweenRounds: module.OptOn,
	}}

	state, err := mod.NewMatch(cfg, players, 1)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}
	bot := module.BotFor(mod)

	pauses, continues, deals := 0, 0, 0
	for step := 0; step < 20000; step++ {
		if done, _, err := mod.Finished(state); err != nil {
			t.Fatalf("Finished: %v", err)
		} else if done {
			break
		}

		log := module.RoundsFor(mod, state)
		if log == nil {
			t.Fatal("Žolíky reported no round log")
		}
		if log.Paused {
			pauses++
			if len(log.WaitingFor) == 0 {
				t.Fatalf("step %d: paused and waiting on nobody", step)
			}
			// A paused table offers exactly one thing, to exactly the seats it
			// is waiting on.
			for _, p := range players {
				offers, err := mod.LegalActions(state, p.ID)
				if err != nil {
					t.Fatalf("LegalActions: %v", err)
				}
				for _, o := range offers {
					if o.Enabled && o.Verb != module.VerbContinue {
						t.Errorf("step %d: a paused table offers %q", step, o.Verb)
					}
				}
			}
			deals = len(log.Rounds)
		}

		awaited := module.AwaitedSeats(mod, state, players[0].ID, players)
		if len(awaited) == 0 {
			t.Fatalf("step %d: a live match is waiting on nobody", step)
		}
		actor := awaited[0]

		offers, err := mod.LegalActions(state, actor)
		if err != nil {
			t.Fatalf("LegalActions: %v", err)
		}
		a, ok := bot.Act(state, module.BotSeat{PlayerID: actor}, offers)
		if !ok {
			// The module's own bot declines during an intermission — it checks
			// whose turn it is, and between deals nobody's it is. The offer
			// list carries it, which is the point of the pause being an offer.
			if a, ok = module.ChooseAction(offers, nil); !ok {
				t.Fatalf("step %d: %s has nothing it may do", step, actor)
			}
		}
		if a.Verb == module.VerbContinue {
			continues++
		}
		next, _, err := mod.Apply(state, actor, a)
		if err != nil {
			fallback, ok := module.ChooseAction(offers, nil)
			if !ok {
				t.Fatalf("step %d: refused (%v) with no fallback", step, err)
			}
			if next, _, err = mod.Apply(state, actor, fallback); err != nil {
				t.Fatalf("step %d: fallback refused too: %v", step, err)
			}
		}
		state = next
	}

	if pauses == 0 {
		t.Fatal("a rummy match asked to pause between deals never stopped")
	}
	if continues == 0 {
		t.Error("the table paused and nobody ever agreed to go on")
	}
	if deals == 0 {
		t.Error("the table paused with no deal behind it to look at")
	}
	t.Logf("paused across %d deals, %d agreements to go on", deals, continues)
}

// TestARummyMatchStillPlaysStraightThrough — the pause is a table setting, and
// turning it off has to leave the game exactly as it was.
func TestARummyMatchStillPlaysStraightThrough(t *testing.T) {
	mod := zolikmod.New()
	players := []module.PlayerRef{
		{ID: "p1", Name: "p1", IsAI: true},
		{ID: "p2", Name: "p2", IsAI: true},
	}
	cfg := module.MatchConfig{Options: module.Options{
		module.OptPauseBetweenRounds: module.OptOff,
	}}

	state, err := mod.NewMatch(cfg, players, 1)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}
	bot := module.BotFor(mod)

	for step := 0; step < 20000; step++ {
		if done, _, err := mod.Finished(state); err != nil || done {
			break
		}
		if log := module.RoundsFor(mod, state); log != nil && log.Paused {
			t.Fatalf("step %d: the table stopped although the pause is off", step)
		}
		actor := module.ActiveSeat(mod, state, players[0].ID, players)
		if actor == "" {
			break
		}
		offers, _ := mod.LegalActions(state, actor)
		a, ok := bot.Act(state, module.BotSeat{PlayerID: actor}, offers)
		if !ok {
			if a, ok = module.ChooseAction(offers, nil); !ok {
				break
			}
		}
		next, _, err := mod.Apply(state, actor, a)
		if err != nil {
			fb, ok := module.ChooseAction(offers, nil)
			if !ok {
				break
			}
			if next, _, err = mod.Apply(state, actor, fb); err != nil {
				break
			}
		}
		state = next
	}
}

// The ledger behind every deal: each row that scored carries an account of
// the cards left in hand, the account adds up to the row, and the seat that
// went out says so instead.
func TestEveryDealIsAccountedFor(t *testing.T) {
	mod := zolikmod.New()
	players := []module.PlayerRef{
		{ID: "p1", Name: "p1", IsAI: true},
		{ID: "p2", Name: "p2", IsAI: true},
		{ID: "p3", Name: "p3", IsAI: true},
	}
	state, err := mod.NewMatch(module.MatchConfig{Options: module.Options{
		module.OptPauseBetweenRounds: module.OptOff,
	}}, players, 7)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}
	state = playOut(t, mod, state, players)

	log := module.RoundsFor(mod, state)
	if log == nil || len(log.Rounds) == 0 {
		t.Fatal("a finished match has no deals to account for")
	}
	accounted := 0
	for _, r := range log.Rounds {
		for _, sc := range r.Scores {
			if err := module.CheckLines(sc); err != nil {
				t.Errorf("deal %d: %v", r.Number, err)
			}
			wentOut := len(r.Winners) == 1 && r.Winners[0] == sc.PlayerID
			switch {
			case wentOut:
				if len(sc.Lines) != 1 || sc.Lines[0].LabelKey != "zolik.line.wentOut" {
					t.Errorf("deal %d: %s went out and the account says %+v", r.Number, sc.PlayerID, sc.Lines)
				}
			case *sc.Shown != 0:
				if len(sc.Lines) != 1 || sc.Lines[0].LabelKey != "zolik.line.inHand" {
					t.Errorf("deal %d: %s scored %d with no hand behind it: %+v", r.Number, sc.PlayerID, *sc.Shown, sc.Lines)
				}
				accounted++
			}
		}
	}
	if accounted == 0 {
		t.Fatal("no seat was ever left holding a card: the account was never exercised")
	}
	t.Logf("%d deals, %d hands accounted for", len(log.Rounds), accounted)
}

// The account for one hand, line by line, and nothing at all for a deal
// scored before the ledger was kept.
func TestTheHandLedgerLines(t *testing.T) {
	gs := rules.GameState{
		Status:      rules.StatusActive,
		GameNumber:  3,
		Phase:       rules.PhaseDraw,
		Rules:       rules.ProfileContinental,
		TurnOrder:   []string{"p1", "p2"},
		Hands:       map[string][]string{"p1": {}, "p2": {}},
		GameScores:  map[string][]int{"p1": {0, 97}, "p2": {15, 0}},
		TotalScores: map[string]int{"p1": 97, "p2": 15},
		DealWinners: []string{"p1", "p2"},
		DealHands: []map[string]rules.HandTally{
			nil,
			{"p1": {
				Cards: 6, Points: 97,
				Jokers: 1, JokerPoints: 50,
				Aces: 1, AcePoints: 25,
				AcesLow: 1, AceLowPoints: 1,
				Faces: 1, FacePoints: 10,
				Pips: 2, PipPoints: 11,
			}},
		},
	}
	raw, err := zolikmod.StateFromRules(gs, nil)
	if err != nil {
		t.Fatalf("StateFromRules: %v", err)
	}
	log, err := zolikmod.New().Rounds(raw)
	if err != nil {
		t.Fatalf("Rounds: %v", err)
	}
	if len(log.Rounds) != 2 {
		t.Fatalf("rounds = %d, want 2", len(log.Rounds))
	}

	// Deal one predates the ledger: the loser's row is its total alone.
	if lines := log.Rounds[0].Scores[1].Lines; lines != nil {
		t.Errorf("a deal with no hands recorded grew an account: %+v", lines)
	}

	p1 := log.Rounds[1].Scores[0]
	if err := module.CheckLines(p1); err != nil {
		t.Fatal(err)
	}
	if len(p1.Lines) != 1 {
		t.Fatalf("lines = %+v", p1.Lines)
	}
	got := map[string]int{}
	for _, s := range p1.Lines[0].Sub {
		got[s.LabelKey] = s.Points
	}
	want := map[string]int{
		"zolik.line.handJokers":  50,
		"zolik.line.handAces":    25,
		"zolik.line.handAcesLow": 1,
		"zolik.line.handFaces":   10,
		"zolik.line.handPips":    11,
	}
	if len(got) != len(want) {
		t.Fatalf("sub-lines = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %d, want %d", k, got[k], v)
		}
	}
	if p2 := log.Rounds[1].Scores[1]; len(p2.Lines) != 1 || p2.Lines[0].LabelKey != "zolik.line.wentOut" {
		t.Errorf("the seat that went out reads %+v", p2.Lines)
	}
}

func playOut(t *testing.T, mod *zolikmod.Module, state module.State, players []module.PlayerRef) module.State {
	t.Helper()
	bot := module.BotFor(mod)
	for step := 0; step < 50000; step++ {
		if done, _, err := mod.Finished(state); err != nil {
			t.Fatalf("Finished: %v", err)
		} else if done {
			return state
		}
		actor := module.ActiveSeat(mod, state, players[0].ID, players)
		if actor == "" {
			t.Fatalf("step %d: a live match is waiting on nobody", step)
		}
		offers, err := mod.LegalActions(state, actor)
		if err != nil {
			t.Fatalf("LegalActions: %v", err)
		}
		a, ok := bot.Act(state, module.BotSeat{PlayerID: actor}, offers)
		if !ok {
			if a, ok = module.ChooseAction(offers, nil); !ok {
				t.Fatalf("step %d: %s has nothing it may do", step, actor)
			}
		}
		next, _, err := mod.Apply(state, actor, a)
		if err != nil {
			fb, ok := module.ChooseAction(offers, nil)
			if !ok {
				t.Fatalf("step %d: refused (%v) with no fallback", step, err)
			}
			if next, _, err = mod.Apply(state, actor, fb); err != nil {
				t.Fatalf("step %d: fallback refused too: %v", step, err)
			}
		}
		state = next
	}
	t.Fatal("the match never finished")
	return nil
}
