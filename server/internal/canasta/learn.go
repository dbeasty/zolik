package canasta

import (
	"fmt"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
)

// Canasta as the learning machinery sees it (internal/learn).
//
// For now only the part the bench needs: a table, the hand-written bot, and
// how a seat finished. The encoder and candidates that let a network play
// come next, and turn this into a learn.Game.
type learnGame struct{}

func init() { learn.Register(learnGame{}) }

func (learnGame) Name() string              { return "canasta" }
func (learnGame) Module() module.GameModule { return New() }
func (learnGame) Heuristic() module.Bot     { return bot{} }

// Config is the variation as a lobby deals it, unchanged: a bench on shortened
// rules would measure a different game.
func (learnGame) Config(_ int, variation string) module.MatchConfig {
	if variation == "" {
		variation = "classic"
	}
	return module.MatchConfig{Variation: variation}
}

// Outcome is the partnership's score. Partners share it, which is what makes
// the bench's per-side averages mean what they should.
func (learnGame) Outcome(raw module.State, seat string) (float64, error) {
	s, err := decode(raw)
	if err != nil {
		return 0, err
	}
	t, ok := s.TeamOf[seat]
	if !ok || t < 0 || t >= len(s.Teams) {
		return 0, fmt.Errorf("canasta: no team for %q", seat)
	}
	return float64(s.Teams[t].Score), nil
}
