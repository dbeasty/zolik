package marias

import (
	"fmt"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
)

// Mariáš on the duplicate bench (internal/learn): enough to deal a table,
// play it out between any two bots, and say how each seat did. Not yet a
// game a network can learn.
//
//	go run ./cmd/gamebench -game marias -seats 3 -a hard-1000k -b hard -seeds 600
//	go run ./cmd/gamebench -game marias -seats 4 -a hard -b medium -seeds 50
type learnGame struct{}

func init() { learn.Register(learnGame{}) }

var (
	_ learn.Benchable = learnGame{}
	_ learn.Styled    = learnGame{}
)

func (learnGame) Name() string              { return "marias" }
func (learnGame) Module() module.GameModule { return New() }
func (learnGame) Heuristic() module.Bot     { return bot{} }

// Config is a nine-deal match, every seat choosing trumps three times, with
// no pause between deals; at four seats NewMatch rounds it up to twelve.
func (learnGame) Config(_ int, variation string) module.MatchConfig {
	if variation == "" {
		variation = variationVoleny
	}
	return module.MatchConfig{
		Variation: variation,
		Options:   module.Options{OptDeals: 9, module.OptPauseBetweenRounds: module.OptOff},
	}
}

// Outcome is the seat's score at the end of the match, in units.
func (learnGame) Outcome(raw module.State, seat string) (float64, error) {
	s, err := decode(raw)
	if err != nil {
		return 0, err
	}
	v, ok := s.Scores[seat]
	if !ok {
		return 0, fmt.Errorf("marias: no seat %q", seat)
	}
	return float64(v), nil
}

// Styles are hard seats searching under other bounds than the shipped one,
// to measure what a bound costs:
//
//	hard-capped  the shipped budget made a hard ceiling: the last sample is
//	             cut off at two million nodes rather than running past
//	hard-<n>k    a capped budget of n thousand nodes a decision
func (learnGame) Styles() map[string]module.Bot {
	capped := defaultLimits
	capped.capped = true
	out := map[string]module.Bot{"hard-capped": bot{skill: module.SkillHard, limits: capped}}
	for _, k := range []int{250, 500, 1000, 1500} {
		out[fmt.Sprintf("hard-%dk", k)] = bot{skill: module.SkillHard, limits: searchLimits{total: k * 1000, perSample: min(400_000, k*1000), capped: true}}
	}
	return out
}
