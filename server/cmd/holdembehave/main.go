// Command holdembehave says how a Hold'em bot bets big, not only how well it
// plays: how often it shoves or overbets, how often that is called, and what
// the called ones win or lose.
//
//	go run ./cmd/holdembehave -a net:final.bin -b solid
//	go run ./cmd/holdembehave -seats 6 -a net:final.bin -b hard -seeds 200
//
// Written for one question — whether a model shoves hands worse than the
// range that calls it — so it counts, for each contender, the hands in which
// it made a *big bet*: a raise all in, or a raise of more than the pot (the
// part above the call against the pot once the call is in, which is how a
// table measures one). A hand counts once, at its first big bet. It is
// called when an opponent calls or raises after it, or the hand reaches a
// showdown, and its result is the seat's chip delta for the whole hand.
//
// Seats alternate between A and B and every seed is played twice with them
// swapped, as gamebench does, on whole matches from -first.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"runtime"
	"strconv"
	"sync"

	"zolik/server/internal/holdem"
	"zolik/server/internal/learn"
	"zolik/server/internal/module"
)

const (
	labelA = "A"
	labelB = "B"
)

// bigBet is one hand's first big bet by one seat.
type bigBet struct {
	player  string
	hand    int
	logAt   int // its index in the hand's log
	street  string
	allIn   bool
	overbet bool
	bb      float64 // the big blind it was made at
}

// bets is one contender's big bets on one set of streets.
type bets struct {
	Big        int
	AllIn      int
	Overbet    int
	Called     int
	CalledWon  int // called, and the hand made the seat chips
	CalledLost int
	CalledBB   []float64
	UncalledBB []float64
}

func (t *bets) add(o *bets) {
	t.Big += o.Big
	t.AllIn += o.AllIn
	t.Overbet += o.Overbet
	t.Called += o.Called
	t.CalledWon += o.CalledWon
	t.CalledLost += o.CalledLost
	t.CalledBB = append(t.CalledBB, o.CalledBB...)
	t.UncalledBB = append(t.UncalledBB, o.UncalledBB...)
}

// Street groups a big bet is counted under.
var groups = []string{"all", "preflop", "postflop"}

// tally is one contender's hands.
type tally struct {
	Hands  int       // hands dealt in
	HandBB []float64 // every hand dealt in, for context
	By     map[string]*bets
}

func newTally() *tally {
	t := &tally{By: map[string]*bets{}}
	for _, g := range groups {
		t.By[g] = &bets{}
	}
	return t
}

func (t *tally) add(o *tally) {
	t.Hands += o.Hands
	t.HandBB = append(t.HandBB, o.HandBB...)
	for _, g := range groups {
		t.By[g].add(o.By[g])
	}
}

// watcher sees every decision at the table.
type watcher struct {
	mu    sync.Mutex
	label map[string]string
	big   map[string]*bigBet         // player/hand -> first big bet
	logs  map[int][]holdem.PublicAct // hand -> longest log seen
	acted map[string]map[int]bool    // player -> hands it decided in
}

func key(player string, hand int) string { return player + "/" + strconv.Itoa(hand) }

func (w *watcher) see(raw module.State, player string, a module.Action, ok bool) {
	var s holdem.GameState
	if json.Unmarshal(raw, &s) != nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(s.HandLog) > len(w.logs[s.HandNumber]) {
		w.logs[s.HandNumber] = append([]holdem.PublicAct(nil), s.HandLog...)
	}
	if !ok || s.Break.Open || s.Status != "active" || s.Current < 0 || s.Seats[s.Current].PlayerID != player {
		return
	}
	if a.Verb != holdem.VerbRaise {
		return
	}
	to, err := strconv.Atoi(a.Params[holdem.ParamAmount])
	if err != nil {
		return
	}
	st := s.Seats[s.Current]
	pot := s.Pot
	for _, x := range s.Seats {
		pot += x.Bet
	}
	owed := s.CurrentBet - st.Bet
	if owed < 0 {
		owed = 0
	}
	allIn := to >= st.Bet+st.Stack
	overbet := to-s.CurrentBet > pot+owed
	if !allIn && !overbet {
		return
	}
	k := key(player, s.HandNumber)
	if _, seen := w.big[k]; seen {
		return
	}
	w.big[k] = &bigBet{player: player, hand: s.HandNumber, logAt: len(s.HandLog), street: s.Street,
		allIn: allIn, overbet: overbet, bb: float64(max(s.BigBlind, 1))}
}

// finish reads the finished match into one tally per label.
func (w *watcher) finish(raw module.State) (map[string]*tally, error) {
	var s holdem.GameState
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	out := map[string]*tally{labelA: newTally(), labelB: newTally()}
	bb := float64(max(s.BigBlind, 1))
	stack := map[string]int{}
	for _, st := range s.Seats {
		stack[st.PlayerID] = s.StartingStack
	}
	for _, h := range s.Hands {
		for player, label := range w.label {
			if stack[player] <= 0 {
				continue
			}
			t := out[label]
			t.Hands++
			delta := float64(h.Deltas[player]) / bb
			t.HandBB = append(t.HandBB, delta)
			b := w.big[key(player, h.Number)]
			if b == nil {
				continue
			}
			called := !h.Uncontested
			for i, act := range w.logs[h.Number] {
				if i > b.logAt && act.Seat >= 0 && act.Seat < len(s.Seats) && s.Seats[act.Seat].PlayerID != player &&
					(act.Verb == holdem.VerbCall || act.Verb == holdem.VerbRaise) {
					called = true
				}
			}
			street := "postflop"
			if b.street == "preflop" {
				street = "preflop"
			}
			for _, g := range []string{"all", street} {
				x := t.By[g]
				x.Big++
				if b.allIn {
					x.AllIn++
				}
				if b.overbet {
					x.Overbet++
				}
				if called {
					x.Called++
					x.CalledBB = append(x.CalledBB, delta)
					if delta > 0 {
						x.CalledWon++
					} else if delta < 0 {
						x.CalledLost++
					}
				} else {
					x.UncalledBB = append(x.UncalledBB, delta)
				}
			}
		}
		for player, st := range h.Stacks {
			stack[player] = st
		}
	}
	return out, nil
}

type observed struct {
	bot module.Bot
	w   *watcher
}

func (o observed) Act(s module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	a, ok := o.bot.Act(s, seat, offers)
	o.w.see(s, seat.PlayerID, a, ok)
	return a, ok
}

func play(g learn.Game, seats int, seed int64, budget int, flip bool, bots map[string]learn.Contender) (map[string]*tally, learn.PlayStats, error) {
	players := learn.Players(seats)
	w := &watcher{label: map[string]string{}, big: map[string]*bigBet{}, logs: map[int][]holdem.PublicAct{}}
	for i, p := range players {
		l := labelB
		if (i%2 == 0) != flip {
			l = labelA
		}
		w.label[p.ID] = l
	}
	final, st, err := learn.PlayOut(g.Module(), g.Config(seats, ""), players, seed, budget, func(id string) (module.Bot, module.Skill) {
		c := bots[w.label[id]]
		return observed{bot: c.Bot, w: w}, c.Skill
	})
	if err != nil || st.Stalled {
		return nil, st, err
	}
	out, err := w.finish(final)
	return out, st, err
}

func main() {
	seats := flag.Int("seats", 2, "seats at the table")
	a := flag.String("a", "hard", "contender A")
	b := flag.String("b", "solid", "contender B")
	first := flag.Int64("first", 1_000_000, "first seed")
	seeds := flag.Int("seeds", 100, "seeds, each played in both seatings")
	budget := flag.Int("actions", 50_000, "action budget per match")
	flag.Parse()

	g, err := learn.LookupGame("holdem")
	if err != nil {
		fail(err)
	}
	bots := map[string]learn.Contender{}
	for label, spec := range map[string]string{labelA: *a, labelB: *b} {
		c, err := learn.ParseContender(g, spec)
		if err != nil {
			fail(err)
		}
		bots[label] = c
	}

	total := map[string]*tally{labelA: newTally(), labelB: newTally()}
	var mu sync.Mutex
	var illegal, stalls int
	var firstErr error
	jobs := make(chan int64)
	var wg sync.WaitGroup
	for i := 0; i < runtime.GOMAXPROCS(0); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for seed := range jobs {
				for _, flip := range []bool{false, true} {
					t, st, err := play(g, *seats, seed, *budget, flip, bots)
					mu.Lock()
					illegal += st.Illegal
					if st.Stalled {
						stalls++
					}
					if err != nil && firstErr == nil {
						firstErr = fmt.Errorf("seed %d: %w", seed, err)
					}
					if t != nil {
						total[labelA].add(t[labelA])
						total[labelB].add(t[labelB])
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
	if firstErr != nil {
		fail(firstErr)
	}

	fmt.Printf("holdem %d seats: A=%s, B=%s; %d seeds x 2 seatings (illegal %d, stalls %d)\n", *seats, *a, *b, *seeds, illegal, stalls)
	fmt.Printf("%-2s %-8s %6s %8s %7s %7s %7s %7s %7s %13s %13s %9s %9s\n",
		"", "street", "hands", "big/100", "allin", "overbet", "called", "won", "lost", "calledBB", "uncalledBB", "lost/100", "bb/100")
	for _, l := range []string{labelA, labelB} {
		t := total[l]
		h := math.Max(1, float64(t.Hands))
		hm, _ := meanErr(t.HandBB)
		for _, g := range groups {
			x := t.By[g]
			cm, cse := meanErr(x.CalledBB)
			um, use := meanErr(x.UncalledBB)
			fmt.Printf("%-2s %-8s %6d %8.2f %7.2f %7.2f %6.1f%% %6.1f%% %6.1f%% %+6.1f ± %4.1f %+6.1f ± %4.1f %9.2f %+9.1f\n",
				l, g, t.Hands, 100*float64(x.Big)/h, 100*float64(x.AllIn)/h, 100*float64(x.Overbet)/h,
				pct(x.Called, x.Big), pct(x.CalledWon, x.Called), pct(x.CalledLost, x.Called),
				cm, cse, um, use, 100*float64(x.CalledLost)/h, 100*hm)
		}
	}
	fmt.Println("big: hands with a raise all in or above the pot, per 100 hands dealt; allin/overbet by kind (a hand can be both)")
	fmt.Println("called: share of big bets an opponent called or raised; won/lost: share of called ones; calledBB/uncalledBB: the hand's result, big blinds")
	fmt.Println("lost/100: called big bets that lost chips, per 100 hands dealt; bb/100: all hands")
	if illegal+stalls > 0 {
		os.Exit(1)
	}
}

func pct(a, b int) float64 { return 100 * float64(a) / math.Max(1, float64(b)) }

func meanErr(xs []float64) (float64, float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	m := sum / float64(len(xs))
	if len(xs) < 2 {
		return m, 0
	}
	var ss float64
	for _, x := range xs {
		ss += (x - m) * (x - m)
	}
	return m, math.Sqrt(ss / float64(len(xs)-1) / float64(len(xs)))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "holdembehave:", err)
	os.Exit(2)
}
