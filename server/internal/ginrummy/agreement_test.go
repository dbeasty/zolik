package ginrummy

import (
	"testing"

	"zolik/server/internal/module"
)

// collectStates plays a handful of seeded matches, varying the preference
// order by step so the corpus visits upcard decisions, ordinary turns, knocks
// and lay-offs rather than always the same shape of position.
func collectStates(t *testing.T) []module.State {
	t.Helper()
	m := New()
	orders := [][]string{
		{"knock", "lay_off", "finish_layoff", "draw", "discard", "pass"},
		{"pass", "draw", "discard", "knock", "lay_off", "finish_layoff"},
		{"draw", "knock", "discard", "lay_off", "finish_layoff", "pass"},
	}

	var states []module.State
	for seed := int64(1); seed <= 8; seed++ {
		cfg := module.MatchConfig{Options: module.Options{OptTargetScore: 100}}
		state, err := m.NewMatch(cfg, players(), seed)
		if err != nil {
			t.Fatalf("seed %d: NewMatch: %v", seed, err)
		}
		for step := 0; step < 400; step++ {
			done, _, err := m.Finished(state)
			if err != nil {
				t.Fatalf("seed %d step %d: Finished: %v", seed, step, err)
			}
			if done {
				break
			}
			states = append(states, state)

			actor, offers, ok := activeOffers(t, m, state)
			if !ok {
				break
			}
			a, ok := module.ChooseAction(offers, orders[step%len(orders)])
			if !ok {
				break
			}
			next, _, err := m.Apply(state, actor, a)
			if err != nil {
				t.Fatalf("seed %d step %d: offered %+v but refused: %v", seed, step, a, err)
			}
			state = next
		}
	}
	return states
}

func activeOffers(t *testing.T, m *Module, state module.State) (string, []module.ActionOffer, bool) {
	t.Helper()
	for _, p := range players() {
		offers, err := m.LegalActions(state, p.ID)
		if err != nil {
			t.Fatalf("LegalActions(%s): %v", p.ID, err)
		}
		for _, o := range offers {
			if o.Enabled {
				return p.ID, offers, true
			}
		}
	}
	return "", nil, false
}

// TestOffersAgreeWithApply is the whole discipline in one test: every enabled
// offer, submitted exactly as declared, must be accepted, and every disabled
// offer must carry a reason.
func TestOffersAgreeWithApply(t *testing.T) {
	m := New()
	for _, state := range collectStates(t) {
		for _, p := range players() {
			offers, err := m.LegalActions(state, p.ID)
			if err != nil {
				t.Fatalf("LegalActions: %v", err)
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
					continue // composite; none of ours are
				}
				if _, _, err := m.Apply(state, p.ID, a); err != nil {
					t.Errorf("offer %q for %s said enabled but Apply refused: %v", o.ID, p.ID, err)
				}
			}
		}
	}
}

// TestPerCardOffersAgreeWithApply is the strongest form of the check: for
// every card in a player's hand, being named as eligible on a knock or
// lay-off offer must equal Apply actually accepting that specific card.
func TestPerCardOffersAgreeWithApply(t *testing.T) {
	m := New()
	for _, state := range collectStates(t) {
		s, err := decode(state)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		for _, p := range players() {
			offers, err := m.LegalActions(state, p.ID)
			if err != nil {
				t.Fatalf("LegalActions: %v", err)
			}
			for _, o := range offers {
				if o.Source == nil || o.Source.Zone != module.FromHand {
					continue
				}
				for _, card := range s.Hands[p.ID] {
					named := hasCard(o.Source.Cards, card)
					var a module.Action
					switch {
					case o.ID == OfferDiscard:
						a = module.Action{OfferID: o.ID, Verb: o.Verb, Cards: []string{card}}
					case len(o.ID) > 6 && o.ID[:6] == "knock:":
						continue // one offer per specific card already; nothing to cross-check
					case len(o.ID) > 4 && o.ID[:4] == "gin:":
						continue
					case len(o.ID) > 8 && o.ID[:8] == "lay_off:":
						a = module.Action{OfferID: o.ID, Verb: o.Verb, Target: o.Target.MeldID, Cards: []string{card}}
					default:
						continue
					}
					_, _, err := m.Apply(state, p.ID, a)
					accepted := err == nil
					if named != accepted && o.Enabled {
						t.Errorf("offer %q: card %s named=%v but Apply accepted=%v", o.ID, card, named, accepted)
					}
				}
			}
		}
	}
}

// TestOffersNeverNameACardYouDoNotHold is the hidden-information half of the
// same discipline: a hand-sourced offer must only ever list cards the player
// actually holds.
func TestOffersNeverNameACardYouDoNotHold(t *testing.T) {
	m := New()
	for _, state := range collectStates(t) {
		s, err := decode(state)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		for _, p := range players() {
			offers, err := m.LegalActions(state, p.ID)
			if err != nil {
				t.Fatalf("LegalActions: %v", err)
			}
			for _, o := range offers {
				if o.Source == nil || o.Source.Zone != module.FromHand {
					continue
				}
				if o.Source.OwnerID != p.ID {
					t.Errorf("offer %q sources a hand not owned by the viewer (%s)", o.ID, o.Source.OwnerID)
				}
				for _, card := range o.Source.Cards {
					if !hasCard(s.Hands[p.ID], card) {
						t.Errorf("offer %q for %s names a card %s not in their hand", o.ID, p.ID, card)
					}
				}
			}
		}
	}
}

// TestOnlyOnePlayerIsEverOffered is the turn-model invariant every module
// shares: exactly one seat has something enabled at a time (or none, only
// once the match is over). An open intermission is the one exception —
// module.Intermission deliberately awaits every seat at once — so those
// states are skipped here rather than mistaken for a broken turn order.
func TestOnlyOnePlayerIsEverOffered(t *testing.T) {
	m := New()
	for _, state := range collectStates(t) {
		if s, err := decode(state); err == nil && s.Intermission.Open {
			continue
		}
		var active []string
		for _, p := range players() {
			offers, err := m.LegalActions(state, p.ID)
			if err != nil {
				t.Fatalf("LegalActions: %v", err)
			}
			for _, o := range offers {
				if o.Enabled {
					active = append(active, p.ID)
					break
				}
			}
		}
		if len(active) > 1 {
			t.Errorf("more than one player has enabled offers at once: %v", active)
		}
	}
}
