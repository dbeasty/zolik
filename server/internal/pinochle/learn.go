package pinochle

import (
	"fmt"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
)

// Pinochle on the duplicate bench (internal/learn).
//
//	go run ./cmd/gamebench -game pinochle -seats 4 -a hard -b medium -seeds 100
//	go run ./cmd/gamebench -game pinochle -seats 3 -variation cutthroat -a hard -b medium
type learnGame struct{}

func init() { learn.Register(learnGame{}) }

var _ learn.Benchable = learnGame{}

func (learnGame) Name() string              { return "pinochle" }
func (learnGame) Module() module.GameModule { return New() }
func (learnGame) Heuristic() module.Bot     { return bot{} }

// Config is the variation that seats this many, played to its own target
// with no pause between hands.
func (learnGame) Config(seats int, variation string) module.MatchConfig {
	if variation == "" {
		switch seats {
		case 2:
			variation = VarTwoHanded
		case 3:
			variation = VarCutthroat
		default:
			variation = VarPartnership
		}
	}
	return module.MatchConfig{Variation: variation, Options: module.Options{module.OptPauseBetweenRounds: module.OptOff}}
}

// Outcome is the seat's side's score at the end of the match, less the best
// of everyone else's.
func (learnGame) Outcome(raw module.State, seat string) (float64, error) {
	s, err := decode(raw)
	if err != nil {
		return 0, err
	}
	if s.seat(seat) < 0 {
		return 0, fmt.Errorf("pinochle: no seat %q", seat)
	}
	return float64(outcome(s.Scores, s.side(seat))), nil
}
