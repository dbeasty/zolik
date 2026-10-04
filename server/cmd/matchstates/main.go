// Command matchstates plays whole matches and writes the score state before
// every deal, from each seat's side, with how the match ended: the data
// ml/winmodel.py fits the match reward's win-probability model on
// (learn.MatchReward, learn.WinFeatures).
//
//	go run ./cmd/matchstates -game canasta -seats 4 -a hard -b closer -seeds 200 > hard-closer.jsonl
//
// A plays the even seats and B the odd ones (partners together), and every
// seed is played again with the two swapped. One JSON line per side per deal
// boundary of a finished match: {"x": WinFeatures, "s": MatchScore, "won": 0|1,
// "a": .., "b": .., "seed": ..}. A stalled match is left out.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"sync"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"

	_ "zolik/server/internal/canasta"
)

type row struct {
	X    []float64        `json:"x"`
	S    learn.MatchScore `json:"s"`
	Won  float64          `json:"won"`
	A    string           `json:"a"`
	B    string           `json:"b"`
	Seed int64            `json:"seed"`
}

func main() {
	game := flag.String("game", "canasta", "game")
	seats := flag.Int("seats", 2, "seats")
	variation := flag.String("variation", "", "variation")
	aSpec := flag.String("a", "hard", "contender on the even seats")
	bSpec := flag.String("b", "closer", "contender on the odd seats")
	first := flag.Int64("first", 2_000_000, "first seed")
	seeds := flag.Int("seeds", 100, "seeds (each played twice)")
	budget := flag.Int("budget", 50_000, "actions per match")
	workers := flag.Int("workers", runtime.GOMAXPROCS(0), "matches at once")
	flag.Parse()

	g, err := learn.LookupGame(*game)
	if err != nil {
		fail(err)
	}
	ms, ok := g.(learn.MatchScored)
	if !ok {
		fail(fmt.Errorf("%s has no match score", *game))
	}
	a, err := learn.ParseContender(g, *aSpec)
	if err != nil {
		fail(err)
	}
	b, err := learn.ParseContender(g, *bSpec)
	if err != nil {
		fail(err)
	}
	players := learn.Players(*seats)
	cfg := g.Config(*seats, *variation)
	// One seat per side: partners share a score.
	var reps []string
	if sides := module.SidesOf(g.Module(), cfg, players); len(sides) > 0 {
		for _, s := range sides {
			reps = append(reps, s[0])
		}
	} else {
		for _, p := range players {
			reps = append(reps, p.ID)
		}
	}

	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	enc := json.NewEncoder(out)
	var mu sync.Mutex
	stalls := 0
	jobs := make(chan int64)
	var wg sync.WaitGroup
	for w := 0; w < *workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for seed := range jobs {
				for swap := 0; swap < 2; swap++ {
					x, y := a, b
					if swap == 1 {
						x, y = b, a
					}
					final, st, err := learn.PlayOut(g.Module(), cfg, players, seed, *budget, func(id string) (module.Bot, module.Skill) {
						for i, p := range players {
							if p.ID == id && i%2 == 1 {
								return y.Bot, y.Skill
							}
						}
						return x.Bot, x.Skill
					})
					if err != nil {
						fail(err)
					}
					if st.Stalled {
						mu.Lock()
						stalls++
						mu.Unlock()
						continue
					}
					var rows []row
					for _, id := range reps {
						h, err := ms.MatchHistory(final, id)
						if err != nil {
							fail(err)
						}
						last := h[len(h)-1]
						if !last.Over {
							continue
						}
						for _, s := range h[:len(h)-1] {
							rows = append(rows, row{X: learn.WinFeatures(s), S: s, Won: last.Won, A: x.Name, B: y.Name, Seed: seed})
						}
					}
					mu.Lock()
					for _, r := range rows {
						if err := enc.Encode(r); err != nil {
							fail(err)
						}
					}
					mu.Unlock()
				}
			}
		}()
	}
	for i := 0; i < *seeds; i++ {
		jobs <- *first + int64(i)
	}
	close(jobs)
	wg.Wait()
	fmt.Fprintf(os.Stderr, "matchstates: %s vs %s, %d seats, %d seeds x2, %d stalled\n", a.Name, b.Name, *seats, *seeds, stalls)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "matchstates:", err)
	os.Exit(2)
}
