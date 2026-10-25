package sietemedio

import (
	"strings"
	"testing"

	"zolik/server/internal/module"
)

// collectStates plays several seeded matches with different tastes, so the
// corpus visits staking, hands part-way through, hands gone over, the bank's
// own turn and settled rounds rather than the same shape of position over and
// over.
func collectStates(t *testing.T) []module.State {
	t.Helper()
	m := New()
	orders := [][]string{
		{"stake", "hit", "stand"},
		{"stake", "stand", "hit"},
	}

	var states []module.State
	for seed := int64(1); seed <= 8; seed++ {
		cfg := module.MatchConfig{
			Variation: VarMexicana,
			Options:   module.Options{OptRounds: 8, OptStartingStack: 200, OptMinStake: 25},
		}
		state, err := m.NewMatch(cfg, table(), seed)
		if err != nil {
			t.Fatalf("seed %d: NewMatch: %v", seed, err)
		}
		for step := 0; step < 600; step++ {
			done, _, err := m.Finished(state)
			if err != nil {
				t.Fatalf("Finished: %v", err)
			}
			if done {
				break
			}
			states = append(states, state)

			awaited := module.AwaitedSeats(m, state, "p1", table())
			if len(awaited) == 0 {
				break
			}
			offers, err := m.LegalActions(state, awaited[0])
			if err != nil {
				t.Fatalf("LegalActions: %v", err)
			}
			a, ok := module.ChooseAction(offers, orders[step%len(orders)])
			if !ok {
				break
			}
			next, _, err := m.Apply(state, awaited[0], a)
			if err != nil {
				t.Fatalf("seed %d step %d: offered %+v but refused: %v", seed, step, a, err)
			}
			state = next
		}
	}
	return states
}

// TestOffersAgreeWithApply is the whole discipline in one test: every enabled
// offer, submitted exactly as declared, must be accepted, and every disabled
// one must say why not.
func TestOffersAgreeWithApply(t *testing.T) {
	m := New()
	checked := 0
	for _, state := range collectStates(t) {
		for _, p := range table() {
			offers, err := m.LegalActions(state, p.ID)
			if err != nil {
				t.Fatalf("LegalActions: %v", err)
			}
			if len(offers) == 0 {
				t.Fatalf("%s was given no offers at all", p.ID)
			}
			for _, o := range offers {
				if !o.Enabled {
					if o.WhyNot == "" {
						t.Errorf("disabled offer %q for %s carries no reason", o.ID, p.ID)
					}
					continue
				}
				if o.Manual {
					continue
				}
				a, ok := module.SubmissionFor(o)
				if !ok {
					t.Errorf("offer %q is on but describes no submission: %+v", o.ID, o)
					continue
				}
				if _, _, err := m.Apply(state, p.ID, a); err != nil {
					t.Errorf("offer %q for %s said enabled but Apply refused: %v", o.ID, p.ID, err)
				}
				checked++
			}
		}
	}
	if checked < 100 {
		t.Errorf("only %d enabled offers were checked; the test proved little", checked)
	}
}

// TestTheWholeStakeRangeIsAccepted — the stake is the one offer in this game
// that carries a range, and its top is another seat's stack. An offer whose
// declared maximum the engine then refuses is the offer list lying about
// somebody else's chips.
func TestTheWholeStakeRangeIsAccepted(t *testing.T) {
	m := New()
	checked := 0
	for _, state := range collectStates(t) {
		for _, p := range table() {
			offers, err := m.LegalActions(state, p.ID)
			if err != nil {
				t.Fatalf("LegalActions: %v", err)
			}
			o := module.FindOffer(offers, OfferStake)
			if o == nil || !o.Enabled || len(o.Params) == 0 {
				continue
			}
			spec := o.Params[0]
			if spec.Max < spec.Min {
				t.Fatalf("%s offered a stake range of %d..%d", p.ID, spec.Min, spec.Max)
			}
			for _, amount := range []int{spec.Min, spec.Max, (spec.Min + spec.Max) / 2} {
				a := module.Action{OfferID: OfferStake, Verb: VerbStake,
					Params: map[string]string{ParamAmount: itoa(amount)}}
				if _, _, err := m.Apply(state, p.ID, a); err != nil {
					t.Errorf("%s was offered %d..%d but %d was refused: %v",
						p.ID, spec.Min, spec.Max, amount, err)
				}
			}
			for _, choice := range spec.Choices {
				a := module.Action{OfferID: OfferStake, Verb: VerbStake,
					Params: map[string]string{ParamAmount: choice.Value}}
				if _, _, err := m.Apply(state, p.ID, a); err != nil {
					t.Errorf("%s: quick stake %q was refused: %v", p.ID, choice.Value, err)
				}
			}
			checked++
		}
	}
	if checked < 20 {
		t.Errorf("only %d stake ranges were checked; the test proved little", checked)
	}
}

// TestTheBoardAgreesWithTheOffers — Seat.Active is what the runtime reads to
// decide whose turn it is, so it has to be the same answer LegalActions gives.
// The staking phase waits on several seats at once, and drops the ones the bank
// cannot cover, which is exactly where the two could disagree unnoticed.
func TestTheBoardAgreesWithTheOffers(t *testing.T) {
	m := New()
	for _, state := range collectStates(t) {
		vm, err := m.View(state, "p1")
		if err != nil {
			t.Fatalf("View: %v", err)
		}
		active := map[string]bool{}
		for _, seat := range vm.Seats {
			if seat.Active {
				active[seat.PlayerID] = true
			}
		}
		for _, p := range table() {
			offers, err := m.LegalActions(state, p.ID)
			if err != nil {
				t.Fatalf("LegalActions: %v", err)
			}
			enabled := false
			for _, o := range offers {
				if o.Live() {
					enabled = true
					break
				}
			}
			if enabled != active[p.ID] {
				t.Fatalf("%s: the board says active=%v, the offers say %v", p.ID, active[p.ID], enabled)
			}
		}
	}
}

// TestOffersNeverNameACard — the negative space of this module's offer list.
//
// Everything here is dealt to you: there is no card to choose, no pile to take
// from and no meld to aim at, so an offer that named a card would either be
// describing a move this game does not have, or leaking one of the cards that
// are face down.
func TestOffersNeverNameACard(t *testing.T) {
	m := New()
	for _, state := range collectStates(t) {
		s := stateOf(t, state)
		for _, p := range table() {
			// Every card this player is not entitled to see.
			var secret []string
			for i := range s.Seats {
				st := &s.Seats[i]
				shown := visible(s, st, p.ID, false)
				secret = append(secret, st.Cards[:len(st.Cards)-len(shown)]...)
			}

			offers, err := m.LegalActions(state, p.ID)
			if err != nil {
				t.Fatalf("LegalActions: %v", err)
			}
			for _, o := range offers {
				for _, sel := range []*module.Selector{o.Source, o.Target} {
					if sel != nil && (len(sel.Cards) > 0 || len(sel.Placements) > 0) {
						t.Errorf("offer %q names cards: %+v", o.ID, sel)
					}
				}
				for _, f := range o.Facts {
					for _, card := range secret {
						if f.Value != "" && strings.Contains(f.Value, card) {
							t.Errorf("offer %q leaks %s in %+v", o.ID, card, f)
						}
					}
				}
			}
		}
	}
}

// TestEveryRefusalPointsAtARuleThatExists — a refusal's RuleIDs are pointers
// into this table's own written rules. A pointer that resolves to nothing is a
// "why not?" that opens an empty page.
func TestEveryRefusalPointsAtARuleThatExists(t *testing.T) {
	m := New()
	cfg := module.MatchConfig{Variation: VarMexicana, Options: module.Options{OptRounds: 8}}
	sections, err := m.Rules(cfg)
	if err != nil {
		t.Fatalf("Rules: %v", err)
	}
	stated := module.RuleIDsIn(sections)

	for _, state := range collectStates(t) {
		for _, p := range table() {
			offers, err := m.LegalActions(state, p.ID)
			if err != nil {
				t.Fatalf("LegalActions: %v", err)
			}
			for _, o := range offers {
				for _, id := range o.RuleIDs {
					if !stated[id] {
						t.Errorf("offer %q points at rule %q, which this table does not state", o.ID, id)
					}
				}
			}
		}
	}
}
