package holdem

import (
	"math"
	"math/rand"
	"strconv"

	"zolik/server/internal/module"
)

// How a seat nobody is sitting at plays poker.
//
// Hold'em is the game the offer list cannot play. Every other module here can
// be played tolerably by module.OfferBot, because in a card game the offers
// *are* the decision: the legal moves are few, concrete, and mostly good. Poker
// has four offers on every street of every hand, and which one is right depends
// entirely on two cards the offer list does not mention. A bot that reads only
// the offers can do no better than pick a favourite verb, and this module's
// favourite was "call" — a calling station that never folded a hand and never
// raised one, which is the worst player in poker and was ours.
//
// So this is a module.Botted implementation, and it decides the way a person
// does: what am I likely to have when the money goes in, and what is it costing
// me to find out.
//
//	Before the flop   a hand-strength score (Bill Chen's formula), against a
//	                  threshold that moves with how many players are still to
//	                  beat, how late the seat is acting, and how much has
//	                  already been raised in front of it.
//	After the flop    a Monte Carlo rollout of the rest of the deck for an
//	                  equity estimate, compared against the pot odds the bet
//	                  in front of it is laying.
//
// Two disciplines hold the whole file together. The first: it is handed the
// *whole* state, other seats' hole cards included, and reads only its own —
// TestBotDoesNotPeek pins that. The second: it never decides something is
// legal. Every action it returns is built from an enabled offer, clamped to
// the range that offer declares, and degrades to the next-best legal verb when
// what it wanted is not on the menu. The engine remains the only authority on
// the rules, exactly as it is for a human.
func (m *Module) Bot() module.Bot { return bot{} }

type bot struct{}

var _ module.Bot = bot{}

func (b bot) Act(raw module.State, playerID string, offers []module.ActionOffer) (module.Action, bool) {
	s, err := decode(raw)
	if err != nil {
		return module.ChooseAction(offers, nil)
	}
	// Between hands, or not this seat's decision at all: there is nothing to
	// weigh and the offer list is the entire answer.
	if s.Break.Open || s.Status != "active" || s.Current < 0 || s.Seats[s.Current].PlayerID != playerID {
		return module.ChooseAction(offers, nil)
	}

	mn := menuOf(offers)
	seat := &s.Seats[s.Current]
	if len(mn.byVerb) == 0 || len(seat.Hole) < 2 {
		return module.ChooseAction(offers, nil)
	}

	if s.Street == streetPreflop {
		return mn.action(preflop(s, seat, mn))
	}
	return mn.action(postflop(s, seat, mn))
}

// choice is a decision before it is a submission: a verb, and for a raise the
// total to raise to.
type choice struct {
	verb string
	to   int
}

// --- before the flop ---------------------------------------------------------

// preflop plays two cards and a price.
//
// There is no equity worth computing here. Every hand is live against random
// cards — the worst starting hand in poker still wins a third of the time
// heads-up — so a rollout would talk a bot into playing 72o, which is exactly
// the mistake the calling station made. What matters before the flop is not how
// often a hand wins a showdown but how often it is *ahead of a hand somebody
// was willing to put money in with*, and that is what a hand-strength score
// approximates.
func preflop(s *GameState, seat *Seat, mn menu) choice {
	score := chen(seat.Hole)
	owed := s.toCall(seat)

	// Worth one bet with. Every extra opponent is another hand that has to be
	// beaten, so the bar rises with the field and drops when few players are
	// left to act behind — position, priced in.
	playable := 5.0 + 0.4*float64(opponentsOf(s))
	if behind(s) <= 1 {
		playable -= 1.0
	}
	// And a raise in front means the chips already in are not random. Three
	// big blinds asks for a real hand; a re-raise to nine asks for a much
	// better one.
	if pressure := float64(s.CurrentBet) / float64(max(s.BigBlind, 1)); pressure > 1 {
		playable += (pressure - 1) * 0.45
	}
	playable = math.Min(playable, 11)
	raiseWorthy := playable + 3.0

	// A stack this short has no fold equity left to save for a better spot and
	// no room to play a flop: the next good hand goes in whole. Getting this
	// wrong is how a bot blinds itself out of a freezeout in second place.
	if short := seat.Stack <= 10*s.BigBlind; short && score >= playable+1.5 && mn.can(VerbRaise) {
		return choice{verb: VerbRaise, to: shove(mn)}
	}

	switch {
	case score >= raiseWorthy && mn.can(VerbRaise):
		return choice{verb: VerbRaise, to: raiseTarget(s, mn, preflopRaiseTo(s))}
	case owed == 0:
		// Nothing to pay and nothing worth raising: take the free card.
		return choice{verb: VerbCheck}
	case score >= playable:
		return choice{verb: VerbCall}
	case score >= playable-2.0 && float64(owed) <= 0.06*float64(potNow(s)+owed)+float64(s.SmallBlind):
		// Getting a price. A blind already half in, or a limped pot behind a
		// big field, is worth a look with a hand that could not open.
		return choice{verb: VerbCall}
	default:
		return choice{verb: VerbFold}
	}
}

// preflopRaiseTo is the total a preflop raise goes to: three big blinds plus
// whatever the limpers have already put in, or three times the last raise when
// re-raising. Both are the sizes a table actually uses, and both leave a hand
// that calls with the wrong price to do it.
func preflopRaiseTo(s *GameState) int {
	if s.CurrentBet > s.BigBlind {
		return 3 * s.CurrentBet
	}
	limped := potNow(s) - s.SmallBlind - s.BigBlind
	if limped < 0 {
		limped = 0
	}
	return 3*s.BigBlind + limped
}

// chen scores two hole cards by Bill Chen's formula.
//
// Published, and pinned by TestChenScoresTheKnownHands against the numbers it
// is known for, so this is a citation rather than a guess: high card, doubled
// for a pair, a point for suits, minus the gap, plus one back for a low
// connector that makes straights from both ends.
func chen(hole []string) float64 {
	if len(hole) < 2 {
		return 0
	}
	hi, lo := rankValue[rankOf(hole[0])], rankValue[rankOf(hole[1])]
	if lo > hi {
		hi, lo = lo, hi
	}

	score := highCardPoints(hi)
	if hi == lo {
		score *= 2
		if score < 5 {
			score = 5
		}
	}
	if suitOf(hole[0]) == suitOf(hole[1]) {
		score += 2
	}
	if hi != lo {
		gap := hi - lo - 1
		switch {
		case gap == 1:
			score -= 1
		case gap == 2:
			score -= 2
		case gap == 3:
			score -= 4
		case gap >= 4:
			score -= 5
		}
		// Both cards under a queen and no more than one gap: a hand that makes
		// straights from either end, which the gap penalty alone undercounts.
		if gap <= 1 && hi < 12 {
			score += 1
		}
	}
	// Chen rounds up to the nearest half point.
	return math.Ceil(score*2) / 2
}

func highCardPoints(v int) float64 {
	switch v {
	case 14:
		return 10
	case 13:
		return 8
	case 12:
		return 7
	case 11:
		return 6
	default:
		return float64(v) / 2
	}
}

// --- after the flop ----------------------------------------------------------

// postflop compares what the hand is worth against what the bet costs.
//
// This is the arithmetic the calling station never did. A bet of half the pot
// asks for a third of the pot to call, so it needs to win a third of the time
// to break even — below that, calling loses money however pretty the cards
// are, and folding is not timidity but the correct play.
func postflop(s *GameState, seat *Seat, mn menu) choice {
	opponents := opponentsOf(s)
	if opponents < 1 {
		return choice{verb: VerbCheck}
	}

	rnd := rand.New(rand.NewSource(seedFor(s, seat)))
	owed := s.toCall(seat)
	eq := equity(seat.Hole, s.Board, opponents, rollouts(opponents), claimedBy(owed, potNow(s)), rnd)

	if owed == 0 {
		switch {
		case eq >= 0.72 && mn.can(VerbRaise):
			// Strong enough to be called by worse: bet, and bet enough to be
			// worth being called.
			return choice{verb: VerbRaise, to: raiseTarget(s, mn, betOf(s, seat, 0.70))}
		case eq >= 0.55 && mn.can(VerbRaise) && rnd.Float64() < 0.6:
			return choice{verb: VerbRaise, to: raiseTarget(s, mn, betOf(s, seat, 0.50))}
		case eq < 0.30 && opponents == 1 && s.Street != streetFlop &&
			mn.can(VerbRaise) && rnd.Float64() < 0.18:
			// A bluff, rarely, heads-up, on a street where a story is
			// believable. Not because this bot can read anybody, but because a
			// player who only ever bets good hands is free to play against.
			return choice{verb: VerbRaise, to: raiseTarget(s, mn, betOf(s, seat, 0.55))}
		}
		return choice{verb: VerbCheck}
	}

	required := float64(owed) / float64(potNow(s)+owed)
	allIn := owed >= seat.Stack

	switch {
	case eq >= required+0.25 && mn.can(VerbRaise) && !allIn:
		return choice{verb: VerbRaise, to: raiseTarget(s, mn, betOf(s, seat, 0.75))}
	case allIn && eq >= required+0.05:
		// The last call of the hand has no implied odds to make up a thin
		// margin later, so it has to be right on its own.
		return choice{verb: VerbCall}
	case !allIn && eq >= required:
		return choice{verb: VerbCall}
	default:
		return choice{verb: VerbFold}
	}
}

// claimedBy is what a bet of this size says the player making it has.
//
// This is the correction that makes a rollout mean anything. Dealt at random,
// queen-jack on an ace-king board wins six times in ten — and folds to a
// three-times-the-pot bet at every table in the world, because the hands that
// bet that much are not random. Reading the bet as a claim, and measuring
// against hands that could make it, is the difference between arithmetic and
// arithmetic about the right question.
//
// Coarse on purpose: three brackets, and a small bet claims nothing at all.
// A bot that treated every bet as the nuts would fold its way to nothing, which
// is the opposite mistake and just as expensive.
//
// `pot` is the pot with the bet already in it, since that is the figure every
// other calculation here uses. Measured against the pot *before* the bet the
// brackets read as: under a third of it claims nothing, up to three times it
// claims a pair, and more than that claims two.
func claimedBy(owed, pot int) int {
	if owed <= 0 || pot <= 0 {
		return highCard
	}
	switch ratio := float64(owed) / float64(pot); {
	case ratio < 0.25:
		return highCard
	case ratio < 0.75:
		return pair
	default:
		return twoPair
	}
}

// equity estimates how often this hand wins the pot, by dealing the rest of
// the deck out and counting — against opponents holding at least `floor`, the
// hand the money in front of this seat is claiming.
//
// Rollouts rather than a lookup table because the table would be a second
// implementation of the hand rankings, and this module already has one that
// the showdown itself uses — so an evaluator bug can never make the bot and
// the pot disagree about who won. A split counts as its fraction, which is why
// this returns a float rather than a win count.
//
// Trials that do not meet the floor are dealt and thrown away, so the count
// this averages over is the number of *relevant* deals rather than the number
// attempted. The attempt cap is what stops a floor nobody can reach — two pair
// on a board that makes none — from spinning; if it bites, the unfiltered
// estimate is returned rather than nothing.
func equity(hole, board []string, opponents, trials, floor int, rnd *rand.Rand) float64 {
	if len(hole) < 2 || opponents < 1 {
		return 1
	}
	seen := make(map[string]bool, len(hole)+len(board))
	for _, c := range hole {
		seen[c] = true
	}
	for _, c := range board {
		seen[c] = true
	}
	deck := make([]string, 0, 52)
	for _, c := range buildDeck() {
		if !seen[c] {
			deck = append(deck, c)
		}
	}

	runout := 5 - len(board)
	if runout < 0 {
		runout = 0
	}
	needed := runout + 2*opponents
	if needed > len(deck) {
		return 0.5
	}

	mine := make([]string, 0, 7)
	theirs := make([]string, 0, 7)
	won, counted := 0.0, 0
	anyhow, dealt := 0.0, 0
	for attempt := 0; attempt < trials*6 && counted < trials; attempt++ {
		// Partial Fisher-Yates: only the cards this trial actually deals get
		// shuffled, which is the difference between 9 swaps and 45.
		for j := 0; j < needed; j++ {
			k := j + rnd.Intn(len(deck)-j)
			deck[j], deck[k] = deck[k], deck[j]
		}
		run := deck[:runout]

		mine = append(append(append(mine[:0], hole...), board...), run...)
		best := Best(mine)

		ahead, split, claims := true, 1, floor <= highCard
		for o := 0; o < opponents; o++ {
			at := runout + 2*o
			theirs = append(append(append(theirs[:0], deck[at], deck[at+1]), board...), run...)
			rank := Best(theirs)
			if rank.Category >= floor {
				claims = true
			}
			switch best.Compare(rank) {
			case -1:
				ahead = false
			case 0:
				split++
			}
			// Nothing left to learn: this hand is beaten and the bet is
			// accounted for.
			if !ahead && claims {
				break
			}
		}

		score := 0.0
		if ahead {
			score = 1 / float64(split)
		}
		anyhow, dealt = anyhow+score, dealt+1
		if claims {
			won, counted = won+score, counted+1
		}
	}
	if counted > 0 {
		return won / float64(counted)
	}
	return anyhow / float64(dealt)
}

// rollouts trades accuracy for time as the table fills.
//
// Each opponent costs a whole seven-card evaluation per trial, and a bot gets
// about a second to think. These counts put the standard error of the estimate
// around two points either way, which is well inside the margins the decisions
// above are drawn at.
func rollouts(opponents int) int {
	switch {
	case opponents <= 1:
		return 400
	case opponents <= 3:
		return 240
	default:
		return 160
	}
}

// --- sizing and legality -----------------------------------------------------

// betOf is a total that leaves the given fraction of the pot in front of this
// seat, counting the call it has to make first. Sizing in pot fractions rather
// than blinds is what makes a bet mean the same thing on the river as on the
// flop.
func betOf(s *GameState, seat *Seat, fraction float64) int {
	owed := s.toCall(seat)
	pot := potNow(s) + owed
	return seat.Bet + owed + int(math.Round(fraction*float64(pot)))
}

// raiseTarget clamps a wanted total into the range the module says is legal.
//
// The range comes off the offer's own parameter spec, so the minimum-raise
// rule lives in exactly one place and this file cannot get it wrong.
func raiseTarget(s *GameState, mn menu, want int) int {
	lo, hi, ok := mn.bounds()
	if !ok {
		return want
	}
	if want < lo {
		want = lo
	}
	if want > hi {
		want = hi
	}
	// A raise that leaves a token stack behind is a shove pretending not to be
	// one: the chips are going in either way, and holding two blinds back only
	// improves the price for whoever calls.
	if hi-want < 2*s.BigBlind {
		want = hi
	}
	return want
}

// shove is everything.
func shove(mn menu) int {
	_, hi, ok := mn.bounds()
	if !ok {
		return 0
	}
	return hi
}

// menu is the offer list indexed by verb, and the only thing in this file
// permitted to say a move is available.
type menu struct{ byVerb map[string]module.ActionOffer }

func menuOf(offers []module.ActionOffer) menu {
	mn := menu{byVerb: make(map[string]module.ActionOffer, len(offers))}
	for _, o := range offers {
		if o.Enabled {
			mn.byVerb[o.Verb] = o
		}
	}
	return mn
}

func (mn menu) can(verb string) bool {
	_, ok := mn.byVerb[verb]
	return ok
}

func (mn menu) bounds() (int, int, bool) {
	o, ok := mn.byVerb[VerbRaise]
	if !ok {
		return 0, 0, false
	}
	for _, p := range o.Params {
		if p.Name == ParamAmount {
			return p.Min, p.Max, true
		}
	}
	return 0, 0, false
}

// fallbacks is what to do when the wanted verb is not on the menu, in order of
// preference.
//
// A raise this seat cannot afford becomes a call; a call with nothing to call
// becomes a check. Neither changes the decision, only how much of it fits.
//
// The last two lines are the ones worth reading, and they are deliberately not
// mirror images. A decision to fold degrades to a check, because a bot that has
// decided a hand is not worth paying for should still take a free card. A
// decision to check does *not* degrade to a call: wanting to check and being
// unable to means a bet has appeared that the decision never considered, and
// paying it on the strength of reasoning that assumed it was free is exactly
// the reflex this whole file replaced.
var fallbacks = map[string][]string{
	VerbRaise: {VerbRaise, VerbCall, VerbCheck, VerbFold},
	VerbCall:  {VerbCall, VerbCheck, VerbFold},
	VerbCheck: {VerbCheck, VerbFold},
	VerbFold:  {VerbCheck, VerbFold},
}

// action turns a decision into a submission built from a real offer.
func (mn menu) action(c choice) (module.Action, bool) {
	for _, verb := range fallbacks[c.verb] {
		o, ok := mn.byVerb[verb]
		if !ok {
			continue
		}
		a := module.Action{OfferID: o.ID, Verb: verb}
		if verb == VerbRaise {
			lo, hi, ok := mn.bounds()
			if !ok {
				continue
			}
			to := c.to
			if to < lo {
				to = lo
			}
			if to > hi {
				to = hi
			}
			a.Params = map[string]string{ParamAmount: strconv.Itoa(to)}
		}
		return a, true
	}
	return module.Action{}, false
}

// --- reading the table -------------------------------------------------------

// potNow is every chip on the table: the pot from streets already closed plus
// what is sitting in front of the seats on this one.
func potNow(s *GameState) int {
	total := s.Pot
	for i := range s.Seats {
		total += s.Seats[i].Bet
	}
	return total
}

// opponentsOf is how many other players are still contesting this pot,
// all-in ones included — they can still win it.
func opponentsOf(s *GameState) int {
	n := len(s.contenders()) - 1
	if n < 0 {
		return 0
	}
	return n
}

// behind is how many players still have a decision to make after this one on
// this street.
//
// Position, measured rather than named. "Button" and "cutoff" are labels for
// this number, and the number works on every street and at every table size
// without a special case for the blinds or for heads-up.
func behind(s *GameState) int {
	n := 0
	for i := range s.Seats {
		if i != s.Current && s.Seats[i].canAct() && !s.Seats[i].Acted {
			n++
		}
	}
	return n
}

// seedFor makes the bot's coin flips a function of the position it is looking
// at rather than of the clock.
//
// The bot bluffs sometimes and varies its bet sizes, and both need randomness;
// neither may make the same state produce different play on a re-read, or a
// test could not pin any of it and a replayed match would not replay. Derived
// from the match seed, so two tables never flip the same way either.
func seedFor(s *GameState, seat *Seat) int64 {
	h := s.Seed
	h = h*1000003 + int64(s.HandNumber)
	h = h*31 + int64(streetIndex(s.Street))
	h = h*31 + int64(s.Current)
	h = h*31 + int64(seat.Committed)
	h = h*31 + int64(s.CurrentBet)
	h = h*31 + int64(s.Pot)
	return h
}

func streetIndex(street string) int {
	switch street {
	case streetFlop:
		return 1
	case streetTurn:
		return 2
	case streetRiver:
		return 3
	case streetShowdown:
		return 4
	default:
		return 0
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
