// Command zolikbehave says how a Žolíky bot plays, not only how well: how many
// turns it takes to go down, how often a deal ends with it never down, what it
// is left holding when someone else goes out, and whether it is caught with
// jokers in hand.
//
//	go run ./cmd/zolikbehave -variation zolik_classic+floor35 -seats 4 -a-seats 1 -a net:final.bin -b hard
//
// A plays in -a-seats seats and B in the rest, and every seed is played once
// per rotation of the table (as gamebench -a-seats), whole matches on seeds
// from -first. Numbers are per seat and deal, split by contender.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"runtime"
	"sort"
	"sync"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
	"zolik/server/internal/rules"

	_ "zolik/server/internal/zolikmod"
)

// tally is one contender's seat-deals.
type tally struct {
	SeatDeals   int
	Out         int   // went out
	Down        int   // down by the deal's end
	TurnsToDown []int // own turns begun up to and including the one it went down in
	Caught      int   // deals someone else went out of
	CaughtDown  int
	Penalty     []int // deal penalty when caught
	PenaltyUp   []int // ... and never down
	PenaltyDown []int // ... and down
	JokersHeld  []int // jokers in hand when caught
	Match       []int // match penalty per seat
	Turns       int
	// Never down at the deal's end, and why: no clean run in hand (which the
	// table requires to go down), and of those, with set material (a rank
	// in three suits, or two and a joker) it could not use.
	NeverDown, NoCleanRun, NoCleanRunWithSets int
	// Draw decisions the adapter gave one candidate, with the pile open and
	// with it locked, and turns that took from the pile.
	Draws, DrawsOne, OpenDraws, OpenDrawsOne, PileTakes int
	// Discards that ended a turn, and of those the ones the next seat took
	// straight off the pile, and of those the ones it put on the table that
	// same turn — laid in a meld or laid off (or went out with).
	Discards, TakenByNext, FedNext int
}

func (t *tally) add(o *tally) {
	t.SeatDeals += o.SeatDeals
	t.Out += o.Out
	t.Down += o.Down
	t.TurnsToDown = append(t.TurnsToDown, o.TurnsToDown...)
	t.Caught += o.Caught
	t.CaughtDown += o.CaughtDown
	t.Penalty = append(t.Penalty, o.Penalty...)
	t.PenaltyUp = append(t.PenaltyUp, o.PenaltyUp...)
	t.PenaltyDown = append(t.PenaltyDown, o.PenaltyDown...)
	t.JokersHeld = append(t.JokersHeld, o.JokersHeld...)
	t.Match = append(t.Match, o.Match...)
	t.Turns += o.Turns
	t.NeverDown += o.NeverDown
	t.NoCleanRun += o.NoCleanRun
	t.NoCleanRunWithSets += o.NoCleanRunWithSets
	t.Draws += o.Draws
	t.DrawsOne += o.DrawsOne
	t.OpenDraws += o.OpenDraws
	t.OpenDrawsOne += o.OpenDrawsOne
	t.PileTakes += o.PileTakes
	t.Discards += o.Discards
	t.TakenByNext += o.TakenByNext
	t.FedNext += o.FedNext
}

// cleanRun reports whether the hand holds minRun naturals of one suit in
// sequence, the ace at either end.
func cleanRun(hand []string, minRun int) bool {
	bySuit := map[string]map[int]bool{}
	for _, c := range hand {
		if rules.IsJoker(c) {
			continue
		}
		s := rules.CardSuit(c)
		if bySuit[s] == nil {
			bySuit[s] = map[int]bool{}
		}
		if rules.IsAce(c) {
			bySuit[s][1], bySuit[s][14] = true, true
		} else {
			bySuit[s][rules.CardRank(c)+2] = true
		}
	}
	for _, ranks := range bySuit {
		for lo := 1; lo+minRun-1 <= 14; lo++ {
			ok := true
			for r := lo; r < lo+minRun; r++ {
				ok = ok && ranks[r]
			}
			if ok {
				return true
			}
		}
	}
	return false
}

// setMaterial reports a rank in three suits, or in two with a joker to spare.
func setMaterial(hand []string) bool {
	jokers := 0
	suits := map[byte]map[string]bool{}
	for _, c := range hand {
		if rules.IsJoker(c) {
			jokers++
			continue
		}
		if suits[c[0]] == nil {
			suits[c[0]] = map[string]bool{}
		}
		suits[c[0]][rules.CardSuit(c)] = true
	}
	for _, ss := range suits {
		if len(ss) >= 3 || (len(ss) == 2 && jokers > 0) {
			return true
		}
	}
	return false
}

type wire struct {
	Rules rules.GameState `json:"rules"`
}

// watcher follows one match through the states its bots are asked to act in.
type watcher struct {
	isA        map[string]bool
	a, b       *tally
	deal       int
	turns      map[string]int
	downAt     map[string]int
	lastTurn   string
	took       bool
	finalised  map[int]bool
	finalCheck bool
	// feed is the discard that ended the last turn, followed through the
	// next seat's turn.
	feed *feed
}

type feed struct {
	by, to, card string
	onTable      int // copies of card on the table when the discard was made
	taken        bool
}

func tableCopies(gs rules.GameState, card string) int {
	n := 0
	for _, melds := range gs.Melds {
		for _, m := range melds {
			for _, c := range m {
				if c == card {
					n++
				}
			}
		}
	}
	return n
}

// settleFeed closes the followed discard: fed when the seat that took it has
// more copies of it on the table than there were (or went out).
func (w *watcher) settleFeed(gs rules.GameState, wentOut bool) {
	f := w.feed
	w.feed = nil
	if f == nil || !f.taken {
		return
	}
	w.of(f.by).TakenByNext++
	if wentOut || tableCopies(gs, f.card) > f.onTable {
		w.of(f.by).FedNext++
	}
}

func (w *watcher) of(seat string) *tally {
	if w.isA[seat] {
		return w.a
	}
	return w.b
}

func (w *watcher) see(raw module.State) {
	var s wire
	if err := json.Unmarshal(raw, &s); err != nil {
		panic(err)
	}
	gs := s.Rules
	if gs.GameNumber != w.deal {
		w.deal, w.turns, w.downAt, w.lastTurn, w.feed = gs.GameNumber, map[string]int{}, map[string]int{}, "", nil
	}
	if gs.Phase == rules.PhaseIntermission && w.feed != nil && len(gs.DealWinners) >= gs.GameNumber {
		w.settleFeed(gs, gs.DealWinners[gs.GameNumber-1] == w.feed.to)
	}
	if w.feed != nil && gs.CurrentTurn == w.feed.to && gs.DiscardTakenCard == w.feed.card {
		w.feed.taken = true
	}
	if gs.Phase != rules.PhaseIntermission && gs.CurrentTurn != "" && gs.CurrentTurn != w.lastTurn {
		if w.lastTurn != "" {
			w.settleFeed(gs, false)
			if n := len(gs.DiscardPile); n > 0 {
				w.of(w.lastTurn).Discards++
				w.feed = &feed{by: w.lastTurn, to: gs.CurrentTurn, card: gs.DiscardPile[n-1], onTable: tableCopies(gs, gs.DiscardPile[n-1])}
			}
		}
		w.turns[gs.CurrentTurn]++
		w.of(gs.CurrentTurn).Turns++
		w.lastTurn, w.took = gs.CurrentTurn, false
	}
	if gs.Phase != rules.PhaseIntermission && gs.CurrentTurn != "" && gs.DiscardTakenCard != "" && !w.took {
		w.took = true
		w.of(gs.CurrentTurn).PileTakes++
	}
	for _, p := range gs.TurnOrder {
		if _, ok := w.downAt[p]; !ok && gs.RoundReqMet[p] {
			w.downAt[p] = w.turns[p]
		}
	}
	if (gs.Phase == rules.PhaseIntermission || w.finalCheck) && !w.finalised[gs.GameNumber] && len(gs.DealWinners) >= gs.GameNumber {
		w.finalised[gs.GameNumber] = true
		winner := gs.DealWinners[gs.GameNumber-1]
		for _, p := range gs.TurnOrder {
			t := w.of(p)
			t.SeatDeals++
			at, down := w.downAt[p]
			if down {
				t.Down++
				t.TurnsToDown = append(t.TurnsToDown, at)
			}
			if !down {
				t.NeverDown++
				if !cleanRun(gs.Hands[p], gs.Rules.MinRunSize) {
					t.NoCleanRun++
					if setMaterial(gs.Hands[p]) {
						t.NoCleanRunWithSets++
					}
				}
			}
			if p == winner {
				t.Out++
				continue
			}
			pen := 0
			if sc := gs.GameScores[p]; len(sc) >= gs.GameNumber {
				pen = sc[gs.GameNumber-1]
			}
			jokers := 0
			for _, c := range gs.Hands[p] {
				if rules.IsJoker(c) {
					jokers++
				}
			}
			t.Caught++
			t.Penalty = append(t.Penalty, pen)
			t.JokersHeld = append(t.JokersHeld, jokers)
			if down {
				t.CaughtDown++
				t.PenaltyDown = append(t.PenaltyDown, pen)
			} else {
				t.PenaltyUp = append(t.PenaltyUp, pen)
			}
		}
	}
}

func (w *watcher) end(raw module.State) {
	w.finalCheck = true
	w.see(raw)
	var s wire
	_ = json.Unmarshal(raw, &s)
	for p, total := range s.Rules.TotalScores {
		w.of(p).Match = append(w.of(p).Match, total)
	}
}

// observed passes every position it is asked about to the watcher first.
type observed struct {
	bot module.Bot
	w   *watcher
	g   learn.Game
}

func (o observed) Act(s module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	o.w.see(s)
	var st wire
	if err := json.Unmarshal(s, &st); err == nil && st.Rules.Phase == rules.PhaseDraw && st.Rules.CurrentTurn == seat.PlayerID {
		if cands, err := o.g.Candidates(s, seat.PlayerID, offers); err == nil {
			t := o.w.of(seat.PlayerID)
			t.Draws++
			open := !rules.IsDiscardLocked(st.Rules.Round, st.Rules.Rules.DiscardDrawMinRound)
			if open {
				t.OpenDraws++
			}
			if len(cands) == 1 {
				t.DrawsOne++
				if open {
					t.OpenDrawsOne++
				}
			}
		}
	}
	return o.bot.Act(s, seat, offers)
}

// play plays one match, A in the seats isA names, and returns what it saw.
func play(g learn.Benchable, cfg module.MatchConfig, players []module.PlayerRef, seed int64, budget int,
	isA map[string]bool, ca, cb learn.Contender) (*watcher, learn.PlayStats, error) {
	w := &watcher{isA: isA, a: &tally{}, b: &tally{}, finalised: map[int]bool{}}
	lg := g.(learn.Game)
	final, st, err := learn.PlayOut(g.Module(), cfg, players, seed, budget, func(id string) (module.Bot, module.Skill) {
		if w.isA[id] {
			return observed{ca.Bot, w, lg}, ca.Skill
		}
		return observed{cb.Bot, w, lg}, cb.Skill
	})
	if err == nil && !st.Stalled {
		w.end(final)
	}
	return w, st, err
}

func main() {
	variation := flag.String("variation", "zolik_classic+floor35", "variation")
	seats := flag.Int("seats", 4, "seats at the table")
	aSeats := flag.Int("a-seats", 1, "seats A plays, B the rest")
	a := flag.String("a", "hard", "contender A")
	b := flag.String("b", "hard", "contender B")
	first := flag.Int64("first", 1_000_000, "first seed")
	seeds := flag.Int("seeds", 100, "seeds, each played once per rotation")
	budget := flag.Int("actions", 50_000, "action budget per match")
	flag.Parse()
	if *aSeats < 1 || *aSeats >= *seats {
		fail(fmt.Errorf("-a-seats must be 1..%d", *seats-1))
	}

	g, err := learn.Lookup("zolik")
	if err != nil {
		fail(err)
	}
	ca, err := learn.ParseContender(g, *a)
	if err != nil {
		fail(err)
	}
	cb, err := learn.ParseContender(g, *b)
	if err != nil {
		fail(err)
	}

	var mu sync.Mutex
	var ta, tb tally
	var illegal, stalls int
	jobs := make(chan int64)
	var wg sync.WaitGroup
	for i := 0; i < runtime.GOMAXPROCS(0); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for seed := range jobs {
				players := learn.Players(*seats)
				cfg := g.Config(*seats, *variation)
				for r := 0; r < *seats; r++ {
					isA := map[string]bool{}
					for j := 0; j < *aSeats; j++ {
						isA[players[(r+j)%*seats].ID] = true
					}
					w, st, err := play(g, cfg, players, seed, *budget, isA, ca, cb)
					if err != nil {
						fail(err)
					}
					mu.Lock()
					illegal += st.Illegal
					if st.Stalled {
						stalls++
					} else {
						ta.add(w.a)
						tb.add(w.b)
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

	fmt.Printf("zolik %d seats %s, A=%s in %d seats, B=%s; %d seeds x %d rotations (illegal %d, stalls %d)\n",
		*seats, *variation, *a, *aSeats, *b, *seeds, *seats, illegal, stalls)
	fmt.Printf("%-3s %9s %6s %6s %8s %7s %6s %11s %9s %10s %10s %9s %10s %9s\n",
		"", "seatdeals", "out%", "down%", "never%", "toDown", "med", "caughtPen", "penUp", "penDown", "jokers", "caughtJ%", "matchPen", "turns/d")
	for _, row := range []struct {
		name string
		t    *tally
	}{{"A", &ta}, {"B", &tb}} {
		t := row.t
		withJ := 0
		for _, j := range t.JokersHeld {
			if j > 0 {
				withJ++
			}
		}
		fmt.Printf("%-3s %9d %6.1f %6.1f %8.1f %7s %6.1f %11s %9s %10s %10s %9.1f %10s %9.1f\n",
			row.name, t.SeatDeals, pct(t.Out, t.SeatDeals), pct(t.Down, t.SeatDeals), 100-pct(t.Down, t.SeatDeals),
			ms(t.TurnsToDown), median(t.TurnsToDown), ms(t.Penalty), ms(t.PenaltyUp), ms(t.PenaltyDown),
			ms(t.JokersHeld), pct(withJ, len(t.JokersHeld)), ms(t.Match), float64(t.Turns)/math.Max(1, float64(t.SeatDeals)))
	}
	fmt.Printf("%-3s %8s %11s %13s %8s %9s %11s %10s %9s %9s %9s\n", "", "never%", "noClean%nv", "noClean+set%", "draws", "1-cand%", "open1-cand%", "pileTake%", "takes/d", "nextTook%", "fedNext%")
	for _, row := range []struct {
		name string
		t    *tally
	}{{"A", &ta}, {"B", &tb}} {
		t := row.t
		fmt.Printf("%-3s %8.1f %11.1f %13.1f %8d %9.1f %11.1f %10.1f %9.2f %9.2f %9.2f\n", row.name,
			pct(t.NeverDown, t.SeatDeals), pct(t.NoCleanRun, t.NeverDown), pct(t.NoCleanRunWithSets, t.NeverDown),
			t.Draws, pct(t.DrawsOne, t.Draws), pct(t.OpenDrawsOne, t.OpenDraws), pct(t.PileTakes, t.Turns),
			float64(t.PileTakes)/math.Max(1, float64(t.SeatDeals)), pct(t.TakenByNext, t.Discards), pct(t.FedNext, t.Discards))
	}
	if illegal+stalls > 0 {
		os.Exit(1)
	}
}

func pct(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return 100 * float64(n) / float64(d)
}

// ms is mean±standard error.
func ms(xs []int) string {
	if len(xs) == 0 {
		return "-"
	}
	m := 0.0
	for _, x := range xs {
		m += float64(x)
	}
	m /= float64(len(xs))
	v := 0.0
	for _, x := range xs {
		v += (float64(x) - m) * (float64(x) - m)
	}
	se := 0.0
	if len(xs) > 1 {
		se = math.Sqrt(v / float64(len(xs)-1) / float64(len(xs)))
	}
	return fmt.Sprintf("%.2f±%.2f", m, se)
}

func median(xs []int) float64 {
	if len(xs) == 0 {
		return 0
	}
	c := append([]int(nil), xs...)
	sort.Ints(c)
	return float64(c[len(c)/2])
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "zolikbehave:", err)
	os.Exit(2)
}
