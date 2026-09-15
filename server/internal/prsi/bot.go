package prsi

import (
	"sort"

	"zolik/server/internal/module"
)

// How a seat nobody is sitting at plays Prší.
//
// This module answered module.OfferBot(VerbPlay, VerbPass, VerbDraw), which is
// the right shape of answer for a game with three verbs and the wrong answer
// for this one, because the whole of Prší is in *which card*. An
// offer-preference bot sends the front of the offer's card list, the list is
// sorted, and card notation sorts the ranks in the order 7, 8, 9, T, J, Q, K,
// A — so the bot led with a seven whenever it held one and an ace as soon as
// it had no sevens left. Those are the two cards in the deck that do something
// to somebody. Played on the first turn they are worth nothing; held until the
// player to your left is one card from out, a seven is two cards back in their
// hand and an ace is a whole turn. The old bot spent them at the moment of
// least value, every time, and then had none left when they mattered.
//
// The queen is worse. It is this game's wild card — playable on anything, and
// it names the suit that follows — which makes it the one card that is never
// dead, and therefore the one card worth keeping until the hand has nothing
// else. And when the old bot did play one, the suit it named came from
// module.defaultParam, which takes the first declared choice: hearts. Always
// hearts, whatever was in its hand.
//
// So this is a module.Botted implementation. It reads the offer list for what
// is legal — the engine remains the only authority on that, and playableCards
// is already the engine's own answer — and adds the four judgements the list
// cannot carry: keep the wild, name a suit you can follow, and time the attack
// cards.
type bot struct{}

var _ module.Bot = bot{}

func (b bot) Act(raw module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	s, err := decode(raw)
	if err != nil {
		return module.ChooseAction(offers, nil)
	}
	if s.Status != "active" || s.Current != seat.PlayerID {
		return module.ChooseAction(offers, nil)
	}

	p := profileFor(seat.Skill)
	play := findOffer(offers, OfferPlay)
	if play != nil && play.Enabled && play.Source != nil && len(play.Source.Cards) > 0 {
		card := b.choose(s, seat.PlayerID, p, play.Source.Cards)
		a := module.Action{OfferID: play.ID, Verb: VerbPlay, Cards: []string{card}}
		if rankOf(card) == rankWild {
			a.Params = map[string]string{"suit": b.declare(s, seat.PlayerID)}
		}
		return a, true
	}
	// Nothing playable: take the cards, or take the skip. The offer list is
	// the whole answer and always was.
	return module.ChooseAction(offers, []string{VerbPass, VerbDraw})
}

// --- how well to play it -----------------------------------------------------

// profile is what a skill setting changes.
//
// Two things are deliberately not in it, because they are not strengths but
// the difference between a weak player and a broken one: every profile names
// the suit it is longest in when it plays a queen, and every profile answers a
// pending draw with a seven rather than picking up two cards it could have
// passed on.
type profile struct {
	skill module.Skill

	// keepsTheWild holds the queen back while any other card is playable. The
	// wild is the only card that is never dead, so it is worth more in hand
	// than any card it could be swapped for on the table.
	keepsTheWild bool
	// timesTheAttack holds a seven or an ace until the player it lands on is
	// close to going out. Below this they are shed like any other card, which
	// is what a beginner does and why a beginner never has one when it counts.
	timesTheAttack bool
	// attackAt is how few cards the next player must hold before an attack
	// card is worth spending. Zero means the default.
	attackAt int

	// Three more knobs stood here and are gone, each removed because a sweep
	// priced it at nothing or worse (TestSweepKnobs, 3000 games a row against
	// a profile with no knobs at all; the standard error is about a point).
	// They are recorded rather than quietly dropped, because every one of them
	// sounds obviously right:
	//
	//	staysLong     lead from the suit the hand holds most of, so the next
	//	              turn has a card that follows it. 48.9%.
	//	shedsShort    the opposite taste — play out of the shortest suit to
	//	              void it. 46.6%, the clearest loser of the three.
	//	pushesAttack  play a seven or an ace ahead of an ordinary card. 50.3%,
	//	              which is to say exactly nothing: an attack card is worth
	//	              about the same whenever it goes in, and all that matters
	//	              is not having spent it before it could matter.
	//
	// What survived is above: keeping the wild is worth about a point and a
	// half, and timing the attack about another one on top of it. Prší is a
	// high-variance game and there is not much more than that in it.
}

var profiles = map[module.Skill]profile{
	module.SkillEasy: {
		skill: module.SkillEasy,
	},
	module.SkillMedium: {
		skill:        module.SkillMedium,
		keepsTheWild: true,
	},
	module.SkillHard: {
		skill:          module.SkillHard,
		keepsTheWild:   true,
		timesTheAttack: true,
		// Two, not three. Both were swept; holding the seven until the
		// opponent is down to two cards measured better than letting it go a
		// turn earlier, which is the opposite of what the first draft assumed
		// and is why the number is a field rather than a literal.
		attackAt: 2,
	},
}

// attackHandSize is the default for profile.attackAt: a player holding this
// many cards or fewer is close enough to out that two cards back, or a turn
// taken away, changes who wins.
const attackHandSize = 2

func profileFor(s module.Skill) profile {
	if p, ok := profiles[s]; ok {
		return p
	}
	return profiles[module.SkillMedium]
}

// --- choosing a card ---------------------------------------------------------

// choose picks which of the legal plays to make.
func (b bot) choose(s *GameState, playerID string, p profile, playable []string) string {
	hand := s.Hands[playerID]
	// The last card wins the game. Nothing below is worth thinking about.
	if len(hand) == 1 {
		return playable[0]
	}
	pressure := nextHandSize(s, playerID)
	at := p.attackAt
	if at <= 0 {
		at = attackHandSize
	}

	cands := make([]candidate, 0, len(playable))
	for _, c := range playable {
		cands = append(cands, candidate{
			card: c,
			wild: rankOf(c) == rankWild,
			// An attack card is being saved unless it is about to be worth
			// something: the player it lands on is nearly out, or there is an
			// obligation on the table that only this card answers.
			hoarded: p.timesTheAttack && isAttack(c) && pressure > at &&
				s.PendingDraw == 0 && !s.SkipPending,
		})
	}
	sort.SliceStable(cands, func(i, j int) bool { return betterPlay(cands[i], cands[j], p) })
	return cands[0].card
}

// candidate is one legal play, with what the choice is drawn on.
type candidate struct {
	card string
	// wild is the queen: kept back because it can be played on anything, which
	// makes it the hand's answer to a suit it cannot otherwise follow.
	wild bool
	// hoarded is an attack card being saved for a moment that is worth it.
	hoarded bool
}

// betterPlay orders the legal plays, best first.
func betterPlay(x, y candidate, p profile) bool {
	if p.keepsTheWild && x.wild != y.wild {
		return !x.wild
	}
	if x.hoarded != y.hoarded {
		return !x.hoarded
	}
	return x.card < y.card
}

func isAttack(card string) bool {
	r := rankOf(card)
	return r == rankDrawTwo || r == rankSkip
}

// nextHandSize is how many cards the player to this one's left is holding.
//
// Their hand *size*, never its contents: a count is what every client already
// draws, and reading anything more would make this a cheat rather than an
// opponent.
func nextHandSize(s *GameState, playerID string) int {
	n := len(s.TurnOrder)
	if n < 2 {
		return 1 << 30
	}
	for i, id := range s.TurnOrder {
		if id != playerID {
			continue
		}
		return len(s.Hands[s.TurnOrder[(i+1)%n]])
	}
	return 1 << 30
}

// declare names the suit a queen switches play to: whichever the hand holds
// most of, so the next turn has something to follow it with.
//
// Ties broken by the fixed suit order rather than by map iteration, because
// Go randomises the latter and the same hand would otherwise name a different
// suit on a re-read — which would make a replayed match diverge.
func (b bot) declare(s *GameState, playerID string) string {
	counts := map[string]int{}
	for _, c := range s.Hands[playerID] {
		if rankOf(c) != rankWild {
			counts[suitOf(c)]++
		}
	}
	best, bestN := suits[0], -1
	for _, suit := range suits {
		if counts[suit] > bestN {
			best, bestN = suit, counts[suit]
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
