package blackjack

import (
	"strings"
	"testing"

	"zolik/server/internal/module"
)

// collectStates plays several seeded matches with different tastes, so the
// corpus visits betting, insurance, split hands, doubled hands and settled
// rounds rather than the same shape of position over and over.
func collectStates(t *testing.T) []module.State {
	t.Helper()
	m := New()
	orders := [][]string{
		{"bet", "insure", "split", "stand", "hit"},
		{"bet", "decline_insurance", "double", "hit", "stand"},
		{"bet", "decline_insurance", "surrender", "stand", "hit"},
	}

	var states []module.State
	for seed := int64(1); seed <= 6; seed++ {
		cfg := module.MatchConfig{Variation: "atlantic", Options: module.Options{OptRounds: 8}}
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

// TestTheBoardAgreesWithTheOffers — Seat.Active is what the runtime reads to
// decide whose turn it is, so it has to be the same answer LegalActions gives.
// Two of this game's phases wait on several seats at once, which is exactly
// where the two could disagree without anybody noticing.
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
				if o.Enabled {
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
// Everything in blackjack is dealt to you: there is no card to choose, no
// pile to take from and no meld to aim at, so an offer that named a card
// would either be describing a move this game does not have, or leaking the
// one card that is hidden.
func TestOffersNeverNameACard(t *testing.T) {
	m := New()
	for _, state := range collectStates(t) {
		s := stateOf(t, state)
		hole := ""
		if len(s.Dealer) == 2 && !s.HoleShown {
			hole = s.Dealer[1]
		}
		for _, p := range table() {
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
					if hole != "" && strings.Contains(f.Value, hole) {
						t.Errorf("offer %q leaks the hole card in %+v", o.ID, f)
					}
				}
			}
		}
	}
}

// TestEveryRefusalPointsAtARuleThatExists — a refusal's RuleIDs are pointers
// into this table's own written rules. A pointer that resolves to nothing is
// a "why not?" that opens an empty page.
func TestEveryRefusalPointsAtARuleThatExists(t *testing.T) {
	m := New()
	cfg := module.MatchConfig{Variation: "atlantic", Options: module.Options{OptRounds: 8}}
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
