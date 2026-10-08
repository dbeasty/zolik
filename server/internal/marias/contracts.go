package marias

import (
	"math"
	"math/rand"

	"zolik/server/internal/module"
	"zolik/server/internal/tricks"
	"zolik/server/internal/tricks/search"
)

// Judging a hand for betl and durch, the two games without trumps.
//
// Both are played in orderPlain, follow-and-beat, and the declarer leads the
// first trick. That makes them games of one suit at a time:
//
//   - Betl loses every trick. The declarer leads once — any trick they win
//     ends the game — so what matters is each suit when the defenders lead
//     it. Following, the declarer must beat the card that is winning if they
//     can, so a card is forced out by any lead under it that the declarer
//     has nothing lower to answer.
//   - Durch wins every trick, so the declarer leads them all and never
//     follows. A card wins if nothing the defenders still hold can beat it:
//     a run down from the ace, or a card led after enough rounds that the
//     defenders cannot have kept a higher one.
//
// gone is the cards known to be in no defender's hand — the declarer's own
// talon. Everything else not in hand is taken to be against them.

// outside counts, per suit, the cards a defender may hold: those below and
// above a rank.
type outside struct {
	have map[string]bool
}

func newOutside(hand, gone []string) outside {
	o := outside{have: map[string]bool{}}
	mine := map[string]bool{}
	for _, c := range hand {
		mine[c] = true
	}
	for _, c := range gone {
		mine[c] = true
	}
	for _, c := range buildDeck() {
		if !mine[c] {
			o.have[c] = true
		}
	}
	return o
}

// count is the defenders' cards of a suit that keep says to count.
func (o outside) count(suit byte, keep func(rank byte) bool) int {
	n := 0
	for i := 0; i < len(orderPlain); i++ {
		if r := orderPlain[i]; o.have[string(r)+string(suit)] && keep(r) {
			n++
		}
	}
	return n
}

// bySuit is the hand's cards of each suit, lowest first.
func bySuit(hand []string) map[byte][]string {
	out := map[byte][]string{}
	for i := len(orderPlain) - 1; i >= 0; i-- {
		for _, c := range hand {
			if tricks.Rank(c) == orderPlain[i] {
				out[tricks.Suit(c)] = append(out[tricks.Suit(c)], c)
			}
		}
	}
	return out
}

// betlDanger is, for each card of a betl hand the defenders could make win
// a trick, a rough chance that they do.
//
// A card is forced out by a lead under it with nothing of the declarer's
// between: following, the declarer must beat what is winning if they can.
// Once forced it wins unless the defender playing after it has a higher
// card of the suit and must beat it in turn. So a card is safe when no
// defender's card sits between it and the declarer's next card down (seven-
// eight-nine, or seven-nine with the eight in the talon), and otherwise as
// safe as the cards above it are many: the defence breaks it when one of
// them holds a forcing card and the other holds no cover. Each defender's
// card is taken to sit with either of them alike.
func betlDanger(hand, gone []string) map[string]float64 {
	o := newOutside(hand, gone)
	out := map[string]float64{}
	for suit, cards := range bySuit(hand) {
		for i, c := range cards {
			floor := byte(0)
			if i > 0 {
				floor = tricks.Rank(cards[i-1])
			}
			forcing := o.count(suit, func(r byte) bool {
				return orderPlain.Beats(tricks.Rank(c), r) && (floor == 0 || orderPlain.Beats(r, floor))
			})
			if forcing == 0 {
				continue
			}
			cover := o.count(suit, func(r byte) bool { return orderPlain.Beats(r, tricks.Rank(c)) })
			// Either defender may be the one short of cover; the other needs
			// one of the forcing cards.
			p := 2 * math.Pow(0.5, float64(cover)) * (1 - math.Pow(0.5, float64(forcing)))
			out[c] = min(1, p)
		}
	}
	return out
}

// betlLeadRisk is the chance that the declarer's first lead wins its trick:
// the best lead is the lowest card of the suit with the most of the
// defenders' cards above it, and it wins only if neither defender holds one.
func betlLeadRisk(hand, gone []string) float64 {
	o := newOutside(hand, gone)
	best := 1.0
	for suit, cards := range bySuit(hand) {
		above := o.count(suit, func(r byte) bool { return orderPlain.Beats(r, tricks.Rank(cards[0])) })
		best = min(best, math.Pow(0.5, float64(above)))
	}
	return best
}

// betlOdds is the rules of thumb's chance of making betl with hand.
func betlOdds(hand, gone []string) float64 {
	odds := 1 - betlLeadRisk(hand, gone)
	for _, p := range betlDanger(hand, gone) {
		odds *= 1 - p
	}
	return odds
}

// durchLosers is the cards of a durch hand that may not win their trick:
// those with a defender's card above them that the earlier rounds of the
// suit cannot be sure to have drawn.
func durchLosers(hand, gone []string) []string {
	o := newOutside(hand, gone)
	var out []string
	for suit, cards := range bySuit(hand) {
		n := o.count(suit, func(byte) bool { return true })
		// Highest first: the j-th card led is safe after j-1 rounds if no
		// defender can still hold a card over it.
		for j := len(cards) - 1; j >= 0; j-- {
			c := cards[j]
			rounds := len(cards) - 1 - j
			above := o.count(suit, func(r byte) bool { return orderPlain.Beats(r, tricks.Rank(c)) })
			if above > 0 && n > rounds {
				out = append(out, c)
			}
		}
	}
	return out
}

// contractHand is a judgement of one hand for betl or durch: the chance the
// rules of thumb give it, and, from twelve cards, which two go to the talon.
type contractHand struct {
	odds    float64
	discard []string
}

// judgeBetl judges hand for betl, laying away `discard` cards first (two
// for a declarer with the talon in hand, none for ten cards).
func judgeBetl(hand []string, discard int) contractHand {
	return bestKeep(hand, discard, betlOdds)
}

// judgeDurch judges hand for durch the same way: certain with every card a
// winner, a chance with one doubtful card, hopeless with more.
func judgeDurch(hand []string, discard int) contractHand {
	return bestKeep(hand, discard, func(keep, gone []string) float64 {
		switch len(durchLosers(keep, gone)) {
		case 0:
			return 1
		case 1:
			return durchOneLoser
		}
		return 0
	})
}

// bestKeep tries every way of laying `discard` cards away and keeps the one
// with the best odds.
func bestKeep(hand []string, discard int, odds func(keep, gone []string) float64) contractHand {
	best := contractHand{odds: -1}
	try := func(gone []string) {
		keep := removeCards(append([]string(nil), hand...), gone...)
		if v := odds(keep, gone); v > best.odds {
			best = contractHand{odds: v, discard: append([]string(nil), gone...)}
		}
	}
	switch discard {
	case 0:
		try(nil)
	case 2:
		for i := range hand {
			for j := i + 1; j < len(hand); j++ {
				try([]string{hand[i], hand[j]})
			}
		}
	}
	return best
}

// The bars a hand has to clear, set by TestContractBars: every betl and
// durch a chooser could have announced in 600 matches, played out against
// the game the bot would have played instead, every seat the skill
// measured (2026-10-07).
//
// Betl pays fifteen and the hand it is played on is a poor hra, so a betl
// made only half the time still gains. Medium seats gained most with
// betlOdds from about 0.47 (+12 units a betl over the hra it replaced, 59%
// made); hard seats, whose hra is better and whose defence is sharper, from
// about 0.55 (+15, 57% made). The bars sit a little above each.
//
// In licitovaný the same bars stand though a hard declarer makes fewer
// betls there (7 of 20 in 60 matches), most of them found in the talon
// after winning at plain sedma, where the alternative is a sedma without
// the hand for it or omyl. Played on the bench, a hard seat that never
// announced those betls lost 2.5 ±0.8 units a match, and one that wanted
// 0.7 of them 0.7 ±0.6.
//
// The solver is no judge of a betl: open-handed defenders see the
// declarer's cards and shed their covers to break it, and made 15% of the
// betls the bots' own defence let through 50-80% of the time — at a
// million nodes a sample. Durch it judges well, and cheaply: the defence
// only has to keep the right card, and to know which it mostly needs to see
// the declarer's hand.
const (
	betlMinOdds     = 0.5
	hardBetlMinOdds = 0.55
	// durchOneLoser is judgeDurch's figure for a hand with one doubtful
	// card: no medium seat plays it, and a hard seat only once the solver
	// makes it at least hardDurchMinSolved of the time. Played out by a
	// hard declarer, those the solver made above 0.2 made 13 of 13; those
	// under it, 1 of 23.
	durchOneLoser      = 0.5
	hardDurchMinSolved = 0.4
	// contractSamples and contractNodes bound the hard seat's check: a
	// durch is decided by the first trick it loses, so a solve is small.
	contractSamples = 30
	contractNodes   = 200_000
)

// noTrumpGame is the betl or durch this seat would play with hand, laying
// `discard` cards away first, and the cards it would lay; "" for neither.
// Easy seats play neither — they bid and play hra.
func (b bot) noTrumpGame(hand []string, discard int, skill module.Skill, r *rand.Rand) (string, []string) {
	if skill == module.SkillEasy {
		return "", nil
	}
	durch, betl := judgeDurch(hand, discard), judgeBetl(hand, discard)
	if durch.odds >= 1 {
		return gameDurch, durch.discard
	}
	if skill == module.SkillHard && durch.odds >= durchOneLoser {
		// Solved over sampled deals, from a source of its own.
		keep := removeCards(append([]string(nil), hand...), durch.discard...)
		rr := rand.New(rand.NewSource(r.Int63()))
		if contractOdds(gameDurch, keep, durch.discard, rr, contractSamples, contractNodes) >= hardDurchMinSolved {
			return gameDurch, durch.discard
		}
	}
	bar := betlMinOdds
	if skill == module.SkillHard {
		bar = hardBetlMinOdds
	}
	if betl.odds >= bar {
		return gameBetl, betl.discard
	}
	return "", nil
}

// noTrumpTalon is the two cards a betl or durch declarer lays away, judged
// as noTrumpGame judged them, or nil if the rules forbid either.
func noTrumpTalon(game string, hand, allowed []string) []string {
	j := judgeBetl(hand, 2)
	if game == gameDurch {
		j = judgeDurch(hand, 2)
	}
	for _, c := range j.discard {
		if !hasCard(allowed, c) {
			return nil
		}
	}
	return j.discard
}

// contractOdds is the share of sampled deals in which the declarer, holding
// hand and leading, makes game (betl or durch) against defenders who see
// every card. Open-handed defenders are stronger than real ones, so this
// errs low. gone is the talon the declarer laid; the rest of the deck is
// dealt at random, ten to each defender.
func contractOdds(game string, hand, gone []string, r *rand.Rand, samples, nodeLimit int) float64 {
	known := map[string]bool{}
	for _, c := range hand {
		known[c] = true
	}
	for _, c := range gone {
		known[c] = true
	}
	var unseen []string
	for _, c := range buildDeck() {
		if !known[c] {
			unseen = append(unseen, c)
		}
	}
	n := len(hand)
	if len(unseen) < 2*n {
		return 0
	}
	value := func(_ []tricks.Play, winner, _ int) (int, bool) {
		if (game == gameBetl) == (winner == 0) {
			return -1, true
		}
		return 0, false
	}
	made, counted := 0, 0
	for i := 0; i < samples; i++ {
		r.Shuffle(len(unseen), func(a, b int) { unseen[a], unseen[b] = unseen[b], unseen[a] })
		res := search.Solve(search.Problem{
			Hands:      [][]string{append([]string(nil), hand...), append([]string(nil), unseen[:n]...), append([]string(nil), unseen[n:2*n]...)},
			Leader:     0,
			Trump:      tricks.NoTrump,
			Order:      orderPlain,
			Policy:     tricks.FollowBeatTrump,
			Maximizer:  []bool{true, false, false},
			TrickValue: value,
			CardClass:  func(string) int { return 0 },
			NodeLimit:  nodeLimit,
		})
		if res.Exhausted {
			continue
		}
		counted++
		for _, v := range res.Values {
			if v == 0 {
				made++
				break
			}
		}
	}
	if counted == 0 {
		return 0
	}
	return float64(made) / float64(counted)
}
