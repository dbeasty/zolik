package holdem

import (
	"math/rand"
	"strconv"

	"zolik/server/internal/module"
)

// Opponents with a style rather than a strength, for the learning pool
// (learn.Styled) and for the bench.
//
// The skill ladder is one player at three levels of care, and a network that
// only ever meets that player learns to beat that player. Real tables are full
// of people who are wrong in one particular direction, and each of these is one
// of those directions taken to its limit: the one who never stops betting, the
// one who never plays a hand, the one who never folds, and the one who tells a
// story on every river. They are caricatures on purpose — a style that is only
// a little off is hard to tell from noise in a few hundred hands, and the
// point is to put something in front of the learner whose mistake is there to
// be found.
//
// Built the way the bot is: every move from an enabled offer, raises clamped
// to the offer's range through the same menu, and a showdown stop answered by
// going on, never by turning a hand over.

// Styles are the opponents this module adds to the training pool.
func (learnGame) Styles() map[string]module.Bot {
	return map[string]module.Bot{
		"maniac":       maniac{},
		"rock":         rock{},
		"station":      station{},
		"riverbluffer": riverBluffer{},
		"solid":        solid,
	}
}

// solid is a tight-aggressive regular: it plays fewer hands than Hard, bets
// and raises the ones it plays, and — the reason it exists — calls a big bet
// or a shove with any strong made hand, top pair with a good kicker or better,
// and with a strong draw when the price is right.
//
// Every other opponent in the pool rewards an overbet. The rock and Hard fold
// most made hands to one, because claimedBy reads a bet past three quarters of
// the pot as a claim to two pair and almost nothing one pair holds beats that;
// the station calls it with anything, which pays the hand that has it and
// punishes nothing in particular. A learner trained against them shoves top
// pair weak kicker and middle pair into a pot half the size of the shove,
// because the shove nearly always takes it down. This player gives a big bet
// the credit of a pair it holds now and no more (madeClaim), so a hand that is shoving worse
// than a solid calling range is called by that range and loses.
//
// It is the heuristic with a stricter profile rather than a new decision
// procedure, so every move is still built from an enabled offer and clamped
// to the offer's range (bot.go), and there is no overbetDoubt: that is an
// exploit of bluffers, and a solid player's call of a shove is the hand's own
// equity against a value range, not a read.
var solid = bot{tuning: &solidProfile}

var solidProfile = profile{
	skill: module.SkillHard,
	// A bet claims a pair held now, never more, and a tenth of the time
	// nothing at all.
	madeClaim:  true,
	bluffShare: 0.1,
	// Tighter than the ladder before the flop: a point of Chen above its bar.
	loose: -1.0,
	// Aggressive with its draws, honest otherwise: a little bluffing so its
	// bets are not free to fold against, none of it on a raise.
	semiBluff: 0.35,
	bluff:     0.1,
	steal:     0.3,
}

// betting reports whether this is the seat's own betting decision, and if so
// the decoded state and the menu. Anything else — a showdown, somebody else's
// turn — has one sensible answer, which is the offer list's own with "go on"
// first.
func betting(raw module.State, playerID string, offers []module.ActionOffer) (*GameState, menu, bool) {
	s, err := decode(raw)
	if err != nil || s.Break.Open || s.Status != "active" || s.Current < 0 ||
		s.Seats[s.Current].PlayerID != playerID || len(s.Seats[s.Current].Hole) < 2 {
		return nil, menu{}, false
	}
	return s, menuOf(offers), true
}

func passOn(offers []module.ActionOffer) (module.Action, bool) {
	return module.ChooseAction(offers, []string{module.VerbContinue})
}

// maniac bets or raises four hands in five, at a pot or two pots, whatever it
// holds, and calls the rest. It never folds.
//
// The opponent that punishes a learner for folding too much, and pays off one
// that learns to wait for a hand and let it do the betting.
type maniac struct{}

func (maniac) Act(raw module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	s, mn, ok := betting(raw, seat.PlayerID, offers)
	if !ok {
		return passOn(offers)
	}
	st := &s.Seats[s.Current]
	rnd := rand.New(rand.NewSource(seedFor(s, st)))
	if mn.can(VerbRaise) && rnd.Float64() < 0.8 {
		fraction := 1.0
		if rnd.Float64() < 0.4 {
			fraction = 2
		}
		return mn.action(choice{verb: VerbRaise, to: raiseTarget(s, mn, betOf(s, st, fraction))})
	}
	// Call, or check when there is nothing to call: the menu degrades it.
	return mn.action(choice{verb: VerbCall})
}

// rock plays only premium hands — a Chen score of nine or more, which is
// about the top tenth of them — and folds everything else before the flop,
// taking the free check when it has one. Whatever it does play, it plays the
// way Hard would.
//
// The opponent whose bets mean exactly what they say: the one a learner
// should steal the blinds from, and fold to.
type rock struct{}

// rockBar is the Chen score a rock pays to see a flop with.
const rockBar = 9

func (rock) Act(raw module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	s, mn, ok := betting(raw, seat.PlayerID, offers)
	if !ok {
		return passOn(offers)
	}
	if s.Street == streetPreflop && chen(s.Seats[s.Current].Hole) < rockBar {
		// Fold, which the menu turns into a check when checking is free.
		return mn.action(choice{verb: VerbFold})
	}
	seat.Skill = module.SkillHard
	return bot{}.Act(raw, seat, offers)
}

// station checks or calls every bet, every street, with every hand. It never
// raises and never folds.
//
// The bot this module used to have (see bot.go), kept as an opponent because
// it is the one every value bet is for: a learner that bluffs it loses, and a
// learner that bets its good hands big wins.
type station struct{}

func (station) Act(raw module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	_, mn, ok := betting(raw, seat.PlayerID, offers)
	if !ok {
		return passOn(offers)
	}
	return mn.action(choice{verb: VerbCall})
}

// riverBluffer plays every street passively and then overbets the river with
// whatever it happens to be holding.
//
// It is the opponent "every bet is honest" cannot play against, and the reason
// it has to be written rather than assembled out of module.OfferBot is that an
// offer-reading bot always picks the *minimum* raise (module.defaultParam), and
// a minimum raise claims nothing. The whole question here is what a bet far
// larger than the pot means. TestHardPicksOffARiverOverbet measures the bot
// against it, and the reads (history.go) are expected to catch it: a little
// under half of its river bets are air, and every one that reaches a showdown
// says so.
type riverBluffer struct{}

func (riverBluffer) Act(raw module.State, _ module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	mn := menuOf(offers)
	s, err := decode(raw)
	if err == nil && s.Street == streetRiver && !s.Break.Open && mn.can(VerbRaise) {
		if lo, hi, ok := mn.bounds(); ok {
			want := 3 * potNow(s)
			if want < lo {
				want = lo
			}
			if want > hi {
				want = hi
			}
			return module.Action{
				OfferID: mn.byVerb[VerbRaise].ID,
				Verb:    VerbRaise,
				Params:  map[string]string{ParamAmount: strconv.Itoa(want)},
			}, true
		}
	}
	return module.ChooseAction(offers, []string{module.VerbContinue, VerbCheck, VerbCall, VerbFold})
}
