package pinochle

import "zolik/server/internal/module"

var (
	_ module.RulesProvider     = (*Module)(nil)
	_ module.RuleIndexProvider = (*Module)(nil)
)

// Rules writes out the table being played: its variation and every option
// that changes a sentence. Each sentence is its own key, chosen by a branch
// here, never assembled from parts.
func (m *Module) Rules(mc module.MatchConfig) ([]module.RuleSection, error) {
	c := resolve(mc)
	t := tableOf(c.variation)
	two := c.variation == VarTwoHanded
	var secs []module.RuleSection

	// --- the goal ---
	goal := []module.RuleItem{}
	switch c.variation {
	case VarCutthroat:
		goal = append(goal, module.Rule("pinochle.rules.goal.cutthroat", nil))
	case VarTwoHanded:
		goal = append(goal, module.Rule("pinochle.rules.goal.twohanded", nil))
	case VarDoubleDeck:
		goal = append(goal, module.Rule("pinochle.rules.goal.doubledeck", nil))
	default:
		goal = append(goal, module.Rule("pinochle.rules.goal.partnership", nil))
	}
	goal = append(goal, module.Rule("pinochle.rules.target", map[string]any{"n": c.target}))
	secs = append(secs, module.SectionOf("pinochle.rules.section.goal", goal))

	// --- the cards ---
	cards := []module.RuleItem{}
	if t.copies == 4 {
		cards = append(cards, module.Rule("pinochle.rules.deck.double", nil))
		cards = append(cards, module.Rule("pinochle.rules.order.double", nil))
	} else {
		cards = append(cards, module.Rule("pinochle.rules.deck.single", nil))
		cards = append(cards, module.Rule("pinochle.rules.order.single", nil))
	}
	if c.scoring == ScoringClassic {
		cards = append(cards, module.Rule("pinochle.rules.points.classic", nil))
	} else {
		cards = append(cards, module.Rule("pinochle.rules.points.counters", nil))
	}
	cards = append(cards, module.Rule("pinochle.rules.lastTrick", map[string]any{"n": t.lastTrick}))
	cards = append(cards, module.Rule("pinochle.rules.handTotal", map[string]any{"n": t.handTotal(c.scoring)}))
	secs = append(secs, module.SectionOf("pinochle.rules.section.cards", cards))

	// --- the deal ---
	deal := []module.RuleItem{module.Rule("pinochle.rules.deal", map[string]any{"n": t.handSize})}
	switch {
	case t.partners:
		deal = append(deal, module.Rule("pinochle.rules.partners", nil))
	case t.widow > 0:
		deal = append(deal, module.Rule("pinochle.rules.widow", map[string]any{"n": t.widow}))
	case two:
		deal = append(deal, module.Rule("pinochle.rules.turnUp", nil))
		deal = append(deal, module.Rule("pinochle.rules.turnUpDix", map[string]any{"n": pointsDix}))
	}
	secs = append(secs, module.SectionOf("pinochle.rules.section.deal", deal))

	// --- the auction ---
	if t.auction {
		bid := []module.RuleItem{
			module.Rule("pinochle.rules.auction", map[string]any{"n": c.minBid}),
			module.Rule("pinochle.rules.bidStep", map[string]any{"n": bidStep, "max": maxBidTop}),
			module.Rule("pinochle.rules.passFinal", nil),
		}
		if c.allPass == AllPassRedeal {
			bid = append(bid, module.Rule("pinochle.rules.allPass.redeal", nil))
		} else {
			bid = append(bid, module.Rule("pinochle.rules.allPass.stuck", map[string]any{"n": c.minBid}))
		}
		bid = append(bid, module.Rule("pinochle.rules.trump", nil))
		if t.widow > 0 {
			bid = append(bid, module.Rule("pinochle.rules.takeWidow", nil))
			bid = append(bid, module.Rule("pinochle.rules.bury", map[string]any{"n": t.widow}))
		}
		if c.pass > 0 {
			bid = append(bid, module.Rule("pinochle.rules.pass", map[string]any{"n": c.pass}))
		}
		secs = append(secs, module.SectionOf("pinochle.rules.section.auction", bid))
	}

	// --- meld ---
	meld := []module.RuleItem{
		module.Rule("pinochle.rules.meld.run", map[string]any{"n": pointsRun}),
		module.Rule("pinochle.rules.meld.royalMarriage", map[string]any{"n": pointsRoyalMarriage}),
		module.Rule("pinochle.rules.meld.marriage", map[string]any{"n": pointsMarriage}),
	}
	if t.hasNines() {
		meld = append(meld, module.Rule("pinochle.rules.meld.dix", map[string]any{"n": pointsDix}))
	}
	meld = append(meld,
		module.Rule("pinochle.rules.meld.arounds", map[string]any{
			"aces": aroundPoints['A'][1], "kings": aroundPoints['K'][1], "queens": aroundPoints['Q'][1], "jacks": aroundPoints['J'][1],
		}),
		module.Rule("pinochle.rules.meld.pinochle", map[string]any{"n": pinochlePoints[1]}),
		module.Rule("pinochle.rules.meld.doubles", map[string]any{
			"run": pointsDoubleRun, "aces": aroundPoints['A'][2], "kings": aroundPoints['K'][2],
			"queens": aroundPoints['Q'][2], "jacks": aroundPoints['J'][2], "pinochle": pinochlePoints[2],
		}),
	)
	if t.copies == 4 {
		meld = append(meld, module.Rule("pinochle.rules.meld.triples", map[string]any{
			"aces": aroundPoints['A'][3], "kings": aroundPoints['K'][3], "queens": aroundPoints['Q'][3], "jacks": aroundPoints['J'][3],
			"pinochle": pinochlePoints[3],
		}))
		meld = append(meld, module.Rule("pinochle.rules.meld.quadruples", map[string]any{
			"aces": aroundPoints['A'][4], "kings": aroundPoints['K'][4], "queens": aroundPoints['Q'][4], "jacks": aroundPoints['J'][4],
			"pinochle": pinochlePoints[4],
		}))
	}
	meld = append(meld, module.Rule("pinochle.rules.meld.classes", nil))
	if two {
		meld = append(meld,
			module.Rule("pinochle.rules.meld.onWin", nil),
			module.Rule("pinochle.rules.meld.onePerTurn", nil),
			module.Rule("pinochle.rules.meld.exchange", map[string]any{"n": pointsDix}),
			module.Rule("pinochle.rules.meld.staysInHand", nil),
		)
	} else {
		meld = append(meld, module.Rule("pinochle.rules.meld.shown", nil))
		if c.meldNeeds == MeldNeedsCounter {
			meld = append(meld, module.Rule("pinochle.rules.meld.needsCounter", nil))
		} else {
			meld = append(meld, module.Rule("pinochle.rules.meld.needsTrick", nil))
		}
	}
	secs = append(secs, module.SectionOf("pinochle.rules.section.meld", meld))

	// --- play ---
	play := []module.RuleItem{}
	if two {
		play = append(play,
			module.Rule("pinochle.rules.lead.twohanded", nil),
			module.Rule("pinochle.rules.turns", nil),
			module.Rule("pinochle.rules.stockFree", nil),
			module.Rule("pinochle.rules.draw", nil),
			module.Rule("pinochle.rules.late", nil),
		)
	} else {
		play = append(play,
			module.Rule("pinochle.rules.lead.bidder", nil),
			module.Rule("pinochle.rules.turns", nil),
		)
	}
	play = append(play,
		module.Rule("pinochle.rules.follow", nil),
		module.Rule("pinochle.rules.beat", nil),
		module.Rule("pinochle.rules.trumpIn", nil),
		module.Rule("pinochle.rules.identical", nil),
	)
	secs = append(secs, module.SectionOf("pinochle.rules.section.play", play))

	// --- scoring ---
	score := []module.RuleItem{}
	if t.auction {
		score = append(score,
			module.Rule("pinochle.rules.made", nil),
			module.Rule("pinochle.rules.set", nil),
		)
		if t.partners {
			score = append(score, module.Rule("pinochle.rules.defenders.partnership", nil))
		} else {
			score = append(score, module.Rule("pinochle.rules.defenders.cutthroat", nil))
		}
		if t.widow > 0 {
			score = append(score, module.Rule("pinochle.rules.buriedCount", nil))
		}
		score = append(score, module.Rule("pinochle.rules.throwIn", nil))
	} else {
		score = append(score, module.Rule("pinochle.rules.score.twohanded", nil))
	}
	secs = append(secs, module.SectionOf("pinochle.rules.section.scoring", score))

	// --- the end ---
	end := []module.RuleItem{module.Rule("pinochle.rules.end", map[string]any{"n": c.target})}
	if t.auction {
		end = append(end, module.Rule("pinochle.rules.bidderOut", nil))
	}
	end = append(end, module.Rule("pinochle.rules.tiePlaysOn", nil))
	secs = append(secs, module.SectionOf("pinochle.rules.section.end", end))
	return secs, nil
}

// ExplainRefusal names the written rules behind each code this engine can
// return, filtered to the ones this table actually states.
func (m *Module) ExplainRefusal(cfg module.MatchConfig, code string) []string {
	return explainWith(module.StatedRuleIDs(m, cfg), code)
}

func explainWith(stated map[string]bool, code string) []string {
	var out []string
	for _, id := range refusalRules(code) {
		if stated[id] {
			out = append(out, id)
		}
	}
	return out
}

func refusalRules(code string) []string {
	switch code {
	case ErrBidTooLow:
		return []string{"pinochle.rules.auction", "pinochle.rules.bidStep"}
	case ErrBidStep, ErrBidTooHigh:
		return []string{"pinochle.rules.bidStep"}
	case ErrAlreadyPassed:
		return []string{"pinochle.rules.passFinal"}
	case ErrNoSuchSuit:
		return []string{"pinochle.rules.trump"}
	case ErrBuryCount:
		return []string{"pinochle.rules.bury"}
	case ErrGiveCount:
		return []string{"pinochle.rules.pass"}
	case ErrMustFollow:
		return []string{"pinochle.rules.follow"}
	case ErrMustBeat:
		return []string{"pinochle.rules.beat"}
	case ErrMustTrump:
		return []string{"pinochle.rules.trumpIn"}
	case ErrNotBidder, ErrTooLate:
		return []string{"pinochle.rules.throwIn"}
	case ErrNoSuchMeld:
		return []string{"pinochle.rules.meld.classes", "pinochle.rules.meld.onePerTurn"}
	case ErrNoExchange:
		return []string{"pinochle.rules.meld.exchange"}
	case ErrNotYourTurn, ErrPickOneCard:
		return []string{"pinochle.rules.turns"}

	// --- not about the rules at all ---------------------------------------
	case ErrGameNotActive, ErrUnknownAction, ErrCardNotInHand, ErrWrongPlayers, ErrNotANumber, ErrWrongPhase:
		return nil
	}
	return nil
}

// remedyFor is what to do instead of a refused move, where one sentence
// says it.
func remedyFor(s *GameState, p, code string) *module.Fact {
	switch code {
	case ErrNotYourTurn:
		if s.Current != "" {
			return &module.Fact{LabelKey: "pinochle.remedy.waitFor", Params: map[string]any{"playerId": s.Current}}
		}
	case ErrBidTooLow:
		return &module.Fact{LabelKey: "pinochle.remedy.bidAtLeast", Params: map[string]any{"n": s.minNext()}}
	case ErrAlreadyPassed:
		return &module.Fact{LabelKey: "pinochle.remedy.passed"}
	case ErrMustFollow, ErrMustBeat, ErrMustTrump:
		return &module.Fact{LabelKey: "pinochle.remedy.playHighlighted"}
	}
	return nil
}
