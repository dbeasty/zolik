package canasta

import (
	"testing"

	"zolik/server/internal/module"
)

// collectStates plays offer-driven games and keeps every state along the way,
// so the checks below run against positions that really occur rather than
// positions somebody thought to write down.
func collectStates(t *testing.T, cfg module.MatchConfig, players []module.PlayerRef, seeds int, cap int) []module.State {
	t.Helper()
	m := New()
	var out []module.State

	for seed := int64(1); seed <= int64(seeds); seed++ {
		state, err := m.NewMatch(cfg, players, seed)
		if err != nil {
			t.Fatalf("NewMatch: %v", err)
		}
		out = append(out, state)

		for step := 0; step < 600 && len(out) < cap; step++ {
			done, _, err := m.Finished(state)
			if err != nil {
				t.Fatalf("Finished: %v", err)
			}
			if done {
				break
			}
			actor := ""
			for _, p := range players {
				offers, err := m.LegalActions(state, p.ID)
				if err != nil {
					t.Fatalf("LegalActions: %v", err)
				}
				for _, o := range offers {
					if o.Enabled {
						actor = p.ID
						break
					}
				}
				if actor != "" {
					break
				}
			}
			if actor == "" {
				break
			}
			offers, _ := m.LegalActions(state, actor)
			// Vary the choice by step so the corpus is not one narrow line of
			// play repeated: a fixed preference order visits a much smaller
			// slice of the game than the game actually has.
			a, ok := pickVarying(offers, step)
			if !ok {
				break
			}
			next, _, err := m.Apply(state, actor, a)
			if err != nil {
				t.Fatalf("seed %d step %d: offered %+v but refused: %v", seed, step, a, err)
			}
			state = next
			out = append(out, state)
		}
		if len(out) >= cap {
			break
		}
	}
	return out
}

// pickVarying chooses among the enabled offers, rotating the preference order
// with the step number.
func pickVarying(offers []module.ActionOffer, step int) (module.Action, bool) {
	orders := [][]string{
		{VerbLayMeld, VerbLayOff, VerbTakePile, VerbDraw, VerbDiscard},
		{VerbTakePile, VerbDraw, VerbLayOff, VerbLayMeld, VerbDiscard},
		{VerbDraw, VerbLayMeld, VerbDiscard, VerbLayOff, VerbTakePile},
	}
	order := orders[step%len(orders)]
	for _, verb := range order {
		for i := range offers {
			o := offers[i]
			if o.Verb != verb || !o.Enabled {
				continue
			}
			a, ok := actionFromOffer(o)
			if ok {
				return a, true
			}
		}
	}
	for i := range offers {
		if a, ok := actionFromOffer(offers[i]); ok && offers[i].Enabled {
			return a, true
		}
	}
	return module.Action{}, false
}

// actionFromOffer builds the concrete submission an offer describes, using only
// what the offer itself declares — the same discipline a UI shell is held to.
func actionFromOffer(o module.ActionOffer) (module.Action, bool) {
	a := module.Action{OfferID: o.ID, Verb: o.Verb}
	if o.Source != nil && o.Source.MinCards > 0 {
		if len(o.Source.Cards) < o.Source.MinCards {
			return a, false
		}
		a.Cards = append([]string(nil), o.Source.Cards[:o.Source.MinCards]...)
	}
	if o.Target != nil && o.Target.MeldID != "" {
		a.Target = o.Target.MeldID
	}
	return a, true
}

// TestOffersAgreeWithApply is the test that makes drift impossible rather than
// merely unlikely.
//
// Every enabled offer, submitted exactly as the offer describes it, must be
// accepted by the engine. An offer that promises a move the engine then refuses
// is worse than no offer at all: it is a control that does nothing, and no
// client-side check would catch it.
func TestOffersAgreeWithApply(t *testing.T) {
	m := New()
	tables := []struct {
		name    string
		players []module.PlayerRef
		cfg     module.MatchConfig
	}{
		{"two players", refs("p1", "p2"), module.MatchConfig{Options: module.Options{OptTargetScore: 500}}},
		{"four players", refs("p1", "p2", "p3", "p4"), module.MatchConfig{Options: module.Options{OptTargetScore: 500}}},
	}

	for _, tab := range tables {
		t.Run(tab.name, func(t *testing.T) {
			states := collectStates(t, tab.cfg, tab.players, 6, 1200)
			if len(states) < 100 {
				t.Fatalf("corpus is only %d states; the check would prove little", len(states))
			}

			checked := 0
			for _, state := range states {
				for _, p := range tab.players {
					offers, err := m.LegalActions(state, p.ID)
					if err != nil {
						t.Fatalf("LegalActions: %v", err)
					}
					for _, o := range offers {
						if !o.Enabled {
							// A disabled offer must say why, and must not
							// dangle cards a player could try to use.
							if o.WhyNot == "" {
								t.Errorf("offer %q is off with no reason", o.ID)
							}
							if o.Source != nil && len(o.Source.Cards) > 0 && o.Source.MinCards > 0 {
								t.Errorf("offer %q is off but still advertises %v", o.ID, o.Source.Cards)
							}
							continue
						}
						a, ok := actionFromOffer(o)
						if !ok {
							t.Errorf("offer %q is on but describes no submission: %+v", o.ID, o)
							continue
						}
						if _, _, err := m.Apply(state, p.ID, a); err != nil {
							t.Fatalf("offer %q was enabled but the engine refused %+v: %v\n%s",
								o.ID, a, err, module.DescribeOffers(offers))
						}
						checked++
					}
				}
			}
			t.Logf("%d states, %d enabled offers all accepted", len(states), checked)
		})
	}
}

// TestPerCardOffersAgreeWithApply goes finer than the offer-level check: for
// every card in hand, being listed on an offer must mean exactly the same thing
// as the engine accepting it.
//
// This is where a second implementation of a rule would show up. Whether a card
// is discardable depends on red threes, on an unfinished initial meld and on
// whether shedding it would be going out without the canastas for it; whether a
// card lays off depends on rank, on wild limits and on whether the meld is
// already a canasta. Both lists are built by probing, and this proves the
// probing is the whole story.
func TestPerCardOffersAgreeWithApply(t *testing.T) {
	m := New()
	players := refs("p1", "p2", "p3", "p4")
	states := collectStates(t, module.MatchConfig{Options: module.Options{OptTargetScore: 500}}, players, 6, 800)

	discardChecks, layOffChecks := 0, 0
	for _, state := range states {
		s, err := decode(state)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if s.Status != "active" || s.Current == "" {
			continue
		}
		p := s.Current
		offers, err := m.LegalActions(state, p)
		if err != nil {
			t.Fatalf("LegalActions: %v", err)
		}

		if o := module.FindOffer(offers, OfferDiscard); o != nil && o.Source != nil {
			listed := map[string]bool{}
			for _, c := range o.Source.Cards {
				listed[c] = true
			}
			for _, c := range sortedUnique(s.Hands[p]) {
				_, _, err := m.Apply(state, p, module.Action{Verb: VerbDiscard, Cards: []string{c}})
				if (err == nil) != listed[c] {
					t.Fatalf("discard %q: offer says %v, engine says %v (%v)", c, listed[c], err == nil, err)
				}
				discardChecks++
			}
		}

		for _, o := range offers {
			if o.Verb != VerbLayOff || o.Target == nil {
				continue
			}
			listed := map[string]bool{}
			if o.Source != nil {
				for _, c := range o.Source.Cards {
					listed[c] = true
				}
			}
			for _, c := range sortedUnique(s.Hands[p]) {
				_, _, err := m.Apply(state, p, module.Action{
					Verb: VerbLayOff, Cards: []string{c}, Target: o.Target.MeldID,
				})
				if (err == nil) != listed[c] {
					t.Fatalf("lay off %q onto %s: offer says %v, engine says %v (%v)",
						c, o.Target.MeldID, listed[c], err == nil, err)
				}
				layOffChecks++
			}
		}
	}
	t.Logf("%d discard and %d lay-off card decisions cross-checked", discardChecks, layOffChecks)
}

// TestOnlyOnePlayerIsEverOffered anything. The runtime has no turn field to
// read — turn order is game state, and game state is opaque — so it works out
// who is on turn by asking who has an enabled offer. If two players ever did,
// the runtime would be wrong through no fault of its own.
func TestOnlyOnePlayerIsEverOffered(t *testing.T) {
	m := New()
	players := refs("p1", "p2", "p3", "p4")
	states := collectStates(t, module.MatchConfig{Options: module.Options{OptTargetScore: 500}}, players, 4, 600)

	for i, state := range states {
		done, _, err := m.Finished(state)
		if err != nil {
			t.Fatalf("Finished: %v", err)
		}
		var active []string
		for _, p := range players {
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
		switch {
		case done && len(active) != 0:
			t.Fatalf("state %d: match is over but %v still have offers", i, active)
		case !done && len(active) != 1:
			t.Fatalf("state %d: %d players have enabled offers, want exactly one (%v)", i, len(active), active)
		}
	}
}

// TestOffersNeverNameACardYouDoNotHold is the hidden-information check on the
// offer list specifically — the place it would be easiest to leak, because
// offers are built from full state and shipped per viewer.
func TestOffersNeverNameACardYouDoNotHold(t *testing.T) {
	m := New()
	players := refs("p1", "p2", "p3", "p4")
	states := collectStates(t, module.MatchConfig{Options: module.Options{OptTargetScore: 500}}, players, 4, 400)

	for _, state := range states {
		s, err := decode(state)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		for _, p := range players {
			offers, err := m.LegalActions(state, p.ID)
			if err != nil {
				t.Fatalf("LegalActions: %v", err)
			}
			for _, o := range offers {
				if o.Source == nil || o.Source.Zone != module.FromHand {
					continue
				}
				if o.Source.OwnerID != p.ID {
					t.Errorf("offer %q hands %s a selector over %s's hand", o.ID, p.ID, o.Source.OwnerID)
				}
				if !hasCards(s.Hands[p.ID], o.Source.Cards) {
					t.Errorf("offer %q names %v, which %s does not hold", o.ID, o.Source.Cards, p.ID)
				}
			}
		}
	}
}

// BenchmarkLegalActions guards the cost of the thing that runs on every
// broadcast. The offer list is built by probing the engine, which is only
// affordable because state is a JSON blob rather than a graph to clone.
func BenchmarkLegalActions(b *testing.B) {
	m := New()
	players := refs("p1", "p2", "p3", "p4")
	state, err := m.NewMatch(module.MatchConfig{}, players, 1)
	if err != nil {
		b.Fatal(err)
	}
	// Whoever is actually on turn — the deal rotates, so naming a seat here
	// would make the benchmark depend on which one dealt.
	s, err := decode(state)
	if err != nil {
		b.Fatal(err)
	}
	onTurn := s.Current

	// A mid-turn position, which is the expensive one: melds on the table mean
	// lay-off offers, and a full hand means per-card discard probing.
	state, _, err = m.Apply(state, onTurn, module.Action{Verb: VerbDraw})
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := m.LegalActions(state, onTurn); err != nil {
			b.Fatal(err)
		}
	}
}
