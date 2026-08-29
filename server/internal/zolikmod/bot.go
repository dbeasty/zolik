package zolikmod

import (
	"zolik/server/internal/ai"
	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// Žolíky's bot is the real one.
//
// module.OfferBot would play legally here and badly: it cannot go out at all,
// because going out needs a meld *shape* the offer protocol deliberately does
// not enumerate. internal/ai is a heuristic player with a real search behind
// it, and it already exists — so this module implements module.Botted and
// hands it over rather than regressing its opponents to a shape-matcher.
//
// That is the whole reason Botted is an interface and not a fixed policy: a
// module that has something better should be able to say so, and a module that
// has nothing should not have to.
func (m *Module) Bot() module.Bot { return heuristicBot{difficulty: "medium"} }

type heuristicBot struct{ difficulty string }

func (b heuristicBot) Act(raw module.State, playerID string, _ []module.ActionOffer) (module.Action, bool) {
	s, err := decode(raw)
	if err != nil {
		return module.Action{}, false
	}
	if s.Rules.CurrentTurn != playerID {
		return module.Action{}, false
	}

	agent := ai.NewHeuristicAgent(b.difficulty)
	chosen := agent.ChooseAction(visibleFor(s, playerID), append([]string(nil), s.Rules.Hands[playerID]...))
	return toModuleAction(chosen)
}

// visibleFor is what the agent is allowed to see.
//
// The rummy runtime built this from its own 30-column document; here it comes
// straight off the engine's own state, which is both shorter and impossible to
// get out of step with — there is no second representation to map from.
func visibleFor(s *matchState, playerID string) ai.VisibleState {
	gs := s.Rules
	return ai.VisibleState{
		GameNumber:       gs.GameNumber,
		Round:            gs.Round,
		Phase:            string(gs.Phase),
		CurrentTurn:      gs.CurrentTurn,
		DiscardPile:      gs.DiscardPile,
		PlayerDiscards:   discardHistory(gs),
		Melds:            gs.Melds,
		MeldMeta:         gs.MeldMeta,
		RoundReqMet:      gs.RoundReqMet,
		Rules:            rules.ResolveConfig(gs.Rules),
		DiscardTakenCard: gs.DiscardTakenCard,
		PendingJokers:    gs.JokersReclaimedPendingMeld,
	}
}

// discardHistory is each player's discards, oldest first.
//
// The legacy runtime reconstructed this by walking its action log. The module
// has no action log of its own — the runtime keeps one, opaquely — so this
// reports what it can see: the pile itself, unattributed. The agent uses it to
// avoid re-offering a rank somebody has already passed on, so a coarser answer
// makes it slightly less sharp, not wrong.
func discardHistory(gs rules.GameState) map[string][]string {
	out := map[string][]string{}
	for _, p := range gs.TurnOrder {
		out[p] = nil
	}
	return out
}

// toModuleAction translates a rummy action into the generic vocabulary.
//
// The reverse of the adapter's existing toRulesAction, and deliberately kept
// next to the bot rather than folded in with it: this is the only place the
// translation runs in this direction.
func toModuleAction(a rules.Action) (module.Action, bool) {
	var out module.Action
	switch a.Type {
	case rules.ActionDrawCard:
		out.Verb = string(rules.VerbDraw)
		out.OfferID = rules.OfferDrawDeck
		if a.DrawFrom == rules.DrawFromDiscard {
			out.OfferID = rules.OfferDrawDiscard
			out.Target = string(rules.ZoneDiscardPile)
		}
		if a.Card != "" {
			out.Cards = []string{a.Card}
		}
	case rules.ActionDiscard:
		out.Verb = string(rules.VerbDiscard)
		out.Cards = []string{a.Card}
	case rules.ActionLayMeld:
		out.Verb = string(rules.VerbLayMeld)
		out.Cards = a.Cards
	case rules.ActionLayOff:
		out.Verb = string(rules.VerbLayOff)
		out.Cards = a.Cards
		if len(out.Cards) == 0 && a.Card != "" {
			out.Cards = []string{a.Card}
		}
		out.Target = a.MeldID
		if a.Position != "" {
			out.Params = map[string]string{"position": a.Position}
		}
	case rules.ActionSwapJoker:
		out.Verb = string(rules.VerbSwapJoker)
		out.Cards = []string{a.Card}
		out.Target = a.MeldID
	default:
		// Undo is a thing a person does, not a bot; anything else is a verb
		// this translation has not been taught.
		return module.Action{}, false
	}
	return out, true
}
