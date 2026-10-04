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
func (m *Module) Bot() module.Bot { return heuristicBot{} }

type heuristicBot struct{}

// Act is the agent's move when the engine takes it, and otherwise the first
// forward move the offer list describes that the engine takes.
//
// The second half is a last resort, not a strategy. The agent answers only the
// seat on turn and only in rummy verbs, so between deals — where the move is
// "continue" — it has nothing to say, and a pick the engine refuses (the
// agent's own rules drifting from the engine's) would otherwise leave the
// runtime to work down the raw offer list, undos included. A seat with a legal
// move that is not a take-back is never handed back empty.
func (b heuristicBot) Act(raw module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	m := New()
	if a, ok := b.choose(raw, seat); ok {
		if _, _, err := m.Apply(raw, seat.PlayerID, a); err == nil {
			return a, true
		}
	}
	for _, o := range offers {
		if o.Undo {
			continue
		}
		if a, ok := module.SubmissionFor(o); ok {
			if _, _, err := m.Apply(raw, seat.PlayerID, a); err == nil {
				return a, true
			}
		}
	}
	return module.Action{}, false
}

func (b heuristicBot) choose(raw module.State, seat module.BotSeat) (module.Action, bool) {
	s, err := decode(raw)
	if err != nil {
		return module.Action{}, false
	}
	if s.Rules.CurrentTurn != seat.PlayerID {
		return module.Action{}, false
	}

	// The seat's own strength, not one hardcoded here. It used to be the
	// literal "medium" — which meant the easy and hard settings the rest of
	// the system already had names, statistics buckets and a discard heuristic
	// for could not be reached at all.
	agent := ai.NewAgent(seat.Skill, seat.Seed)
	visible := ai.VisibleFor(s.Rules, s.Ledger, seat.PlayerID)
	chosen := agent.ChooseAction(visible, append([]string(nil), s.Rules.Hands[seat.PlayerID]...))
	return toModuleAction(chosen)
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
