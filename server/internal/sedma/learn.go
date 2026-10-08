package sedma

import (
	"fmt"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
)

// Sedma on the duplicate bench (internal/learn).
//
//	go run ./cmd/gamebench -game sedma -seats 4 -a hard -b medium -seeds 200
type learnGame struct{}

func init() { learn.Register(learnGame{}) }

var _ learn.Benchable = learnGame{}

func (learnGame) Name() string              { return "sedma" }
func (learnGame) Module() module.GameModule { return New() }
func (learnGame) Heuristic() module.Bot     { return bot{} }

// Config is a match to five stakes, with no pause between hands.
func (learnGame) Config(_ int, _ string) module.MatchConfig {
	return module.MatchConfig{Options: module.Options{OptTarget: defaultTarget, module.OptPauseBetweenRounds: module.OptOff}}
}

// Outcome is the seat's stakes at the end of the match, less the best of
// everyone else's.
func (learnGame) Outcome(raw module.State, seat string) (float64, error) {
	s, err := decode(raw)
	if err != nil {
		return 0, err
	}
	if s.seat(seat) < 0 {
		return 0, fmt.Errorf("sedma: no seat %q", seat)
	}
	best := 0
	for _, p := range s.Players {
		if s.side(p) != s.side(seat) && s.Scores[p] > best {
			best = s.Scores[p]
		}
	}
	return float64(s.Scores[seat] - best), nil
}
