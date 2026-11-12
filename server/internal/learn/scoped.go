package learn

import "zolik/server/internal/module"

// Scoped limits a model-backed bot to the tables its model was measured
// ahead on. HardModel layers a game's network over its heuristic for every AI
// seat; a variation the network was never trained for — or was trained for and
// still loses at — is somewhere an AI seat plays worse than Hard, the one
// thing the AI setting promises not to do. Scoped sends AI seats at such a
// table to the heuristic's Hard instead.
//
// trained answers per position and must be cheap: it runs on every AI move.
// b is returned unchanged when it is not model-backed (the switch is off), so
// the match runtime still sees the bare heuristic and labels it a rule bot.
func Scoped(b, heuristic module.Bot, trained func(module.State) bool) module.Bot {
	if _, layered := b.(interface{ Heuristic() module.Bot }); !layered {
		return b
	}
	return scoped{model: b, heuristic: heuristic, trained: trained}
}

type scoped struct {
	model, heuristic module.Bot
	trained          func(module.State) bool
}

// Heuristic keeps the layered bot's contract: hints, the governor and the
// match runtime recognise a model-backed bot by this method.
func (b scoped) Heuristic() module.Bot { return b.heuristic }

func (b scoped) Act(s module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	if seat.Skill == module.SkillAI && !b.trained(s) {
		seat.Skill = module.SkillHard
		return b.heuristic.Act(s, seat, offers)
	}
	return b.model.Act(s, seat, offers)
}
