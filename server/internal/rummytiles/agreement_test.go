package rummytiles

import (
	"strconv"
	"testing"

	"zolik/server/internal/module"
)

func players(n int) []module.PlayerRef {
	ids := []string{"p1", "p2", "p3", "p4"}
	out := make([]module.PlayerRef, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, module.PlayerRef{ID: ids[i], Name: ids[i]})
	}
	return out
}

func playerIDs(refs []module.PlayerRef) []string {
	out := make([]string, len(refs))
	for i, r := range refs {
		out[i] = r.ID
	}
	return out
}

// activePlayer finds who has an enabled offer right now, the same way the
// runtime does — asking the board rather than assuming a turn order.
func activePlayer(t *testing.T, m *Module, state module.State, refs []module.PlayerRef) (string, bool) {
	t.Helper()
	for _, p := range refs {
		offers, err := m.LegalActions(state, p.ID)
		if err != nil {
			t.Fatalf("LegalActions(%s): %v", p.ID, err)
		}
		for _, o := range offers {
			if o.Enabled {
				return p.ID, true
			}
		}
	}
	return "", false
}

// representativeSubmission builds the same shape module.SubmissionFor does,
// but without the Composite guard — exactly what testing a composite offer's
// first candidate needs, and the same discipline the bot itself is held to
// for the moves it actually composes.
func representativeSubmission(o module.ActionOffer) (module.Action, bool) {
	a := module.Action{OfferID: o.ID, Verb: o.Verb}
	if o.Source != nil && o.Source.MinCards > 0 {
		if len(o.Source.Cards) < o.Source.MinCards {
			return module.Action{}, false
		}
		a.Cards = append([]string(nil), o.Source.Cards[:o.Source.MinCards]...)
	}
	if o.Target != nil && o.Target.MeldID != "" {
		a.Target = o.Target.MeldID
	}
	for _, p := range o.Params {
		if a.Params == nil {
			a.Params = map[string]string{}
		}
		switch p.Kind {
		case module.ParamKindInt:
			v := p.Default
			if v < p.Min || v > p.Max {
				v = p.Min
			}
			a.Params[p.Name] = strconv.Itoa(v)
		default:
			if len(p.Choices) == 0 {
				return module.Action{}, false
			}
			a.Params[p.Name] = p.Choices[0].Value
		}
	}
	return a, true
}

// collectStates plays several seeded matches with both seats driven by the
// greedy bot, recording every state along the way — a corpus that actually
// visits placed sets, laid-off tiles, split runs and swapped jokers, which a
// naive offer-follower never would (place/add/take are Composite).
func collectStates(t *testing.T, seats int, seeds int64) []module.State {
	t.Helper()
	m := New()
	refs := players(seats)
	b := bot{}

	var states []module.State
	for seed := int64(1); seed <= seeds; seed++ {
		state, err := m.NewMatch(module.MatchConfig{Options: module.Options{OptTargetScore: 150}}, refs, seed)
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
			actor, ok := activePlayer(t, m, state, refs)
			if !ok {
				break
			}
			states = append(states, state)

			offers, err := m.LegalActions(state, actor)
			if err != nil {
				t.Fatalf("seed %d step %d: LegalActions: %v", seed, step, err)
			}
			a, ok := b.Act(state, module.BotSeat{PlayerID: actor}, offers)
			if !ok {
				break
			}
			next, _, err := m.Apply(state, actor, a)
			if err != nil {
				t.Fatalf("seed %d step %d: bot chose %+v but Apply refused: %v", seed, step, a, err)
			}
			state = next
		}
	}
	return states
}

// TestOffersAgreeWithApply is the whole discipline: every enabled offer's
// first representative submission must be accepted, and every disabled offer
// must carry a reason.
func TestOffersAgreeWithApply(t *testing.T) {
	m := New()
	for _, state := range collectStates(t, 2, 6) {
		s, err := decode(state)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		for _, p := range s.Players {
			offers, err := m.LegalActions(state, p)
			if err != nil {
				t.Fatalf("LegalActions: %v", err)
			}
			for _, o := range offers {
				if !o.Enabled {
					if o.WhyNot == "" {
						t.Errorf("disabled offer %q for %s carries no reason", o.ID, p)
					}
					continue
				}
				a, ok := representativeSubmission(o)
				if !ok {
					t.Errorf("enabled offer %q for %s has no usable submission", o.ID, p)
					continue
				}
				if _, _, err := m.Apply(state, p, a); err != nil {
					t.Errorf("offer %q for %s said enabled but Apply refused %+v: %v", o.ID, p, a, err)
				}
			}
		}
	}
}

// TestOnlyOnePlayerIsEverOffered — the turn-model invariant, with an open
// intermission the deliberate exception (module.Intermission awaits every
// seat at once).
func TestOnlyOnePlayerIsEverOffered(t *testing.T) {
	m := New()
	for _, state := range collectStates(t, 2, 6) {
		s, err := decode(state)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if s.Intermission.Open {
			continue
		}
		var active []string
		for _, p := range s.Players {
			offers, err := m.LegalActions(state, p)
			if err != nil {
				t.Fatalf("LegalActions: %v", err)
			}
			for _, o := range offers {
				if o.Enabled {
					active = append(active, p)
					break
				}
			}
		}
		if len(active) > 1 {
			t.Errorf("more than one player has enabled offers at once: %v", active)
		}
	}
}

// TestOffersNeverNameATileYouDoNotHold is the hidden-information half: a
// hand-sourced offer must only ever list tiles the player actually holds.
func TestOffersNeverNameATileYouDoNotHold(t *testing.T) {
	m := New()
	for _, state := range collectStates(t, 2, 6) {
		s, err := decode(state)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		for _, p := range s.Players {
			offers, err := m.LegalActions(state, p)
			if err != nil {
				t.Fatalf("LegalActions: %v", err)
			}
			for _, o := range offers {
				if o.Source == nil || o.Source.Zone != module.FromHand {
					continue
				}
				if o.Source.OwnerID != p {
					t.Errorf("offer %q sources a hand not owned by the viewer (%s)", o.ID, o.Source.OwnerID)
				}
				for _, tile := range o.Source.Cards {
					if !hasTile(s.Hands[p], tile) {
						t.Errorf("offer %q for %s names a tile %s not in their hand", o.ID, p, tile)
					}
				}
			}
		}
	}
}
