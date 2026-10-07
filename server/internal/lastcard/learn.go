package lastcard

import (
	"fmt"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
)

// Last Card on the duplicate bench (internal/learn): enough to deal a table,
// play it out between any two bots, and say how each seat did. Not yet a
// game a network can learn.
//
//	go run ./cmd/gamebench -game lastcard -seats 3 -a hard -b hard-no-swap -seeds 600
type learnGame struct{}

func init() { learn.Register(learnGame{}) }

var (
	_ learn.Benchable = learnGame{}
	_ learn.Styled    = learnGame{}
)

func (learnGame) Name() string              { return "lastcard" }
func (learnGame) Module() module.GameModule { return New() }
func (learnGame) Heuristic() module.Bot     { return bot{} }

// Config is a match to 200 with every house rule on — the table where the
// most judgement is in play — and no pause between deals.
func (learnGame) Config(_ int, _ string) module.MatchConfig {
	return module.MatchConfig{Options: module.Options{
		OptTargetScore:               200,
		OptStacking:                  stackAny,
		OptSevenZero:                 module.OptOn,
		module.OptPauseBetweenRounds: module.OptOff,
	}}
}

// Outcome is the seat's points at the end of the match.
func (learnGame) Outcome(raw module.State, seat string) (float64, error) {
	s, err := decode(raw)
	if err != nil {
		return 0, err
	}
	v, ok := s.Scores[seat]
	if !ok {
		return 0, fmt.Errorf("lastcard: no seat %q", seat)
	}
	return float64(v), nil
}

// Styles are the hard profile with one judgement taken out at a time, to
// price what each is worth on a full table.
func (learnGame) Styles() map[string]module.Bot {
	hard := profiles[module.SkillHard]
	without := func(f func(*profile)) module.Bot {
		p := hard
		f(&p)
		return bot{fixed: &p}
	}
	return map[string]module.Bot{
		"hard-no-swap":      without(func(p *profile) { p.swapsDown = false }),
		"hard-no-bluff":     without(func(p *profile) { p.bluffs = false }),
		"hard-no-challenge": without(func(p *profile) { p.challengeAt = 0 }),
		"hard-no-attack":    without(func(p *profile) { p.attackFirst = false }),
	}
}
