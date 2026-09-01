package ginrummy

import (
	"strconv"
	"strings"

	"zolik/server/internal/module"
)

// bot is Gin Rummy's own player.
//
// module.OfferBot cannot play this game at all: every legal discard is
// offered and which one to throw is the entire game (§2.6 of the plan this
// module implements), so an offer-preference bot would throw whichever card
// sorts first. This one reads the same offers a client would, plus the
// Facts they already carry (a knock offer's resulting deadwood), and adds
// nothing the engine has not already computed and offered.
type bot struct{}

var _ module.Bot = bot{}

func (b bot) Act(raw module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	s, err := decode(raw)
	if err != nil {
		return module.Action{}, false
	}
	skill := seat.Skill
	if skill == "" {
		skill = module.SkillMedium
	}

	switch s.Phase {
	case phaseUpcardNonDealer, phaseUpcardDealer:
		return chooseUpcard(s, seat.PlayerID, offers)
	case phaseDraw:
		return chooseDraw(s, seat.PlayerID, offers)
	case phaseDiscard:
		return chooseDiscardOrKnock(s, seat.PlayerID, skill, offers)
	case phaseLayoff:
		return chooseLayoff(offers)
	default:
		return module.ChooseAction(offers, nil)
	}
}

// chooseUpcard takes the upcard when adding it beats the hand's own best
// achievable deadwood, and passes otherwise — a stock draw is a gamble on an
// unseen card, so it only has to be no worse than a known one.
func chooseUpcard(s *GameState, playerID string, offers []module.ActionOffer) (module.Action, bool) {
	hand := s.Hands[playerID]
	take := findOffer(offers, OfferDrawDiscard)
	if take != nil && take.Enabled && len(s.DiscardPile) > 0 {
		up := s.DiscardPile[len(s.DiscardPile)-1]
		withCard := append(append([]string(nil), hand...), up)
		if bestDiscardDeadwood(withCard) < bestDiscardDeadwood(hand) {
			return module.SubmissionFor(*take)
		}
	}
	if pass := findOffer(offers, OfferPassUpcard); pass != nil && pass.Enabled {
		return module.SubmissionFor(*pass)
	}
	if take != nil && take.Enabled {
		return module.SubmissionFor(*take)
	}
	return module.ChooseAction(offers, nil)
}

// chooseDraw is the same comparison, once normal play has begun: the stock is
// unknown, so the discard pile's top card only has to beat what is already in
// hand.
func chooseDraw(s *GameState, playerID string, offers []module.ActionOffer) (module.Action, bool) {
	hand := s.Hands[playerID]
	discardOffer := findOffer(offers, OfferDrawDiscard)
	if discardOffer != nil && discardOffer.Enabled && len(s.DiscardPile) > 0 {
		top := s.DiscardPile[len(s.DiscardPile)-1]
		withCard := append(append([]string(nil), hand...), top)
		if bestDiscardDeadwood(withCard) < bestDiscardDeadwood(hand) {
			return module.SubmissionFor(*discardOffer)
		}
	}
	if stockOffer := findOffer(offers, OfferDrawStock); stockOffer != nil && stockOffer.Enabled {
		return module.SubmissionFor(*stockOffer)
	}
	if discardOffer != nil && discardOffer.Enabled {
		return module.SubmissionFor(*discardOffer)
	}
	return module.ChooseAction(offers, nil)
}

// chooseDiscardOrKnock takes gin or the lowest-deadwood knock the moment
// either is offered.
//
// The plan's first draft of this had a harder skill hold out for a bigger
// margin before knocking. Self-play (TestBot_StrengthRisesMonotonicallyAlong-
// Skill) found that backfires: in a two-player race, whoever ends a hand
// first keeps the tempo, so patience mostly hands the easier skill more turns
// to reach its own knock — an expert waiting for gin loses to a novice who
// knocked three turns earlier almost every time. Every skill knocks the
// moment it may, and the skill dial moves to the one decision that does not
// have that failure mode: which card to discard when it cannot yet knock.
func chooseDiscardOrKnock(s *GameState, playerID string, skill module.Skill, offers []module.ActionOffer) (module.Action, bool) {
	var bestKnock *module.ActionOffer
	bestDeadwood := 1 << 30
	for i := range offers {
		o := &offers[i]
		if !o.Enabled {
			continue
		}
		if o.ID == OfferBigGin || strings.HasPrefix(o.ID, "gin:") {
			return module.SubmissionFor(*o)
		}
		if strings.HasPrefix(o.ID, "knock:") {
			if dw := factInt(o.Facts, "ginrummy.fact.deadwood"); dw < bestDeadwood {
				bestDeadwood, bestKnock = dw, o
			}
		}
	}
	if bestKnock != nil {
		return module.SubmissionFor(*bestKnock)
	}

	if discard := findOffer(offers, OfferDiscard); discard != nil && discard.Enabled && len(discard.Source.Cards) > 0 {
		card := discardCard(s, playerID, skill)
		return module.Action{OfferID: discard.ID, Verb: discard.Verb, Cards: []string{card}}, true
	}
	return module.ChooseAction(offers, []string{VerbDiscard})
}

// discardCard picks the card to throw when no knock is available yet.
//
// Every skill minimises its own resulting deadwood first — there is no
// reason ever to prefer worse. Medium and hard skills break a near-tie (hard
// tolerating up to one extra point) in favor of the card the opponent has
// shown the least interest in — the danger score below — per §2.6 of the
// plan: "subtract a danger score for cards the opponent has shown interest
// in... from the module's own discard log." Easy ignores danger entirely,
// which is what makes it the one worth beating.
func discardCard(s *GameState, playerID string, skill module.Skill) string {
	hand := s.Hands[playerID]
	interest := s.Interest[other(s.Players, playerID)]

	type candidate struct {
		card     string
		deadwood int
		danger   int
	}
	cands := make([]candidate, 0, len(hand))
	seen := map[string]bool{}
	for _, c := range hand {
		if seen[c] {
			continue
		}
		seen[c] = true
		dw, _ := Deadwood(removeCard(hand, c))
		cands = append(cands, candidate{card: c, deadwood: dw, danger: dangerScore(c, interest)})
	}

	if skill == module.SkillEasy {
		best := cands[0]
		for _, c := range cands[1:] {
			if c.deadwood < best.deadwood {
				best = c
			}
		}
		return best.card
	}

	tolerance := 0
	if skill == module.SkillHard {
		tolerance = 1
	}
	minDW := cands[0].deadwood
	for _, c := range cands[1:] {
		if c.deadwood < minDW {
			minDW = c.deadwood
		}
	}
	within := func(c candidate) bool { return c.deadwood <= minDW+tolerance }

	best := cands[0]
	for _, c := range cands[1:] {
		switch {
		case within(c) && !within(best):
			best = c
		case within(c) && within(best):
			if c.danger < best.danger || (c.danger == best.danger && c.deadwood < best.deadwood) {
				best = c
			}
		case !within(c) && !within(best) && c.deadwood < best.deadwood:
			best = c
		}
	}
	return best.card
}

// dangerScore is how much a card would help the opponent, read from what they
// have already shown interest in this hand: a full point for a card of a rank
// they took (it could complete a set), a smaller one for a card within two
// ranks of one they took in the same suit (it could complete or extend a
// run).
func dangerScore(card string, interest []string) int {
	score := 0
	for _, shown := range interest {
		if rankOf(shown) == rankOf(card) {
			score += 2
			continue
		}
		if suitOf(shown) == suitOf(card) {
			d := rankIndex[shown[0]] - rankIndex[card[0]]
			if d < 0 {
				d = -d
			}
			if d <= 2 {
				score++
			}
		}
	}
	return score
}

// chooseLayoff always lays off when it can — there is no bluff to protect
// once the hand has been laid on the table — and finishes otherwise.
func chooseLayoff(offers []module.ActionOffer) (module.Action, bool) {
	for _, o := range offers {
		if strings.HasPrefix(o.ID, "lay_off:") && o.Enabled {
			return module.SubmissionFor(o)
		}
	}
	if finish := findOffer(offers, OfferFinishLayoff); finish != nil && finish.Enabled {
		return module.SubmissionFor(*finish)
	}
	return module.ChooseAction(offers, nil)
}

// bestDiscardDeadwood is the lowest deadwood achievable by discarding one
// card from hand — the same figure a knock offer's Fact carries, computed
// here for a hand shape no offer describes yet (an unseen draw candidate).
func bestDiscardDeadwood(hand []string) int {
	best := -1
	seen := map[string]bool{}
	for _, c := range hand {
		if seen[c] {
			continue
		}
		seen[c] = true
		if dw, _ := Deadwood(removeCard(hand, c)); best < 0 || dw < best {
			best = dw
		}
	}
	if best < 0 {
		dw, _ := Deadwood(hand)
		return dw
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

func factInt(facts []module.Fact, labelKey string) int {
	for _, f := range facts {
		if f.LabelKey == labelKey {
			n, _ := strconv.Atoi(f.Value)
			return n
		}
	}
	return 1 << 30
}
