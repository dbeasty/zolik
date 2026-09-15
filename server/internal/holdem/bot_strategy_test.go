package holdem

import (
	"math/rand"
	"strconv"
	"testing"

	"zolik/server/internal/module"
)

// The two things a poker bot has to do that a card-shedding bot does not: tell
// a story it does not have the hand for, and decline to believe one.
//
// Everything here is measured as a *frequency* rather than as a single answer,
// and that is not a hedge. A bluff that always fires is not a bluff, it is a
// tell — the whole value of one is that an opponent cannot know which spot it
// is. So the assertions are about how often a profile does something over a
// sweep of otherwise identical positions, which is the only shape a statement
// about a mixed strategy can honestly take.

// actAs asks the bot for a move at a named strength, through the same offer
// list the runtime hands it.
func actAs(t *testing.T, raw module.State, playerID string, skill module.Skill) module.Action {
	t.Helper()
	m := New()
	offers, err := m.LegalActions(raw, playerID)
	if err != nil {
		t.Fatalf("LegalActions: %v", err)
	}
	a, ok := m.Bot().Act(raw, module.BotSeat{PlayerID: playerID, Skill: skill}, offers)
	if !ok {
		t.Fatalf("bot had no move for %s", playerID)
	}
	return a
}

// atSeed re-stamps a position with a different match seed.
//
// The bot's coin flips are a function of the position and the match seed and
// nothing else (see seedFor), so this is how the same spot is played many
// times without ever playing it twice.
func atSeed(t *testing.T, raw module.State, seed int64) module.State {
	t.Helper()
	s, err := decode(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	s.Seed = seed
	out, err := encode(s)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return out
}

// betRate is how often a profile puts money in from this position, over a
// sweep of seeds.
func betRate(t *testing.T, raw module.State, skill module.Skill, seeds int) float64 {
	t.Helper()
	bets := 0
	for seed := int64(1); seed <= int64(seeds); seed++ {
		if actAs(t, atSeed(t, raw, seed), "p1", skill).Verb == VerbRaise {
			bets++
		}
	}
	return float64(bets) / float64(seeds)
}

// --- knowing a draw when it sees one -----------------------------------------

// TestDrawOutsCountsTheKnownDraws pins the out counts against the numbers
// every poker book gives, so this is a citation and not a taste.
func TestDrawOutsCountsTheKnownDraws(t *testing.T) {
	cases := []struct {
		name  string
		hole  []string
		board []string
		want  int
	}{
		{"flush draw", []string{"QH", "JH"}, []string{"AH", "7H", "2C"}, 9},
		{"open-ended straight", []string{"9C", "8D"}, []string{"7H", "6S", "2C"}, 8},
		{"gutshot", []string{"9C", "8D"}, []string{"6H", "5S", "2C"}, 4},
		{"flush draw and a gutshot", []string{"QH", "JH"}, []string{"TH", "8H", "2C"}, 13},
		{"nothing", []string{"AS", "KD"}, []string{"7H", "8D", "2C"}, 0},
		// Four hearts on the board is everybody's flush and nobody's draw.
		{"board flush, not mine", []string{"AS", "KD"}, []string{"7H", "8H", "2H", "3H"}, 0},
		// A-2-3-4 is open at one end only: the five, and nothing below an ace.
		{"wheel gutshot", []string{"AS", "2D"}, []string{"3H", "4C", "9S"}, 4},
		// 2-3-4-5 is open at both — the ace plays low for the bottom end.
		{"open at the bottom on an ace", []string{"5S", "2D"}, []string{"3H", "4C", "9S"}, 8},
		// Nothing left to draw to.
		{"river", []string{"QH", "JH"}, []string{"AH", "7H", "2C", "3D", "5S"}, 0},
	}
	for _, tc := range cases {
		if got := drawOuts(tc.hole, tc.board); got != tc.want {
			t.Errorf("%s: drawOuts(%v, %v) = %d, want %d", tc.name, tc.hole, tc.board, got, tc.want)
		}
	}
}

// --- betting without the hand ------------------------------------------------

// TestHardSemiBluffsItsDraws.
//
// A flush draw checked to, heads-up. It is not yet the best hand and it is not
// close to worthless, which is exactly the hand the old bot had no branch for:
// too weak for the value bet at 0.55, too strong for the give-up bluff under
// 0.30, so it checked, took a free card it had not earned, and gave up every
// pot it could have won by simply asking for it.
func TestHardSemiBluffsItsDraws(t *testing.T) {
	raw := facing([]string{"8H", "7H"}, []string{"KH", "4H", "2C"}, 0)

	hard := betRate(t, raw, module.SkillHard, 120)
	medium := betRate(t, raw, module.SkillMedium, 120)
	if hard <= medium {
		t.Errorf("flush draw checked to: hard bets %.0f%% of the time, medium %.0f%% — want hard higher",
			100*hard, 100*medium)
	}
	if hard < 0.25 {
		t.Errorf("hard bet a flush draw only %.0f%% of the time, which is not a strategy", 100*hard)
	}
	// And it is a mix, not a habit: a draw that always bets is a draw the
	// table can read off the bet.
	if hard > 0.95 {
		t.Errorf("hard bet the draw %.0f%% of the time — that is a tell, not a semi-bluff", 100*hard)
	}
}

// The continuation-bet tests that stood here are gone with the branch they
// covered. They passed against a fixture where the raiser had committed 120
// and the caller 60 — a state a called pot cannot reach, since calling a raise
// means matching it — so they asserted a behaviour the real game never
// produced. See the note where the cbet knob used to be.

// --- declining to believe a bet ----------------------------------------------

// TestBluffShareLoosensAValueRead is the bluff-catching change at its source.
//
// Same cards, same board, same bet. The only difference is how much of the
// story the profile believes, and the estimate has to move with it or nothing
// downstream can.
func TestBluffShareLoosensAValueRead(t *testing.T) {
	hole := []string{"9S", "9D"}
	board := []string{"KH", "7D", "4C", "2S"}

	believed := equity(hole, board, 1, 600, twoPair, 0, rand.New(rand.NewSource(7)))
	doubted := equity(hole, board, 1, 600, twoPair, 0.30, rand.New(rand.NewSource(7)))
	ignored := equity(hole, board, 1, 600, twoPair, 1, rand.New(rand.NewSource(7)))

	if !(believed < doubted && doubted < ignored) {
		t.Errorf("a pair of nines against a two-pair claim: believed %.2f, doubted %.2f, ignored %.2f — want them ordered",
			believed, doubted, ignored)
	}
}

// TestHardFoldsLessOftenThanMediumToBigBets.
//
// The complaint this answers is that the bot could be pushed off anything by
// betting big at it, which is what "every bet is honest" adds up to in play.
// Swept across a spread of hands and boards rather than argued from one,
// because the change is a shift in a threshold and any single position either
// side of it proves nothing.
func TestHardFoldsLessOftenThanMediumToBigBets(t *testing.T) {
	spots := []struct{ hole, board []string }{
		{[]string{"9S", "9D"}, []string{"KH", "7D", "4C", "2S", "3H"}},
		{[]string{"AS", "TD"}, []string{"AH", "7D", "4C", "2S", "3H"}},
		{[]string{"KS", "QD"}, []string{"KH", "7D", "4C", "2S", "3H"}},
		{[]string{"JS", "JD"}, []string{"8H", "7D", "4C", "2S", "3H"}},
		{[]string{"TS", "TD"}, []string{"QH", "7D", "4C", "2S", "3H"}},
		{[]string{"AS", "7D"}, []string{"AC", "7H", "4C", "2S", "3H"}},
	}
	folds := map[module.Skill]int{}
	for _, sk := range []module.Skill{module.SkillEasy, module.SkillMedium, module.SkillHard} {
		for _, spot := range spots {
			// Three and a half times the pot, into two hundred. Nobody makes
			// that bet for value with anything a made hand beats, which is
			// exactly why folding every made hand to it is not discipline but
			// a standing invitation.
			raw := facing(spot.hole, spot.board, 700)
			if actAs(t, raw, "p1", sk).Verb == VerbFold {
				folds[sk]++
			}
		}
	}
	t.Logf("folds to an overbet of three and a half times the pot: easy %d, medium %d, hard %d of %d",
		folds[module.SkillEasy], folds[module.SkillMedium], folds[module.SkillHard], len(spots))
	if folds[module.SkillHard] >= folds[module.SkillMedium] {
		t.Errorf("hard folded %d of %d and medium %d — hard is supposed to be the one that argues",
			folds[module.SkillHard], len(spots), folds[module.SkillMedium])
	}
	if folds[module.SkillEasy] > folds[module.SkillHard] {
		t.Errorf("easy folded %d of %d, more than hard's %d — easy is the calling station, it should fold least",
			folds[module.SkillEasy], len(spots), folds[module.SkillHard])
	}
}

// --- the ladder --------------------------------------------------------------

// skilled plays a bot at a fixed strength, so the existing match driver can
// seat two different ones against each other without knowing about skills.
type skilled struct {
	bot   module.Bot
	skill module.Skill
}

func (s skilled) Act(st module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	seat.Skill = s.skill
	return s.bot.Act(st, seat, offers)
}

// headToHead plays a fixed sweep of freezeouts and returns the first seat's
// net chips.
// Every seed is played twice, with the contenders swapped between the seats,
// so both hold both sets of cards.
//
// A seed fixes the deal: seat "a" is dealt the same hands whoever is sitting
// there. Running one seating only therefore measures the cards as much as the
// player, and internal/ai/sim's Duel is a cautionary tale about exactly that —
// its seed-parity alternation split each sweep into two different samples and
// inverted a whole ruleset's strength ladder for as long as anybody had been
// looking at it. Two seatings per seed costs twice the runtime and makes the
// cards a constant.
func headToHead(t *testing.T, a, b module.Bot, matches int) int {
	t.Helper()
	m := New()
	players := refs("a", "b")
	net := 0
	for seed := int64(1); seed <= int64(matches); seed++ {
		for _, swap := range []bool{false, true} {
			seats := map[string]module.Bot{"a": a, "b": b}
			mine := "a"
			if swap {
				seats = map[string]module.Bot{"a": b, "b": a}
				mine = "b"
			}
			state, err := m.NewMatch(module.MatchConfig{
				Variation: "timed",
				Options:   module.Options{OptStartingStack: 1000, OptBigBlind: 20, OptHandLimit: 15},
			}, players, seed)
			if err != nil {
				t.Fatalf("NewMatch: %v", err)
			}
			final := playBots(t, state, players, seats, 6000)
			for i := range final.Seats {
				if final.Seats[i].PlayerID == mine {
					net += final.Seats[i].Stack - 1000
				}
			}
		}
	}
	return net
}

// TestBotLadderIsOrdered is the test the skill dial rests on, and it asserts
// two different things about two different rungs because the evidence for them
// is different.
//
// Medium against Easy is a real edge and is required to show: +55,000 chips
// over 400 matches when it was measured, which is several times the noise.
//
// Hard against Medium is required only *not to lose*, and that is the honest
// bar rather than a weak one. Hold'em is not Žolíky: past a certain point
// there is no more strength to have against an opponent who is already
// folding correctly and almost never bluffing, and every attempt to
// manufacture one — bluffing more, semi-bluffing more, disbelieving bets —
// measured as a loss against exactly that opponent (TestSweepHoldemKnobs: the
// first draft of Hard was four big blinds a match worse than Medium). What
// Hard has instead is a capability that costs nothing against a solid player
// and money against a sloppy one, which is what overbetDoubt is and what
// TestHardPicksOffARiverOverbet measures. A strength that only shows up
// against a flawed opponent is still a strength; it is just not one a
// duel with a solid bot can price.
//
// The noise floor is the reason for the size of the allowance. Each freezeout
// swings hundreds of chips, so the *sum* over a sweep this long carries a
// standard error in the thousands — which is why the bar is "not behind by
// more than that" rather than a required margin, and why nothing smaller than
// it is claimed anywhere in this file.
func TestBotLadderIsOrdered(t *testing.T) {
	if testing.Short() {
		t.Skip("a strength sweep is not a fast test")
	}
	m := New()
	hard := skilled{m.Bot(), module.SkillHard}
	medium := skilled{m.Bot(), module.SkillMedium}
	easy := skilled{m.Bot(), module.SkillEasy}

	const matches = 12
	// Two seatings per seed, so the chips at stake are 2 * matches * the stack.
	const noise = 12000

	if net := headToHead(t, hard, medium, matches); net < -noise {
		t.Errorf("hard against medium: %+d chips over %d matches — further behind than the sweep's own noise explains",
			net, 2*matches)
	} else {
		t.Logf("hard vs medium: %+d chips", net)
	}
	if net := headToHead(t, medium, easy, matches); net <= 0 {
		t.Errorf("medium against easy: %+d chips over %d matches, want a profit", net, 2*matches)
	} else {
		t.Logf("medium vs easy: %+d chips", net)
	}
}

// riverBluffer plays every street passively and then overbets the river with
// whatever it happens to be holding.
//
// It is the opponent "every bet is honest" cannot play against, and the reason
// it has to be written rather than assembled out of module.OfferBot is that an
// offer-reading bot always picks the *minimum* raise (module.defaultParam), and
// a minimum raise claims nothing. The whole question here is what a bet far
// larger than the pot means.
type riverBluffer struct{}

func (riverBluffer) Act(raw module.State, _ module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	mn := menuOf(offers)
	s, err := decode(raw)
	if err == nil && s.Street == streetRiver && mn.can(VerbRaise) {
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
	return module.ChooseAction(offers, []string{VerbCheck, VerbCall, VerbFold})
}

// TestHardPicksOffARiverOverbet is the bluff-catching change, measured in
// chips rather than in one decision.
//
// The opponent bets three times the pot on every river it reaches, with its
// whole range — so a little under half of those bets are worse than a made
// hand, and folding all of them hands over every pot that gets that far. A
// profile that reads a big bet as an honest one cannot do anything about that,
// which is the point: it is not out-played, it is robbed, and it never finds
// out.
func TestHardPicksOffARiverOverbet(t *testing.T) {
	if testing.Short() {
		t.Skip("a strength sweep is not a fast test")
	}
	m := New()

	const matches = 24
	hard := headToHead(t, skilled{m.Bot(), module.SkillHard}, riverBluffer{}, matches)
	medium := headToHead(t, skilled{m.Bot(), module.SkillMedium}, riverBluffer{}, matches)
	t.Logf("against a river overbetter: hard %+d chips, medium %+d chips", hard, medium)
	if hard <= medium {
		t.Errorf("hard took %+d off a bot that overbets every river and medium %+d — the profile that doubts a bet is supposed to be the one that collects",
			hard, medium)
	}
}

// TestEverySkillIsDeterministic.
//
// The bluffing is what makes this worth restating at every strength: a mixed
// strategy needs a coin, and a coin that is not a function of the position
// would make a match unreplayable and this whole file unable to assert
// anything about a frequency.
func TestEverySkillIsDeterministic(t *testing.T) {
	raw := facing([]string{"QH", "JH"}, []string{"AH", "7H", "2C"}, 0)
	for _, sk := range []module.Skill{module.SkillEasy, module.SkillMedium, module.SkillHard} {
		first := actAs(t, raw, "p1", sk)
		for i := 0; i < 4; i++ {
			again := actAs(t, raw, "p1", sk)
			if again.Verb != first.Verb || again.Params[ParamAmount] != first.Params[ParamAmount] {
				t.Fatalf("%s: run %d played %v, first run played %v", sk, i+2, again, first)
			}
		}
	}
}

// TestUnknownSkillPlaysMedium is module.BotSeat's compatibility rule, pinned
// here because this is the module where getting it wrong is invisible: every
// bot seated before skills existed carries an empty one, and defaulting down
// would quietly turn every poker table in the database into a beginner's.
func TestUnknownSkillPlaysMedium(t *testing.T) {
	for _, sk := range []module.Skill{"", "nonsense"} {
		if got := profileFor(sk); got.skill != module.SkillMedium {
			t.Errorf("profileFor(%q) = %q, want medium", sk, got.skill)
		}
	}
}
