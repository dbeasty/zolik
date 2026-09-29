package holdem

import (
	"fmt"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
)

// Hold'em as the learning machinery sees it (internal/learn).
//
// For now only the part the bench needs: a table, the hand-written bot, and
// how a seat finished. The encoder and candidates that let a network play
// come next, and turn this into a learn.Game.
type learnGame struct{}

func init() { learn.Register(learnGame{}) }

func (learnGame) Name() string              { return "holdem" }
func (learnGame) Module() module.GameModule { return New() }
func (learnGame) Heuristic() module.Bot     { return bot{} }

// Config is the table the bot ladder has always been measured on
// (bot_strategy_test.go's headToHead): a 15-hand timed match, fifty big blinds
// deep. Short enough that a sweep of hundreds of seeds is quick, long enough
// that a hand's result is not the whole match.
func (learnGame) Config(_ int, variation string) module.MatchConfig {
	if variation == "" {
		variation = "timed"
	}
	return module.MatchConfig{
		Variation: variation,
		Options:   module.Options{OptStartingStack: 1000, OptBigBlind: 20, OptHandLimit: 15},
	}
}

// Outcome is chips won or lost, in big blinds.
func (learnGame) Outcome(raw module.State, seat string) (float64, error) {
	s, err := decode(raw)
	if err != nil {
		return 0, err
	}
	for _, st := range s.Seats {
		if st.PlayerID == seat {
			return float64(st.Stack-s.StartingStack) / float64(s.BigBlind), nil
		}
	}
	return 0, fmt.Errorf("holdem: no seat %q", seat)
}
