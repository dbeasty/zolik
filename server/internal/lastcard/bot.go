package lastcard

import (
	"sort"

	"zolik/server/internal/module"
)

// How a seat nobody is sitting at plays Last Card.
//
// The offer list says what is legal — the engine is the only authority on
// that — and this adds the judgements the list cannot carry: which card, and
// which colour a wild names. Prší's bot taught the shape of the answer: the
// cards that do something to somebody are worth most held until the player
// they land on is nearly out, and the wild is the one card that is never dead.
type bot struct {
	// fixed plays one profile whatever skill the seat asks for — a bench
	// style (learn.go), never a shipped setting.
	fixed *profile
}

var _ module.Bot = bot{}

func (b bot) Act(raw module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	s, err := decode(raw)
	if err != nil || s.Status != "active" {
		return module.ChooseAction(offers, nil)
	}
	if s.Intermission.Open {
		return module.ChooseAction(offers, []string{module.VerbContinue})
	}
	if s.Current != seat.PlayerID {
		return module.ChooseAction(offers, nil)
	}
	p := profileFor(seat.Skill)
	if b.fixed != nil {
		p = *b.fixed
	}

	// A Wild Draw Four waiting on this seat comes before anything else.
	if o := findOffer(offers, OfferChallenge); o != nil && o.Enabled {
		if p.challenges(s) {
			return module.Action{OfferID: o.ID, Verb: VerbChallenge}, true
		}
		if a := findOffer(offers, OfferAccept); a != nil && a.Enabled {
			return module.Action{OfferID: a.ID, Verb: VerbAccept}, true
		}
	}
	if o := findOffer(offers, OfferCatch); o != nil && o.Enabled && p.catches {
		return module.Action{OfferID: o.ID, Verb: VerbCatch}, true
	}
	if o := findOffer(offers, OfferCall); o != nil && o.Enabled && !p.forgets(s) {
		return module.Action{OfferID: o.ID, Verb: VerbCall}, true
	}

	play := findOffer(offers, OfferPlay)
	if play != nil && play.Enabled && play.Source != nil {
		if cards := b.honest(s, seat.PlayerID, p, play.Source.Cards); len(cards) > 0 {
			card := b.choose(s, seat.PlayerID, p, cards)
			a := module.Action{OfferID: play.ID, Verb: VerbPlay, Cards: []string{card}}
			if isWild(card) {
				a.Params = map[string]string{"colour": b.declare(s, seat.PlayerID, card)}
			}
			return a, true
		}
	}
	// Nothing worth playing: draw, or keep what was drawn.
	return module.ChooseAction(offers, []string{VerbDraw, VerbPass})
}

// honest takes the bluffs out of the legal plays: a Wild Draw Four played
// while holding the colour in play, which a table with the challenge allows.
// Only a profile that bluffs keeps one, and only when the next player is
// close enough to out that four cards is worth the risk of a challenge.
func (b bot) honest(s *GameState, playerID string, p profile, playable []string) []string {
	// Stacked on a draw card a Wild Draw Four is an answer, never a bluff.
	if s.PendingDraw > 0 || !s.holdsColour(s.Hands[playerID]) {
		return playable
	}
	if p.bluffs && nextHandSize(s, playerID) <= 2 {
		return playable
	}
	out := make([]string, 0, len(playable))
	for _, c := range playable {
		if c != cardWildDrawFour {
			out = append(out, c)
		}
	}
	return out
}

// nextHandSize is how many cards the next player in the direction of play is
// holding — their hand's size, never its contents.
func nextHandSize(s *GameState, playerID string) int {
	if len(s.inPlay()) < 2 {
		return 1 << 30
	}
	return len(s.Hands[s.nextPlayer(playerID)])
}

// profile is what a skill setting changes.
//
// Naming the colour the hand holds most of is in every profile: naming one
// at random is not a weaker player but a broken one.
//
// Each knob here was priced by a sweep (2 400 two-handed games against the
// profile below it, both seats; the standard error is about a point):
//
//	keepsTheWild  over spending wilds first        +5   (Medium over Easy)
//	attackFirst   over keepsTheWild alone          +7   two-handed, +1 at four
//
// Two more were tried and are not here, because each sounds right:
//
//	staysLong       prefer the play that leaves the hand long in the colour
//	                in play. -2 next to attackFirst; nothing on its own.
//	timesTheAttack  hold Skip, Reverse and Draw Two until the next player is
//	                down to two cards, as Prší's Hard does with its sevens.
//	                Worth 3 points less than spending them at once: two-handed
//	                every action card is a second turn, and a second turn now
//	                beats a sharper one later.
type profile struct {
	skill module.Skill

	// wildsFirst spends a wild as soon as one is playable — a beginner's
	// habit, and what makes Easy easy.
	wildsFirst bool
	// keepsTheWild holds the wilds back while any coloured card is playable,
	// and the Wild Draw Four behind the plain Wild.
	keepsTheWild bool
	// attackFirst plays a Skip, Reverse or Draw Two ahead of a plain card.
	attackFirst bool

	// dumpsWhenThreatened, where everyone pays for their own hand, plays the
	// costliest card first once another player is down to two cards — a wild
	// kept for later is fifty points if later never comes.
	dumpsWhenThreatened bool

	// swapsDown plays a 7 first, where sevens swap hands, when it would trade
	// for a hand at least two cards shorter.
	swapsDown bool

	// sometimesForgets leaves "Last card!" unsaid one time in three — a
	// beginner's slip, and a chance for the table to catch them.
	sometimesForgets bool
	// catches catches a player who went to one card in silence.
	catches bool
	// bluffs plays a Wild Draw Four while holding the colour, when the next
	// player is nearly out.
	bluffs bool
	// challengeAt challenges a Wild Draw Four played from a hand that had at
	// least this many cards left — the more cards, the likelier one was the
	// colour. Zero never challenges.
	challengeAt int
}

// forgets is whether this turn's "Last card!" goes unsaid. Fixed by the state
// rather than drawn at random, so a replayed match makes the same slip.
func (p profile) forgets(s *GameState) bool {
	return p.sometimesForgets && (s.Seed+int64(s.DealNumber)*31+int64(len(s.DiscardPile)))%3 == 0
}

// challenges decides on a Wild Draw Four from what the table can see: how
// many cards its player still holds.
func (p profile) challenges(s *GameState) bool {
	if p.challengeAt <= 0 || s.DrawFour == nil {
		return false
	}
	return len(s.Hands[s.DrawFour.Player]) >= p.challengeAt
}

var profiles = map[module.Skill]profile{
	module.SkillEasy: {
		skill:            module.SkillEasy,
		wildsFirst:       true,
		sometimesForgets: true,
	},
	module.SkillMedium: {
		skill:        module.SkillMedium,
		keepsTheWild: true,
		catches:      true,
		// Where everyone pays for their own hand: 38.4% against three-seat
		// Hard tables without it (1 800 games to 200, par 33.3%).
		dumpsWhenThreatened: true,
	},
	module.SkillHard: {
		skill:        module.SkillHard,
		keepsTheWild: true,
		attackFirst:  true,
		catches:      true,
		// Neither of these two measures against the bots below it — they
		// never bluff, so a challenge can only lose (4 000 games: 56.2% with
		// both, 56.5% without the challenge, 55.4% challenging from four
		// cards). They are here for the human across the table: a bot that
		// never challenges lets a player bluff for free, and one that never
		// bluffs is a bot whose Wild Draw Four is always honest. So the
		// challenge waits for a hand big enough that holding the colour was
		// near certain.
		bluffs:      true,
		challengeAt: 8,
		// Where sevens swap hands: 36.1% against three-seat Hard tables
		// without it (6 000 games, par 33.3%).
		swapsDown:           true,
		dumpsWhenThreatened: true,
	},
}

func profileFor(s module.Skill) profile {
	// This game ships no trained network, so an AI seat — from a table set
	// up before that choice was withdrawn — plays the strongest it has.
	if s == module.SkillAI {
		s = module.SkillHard
	}
	if p, ok := profiles[s]; ok {
		return p
	}
	return profiles[module.SkillMedium]
}

// candidate is one legal play, with what the choice is drawn on.
type candidate struct {
	card string
	// wild ranks 0 for a coloured card, 1 for a Wild, 2 for a Wild Draw Four:
	// the order to spend them in.
	wild int
	// attack is a Skip, Reverse or Draw Two.
	attack bool
	// swap is a 7 that would trade this hand for a much shorter one.
	swap bool
}

// choose picks which of the legal plays to make.
func (b bot) choose(s *GameState, playerID string, p profile, playable []string) string {
	if len(s.Hands[playerID]) == 1 || len(playable) == 1 {
		return playable[0]
	}
	shortest := 1 << 30
	if s.SevenZero {
		for _, q := range s.inPlay() {
			if q != playerID && len(s.Hands[q]) < shortest {
				shortest = len(s.Hands[q])
			}
		}
	}
	if p.dumpsWhenThreatened && s.Scoring == scoreLowest && s.TargetScore > 0 && someoneNearlyOut(s, playerID) {
		best := playable[0]
		for _, c := range playable {
			if cardPoints(c) > cardPoints(best) {
				best = c
			}
		}
		return best
	}
	cands := make([]candidate, 0, len(playable))
	for _, c := range playable {
		cd := candidate{card: c, attack: isAttack(c)}
		// After playing the 7 this hand is one card shorter; the trade is
		// worth making when the hand it gets back is shorter still.
		cd.swap = p.swapsDown && s.SevenZero && faceOf(c) == "7" && !isWild(c) &&
			shortest <= len(s.Hands[playerID])-3
		switch c {
		case cardWild:
			cd.wild = 1
		case cardWildDrawFour:
			cd.wild = 2
		}
		cands = append(cands, cd)
	}
	sort.SliceStable(cands, func(i, j int) bool { return betterPlay(cands[i], cands[j], p) })
	return cands[0].card
}

// betterPlay orders the legal plays, best first.
func betterPlay(x, y candidate, p profile) bool {
	if x.swap != y.swap {
		return x.swap
	}
	if p.wildsFirst && x.wild != y.wild {
		return x.wild > y.wild
	}
	if p.keepsTheWild && x.wild != y.wild {
		return x.wild < y.wild
	}
	if p.attackFirst && x.attack != y.attack {
		return x.attack
	}
	return x.card < y.card
}

func isAttack(card string) bool {
	switch faceOf(card) {
	case faceSkip, faceReverse, faceDrawTwo:
		return !isWild(card)
	}
	return false
}

func colourCounts(hand []string) map[string]int {
	out := map[string]int{}
	for _, c := range hand {
		if col := colourOf(c); col != "" {
			out[col]++
		}
	}
	return out
}

// declare names the colour a wild switches play to: whichever the rest of the
// hand holds most of. Ties go by the fixed colour order rather than map
// iteration, so a replayed match makes the same choice.
func (b bot) declare(s *GameState, playerID, playing string) string {
	return longestColour(removeCard(s.Hands[playerID], playing))
}

func findOffer(offers []module.ActionOffer, id string) *module.ActionOffer {
	for i := range offers {
		if offers[i].ID == id {
			return &offers[i]
		}
	}
	return nil
}

// someoneNearlyOut is whether another player holds two cards or fewer — a
// count every player can see.
func someoneNearlyOut(s *GameState, playerID string) bool {
	for _, p := range s.inPlay() {
		if p != playerID && len(s.Hands[p]) <= 2 {
			return true
		}
	}
	return false
}
