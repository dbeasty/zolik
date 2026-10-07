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
type bot struct{}

var _ module.Bot = bot{}

func (b bot) Act(raw module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	s, err := decode(raw)
	if err != nil || s.Status != "active" || s.Current != seat.PlayerID {
		return module.ChooseAction(offers, nil)
	}
	p := profileFor(seat.Skill)
	play := findOffer(offers, OfferPlay)
	if play != nil && play.Enabled && play.Source != nil && len(play.Source.Cards) > 0 {
		card := b.choose(s, seat.PlayerID, p, play.Source.Cards)
		a := module.Action{OfferID: play.ID, Verb: VerbPlay, Cards: []string{card}}
		if isWild(card) {
			a.Params = map[string]string{"colour": b.declare(s, seat.PlayerID, card)}
		}
		return a, true
	}
	// Nothing playable: draw, or keep what was drawn.
	return module.ChooseAction(offers, []string{VerbDraw, VerbPass})
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
}

var profiles = map[module.Skill]profile{
	module.SkillEasy: {
		skill:      module.SkillEasy,
		wildsFirst: true,
	},
	module.SkillMedium: {
		skill:        module.SkillMedium,
		keepsTheWild: true,
	},
	module.SkillHard: {
		skill:        module.SkillHard,
		keepsTheWild: true,
		attackFirst:  true,
	},
}

func profileFor(s module.Skill) profile {
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
}

// choose picks which of the legal plays to make.
func (b bot) choose(s *GameState, playerID string, p profile, playable []string) string {
	if len(s.Hands[playerID]) == 1 || len(playable) == 1 {
		return playable[0]
	}
	cands := make([]candidate, 0, len(playable))
	for _, c := range playable {
		cd := candidate{card: c, attack: isAttack(c)}
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
	counts := colourCounts(removeCard(s.Hands[playerID], playing))
	best, bestN := colours[0], -1
	for _, c := range colours {
		if counts[c] > bestN {
			best, bestN = c, counts[c]
		}
	}
	return best
}

func findOffer(offers []module.ActionOffer, id string) *module.ActionOffer {
	for i := range offers {
		if offers[i].ID == id {
			return &offers[i]
		}
	}
	return nil
}
