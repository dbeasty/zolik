package lastcard

import (
	"testing"

	"zolik/server/internal/module"
)

// narrate applies a move and returns the strip's lines for it, in order.
func narrate(t *testing.T, raw module.State, pid string, a module.Action) (module.State, []module.Fact) {
	t.Helper()
	m := New()
	out, events, err := m.Apply(raw, pid, a)
	if err != nil {
		t.Fatalf("%s %v: %v", pid, a, err)
	}
	var lines []module.Fact
	for _, ev := range events {
		if mv, ok := module.NarrateEvent(m, out, ev); ok {
			lines = append(lines, mv.Fact)
		}
	}
	return out, lines
}

// A play and what it did are one line: the draw and the lost turn a Draw
// Two causes are not narrated again as moves of their own.
func TestAPlayIsOneLineThatSaysWhatItDid(t *testing.T) {
	for _, c := range []struct {
		name string
		hand []string
		play module.Action
		key  string
	}{
		{"skip", []string{"T-S", "A-7"}, playCard("T-S"), "lastcard.move.playedSkip"},
		{"reverse", []string{"T-R", "A-7"}, playCard("T-R"), "lastcard.move.playedReverseAnticlockwise"},
		{"draw two", []string{"T-D", "A-7"}, playCard("T-D"), "lastcard.move.playedDraw"},
		{"wild", []string{cardWild, "A-7"}, playWild(cardWild, "V"), "lastcard.move.playedWild"},
		{"number", []string{"T-9", "A-7"}, playCard("T-9"), "lastcard.move.played"},
	} {
		raw := withState(t, func(s *GameState) { s.Hands["p1"] = c.hand })
		_, lines := narrate(t, raw, "p1", c.play)
		if len(lines) != 1 || lines[0].LabelKey != c.key {
			t.Errorf("%s: lines %+v, want one %s", c.name, lines, c.key)
		}
	}
}

func TestTheDrawTwoLineNamesWhoDrewAndHowMany(t *testing.T) {
	raw := withState(t, func(s *GameState) { s.Hands["p1"] = []string{"T-D", "A-7"} })
	_, lines := narrate(t, raw, "p1", playCard("T-D"))
	if p := lines[0].Params; p["target"] != "p2" || p["n"] != 2 {
		t.Errorf("params %+v, want target p2 and n 2", p)
	}
}

func TestTheWildOnThePileCarriesItsNamedColour(t *testing.T) {
	raw := withState(t, func(s *GameState) { s.Hands["p1"] = []string{cardWild, "A-7"} })
	raw = apply(t, raw, "p1", playWild(cardWild, "V"))
	vm, err := New().View(raw, "p2")
	if err != nil {
		t.Fatal(err)
	}
	for _, z := range vm.Zones {
		if z.ID != discardZoneID {
			continue
		}
		top := z.Cards[len(z.Cards)-1]
		if top.Card != cardWild || top.As != "V" {
			t.Errorf("top of the pile %+v, want the wild as V", top)
		}
		return
	}
	t.Fatal("no discard zone")
}

// Every event a whole match produces either narrates or is a known silent
// one — a new event the strip forgets is a move nobody hears about.
func TestEveryEventOfAMatchIsNarratedOrKnownSilent(t *testing.T) {
	silent := map[string]bool{"last_card": true, "game_ended": true, "deal_started": true}
	m := New()
	ps := players("p1", "p2", "p3")
	cfg := module.MatchConfig{Options: module.Options{
		OptStacking: stackAny, OptSevenZero: module.OptOn, OptTargetScore: 200,
	}}
	state, _ := m.NewMatch(cfg, ps, 3)
	skills := map[string]module.Skill{"p1": module.SkillEasy, "p2": module.SkillHard, "p3": module.SkillMedium}
	seen := map[string]bool{}
	for step := 0; step < 20000; step++ {
		if done, _, _ := m.Finished(state); done {
			break
		}
		s, _ := decode(state)
		actor := s.Current
		if s.Intermission.Open {
			actor = s.Intermission.Waiting(s.TurnOrder)[0]
		}
		offers, _ := m.LegalActions(state, actor)
		a, _ := m.Bot().Act(state, module.BotSeat{PlayerID: actor, Skill: skills[actor]}, offers)
		next, events, err := m.Apply(state, actor, a)
		if err != nil {
			t.Fatal(err)
		}
		for _, ev := range events {
			if effect, _ := ev.Data["effect"].(bool); effect || silent[ev.Type] {
				continue
			}
			if _, ok := module.NarrateEvent(m, next, ev); !ok {
				t.Fatalf("event %q is neither narrated nor known silent", ev.Type)
			}
			seen[ev.Type] = true
		}
		state = next
	}
	t.Logf("narrated: %v", seen)
}
