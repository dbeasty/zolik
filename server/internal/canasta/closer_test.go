package canasta

import (
	"fmt"
	"sync"
	"testing"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
)

// The closer plays whole matches, legally, at every table it can be seated
// at: alone with copies of itself, and against Hard. Classic seats two to
// four (ruleset.MaxSeats); Samba two to six.
func TestCloserPlaysLegalMatches(t *testing.T) {
	closer := learnGame{}.Styles()["closer"]
	tables := []struct {
		variation string
		seats     int
	}{
		{"classic", 2}, {"classic", 3}, {"classic", 4},
		{"samba", 2}, {"samba", 3}, {"samba", 4}, {"samba", 6},
	}
	for _, tb := range tables {
		for _, mixed := range []bool{false, true} {
			tb, mixed := tb, mixed
			name := fmt.Sprintf("%s/%d/all-closer", tb.variation, tb.seats)
			if mixed {
				name = fmt.Sprintf("%s/%d/vs-hard", tb.variation, tb.seats)
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				cfg := module.MatchConfig{Variation: tb.variation}
				if testing.Short() {
					// Several deals still, at a fraction of the time.
					target := 1500
					if tb.variation == "samba" {
						target = 3000
					}
					cfg.Options = module.Options{OptTargetScore: target}
				}
				players := learn.Players(tb.seats)
				for seed := int64(1); seed <= 2; seed++ {
					final, st, err := learn.PlayOut(New(), cfg, players, 7000+seed, 50_000, func(id string) (module.Bot, module.Skill) {
						if mixed && id[len(id)-1]%2 == 1 {
							return bot{}, module.SkillHard
						}
						return closer, ""
					})
					if err != nil {
						t.Fatal(err)
					}
					if st.Illegal > 0 || st.Stalled {
						t.Fatalf("seed %d: illegal %d, stalled %v (%s) after %d actions", seed, st.Illegal, st.Stalled, st.Why, st.Actions)
					}
					s, err := decode(final)
					if err != nil {
						t.Fatal(err)
					}
					if s.Status != "completed" || len(s.Deals) == 0 {
						t.Fatalf("seed %d: status %q after %d deals", seed, s.Status, len(s.Deals))
					}
				}
			})
		}
	}
}

// turnCounter counts each seat's own turns in each deal, from the positions
// its bot is asked to act in.
type turnCounter struct {
	mu    sync.Mutex
	turns map[int]map[string]int
	last  map[int]string
}

func (c *turnCounter) wrap(b module.Bot) module.Bot {
	return botFunc(func(raw module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
		if s, err := decode(raw); err == nil && s.Status == "active" && !s.Break.Open && s.Current == seat.PlayerID {
			c.mu.Lock()
			if c.turns[s.DealNumber] == nil {
				c.turns[s.DealNumber] = map[string]int{}
			}
			if c.last[s.DealNumber] != s.Current {
				c.last[s.DealNumber] = s.Current
				c.turns[s.DealNumber][s.Current]++
			}
			c.mu.Unlock()
		}
		return b.Act(raw, seat, offers)
	})
}

type botFunc func(module.State, module.BotSeat, []module.ActionOffer) (module.Action, bool)

func (f botFunc) Act(s module.State, seat module.BotSeat, o []module.ActionOffer) (module.Action, bool) {
	return f(s, seat, o)
}

// The closer's whole point: on a fixed seed set, heads-up against Hard, it
// goes out more often than Hard does and on an earlier turn of its own.
func TestCloserGoesOutEarlierThanHard(t *testing.T) {
	seeds := 6
	if testing.Short() {
		seeds = 3
	}
	closer := learnGame{}.Styles()["closer"]
	players := learn.Players(2)
	type result struct {
		outTurns map[bool][]int // by "is the closer"
		err      error
	}
	results := make([]result, 2*seeds)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			seed, closerSeat := int64(9100+i/2), players[i%2].ID
			c := &turnCounter{turns: map[int]map[string]int{}, last: map[int]string{}}
			final, st, err := learn.PlayOut(New(), module.MatchConfig{Variation: "classic"}, players, seed, 50_000,
				func(id string) (module.Bot, module.Skill) {
					if id == closerSeat {
						return c.wrap(closer), ""
					}
					return c.wrap(bot{}), module.SkillHard
				})
			if err == nil && (st.Illegal > 0 || st.Stalled) {
				err = fmt.Errorf("seed %d: illegal %d, stalled %v", seed, st.Illegal, st.Stalled)
			}
			r := result{outTurns: map[bool][]int{}, err: err}
			if err == nil {
				s, derr := decode(final)
				if derr != nil {
					r.err = derr
				} else {
					for _, d := range s.Deals {
						if d.WentOut != "" {
							isCloser := d.WentOut == closerSeat
							r.outTurns[isCloser] = append(r.outTurns[isCloser], c.turns[d.DealNumber][d.WentOut])
						}
					}
				}
			}
			results[i] = r
		}(i)
	}
	wg.Wait()
	all := map[bool][]int{}
	for _, r := range results {
		if r.err != nil {
			t.Fatal(r.err)
		}
		for k, v := range r.outTurns {
			all[k] = append(all[k], v...)
		}
	}
	mean := func(xs []int) float64 {
		sum := 0
		for _, x := range xs {
			sum += x
		}
		return float64(sum) / float64(max(1, len(xs)))
	}
	c, h := all[true], all[false]
	t.Logf("closer went out %d times, on own turn %.1f on average; hard %d times, on turn %.1f", len(c), mean(c), len(h), mean(h))
	if len(c) <= len(h) {
		t.Errorf("closer went out %d times, hard %d", len(c), len(h))
	}
	if mean(c) >= mean(h) {
		t.Errorf("closer went out on turn %.1f on average, hard on %.1f", mean(c), mean(h))
	}
}

// A closer whose side holds its canasta sheds a card its side can no longer
// meld before anything else, and the dearer of two singles before the
// cheaper, where Hard keeps the dear card and throws the cheap one.
func TestCloserShedsDeadAndDearCards(t *testing.T) {
	dead := discardCandidate{card: "KH", dead: true, value: 10, ordinal: "KH"}
	dear := discardCandidate{card: "AS", value: 20, ordinal: "AS"}
	cheap := discardCandidate{card: "4C", value: 5, ordinal: "4C"}
	pair := discardCandidate{card: "QS", building: true, value: 10, ordinal: "QS"}
	wild := discardCandidate{card: "2D", wild: true, value: 20, ordinal: "2D"}
	for _, c := range []struct {
		x, y discardCandidate
	}{{dead, dear}, {dear, cheap}, {cheap, pair}, {pair, wild}, {dead, wild}} {
		if !betterShed(c.x, c.y) || betterShed(c.y, c.x) {
			t.Errorf("shed %s before %s", c.y.card, c.x.card)
		}
	}
}
