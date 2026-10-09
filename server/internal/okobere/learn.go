package okobere

import (
	"fmt"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
)

// Oko bere on the duplicate bench (internal/learn).
//
//	go run ./cmd/gamebench -game okobere -seats 4 -a hard -b medium -seeds 200
type learnGame struct{}

func init() { learn.Register(learnGame{}) }

var _ learn.Benchable = learnGame{}

func (learnGame) Name() string              { return "okobere" }
func (learnGame) Module() module.GameModule { return New() }
func (learnGame) Heuristic() module.Bot     { return bot{} }

// Config is the default table, with no pause between rounds.
func (learnGame) Config(_ int, _ string) module.MatchConfig {
	return module.MatchConfig{Options: module.Options{module.OptPauseBetweenRounds: module.OptOff}}
}

// Outcome is the seat's chips at the end, less what it started with.
func (learnGame) Outcome(raw module.State, seat string) (float64, error) {
	s, err := decode(raw)
	if err != nil {
		return 0, err
	}
	if s.seat(seat) == nil {
		return 0, fmt.Errorf("okobere: no seat %q", seat)
	}
	return float64(s.worth(seat) - s.StartingStack), nil
}
