package ai

import (
	"zolik/server/internal/rules"
)

// Counting the deal.
//
// Everything in this file is derived from VisibleState and the agent's own
// hand, and from nothing else. That is the property that makes a strong bot
// fair rather than cheating: an opponent sitting at the table with a good
// memory could compute every number here, and TestAgentDoesNotPeek pins that
// permuting the hidden hands changes none of them.
//
// The central number is *outs*: how many copies of a given card are still
// unaccounted for. A pack count fixes the total (rules.BuildDeck deals one
// copy of every card per pack, jokers included), and four public places
// account for copies — the agent's own hand, every meld on the table, the
// discard pile as it currently stands, and the cards seats were publicly seen
// to take off that pile. Whatever is left is in the stock or in a hand nobody
// has shown, which is exactly the set the agent might still draw.
//
// The subtlety worth writing down is the *reshuffle*. When the stock runs out
// the engine recycles the pile back into it (rules.ApplyAction) and the pile
// becomes empty. A counter that accumulated "cards that have been discarded"
// would go on believing those cards were gone; this one counts the pile as it
// stands right now, so at the moment of the reshuffle every one of those cards
// correctly returns to the unseen pool without a single line of code noticing.
// Accumulate only what cannot be re-derived — which is the pickups — and the
// hard case handles itself.

type knowledge struct {
	prof  Profile
	actor string
	// unseen counts copies of each exact card that are unaccounted for. A
	// card absent from the map has never been seen at all, so every copy of
	// it is still out there — which is why buildUnseen records a zero
	// explicitly rather than dropping the entry. Getting that backwards makes
	// every unseen card look dead, which is the most confidently wrong a
	// counter can be.
	unseen map[string]int
	// packs is how many copies of any one card the deal started with, and
	// counting reports whether the map above was ever built.
	packs    int
	counting bool
	// passed is the ranks other seats have discarded within recall.
	passed map[int]bool
	// wanted is the exact cards some other seat was seen to take, and the
	// ranks adjacent to them — the cards it would be generous to discard.
	wanted map[string]bool
	// counts is each seat's hand size, and shortest is the smallest of them
	// among the opponents.
	counts   map[string]int
	shortest int
	// endgame reports that somebody is close enough to going out that
	// carrying penalty points has stopped being an investment.
	endgame bool
	seats   int
}

// endgameHandSize is the default for Profile.EndgameAt. Two, because a seat
// with two cards can go out next turn on one lay-off and a discard, and by the
// time it holds one it is too late to shed anything expensive.
const endgameHandSize = 2

func newKnowledge(v VisibleState, hand []string, actor string, prof Profile) knowledge {
	k := knowledge{
		prof:     prof,
		actor:    actor,
		unseen:   map[string]int{},
		passed:   map[int]bool{},
		wanted:   map[string]bool{},
		counts:   v.HandCounts,
		shortest: 1 << 30,
		seats:    len(v.RoundReqMet),
	}
	if k.seats == 0 {
		k.seats = len(v.HandCounts)
	}

	if prof.KeepPartials == KeepByOuts {
		k.buildUnseen(v, hand)
	}
	if prof.Recall > 0 {
		for seat, cards := range v.DiscardsBy(prof.Recall, k.seats) {
			if seat == actor {
				continue
			}
			for _, c := range cards {
				k.passed[rules.CardRank(c)] = true
			}
		}
		if prof.ReadPickups {
			for seat, cards := range v.KnownHeld {
				if seat == actor {
					continue
				}
				for _, c := range cards {
					k.markWanted(c)
				}
			}
		}
	}
	if prof.ReadHandCounts {
		for seat, n := range v.HandCounts {
			if seat == actor {
				continue
			}
			if n < k.shortest {
				k.shortest = n
			}
		}
		at := prof.EndgameAt
		if at <= 0 {
			at = endgameHandSize
		}
		k.endgame = k.shortest <= at
	}
	return k
}

// buildUnseen accounts for every copy of every card the agent can see, and
// leaves the remainder.
func (k *knowledge) buildUnseen(v VisibleState, hand []string) {
	packs := v.DeckCount
	if packs <= 0 {
		packs = defaultPacks
	}
	seen := map[string]int{}
	for _, c := range hand {
		seen[c]++
	}
	for _, c := range v.DiscardPile {
		seen[c]++
	}
	for _, melds := range v.Melds {
		for _, m := range melds {
			for _, c := range m {
				seen[c]++
			}
		}
	}
	for seat, cards := range v.KnownHeld {
		if seat == k.actor {
			continue // the agent's own hand is already counted, exactly
		}
		for _, c := range cards {
			seen[c]++
		}
	}
	for card, n := range seen {
		left := packs - n
		if left < 0 {
			left = 0
		}
		k.unseen[card] = left
	}
	k.packs, k.counting = packs, true
}

// copiesLeft is how many copies of an exact card the agent has not seen.
//
// A profile that does not count answers "as many as ever", which makes every
// fragment look equally alive and is exactly the behaviour of a player who is
// not counting. A profile that does count answers from the map — and a card
// missing from the map has been seen nowhere, so all of its copies are live.
func (k knowledge) copiesLeft(card string) int {
	if !k.counting {
		return defaultPacks
	}
	if n, ok := k.unseen[card]; ok {
		return n
	}
	return k.packs
}

// defaultPacks is rules.DeckCountForPlayers' own fallback, for a snapshot that
// did not say.
const defaultPacks = 2

// rankPassed reports that another seat has thrown this rank away within
// recall, which is weak evidence they will not want it now.
func (k knowledge) rankPassed(card string) bool { return k.passed[rules.CardRank(card)] }

// dangerousToOpponents reports that some seat demonstrably wants this card, on
// the evidence of what they took off the pile.
func (k knowledge) dangerousToOpponents(card string) bool { return k.wanted[card] }

// markWanted records a card an opponent took, and the cards next to it.
//
// Taking a nine of hearts says more than "he wants that nine". It says he is
// building around it, so the eight and ten of hearts and the other nines are
// all cards it would be generous to hand him. Adjacency is the whole reason
// this is worth tracking rather than just remembering the exact card, which he
// has already got.
func (k *knowledge) markWanted(card string) {
	if rules.IsJoker(card) {
		return
	}
	k.wanted[card] = true
	rank, suit := rules.CardRank(card), rules.CardSuit(card)
	for _, s := range suits {
		k.wanted[cardOf(rank, s)] = true
	}
	for _, adj := range adjacentRanks(rank) {
		k.wanted[cardOf(adj, suit)] = true
	}
}

var suits = []string{"H", "C", "D", "S"}

// cardOf rebuilds a card string from a rank index and a suit.
func cardOf(rank int, suit string) string {
	if rank < 0 || rank >= len(rules.RankOrder) || suit == "" {
		return ""
	}
	return string(rules.RankOrder[rank]) + suit
}

// adjacentRanks is the ranks a run could reach this one from.
//
// rules.RankOrder runs 2..A, so K and A are neighbours at the top. The ace is
// also a run's bottom card (A-2-3-4), which the linear order cannot express —
// so the two ends are stitched together here, in the one place adjacency is
// defined, rather than every caller remembering it.
func adjacentRanks(rank int) []int {
	last := len(rules.RankOrder) - 1 // index of A
	var out []int
	if rank > 0 {
		out = append(out, rank-1)
	}
	if rank < last {
		out = append(out, rank+1)
	}
	switch rank {
	case 0: // 2 is also reachable from A-low
		out = append(out, last)
	case last: // A is also adjacent to 2, as a run's bottom card
		out = append(out, 0)
	}
	return out
}

// keepValue scores how much the agent wants to hold on to one card.
//
// Higher is keep. It replaced a bare "is this card in a finished meld"
// boolean, and the reason is the agent's oldest and most visible bad habit:
// holding K♥ K♠ and throwing one of them, because a king is ten penalty points
// and neither king was in a *finished* meld. Every human who played against it
// noticed.
//
// The scale is deliberately coarse — these are buckets, not a valuation — and
// the policy decides how many of the buckets exist:
//
//	KeepFinished   only a complete meld is worth protecting (the old rule)
//	KeepFragments  pairs and part-runs too, flat
//	KeepByOuts     fragments in proportion to the outs still live for them,
//	               so a pair whose other six copies are all showing is not a
//	               pair at all — it is twenty penalty points pretending to be
//	               one.
func keepValue(hand []string, idx int, inFinished bool, k knowledge, cfg rules.RulesConfig) int {
	if inFinished {
		return keepFinished
	}
	if k.prof.KeepPartials == KeepFinished {
		return 0
	}
	card := hand[idx]
	if rules.IsJoker(card) {
		// A joker is meld material for anything. Never shed one as though it
		// were a loose card; the engine mostly forbids it anyway, and where it
		// does not, doing so is a mistake no strength should make on purpose.
		return keepFinished
	}
	partners := fragmentPartners(hand, idx, cfg)
	if partners == 0 {
		return 0
	}
	if k.prof.KeepPartials == KeepFragments {
		return keepFragment
	}
	// The thresholds are deliberately generous, and a sweep is the reason.
	// Two packs put eight of every rank in play, so a pair with nothing
	// showing has a dozen outs between the other suits, the neighbours in
	// suit and the four jokers — which means counting mostly agrees with not
	// counting, and only parts company on the fragment that is genuinely
	// dead. Tightening the bars to make it disagree more often was tried and
	// measured worse under a point floor: the agent dumped material that a
	// joker could still have finished. So counting is a veto on dead cards
	// rather than a valuation of live ones.
	outs := fragmentOuts(hand, idx, k, cfg)
	switch {
	case outs >= 3:
		return keepFragment
	case outs > 0:
		return keepThinFragment
	default:
		// Dead: nothing that completes this fragment is still out there.
		return 0
	}
}

const (
	keepFinished     = 3
	keepFragment     = 2
	keepThinFragment = 1
)

// fragmentPartners counts the other cards in hand that this one is building
// with — same rank for a set, same suit and within a run's reach for a run.
func fragmentPartners(hand []string, idx int, cfg rules.RulesConfig) int {
	card := hand[idx]
	if rules.IsJoker(card) {
		return 0
	}
	rank, suit := rules.CardRank(card), rules.CardSuit(card)
	n := 0
	for i, other := range hand {
		if i == idx || rules.IsJoker(other) {
			continue
		}
		if rules.CardRank(other) == rank {
			n++
			continue
		}
		if rules.CardSuit(other) == suit && runDistance(rank, rules.CardRank(other)) <= 2 {
			n++
		}
	}
	return n
}

// runDistance is how far apart two ranks are, or a large number when either is
// not a rank at all.
func runDistance(a, b int) int {
	if a < 0 || b < 0 {
		return 1 << 20
	}
	if a > b {
		a, b = b, a
	}
	return b - a
}

// fragmentOuts counts the cards still unaccounted for that would extend this
// fragment: the other suits of the same rank, the two ranks either side in
// suit, and every joker still out.
func fragmentOuts(hand []string, idx int, k knowledge, cfg rules.RulesConfig) int {
	card := hand[idx]
	rank, suit := rules.CardRank(card), rules.CardSuit(card)
	outs := 0
	for _, s := range suits {
		if s == suit {
			outs += k.copiesLeft(card) // the other copies of this exact card
			continue
		}
		outs += k.copiesLeft(cardOf(rank, s))
	}
	for _, adj := range adjacentRanks(rank) {
		outs += k.copiesLeft(cardOf(adj, suit))
	}
	outs += k.copiesLeft("JOKER1") + k.copiesLeft("JOKER2")
	return outs
}
