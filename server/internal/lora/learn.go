package lora

import (
	"fmt"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
)

// Lóra on the duplicate bench (internal/learn).
//
//	go run ./cmd/gamebench -game lora -seats 4 -a hard -b medium -seeds 200
type learnGame struct{}

func init() { learn.Register(learnGame{}) }

var _ learn.Benchable = learnGame{}

func (learnGame) Name() string              { return "lora" }
func (learnGame) Module() module.GameModule { return New() }
func (learnGame) Heuristic() module.Bot     { return bot{} }

// Config is one tálie with its maturita, and no pause between deals.
func (learnGame) Config(_ int, _ string) module.MatchConfig {
	return module.MatchConfig{Options: module.Options{OptTalie: 1, module.OptPauseBetweenRounds: module.OptOff}}
}

// Outcome is the others' average penalty less the seat's own: positive is
// good.
func (learnGame) Outcome(raw module.State, seat string) (float64, error) {
	s, err := decode(raw)
	if err != nil {
		return 0, err
	}
	if s.seat(seat) < 0 {
		return 0, fmt.Errorf("lora: no seat %q", seat)
	}
	others := 0
	for _, p := range s.Players {
		if p != seat {
			others += s.Scores[p]
		}
	}
	return float64(others)/float64(len(s.Players)-1) - float64(s.Scores[seat]), nil
}
