package sim

import (
	"fmt"
	"slices"
	"sync/atomic"
	"testing"

	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// TestDiscardOfferAgreesWithTheEngine sweeps positions heuristic play actually
// reaches and asserts the discard offer says what the engine does: enabled
// exactly when some card in hand would be accepted as the discard, listing
// exactly those cards.
//
// The offer once went dark whenever the card a down player had just taken off
// the pile sat first in hand — the probe read that card's refusal as a verdict
// on the verb, so a player with several legal discards was shown none. A client
// or bot that trusts Enabled has no way to end that turn, which is a dead
// position from the seat's side however legal the engine thinks it is.
func TestDiscardOfferAgreesWithTheEngine(t *testing.T) {
	if testing.Short() {
		t.Skip("a seed sweep is not a fast test")
	}
	// Seeds 1..300 at two, three and four seats on every ruleset: 2 700 whole
	// matches. The check is cheap; the play is not — a Continental match costs
	// the agents about a second — so the sweep is cut into parallel chunks.
	const seeds, chunk = 300, 25
	var heldBack atomic.Int64
	t.Run("sweep", func(t *testing.T) {
		for _, rs := range rulesets() {
			for seats := 2; seats <= 4; seats++ {
				for from := int64(1); from <= seeds; from += chunk {
					rs, seats, from := rs, seats, from
					t.Run(fmt.Sprintf("%s/%dseats/%d", rs.name, seats, from), func(t *testing.T) {
						t.Parallel()
						for seed := from; seed < from+chunk; seed++ {
							if !sweepDiscardOffer(t, rs, seats, seed, &heldBack) {
								return
							}
						}
					})
				}
			}
		}
	})
	// The situation the bug lived in has to have come up, or the sweep proved
	// nothing about it.
	if n := heldBack.Load(); n == 0 {
		t.Error("no swept position held back a taken card; the sweep is vacuous")
	} else {
		t.Logf("%d positions held back a taken card", n)
	}
}

// sweepDiscardOffer plays one match, checking the discard offer at every
// position an agent acts in after drawing. False once it has failed.
func sweepDiscardOffer(t *testing.T, rs ruleset, seats int, seed int64, heldBack *atomic.Int64) bool {
	var ss []Seat
	for i := 0; i < seats; i++ {
		ss = append(ss, Seat{
			ID:    fmt.Sprintf("p%d", i),
			Skill: module.Skills[(int(seed)+i)%len(module.Skills)],
		})
	}
	failed := false
	r := Play(Options{
		Rules: rs.cfg, Seed: seed, Seats: ss, MaxActions: 4000,
		Observe: func(st rules.GameState, actor string) {
			if failed || st.Phase == rules.PhaseDraw {
				return
			}
			if msg, held := checkDiscardOffer(st, actor); msg != "" {
				failed = true
				t.Errorf("%d seats, seed %d, %s holding %v (taken %q): %s",
					seats, seed, actor, st.Hands[actor], st.DiscardTakenCard, msg)
			} else if held {
				heldBack.Add(1)
			}
		},
	})
	if !failed && r.Stalled {
		t.Errorf("%d seats, seed %d: stalled: %v", seats, seed, r.LastErr)
		failed = true
	}
	return !failed
}

// checkDiscardOffer compares the discard offer with the engine's own answer
// for every card in hand, and reports whether the taken card was held back.
func checkDiscardOffer(st rules.GameState, actor string) (string, bool) {
	var accepted []string
	for _, c := range st.Hands[actor] {
		if slices.Contains(accepted, c) {
			continue
		}
		if _, err := rules.ApplyAction(st, actor, rules.Action{Type: rules.ActionDiscard, Card: c}); err == nil {
			accepted = append(accepted, c)
		}
	}
	d := rules.DiscardOffer(st, actor)
	switch {
	case len(accepted) > 0 && !d.Enabled:
		return fmt.Sprintf("engine accepts %v but the offer is disabled (%s)", accepted, d.WhyNot), false
	case len(accepted) == 0 && d.Enabled:
		return fmt.Sprintf("offer enabled with cards %v but the engine accepts none", d.Source.Cards), false
	}
	if d.Enabled && !sameSet(d.Source.Cards, accepted) {
		return fmt.Sprintf("offer lists %v, engine accepts %v", d.Source.Cards, accepted), false
	}
	held := rules.HeldBackTakenCard(st, actor)
	if held != "" {
		if slices.Contains(d.Source.Cards, held) {
			return fmt.Sprintf("held-back %s listed as discardable", held), true
		}
		if !slices.ContainsFunc(d.Source.Refused, func(r rules.CardRefusal) bool {
			return r.Card == held && r.WhyNot == rules.ErrDiscardTakenCard
		}) {
			return fmt.Sprintf("held-back %s not refused on the offer: %v", held, d.Source.Refused), true
		}
	}
	return "", held != ""
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, c := range a {
		if !slices.Contains(b, c) {
			return false
		}
	}
	return true
}
