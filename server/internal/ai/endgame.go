package ai

import (
	"sort"

	"zolik/server/internal/rules"
)

// Choosing between lay-offs, and finishing the deal.
//
// findLayOff answers "is there a legal lay-off"; everything here answers the
// two questions after it. *Which* one — because the original took whichever it
// found first in sorted-owner order, an ordering chosen for determinism and
// arbitrary for everything else. And *whether the turn can finish the deal* —
// because going out was previously emergent: shed one card, shed another, and
// eventually the hand runs dry, having spent on the way the exact card a
// complete go-out wanted.

// chooseLayOff picks the lay-off this profile would make.
//
// LayOffFirstFit is the original behaviour, kept verbatim rather than
// re-derived, so that Medium plays exactly as it always did and is a fixed
// reference for everything measured against it.
func (a *HeuristicAgent) chooseLayOff(v VisibleState, hand []string, k knowledge) (meldID, card string, ok bool) {
	if a.prof.LayOffPolicy == LayOffFirstFit {
		return findLayOff(v.MeldMeta, v.Melds, hand, v.Rules, v.GameNumber)
	}
	opts := layOffOptions(v, hand)
	if len(opts) == 0 {
		return "", "", false
	}
	sort.SliceStable(opts, func(i, j int) bool { return betterLayOff(opts[i], opts[j], a.prof, k) })
	return opts[0].meldID, opts[0].card, true
}

// layOffOption is one legal lay-off, with what it is worth and what it costs.
type layOffOption struct {
	meldID string
	card   string
	// pts is the penalty this sheds — the whole point of a lay-off once the
	// race to go out is not yet decided.
	pts int
}

// layOffOptions is every legal lay-off, found the same way findLayOffAmong
// finds the first one — including every one of its guards. Those guards are
// not incidental: they are what stops the agent shedding its last discardable
// card, and what stops it triggering a joker reclaim it has nowhere to put.
func layOffOptions(v VisibleState, hand []string) []layOffOption {
	var out []layOffOption
	cfg := v.Rules
	if !cfg.IsFinalDeal(v.GameNumber) && len(hand) == 1 {
		return nil
	}
	for _, owner := range sortedOwners(v.MeldMeta) {
		metas := v.MeldMeta[owner]
		ownerMelds := v.Melds[owner]
		for i, mi := range metas {
			if i >= len(ownerMelds) {
				continue
			}
			existing := ownerMelds[i]
			seen := map[string]bool{}
			for _, c := range hand {
				if seen[c] {
					continue // a duplicate card is the same lay-off twice
				}
				cand := append(append([]string(nil), existing...), c)
				if _, err := rules.ValidateMeld(cand, cfg); err != nil {
					continue
				}
				if !handCanStillDiscard(removeCardsOnce(hand, []string{c}), cfg, true) {
					continue
				}
				if cfg.JokerReclaimMustPlay {
					if joker, replaced, would := layOffWouldReclaim(existing, mi, c, cfg); would {
						postHand := append(removeCardsOnce(hand, []string{c}), joker)
						if !reclaimedJokerPlayable(v.MeldMeta, v.Melds, owner, i, replaced, postHand, joker, cfg, v.GameNumber) {
							continue
						}
					}
				}
				seen[c] = true
				out = append(out, layOffOption{
					meldID: mi.MeldID,
					card:   c,
					pts:    rules.PenaltyPoints(c, false),
				})
			}
		}
	}
	return out
}

// betterLayOff orders lay-offs, best first.
//
// Points, then the card itself. A lay-off is the only way to shed a card
// without giving it to anybody, so the expensive card goes.
//
// A third rule stood here and was measured out: preferring not to extend a run
// that the next seat could go out on. It sounds like the sharper play and
// priced at nothing, twice — see the note in profile.go.
func betterLayOff(x, y layOffOption, p Profile, k knowledge) bool {
	if x.pts != y.pts {
		return x.pts > y.pts
	}
	// Stable tie-break on the card itself, so the same table always produces
	// the same play — the property the sorted-owner walk was protecting.
	return x.card < y.card
}
