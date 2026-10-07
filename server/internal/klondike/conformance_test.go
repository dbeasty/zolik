package klondike

import (
	"testing"

	"zolik/server/internal/module"
)

// TestEveryGameEnds plays whole games from the offers alone, across the
// variations and a spread of deals: every one finishes, all 52 cards are
// accounted for at every step, and some of them are even won.
func TestEveryGameEnds(t *testing.T) {
	m := New()
	won := 0
	games := 0
	for _, variation := range []string{"classic", "draw3", "vegas"} {
		for seed := int64(1); seed <= 12; seed++ {
			raw, _ := m.NewMatch(module.MatchConfig{Variation: variation}, solo, seed)
			final, res, err := module.PlayWithOffers(m, raw, solo, module.DriverOptions{
				MaxActions: 20000, Prefer: []string{"autofinish", "move", "draw", "recycle"},
				OnEvents: func(_ string, _ []module.Event, _, after module.State) {
					s, _ := decode(after)
					n := len(s.Stock) + len(s.Waste) + s.foundationCount()
					for _, c := range s.Cols {
						n += len(c.Down) + len(c.Up)
					}
					if n != 52 {
						t.Fatalf("%s seed %d: %d cards on the table", variation, seed, n)
					}
				},
			})
			if err != nil {
				t.Fatalf("%s seed %d: %v", variation, seed, err)
			}
			if !res.Finished {
				t.Fatalf("%s seed %d: still going after %d actions", variation, seed, res.Actions)
			}
			games++
			if len(res.Winners) == 1 {
				won++
			}
			_ = final
		}
	}
	t.Logf("a first-offer driver won %d of %d", won, games)
}

func TestOffersAgreeWithApply(t *testing.T) {
	m := New()
	checked := 0
	for seed := int64(1); seed <= 10; seed++ {
		raw, _ := m.NewMatch(module.MatchConfig{}, solo, seed)
		for step := 0; step < 300; step++ {
			offers, _ := m.LegalActions(raw, "p1")
			for _, o := range offers {
				if !o.Enabled {
					if o.WhyNot == "" {
						t.Fatalf("%s off with no reason", o.ID)
					}
					// The engine gives the same refusal.
					a := module.Action{OfferID: o.ID, Verb: o.Verb}
					if _, _, err := m.Apply(raw, "p1", a); module.CodeOf(err) != o.WhyNot {
						t.Fatalf("%s: offer says %s, engine says %v", o.ID, o.WhyNot, err)
					}
					continue
				}
				a, ok := module.SubmissionFor(o)
				if !ok {
					t.Fatalf("%s: no submission", o.ID)
				}
				if _, _, err := m.Apply(raw, "p1", a); err != nil {
					t.Fatalf("%s enabled but refused: %v", o.ID, err)
				}
				checked++
			}
			a, ok := module.ChooseAction(offers, []string{"move", "draw", "recycle"})
			if !ok {
				break
			}
			raw, _, _ = m.Apply(raw, "p1", a)
			if done, _, _ := m.Finished(raw); done {
				break
			}
		}
	}
	if checked < 100 {
		t.Fatalf("only %d offers checked", checked)
	}
}

// sensible plays progress first, then the stock, and only then rearranges —
// roughly what a person does. It exists to show the rules let games be won.
func sensible(t *testing.T, m *Module, raw []byte) (module.Action, bool) {
	offers, _ := m.LegalActions(raw, "p1")
	var fallback *module.ActionOffer
	for i := range offers {
		o := &offers[i]
		if !o.Enabled {
			continue
		}
		switch o.Verb {
		case VerbAutoFinish:
			return module.Action{OfferID: o.ID, Verb: o.Verb}, true
		case VerbMove:
			a, _ := module.SubmissionFor(*o)
			next, _, err := m.Apply(raw, "p1", a)
			if err != nil {
				t.Fatal(err)
			}
			if s, _ := decode(next); s.Idle == 0 {
				return a, true
			}
			if fallback == nil {
				fallback = o
			}
		}
	}
	for _, verb := range []string{VerbDraw, VerbRecycle} {
		for _, o := range offers {
			if o.Enabled && o.Verb == verb {
				return module.Action{OfferID: o.ID, Verb: o.Verb}, true
			}
		}
	}
	if fallback != nil {
		a, _ := module.SubmissionFor(*fallback)
		return a, true
	}
	return module.Action{Verb: VerbGiveUp}, true
}

func TestASensiblePlayerWinsSome(t *testing.T) {
	m := New()
	won := 0
	const games = 60
	for seed := int64(1); seed <= games; seed++ {
		raw, _ := m.NewMatch(module.MatchConfig{}, solo, seed)
		for step := 0; step < 3000; step++ {
			a, _ := sensible(t, m, raw)
			next, _, err := m.Apply(raw, "p1", a)
			if err != nil {
				t.Fatalf("seed %d: %+v: %v", seed, a, err)
			}
			raw = next
			if done, winners, _ := m.Finished(raw); done {
				if len(winners) == 1 {
					won++
				}
				break
			}
		}
		if done, _, _ := m.Finished(raw); !done {
			t.Fatalf("seed %d did not finish", seed)
		}
	}
	t.Logf("won %d of %d", won, games)
	if won == 0 {
		t.Fatal("no game was ever won")
	}
}
