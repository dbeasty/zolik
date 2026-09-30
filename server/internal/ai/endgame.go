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
//
// Neither policy will fill the table's last open meld unless that is how the
// turn goes out; see closesTable.
func (a *HeuristicAgent) chooseLayOff(v VisibleState, hand []string, k knowledge) (meldID, card string, ok bool) {
	crunch := wildCrunch(hand, v.Rules)
	if a.prof.LayOffPolicy == LayOffFirstFit {
		meldID, card, ok = findLayOff(v.MeldMeta, v.Melds, hand, v.Rules, v.GameNumber)
		if !ok || !closesTable(v, hand, meldID, card) {
			return meldID, card, ok
		}
	}
	var opts []layOffOption
	for _, o := range layOffOptions(v, hand) {
		if !closesTable(v, hand, o.meldID, o.card) {
			opts = append(opts, o)
		}
	}
	if len(opts) == 0 {
		return "", "", false
	}
	if a.prof.LayOffPolicy == LayOffFirstFit {
		// Only reached when the first fit was refused. The order is still
		// first-fit's own: naturals before wilds, reversed in a crunch.
		sort.SliceStable(opts, func(i, j int) bool {
			if opts[i].wild != opts[j].wild {
				return opts[i].wild == crunch
			}
			return false
		})
		return opts[0].meldID, opts[0].card, true
	}
	sort.SliceStable(opts, func(i, j int) bool { return betterLayOff(opts[i], opts[j], a.prof, k, crunch) })
	return opts[0].meldID, opts[0].card, true
}

// closesTable reports that laying one card onto this meld would leave nothing
// open anywhere on the table — every meld a full four-suit set — while the
// player still has cards to get rid of.
//
// That position has no exit. A down player sheds cards only by laying them
// off or by laying a new meld, and a new meld needs a hand big enough to leave
// a card over for the discard; once every seat is down to one or two cards and
// every set is full, nobody at the table has a legal way to get any smaller,
// and the deal cycles the stock forever. Continental's first deal, two sets
// and nothing else, reached it in roughly one match in twelve: each seat laid
// off its fourth suits as fast as it could and the last lay-off shut the table
// on everybody, the player who made it included.
//
// So the last opening is kept for going out on. A lay-off that fills it is
// fine when it is the turn's second-last card, because the discard then ends
// the deal; otherwise the card stays in hand, where it is the go-out card for
// the turn the hand gets down to it, and the opening stays there for every
// other seat to go out on too.
//
// "Open" means some card that is not already on the table would fit. A set of
// three aces is closed if both packs' fourth aces and every joker are already
// laid somewhere; counting it open would let the last real opening be filled.
func closesTable(v VisibleState, hand []string, meldID string, card string) bool {
	if len(hand) <= 2 {
		return false // the lay-off and a discard empty the hand
	}
	packs := v.DeckCount
	if packs <= 0 {
		packs = defaultPacks
	}
	laid := map[string]int{card: 1}
	for _, melds := range v.Melds {
		for _, m := range melds {
			for _, c := range m {
				laid[c]++
			}
		}
	}
	for _, owner := range sortedOwners(v.MeldMeta) {
		for i, mi := range v.MeldMeta[owner] {
			if i >= len(v.Melds[owner]) {
				continue
			}
			meld := v.Melds[owner][i]
			if mi.MeldID == meldID {
				meld = append(append([]string(nil), meld...), card)
			}
			if meldFillable(meld, laid, packs, v.Rules) {
				return false
			}
		}
	}
	return true
}

// meldFillable reports whether any card with a copy still off the table would
// extend this meld.
func meldFillable(meld []string, laid map[string]int, packs int, cfg rules.RulesConfig) bool {
	for _, c := range everyCard {
		if laid[c] >= packs {
			continue
		}
		if _, err := rules.ValidateMeld(append(append([]string(nil), meld...), c), cfg); err == nil {
			return true
		}
	}
	return false
}

// everyCard is one copy of each distinct card in a pack.
var everyCard = func() []string {
	out := []string{"JOKER1", "JOKER2"}
	for i := range len(rules.RankOrder) {
		for _, s := range suits {
			out = append(out, cardOf(i, s))
		}
	}
	return out
}()

// layOffOption is one legal lay-off, with what it is worth and what it costs.
type layOffOption struct {
	meldID string
	card   string
	// pts is the penalty this sheds — the whole point of a lay-off once the
	// race to go out is not yet decided.
	pts int
	// wild is a joker, which is the one card whose penalty points say the
	// opposite of what it is worth. See betterLayOff.
	wild bool
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
					wild:   rules.IsJoker(c),
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
// Except a wild one, and that exception is the whole reason this function was
// revisited. A joker is fifty penalty points, the most of any card in the
// deck, so "shed the expensive card first" named a joker every single time one
// would fit — and a joker fits nearly everywhere, which is what a joker is
// for. LayOffHighestPoints therefore spent the hand's wild card on a one-card
// shed at the first opportunity it got, on a set of fours if that was what the
// table happened to offer, and then had nothing left to finish the run it was
// building with. It is the "the AI throws its jokers away" report, arriving by
// the lay-off rather than by the discard.
//
// So a wild goes last, right up until the endgame flips the sign on it: once
// somebody is about to go out, the fifty points are about to be scored and the
// meld the joker was being saved for is never going to be laid. Then it is the
// first card off, for exactly the reason it was the last one before.
//
// A third rule stood here and was measured out: preferring not to extend a run
// that the next seat could go out on. It sounds like the sharper play and
// priced at nothing, twice — see the note in profile.go.
func betterLayOff(x, y layOffOption, p Profile, k knowledge, crunch bool) bool {
	if x.wild != y.wild {
		// Two ways round. The endgame is the deal ending under this agent, and
		// a crunch is the hand running out of anything it is legally allowed
		// to discard — see wildCrunch. Both mean the same thing about a joker:
		// there is no later left to save it for.
		if k.endgame || crunch {
			return x.wild
		}
		return !x.wild
	}
	if x.pts != y.pts {
		return x.pts > y.pts
	}
	// Stable tie-break on the card itself, so the same table always produces
	// the same play — the property the sorted-owner walk was protecting.
	return x.card < y.card
}
