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

func (b bot) Act(raw module.State, botSeat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	playerID := botSeat.PlayerID
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

	p := profileFor(botSeat.Skill)
	rnd := rand.New(rand.NewSource(seedFor(s, seat)))
	if s.Street == streetPreflop {
		return mn.action(preflop(s, seat, mn, p, rnd))
	}
	return mn.action(postflop(s, seat, mn, p, rnd))
}

// --- how well to play it -----------------------------------------------------

// profile is what a skill setting changes about how this seat plays poker.
//
// The knobs are split the way the ones in internal/ai/profile.go are, and for
// the same reason: a weak poker player is not one who calculates badly, it is
// one who does not ask the second question. The first question — what am I
// likely to have at showdown — every profile answers the same way, out of the
// same rollout. The second — what is the *other* player likely to have, given
// that they are betting — is the one this table turns up and down, and it is
// where both of the complaints this file was revisited for live.
type profile struct {
	skill module.Skill

	// --- reading the table ---

	// bluffShare is how much of an opponent's betting range this profile
	// treats as a bluff rather than as the hand the size of the bet claims.
	//
	// This is the "does not get bluffed" dial, and it is the correction to a
	// model that was right in one direction only. claimedBy reads a big bet as
	// a claim to a real hand and equity measures against hands that could make
	// it — which stops the bot calling three-times-pot with queen high, and
	// which, taken literally, also means nobody ever bluffs. A bot that
	// believes that folds every hand it cannot beat a value range with, which
	// at a table with any aggression at all is most of them, and it can be
	// robbed by anyone who notices.
	//
	// So an opponent's range is a mixture: mostly the hand the bet claims, and
	// this much of the time anything at all. Zero believes every bet
	// completely; one reads no meaning into a bet's size and is the calling
	// station this file was written to replace.
	bluffShare float64

	// --- aggression ---

	// cbet is the chance of betting the flop heads-up as the seat that raised
	// before it, whatever the flop brought. Most flops miss most hands, and
	// the player who showed strength first is the one both players expect to
	// have hit.
	cbet float64
	// semiBluff is the chance of betting or raising a draw that is not yet
	// worth a value bet. The pot can be won twice — now, because the bet
	// folds a better hand, or later, because the draw comes in — and a hand
	// that only ever checks its draws collects on the second of those.
	semiBluff float64
	// bluff is the chance of betting a hand with nothing at all when checked
	// to, heads-up, on a street where the story holds up.
	bluff float64
	// bluffRaise is the chance of answering a bet with a raise on nothing. The
	// most expensive bluff to make and the most expensive one to face, so it
	// is the smallest number here.
	bluffRaise float64
	// steal is the chance of opening from late position with a hand that could
	// not open from anywhere else, when nobody has shown any interest.
	steal float64

	// --- discipline ---

	// loose is how many Chen points below its own bar this profile still pays
	// to see a flop. The beginner's defining mistake, priced in points rather
	// than modelled as randomness: it is not that a novice picks bad hands at
	// random, it is that their bar is lower than it should be.
	loose float64
}

// profiles is the whole ladder, one row per skill.
//
// Medium is deliberately the bot exactly as it played before any of this
// existed — every number that made it up is either zero here or the literal
// that used to be inline — so it stays the fixed reference the other two are
// measured against, and every test written against the old bot still describes
// it. See TestBotLadderIsOrdered.
var profiles = map[module.Skill]profile{
	module.SkillEasy: {
		skill: module.SkillEasy,
		// Reads nothing into a bet's size, which is the same thing as
		// believing everyone is always bluffing: the calling station, exactly.
		// It is the right way for this game's weak setting to be weak, because
		// it is the mistake real beginners make and it loses money slowly
		// rather than looking broken.
		bluffShare: 1.0,
		loose:      1.5,
	},
	module.SkillMedium: {
		skill: module.SkillMedium,
		// The one bluff the old bot made: rarely, heads-up, on a street where
		// a story is believable.
		bluff: 0.18,
	},
	module.SkillHard: {
		skill: module.SkillHard,
		// Credits the claim — a big bet usually is what it says — but not
		// completely, which is the whole difference between folding correctly
		// and folding to anyone willing to bet big twice.
		bluffShare: 0.30,
		cbet:       0.65,
		semiBluff:  0.55,
		bluff:      0.22,
		bluffRaise: 0.14,
		steal:      0.35,
	},
}

// profileFor is the strength a skill plays at.
//
// An unknown or empty skill is Medium, not Easy — module.BotSeat's rule: a
// seat taken before skills existed was playing what is now called Medium, and
// defaulting down would silently weaken every table already in the database.
func profileFor(s module.Skill) profile {
	if p, ok := profiles[s]; ok {
		return p
	}
	return profiles[module.SkillMedium]
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
func preflop(s *GameState, seat *Seat, mn menu, p profile, rnd *rand.Rand) choice {
	score := chen(seat.Hole)
	owed := s.toCall(seat)

	// Worth one bet with. Every extra opponent is another hand that has to be
	// beaten, so the bar rises with the field and drops when few players are
	// left to act behind — position, priced in.
	playable := 5.0 + 0.4*float64(opponentsOf(s))
	if behind(s) <= 1 {
		playable -= 1.0
	}
	playable -= p.loose
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

	// A steal: last to act, nobody in front has raised, and a hand that is
	// close enough to playable that being called is not a disaster. The chips
	// in the middle are two blinds nobody has defended, and a seat that only
	// ever raises hands it likes never collects them. Priced as a *raise*
	// rather than as a bluff, because it is one — the hand is usually ahead of
	// the random cards still to act behind, it is simply not ahead enough to
	// open with from any other seat.
	case p.steal > 0 && mn.can(VerbRaise) && behind(s) <= 1 && opponentsOf(s) <= 2 &&
		s.CurrentBet <= s.BigBlind && score >= playable-2.0 && rnd.Float64() < p.steal:
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
func postflop(s *GameState, seat *Seat, mn menu, p profile, rnd *rand.Rand) choice {
	opponents := opponentsOf(s)
	if opponents < 1 {
		return choice{verb: VerbCheck}
	}

	owed := s.toCall(seat)
	eq := equity(seat.Hole, s.Board, opponents, rollouts(opponents),
		claimedBy(owed, potNow(s)), bluffShareOf(p, owed, potNow(s)), rnd)
	// A hand that is behind now but will not be behind for long. equity
	// already counts the times the draw comes in; what it cannot count is the
	// pot won without getting there, because the bet folded a better hand.
	// That is the whole difference between a draw and a busted hand, and the
	// only thing that makes betting one correct.
	drawing := len(s.Board) < 5 && drawOuts(seat.Hole, s.Board) >= 8

	if owed == 0 {
		switch {
		case eq >= 0.72 && mn.can(VerbRaise):
			// Strong enough to be called by worse: bet, and bet enough to be
			// worth being called.
			return choice{verb: VerbRaise, to: raiseTarget(s, mn, betOf(s, seat, 0.70))}
		case eq >= 0.55 && mn.can(VerbRaise) && rnd.Float64() < 0.6:
			return choice{verb: VerbRaise, to: raiseTarget(s, mn, betOf(s, seat, 0.50))}
		case p.semiBluff > 0 && drawing && eq < 0.55 && eq >= 0.28 &&
			opponents == 1 && mn.can(VerbRaise) && rnd.Float64() < p.semiBluff:
			// The semi-bluff. Bet the draw: it wins the pot now often enough
			// to be worth a bet on its own, and when it is called it still has
			// the outs it started with. Heads-up only — fold equity against
			// three opponents is a third of what it is against one, and a draw
			// bet into a field is just a donation with extra steps.
			return choice{verb: VerbRaise, to: raiseTarget(s, mn, betOf(s, seat, 0.60))}
		case p.cbet > 0 && s.Street == streetFlop && opponents == 1 &&
			tookTheLead(s, seat) && mn.can(VerbRaise) && rnd.Float64() < p.cbet:
			// The continuation bet. Two unpaired cards miss a flop about two
			// times in three, which is as true of the caller as of the raiser
			// — so the seat that raised before the flop bets it regardless,
			// and is right more often than it is wrong. Without this the bot
			// announced every flop it missed by checking, which is a tell a
			// human opponent picks up inside one session.
			return choice{verb: VerbRaise, to: raiseTarget(s, mn, betOf(s, seat, 0.55))}
		case p.bluff > 0 && eq < 0.30 && opponents == 1 && s.Street != streetFlop &&
			mn.can(VerbRaise) && rnd.Float64() < p.bluff:
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
	case p.semiBluff > 0 && drawing && !allIn && opponents == 1 && mn.can(VerbRaise) &&
		eq >= required-0.08 && rnd.Float64() < p.semiBluff:
		// The same semi-bluff from the other side. A draw facing a bet is
		// roughly a break-even call; raising it adds the times the raise
		// simply ends the hand, and that is what turns break-even into a
		// profit. The tolerance below `required` is what makes it a raise
		// rather than a fancy way of calling.
		return choice{verb: VerbRaise, to: raiseTarget(s, mn, betOf(s, seat, 0.70))}
	case allIn && eq >= required+0.05:
		// The last call of the hand has no implied odds to make up a thin
		// margin later, so it has to be right on its own.
		return choice{verb: VerbCall}
	case !allIn && eq >= required:
		return choice{verb: VerbCall}
	case p.bluffRaise > 0 && !allIn && opponents == 1 && s.Street != streetFlop &&
		mn.can(VerbRaise) && eq < required && float64(owed) <= 0.6*float64(potNow(s)) &&
		drawOuts(seat.Hole, s.Board) >= 4 && rnd.Float64() < p.bluffRaise:
		// Raising as a bluff, on a hand that was drawing and did not get
		// there. The busted draw is the right hand to do it with and the
		// reason is arithmetic rather than style: it has no showdown value to
		// give up, so the fold it wins is worth the whole pot, and the cards
		// it was drawing with are cards the opponent now cannot hold.
		return choice{verb: VerbRaise, to: raiseTarget(s, mn, betOf(s, seat, 0.80))}
	default:
		return choice{verb: VerbFold}
	}
}

// tookTheLead reports that this seat put the last raise in before the flop.
//
// Derived rather than recorded: GameState keeps no action log — it is
// re-marshalled to Mongo on every action and a per-hand history would be paid
// for on every fold — but it does keep what each seat has committed to the
// hand, and on a flop nobody has bet into yet that figure *is* the preflop
// betting. The seat that put in more than everybody still in the hand is the
// one that raised last.
func tookTheLead(s *GameState, seat *Seat) bool {
	if seat.Committed <= s.BigBlind {
		return false
	}
	for i := range s.Seats {
		if i == s.Current || !s.Seats[i].inHand() {
			continue
		}
		if s.Seats[i].Committed >= seat.Committed {
			return false
		}
	}
	return true
}

// drawOuts is how many cards in the deck would turn this hand into a straight
// or a flush.
//
// Counted structurally rather than by rollout, because it is asked on every
// postflop decision and a second Monte Carlo pass would double what the bot
// costs to think. It is also the more honest number for what it is used for:
// equity already knows how often a draw gets there, and what the semi-bluff
// needs to know is whether this hand is a draw *at all* — whether there is a
// card that changes it from losing to winning, or only a showdown it is
// already behind in.
//
// Nine outs for a flush draw, eight for an open-ended straight, four for a
// gutshot, added because a hand can be both. Anything at or above eight is a
// real draw; four is a story to bluff with rather than a hand to bet.
func drawOuts(hole, board []string) int {
	if len(board) >= 5 || len(board) == 0 {
		return 0
	}
	all := append(append(make([]string, 0, len(hole)+len(board)), hole...), board...)

	outs := 0
	// A suit is only a draw if this hand actually holds one of it; four to a
	// flush entirely on the board is everybody's draw and nobody's edge.
	for _, suit := range []string{"S", "H", "D", "C"} {
		total, mine := 0, 0
		for _, c := range all {
			if suitOf(c) == suit {
				total++
			}
		}
		for _, c := range hole {
			if suitOf(c) == suit {
				mine++
			}
		}
		if total == 4 && mine > 0 {
			outs += 9
		}
	}

	have := [15]bool{}
	for _, c := range all {
		v := rankValue[rankOf(c)]
		if v < 2 || v > 14 {
			continue
		}
		have[v] = true
		if v == 14 {
			have[1] = true // the wheel: A-2-3-4-5
		}
	}
	// Four in a row with a missing card live at *both* ends is open-ended:
	// eight outs. Both ends is the condition, not either — A-2-3-4 has only
	// the five, and counting it as eight is how a bot talks itself into
	// betting half a draw.
	openEnded := false
	for low := 1; low+3 <= 14; low++ {
		if !(have[low] && have[low+1] && have[low+2] && have[low+3]) {
			continue
		}
		lowEnd := low-1 >= 1 && !have[low-1]
		highEnd := low+4 <= 14 && !have[low+4]
		if lowEnd && highEnd {
			openEnded = true
		}
	}
	// Otherwise, four ranks inside some five-card window: one card fills it,
	// four outs. A completed straight fills all five and is not a draw at all.
	gutshot := false
	for low := 1; low+4 <= 14; low++ {
		n := 0
		for i := 0; i < 5; i++ {
			if have[low+i] {
				n++
			}
		}
		if n == 4 {
			gutshot = true
		}
	}
	switch {
	case openEnded:
		outs += 8
	case gutshot:
		outs += 4
	}
	return outs
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

// bluffShareOf is how much of *this* bet to disbelieve.
//
// claimedBy reads a bigger bet as a bigger claim, and stops there, which gets
// the ordinary range of bet sizes right and the top of it exactly backwards. A
// bet of three times the pot is not three times the hand a pot-sized bet is.
// Nobody makes that bet for value with a middling hand, because nothing worse
// would ever pay it — so the range behind it splits into the hands that want a
// call at any price and the hands that want a fold at any price, and there is
// very little in between. That is what polarised means, and its practical
// consequence is that the biggest bets at the table contain the *most* bluffs,
// not the fewest.
//
// Read literally, the old model therefore gave an opponent a free roll: bet
// enough and this bot folded whatever it held, two pair included, and the
// bigger the bet the more certain the fold. So the share of the range treated
// as a bluff grows once a bet passes the size of the pot — up to a cap,
// because a profile that ends up disbelieving everything is the calling
// station this file exists to replace, and it is only Easy that is supposed to
// be one.
func bluffShareOf(p profile, owed, pot int) float64 {
	if p.bluffShare <= 0 || owed <= 0 || pot <= 0 {
		return p.bluffShare
	}
	// Measured against the pot with the bet already in it, the same figure
	// claimedBy brackets on, so the two readings of one bet cannot drift: 0.5
	// is a pot-sized bet and anything above it an overbet.
	ratio := float64(owed) / float64(pot)
	if ratio <= 0.5 {
		return p.bluffShare
	}
	share := p.bluffShare * (1 + 2*(ratio-0.5))
	cap := math.Max(p.bluffShare, 0.60)
	return math.Min(share, cap)
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
//
// bluffShare is what stops the floor being taken literally, and it is the
// second half of a model that was only ever half-written. Filtering to hands
// that could make the bet says every bet is honest; nobody plays that way, and
// a bot that assumes it folds every hand it cannot beat a value range with. So
// the answer is a blend of the two counts this function was already keeping —
// mostly the filtered one, because a big bet usually is what it says, and
// bluffShare of the unfiltered one, because sometimes it is not. At zero the
// arithmetic is exactly what it was before the parameter existed.
func equity(hole, board []string, opponents, trials, floor int, bluffShare float64, rnd *rand.Rand) float64 {
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
	if dealt == 0 {
		return 0.5
	}
	unfiltered := anyhow / float64(dealt)
	if counted == 0 {
		// No trial ever produced a hand that could have made this bet, so
		// there is no value range to blend with.
		return unfiltered
	}
	filtered := won / float64(counted)
	if bluffShare <= 0 {
		return filtered
	}
	if bluffShare >= 1 {
		return unfiltered
	}
	return (1-bluffShare)*filtered + bluffShare*unfiltered
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
