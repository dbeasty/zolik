package snaps

import (
	"fmt"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
)

// Šnaps on the duplicate bench (internal/learn): enough to deal a table,
// play it out between any two bots, and say how each seat did.
//
//	go run ./cmd/gamebench -game snaps -seats 2 -a hard -b medium -seeds 400
type learnGame struct{}

func init() { learn.Register(learnGame{}) }

var _ learn.Benchable = learnGame{}

func (learnGame) Name() string              { return "snaps" }
func (learnGame) Module() module.GameModule { return New() }
func (learnGame) Heuristic() module.Bot     { return bot{} }

// Config is a match to seven game points, with no pause between deals.
func (learnGame) Config(_ int, variation string) module.MatchConfig {
	if variation == "" {
		variation = variationSnaps
	}
	return module.MatchConfig{
		Variation: variation,
		Options:   module.Options{OptTarget: defaultTarget, module.OptPauseBetweenRounds: module.OptOff},
	}
}

// Outcome is the seat's game points at the end of the match, less the
// other's: what the match was won or lost by.
func (learnGame) Outcome(raw module.State, seat string) (float64, error) {
	s, err := decode(raw)
	if err != nil {
		return 0, err
	}
	if s.seat(seat) < 0 {
		return 0, fmt.Errorf("snaps: no seat %q", seat)
	}
	return float64(s.Scores[seat] - s.Scores[s.other(seat)]), nil
}
