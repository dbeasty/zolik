package ai

import (
	"zolik/server/internal/module"
)

// What separates a rookie from a shark.
//
// Before this file there was a `difficulty string` threaded into exactly two
// places in pickSmartDiscard, and one of those two signals could not fire at
// all in production because the adapter handed the agent an empty discard
// history. So there was, in practice, one bot. This is the dial.
//
// Every knob is named once, here, and read by name at the point it applies.
// The rule the file exists to enforce is that there are no `if difficulty ==
// "hard"` comparisons anywhere else: a strength is a set of capabilities, and
// adding a fifth level should be a row in a table rather than a search through
// the agent for string literals.
//
// The knobs fall into three groups, and they are genuinely different kinds of
// thing:
//
//	Perception   what the agent is allowed to notice. A weak bot is not one
//	             that reasons badly, it is one that does not look.
//	Planning     how far ahead it works, and how much of the hand it is
//	             willing to commit.
//	Fallibility  how often it does the second-best thing on purpose.
//
// Fallibility is last and smallest for a reason. Making a bot weak by making
// it random is the cheap way and it reads as broken rather than as beatable —
// so most of the distance between Easy and Expert is perception and planning,
// and the dice only break the remaining tie.
type Profile struct {
	Skill module.Skill

	// --- Perception ---

	// ReadTableDanger avoids discarding a card that extends a meld already on
	// the table. The single biggest source of "the AI just fed me my run".
	ReadTableDanger bool
	// Recall is how much of the deal's history the agent remembers, in laps
	// of the table. 0 remembers nothing; RecallPerfect remembers everything
	// back to the deal.
	//
	// Memory depth is the main axis of strength here, which was not obvious
	// until the ledger existed: nearly everything a strong rummy player knows
	// that a weak one does not — which ranks are dead, which of my partials
	// still have outs, what that opponent picked up and therefore wants — is
	// downstream of remembering what has been shown.
	Recall int
	// ReadHandCounts notices that an opponent is close to going out, and
	// switches from building to dumping penalty points when they are.
	//
	// The one perception knob that measured unambiguously worth its code:
	// taking it away from Hard cost eight wins in two hundred three-handed
	// deals. It is also the knob that does nothing on its own — it can only
	// tell the discard to stop protecting material, so a profile that
	// protects none of it (Medium) plays identically with and without.
	ReadHandCounts bool
	// EndgameAt is how few cards an opponent must hold before that switch
	// happens. Zero means the default of two.
	//
	// Two is the last moment it can possibly matter: a seat with two cards
	// goes out next turn on a lay-off and a discard. Three is a turn earlier
	// — a guess, sometimes a wasted one, and the difference between reacting
	// to the endgame and seeing it coming.
	EndgameAt int
	// EndgameDumpsUnsafe stops protecting the table once the endgame starts:
	// feeding an opponent's meld costs nothing if the deal is ending before
	// they can use it, and the ten points in hand are about to be scored.
	EndgameDumpsUnsafe bool
	// ReadPickups tracks which cards an opponent took off the discard pile
	// and therefore demonstrably wants. Needs Recall > 0 to mean anything.
	ReadPickups bool

	// --- Planning ---

	// MeldDither is the chance of putting off going down for a turn when the
	// contract could be completed now.
	//
	// This is the *only* safe way to make melding weaker, and the first
	// attempt got it wrong in a way worth recording. Taking the contract
	// search away from the weak profile — "a beginner lays the first meld
	// they see" — produced a bot that could not go down at all under any
	// ruleset with a point floor or a per-type quota, because no single meld
	// satisfies either. A table of them never finished a deal. That is not a
	// weak opponent, it is a broken game, and the self-play gate caught it on
	// the first run.
	//
	// So every strength plans the contract, and weakness is dithering about
	// when to use the plan: a real beginner sits on a hand they could come
	// down with, waiting for something better. It costs tempo, which is the
	// right price, and it cannot strand a turn — see the guard in
	// ChooseAction, which never dithers once cards are already on the table.
	MeldDither float64
	// KeepPartials decides how much unfinished material the discard is
	// willing to protect. See keepValue.
	KeepPartials KeepPolicy
	// LayOffPolicy decides *which* lay-off to make when several are legal.
	LayOffPolicy LayOffPolicy
	// Four more knobs stood here and are gone, each removed because the
	// sweep in internal/ai/sim priced it at nothing or worse. They are
	// recorded rather than quietly dropped, because every one of them sounds
	// obviously right and the next person to have the idea deserves the
	// numbers:
	//
	//	DrawSpeculative  take the top discard when it improves the hand's
	//	                 shape, not only when it lands somewhere. Lost 141 of
	//	                 200 deals and sixty penalty points a match. Taking a
	//	                 card commits you to it for the turn, tells the table
	//	                 what you are building, and buys a maybe from a pile
	//	                 an opponent has already judged worthless.
	//	KeepWeight       price unfinished material in penalty points instead
	//	                 of ranking it above them. Helped on top of Medium,
	//	                 cost 14 of 200 on top of Hard. Not robust, so not
	//	                 shipped.
	//	PlanGoOut        search for a sequence of lay-offs that empties the
	//	                 hand this turn. Measured *exactly* neutral, twice, at
	//	                 two seats and at three: shedding the most expensive
	//	                 card repeatedly already finds the same move nearly
	//	                 every time.
	//	JokerEconomy     prefer a lay-off that buys a joker back off the
	//	                 table. Neutral at both table sizes.
	//
	// The lesson worth keeping is that only two of the eight ideas in the
	// original plan survived measurement, and neither was the clever one.

	// --- Fallibility ---

	// BlunderRate is the chance per discard of naming the second-best card
	// instead of the best. Never an illegal card and never one that strands
	// the turn — see blunder in heuristic.go.
	BlunderRate float64
	// MissRate is the chance per turn of failing to notice an available
	// lay-off. Models the real beginner failure: not bad judgement, but not
	// seeing that the 7 in hand goes on the run at the other end of the
	// table.
	MissRate float64
}

// RecallPerfect is a Recall depth that never forgets. Expressed as a lap count
// larger than any deal can reach so that Recall stays one comparable number
// rather than a depth plus a flag.
const RecallPerfect = 1 << 30

// KeepPolicy is how much unfinished material a discard protects.
type KeepPolicy int

const (
	// KeepFinished protects only cards in a complete, layable meld — the
	// original behaviour. A pair of kings is two loose ten-point cards.
	KeepFinished KeepPolicy = iota
	// KeepFragments also protects pairs and part-runs, flat.
	KeepFragments
	// KeepByOuts protects fragments in proportion to how many cards that
	// would complete them are still unaccounted for. Needs the ledger: a
	// pair of kings with the other six kings already shown is not a pair, it
	// is twenty penalty points.
	KeepByOuts
)

// LayOffPolicy is how a lay-off is chosen when several are legal.
type LayOffPolicy int

const (
	// LayOffFirstFit takes the first legal lay-off in sorted-owner order.
	// Deterministic, which is why it was written, and arbitrary.
	LayOffFirstFit LayOffPolicy = iota
	// LayOffHighestPoints sheds the most expensive card that fits.
	LayOffHighestPoints
)

// profiles is the whole strength ladder, one row per skill.
//
// Medium is deliberately the agent exactly as it played before any of this
// existed. That is what makes the rest measurable: every other level is
// defined by how it does against a fixed reference, and the reference is the
// bot people have been playing against.
var profiles = map[module.Skill]Profile{
	module.SkillEasy: {
		Skill:           module.SkillEasy,
		ReadTableDanger: false,
		Recall:          0,
		MeldDither:      0.25,
		KeepPartials:    KeepFinished,
		LayOffPolicy:    LayOffFirstFit,
		BlunderRate:     0.12,
		MissRate:        0.25,
	},
	module.SkillMedium: {
		Skill:           module.SkillMedium,
		ReadTableDanger: true,
		Recall:          0,
		MeldDither:      0.05,
		KeepPartials:    KeepFinished,
		LayOffPolicy:    LayOffFirstFit,
		BlunderRate:     0.03,
		MissRate:        0.05,
	},
	module.SkillHard: {
		Skill:           module.SkillHard,
		ReadTableDanger: true,
		// Hard is the one that reads the table. It remembers the whole
		// deal's discards, tracks what every seat was seen taking off the
		// pile, counts what is still unaccounted for against its own
		// unfinished material, and watches how close anyone is to going out.
		//
		// That set is what a fourth level was going to be. It is here because
		// the sweep put it here: each piece measures somewhere between
		// neutral and clearly positive on top of Medium, and nothing built on
		// top of *it* measured positive at all.
		Recall:         RecallPerfect,
		ReadHandCounts: true,
		ReadPickups:    true,
		KeepPartials:   KeepByOuts,
		LayOffPolicy:   LayOffHighestPoints,
	},
}

// ProfileFor is the strength a skill plays at.
//
// An unknown or empty skill is Medium, not Easy. That is the compatibility
// rule the whole change rests on: every bot seated before skills existed has
// an empty AIDifficulty, and every one of them was playing the agent that is
// now called Medium. Defaulting down would silently weaken every table in the
// database.
func ProfileFor(s module.Skill) Profile {
	if p, ok := profiles[s]; ok {
		return p
	}
	return profiles[module.SkillMedium]
}
