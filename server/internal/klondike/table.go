package klondike

import "zolik/server/internal/module"

// Option names.
const (
	OptDraw       = "draw"
	OptRedeals    = "redeals"
	OptScoring    = "scoring"
	OptTakeBack   = "foundationTakeBack"
	OptAutoFinish = "autoFinish"
)

// Scoring schemes, as option values.
const (
	scoreStandard = 0
	scoreVegas    = 1
	scoreNone     = 2
)

// redealsUnlimited is the option value for "as many times as you like".
const redealsUnlimited = 99

// variations are the shipped rulesets. Every house rule is an option; a
// variation is only a named bundle of their values.
var variations = map[string]struct {
	draw, redeals, scoring int
}{
	"classic": {1, redealsUnlimited, scoreStandard},
	"draw3":   {3, redealsUnlimited, scoreStandard},
	"vegas":   {3, 0, scoreVegas},
}

func resolveVariation(cfg module.MatchConfig) struct{ draw, redeals, scoring int } {
	if v, ok := variations[cfg.Variation]; ok {
		return v
	}
	return variations["classic"]
}

func defaultsFor(id string) map[string]int {
	v := variations[id]
	return map[string]int{
		OptDraw:       v.draw,
		OptRedeals:    v.redeals,
		OptScoring:    v.scoring,
		OptTakeBack:   module.OptOn,
		OptAutoFinish: module.OptOn,
	}
}

// tableOf resolves a lobby's variation and options into the table a match is
// played on. One function, called by NewMatch, Rules and ExplainRefusal, so a
// rules screen cannot describe a table nobody is sitting at.
func tableOf(cfg module.MatchConfig) *GameState {
	v := resolveVariation(cfg)
	return &GameState{
		Variation:  cfg.Variation,
		Draw:       cfg.Opt(OptDraw, v.draw),
		Redeals:    cfg.Opt(OptRedeals, v.redeals),
		Scoring:    cfg.Opt(OptScoring, v.scoring),
		TakeBack:   cfg.Opt(OptTakeBack, module.OptOn) == module.OptOn,
		AutoFinish: cfg.Opt(OptAutoFinish, module.OptOn) == module.OptOn,
	}
}
