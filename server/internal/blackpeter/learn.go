package blackpeter

import (
	"fmt"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
)

// Black Peter on the duplicate bench (internal/learn). There is nothing for a
// stronger bot to find — see bot.go — so this is here to prove that, not to
// train against.
//
//	go run ./cmd/gamebench -game blackpeter -seats 4 -a hard -b medium -seeds 200
type learnGame struct{}

func init() { learn.Register(learnGame{}) }

var _ learn.Benchable = learnGame{}

func (learnGame) Name() string              { return "blackpeter" }
func (learnGame) Module() module.GameModule { return New() }
func (learnGame) Heuristic() module.Bot     { return bot{} }

// Config is the default table, with no pause between rounds.
func (learnGame) Config(_ int, _ string) module.MatchConfig {
	return module.MatchConfig{Options: module.Options{module.OptPauseBetweenRounds: module.OptOff}}
}

// Outcome is the seat's marks, negated: fewer is better.
func (learnGame) Outcome(raw module.State, seat string) (float64, error) {
	s, err := decode(raw)
	if err != nil {
		return 0, err
	}
	st := s.seat(seat)
	if st == nil {
		return 0, fmt.Errorf("blackpeter: no seat %q", seat)
	}
	return -float64(st.Marks), nil
}
