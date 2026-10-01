// Command canastabehave says how a Canasta bot plays, not only how well: how
// soon a deal ends and who ends it, whether the stock runs out, how many
// canastas each side makes and of which kind, how often it takes the pile and
// spends its wilds, and — the question it was written for — what a seat is
// left holding when somebody else goes out.
//
//	go run ./cmd/canastabehave -seats 2 -a net:final.bin -b closer
//	go run ./cmd/canastabehave -seats 4 -a net:final.bin -partner hard -b closer
//
// Seats alternate between the sides by partnership, and every seed is played
// twice with the sides swapped, as gamebench does: whole matches on seeds from
// -first. -partner seats a third contender beside A at a partnership table
// (A's first seat is A, the rest of its side is the partner), so a model can
// be measured with a heuristic partner. The match line is A's side minus B's,
// per match, with its standard error — gamebench's number — and the rest is
// per seat and deal, split by contender, and per side and deal.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"runtime"
	"sort"
	"sync"

	"zolik/server/internal/canasta"
	"zolik/server/internal/learn"
	"zolik/server/internal/module"
)

// Contender labels. P is A's partner when -partner is set.
const (
	labelA = "A"
	labelP = "P"
	labelB = "B"
)

// seatTally is one contender's seat-deals.
type seatTally struct {
	SeatDeals int
	Turns     int   // own turns begun
	Out       int   // deals this seat went out of
	OutTurn   []int // own turn it went out on
	// OpenTurn and QuotaTurn are the own turn on which this seat first saw
	// its side opened, and holding the canastas it needs to go out: at the
	// end of that turn, as the next position it acted in showed it.
	OpenTurn    []int
	QuotaTurn   []int
	PileTakes   int
	WildsMelded int // wild cards this seat put on the table (melds, lay-offs, captures)
	WildsLeft   []int
	// Caught is a deal somebody else went out of; its cards and points are
	// what this seat still held.
	Caught       int
	CaughtCards  []int
	CaughtPoints []int
	// Exhausted is a deal the stock ended, with what this seat held.
	Exhausted       int
	ExhaustedCards  []int
	ExhaustedPoints []int
}

func (t *seatTally) add(o *seatTally) {
	t.SeatDeals += o.SeatDeals
	t.Turns += o.Turns
	t.Out += o.Out
	t.OutTurn = append(t.OutTurn, o.OutTurn...)
	t.OpenTurn = append(t.OpenTurn, o.OpenTurn...)
	t.QuotaTurn = append(t.QuotaTurn, o.QuotaTurn...)
	t.PileTakes += o.PileTakes
	t.WildsMelded += o.WildsMelded
	t.WildsLeft = append(t.WildsLeft, o.WildsLeft...)
	t.Caught += o.Caught
	t.CaughtCards = append(t.CaughtCards, o.CaughtCards...)
	t.CaughtPoints = append(t.CaughtPoints, o.CaughtPoints...)
	t.Exhausted += o.Exhausted
	t.ExhaustedCards = append(t.ExhaustedCards, o.ExhaustedCards...)
	t.ExhaustedPoints = append(t.ExhaustedPoints, o.ExhaustedPoints...)
}

// How a deal ended, from one side's point of view.
const (
	endOwn       = iota // this side went out
	endOther            // another side went out
	endExhausted        // the stock ran out
	endKinds
)

var endNames = [endKinds]string{"own out", "other out", "exhausted"}

// Buckets of the out-goer's own turns, for deals another side ended.
var turnBuckets = []struct {
	name   string
	lo, hi int
}{{"1-5", 1, 5}, {"6-10", 6, 10}, {"11-15", 11, 15}, {"16+", 16, 1 << 30}}

// sideTally is one side's deals: the score breakdown, and the deal's net
// (this side's deal total minus the best other side's), by how it ended.
type sideTally struct {
	Deals              int
	Naturals, Mixed    int
	MeldCards          []int
	Canastas           []int
	RedThrees          []int
	GoingOut           []int
	InHand             []int
	Total              []int
	Net                []int
	NetBy              [endKinds][]int
	InHandBy           [endKinds][]int
	OtherOutNet        [][]int // by turnBuckets
	OtherOutHandPoints [][]int
}

func newSideTally() *sideTally {
	return &sideTally{OtherOutNet: make([][]int, len(turnBuckets)), OtherOutHandPoints: make([][]int, len(turnBuckets))}
}

func (t *sideTally) add(o *sideTally) {
	t.Deals += o.Deals
	t.Naturals += o.Naturals
	t.Mixed += o.Mixed
	t.MeldCards = append(t.MeldCards, o.MeldCards...)
	t.Canastas = append(t.Canastas, o.Canastas...)
	t.RedThrees = append(t.RedThrees, o.RedThrees...)
	t.GoingOut = append(t.GoingOut, o.GoingOut...)
	t.InHand = append(t.InHand, o.InHand...)
	t.Total = append(t.Total, o.Total...)
	t.Net = append(t.Net, o.Net...)
	for k := range t.NetBy {
		t.NetBy[k] = append(t.NetBy[k], o.NetBy[k]...)
		t.InHandBy[k] = append(t.InHandBy[k], o.InHandBy[k]...)
	}
	for i := range turnBuckets {
		t.OtherOutNet[i] = append(t.OtherOutNet[i], o.OtherOutNet[i]...)
		t.OtherOutHandPoints[i] = append(t.OtherOutHandPoints[i], o.OtherOutHandPoints[i]...)
	}
}

// tableTally is whole-deal numbers.
type tableTally struct {
	Deals      int
	Exhausted  int
	OutTurns   []int // table turns before someone went out, in deals that ended so
	Turns      []int // table turns in every deal
	DealsMatch []int
}

func (t *tableTally) add(o *tableTally) {
	t.Deals += o.Deals
	t.Exhausted += o.Exhausted
	t.OutTurns = append(t.OutTurns, o.OutTurns...)
	t.Turns = append(t.Turns, o.Turns...)
	t.DealsMatch = append(t.DealsMatch, o.DealsMatch...)
}

// report is everything one or many matches produced.
type report struct {
	Seat  map[string]*seatTally
	Side  map[string]*sideTally
	Table tableTally
}

func newReport() *report {
	return &report{
		Seat: map[string]*seatTally{labelA: {}, labelP: {}, labelB: {}},
		Side: map[string]*sideTally{labelA: newSideTally(), labelB: newSideTally()},
	}
}

func (r *report) add(o *report) {
	for k, v := range o.Seat {
		r.Seat[k].add(v)
	}
	for k, v := range o.Side {
		r.Side[k].add(v)
	}
	r.Table.add(&o.Table)
}

// dealWatch is what can only be seen during a deal: turns, and what each
// seat's own moves spent.
type dealWatch struct {
	turns      map[string]int
	tableTurns int
	last       string
	pileTakes  map[string]int
	wilds      map[string]int
	opened     map[string]int // own turn the seat's side was first seen opened
	quota      map[string]int // ... and first seen with its canastas
}

// watcher follows one match through the positions its bots are asked about
// and the moves they make.
type watcher struct {
	label map[string]string // seat -> A, P or B
	side  map[string]string // seat -> A or B
	deals map[int]*dealWatch
}

func newWatcher(label, side map[string]string) *watcher {
	return &watcher{label: label, side: side, deals: map[int]*dealWatch{}}
}

func (w *watcher) deal(n int) *dealWatch {
	d := w.deals[n]
	if d == nil {
		d = &dealWatch{turns: map[string]int{}, pileTakes: map[string]int{}, wilds: map[string]int{},
			opened: map[string]int{}, quota: map[string]int{}}
		w.deals[n] = d
	}
	return d
}

// see notes a position a seat is about to act in, and the move it made there.
func (w *watcher) see(raw module.State, seat string, a module.Action, ok bool) {
	var s canasta.GameState
	if err := json.Unmarshal(raw, &s); err != nil || s.Status != "active" || s.Break.Open || s.Current != seat {
		return
	}
	d := w.deal(s.DealNumber)
	if s.Current != d.last {
		d.last = s.Current
		d.turns[seat]++
		d.tableTurns++
	}
	for i := range s.Teams {
		t := &s.Teams[i]
		if s.TeamOf[seat] != t.ID {
			continue
		}
		if _, seen := d.opened[seat]; !seen && t.HasMelded {
			d.opened[seat] = d.turns[seat]
		}
		if _, seen := d.quota[seat]; !seen && canastas(t) >= s.CanastasToGoOut {
			d.quota[seat] = d.turns[seat]
		}
	}
	if !ok {
		return
	}
	switch a.Verb {
	case canasta.VerbTakePile:
		d.pileTakes[seat]++
		d.wilds[seat] += countWilds(a.Cards)
	case canasta.VerbLayMeld, canasta.VerbLayOff, canasta.VerbTakeTop:
		d.wilds[seat] += countWilds(a.Cards)
	}
}

func canastas(t *canasta.Team) int {
	n := 0
	for _, m := range t.Melds {
		if len(m.Cards) >= 7 {
			n++
		}
	}
	return n
}

func countWilds(cards []string) int {
	n := 0
	for _, c := range cards {
		if isWild(c) {
			n++
		}
	}
	return n
}

// isWild is Canasta's: jokers and deuces.
func isWild(c string) bool {
	return len(c) > 0 && (c[0] == '2' || (len(c) >= 5 && c[:5] == "JOKER"))
}

// finish reads the finished match's deals into a report.
func (w *watcher) finish(raw module.State) (*report, error) {
	var s canasta.GameState
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	out := newReport()
	teamSide := map[int]string{}
	teamPlayers := map[int][]string{}
	for _, t := range s.Teams {
		teamPlayers[t.ID] = t.Players
		if len(t.Players) > 0 {
			teamSide[t.ID] = w.side[t.Players[0]]
		}
	}
	teamOfSeat := map[string]int{}
	for id, t := range teamPlayers {
		for _, p := range t {
			teamOfSeat[p] = id
		}
	}
	out.Table.DealsMatch = append(out.Table.DealsMatch, len(s.Deals))
	for _, dr := range s.Deals {
		d := w.deal(dr.DealNumber)
		out.Table.Deals++
		out.Table.Turns = append(out.Table.Turns, d.tableTurns)
		if dr.Exhausted {
			out.Table.Exhausted++
		}
		if dr.WentOut != "" {
			out.Table.OutTurns = append(out.Table.OutTurns, d.tableTurns)
		}
		outTeam := -1
		if dr.WentOut != "" {
			outTeam = teamOfSeat[dr.WentOut]
		}
		totals := map[int]int{}
		for _, tr := range dr.Teams {
			totals[tr.TeamID] = tr.Total
		}
		held := map[string]canasta.HandTally{}
		for _, tr := range dr.Teams {
			for _, h := range tr.Hands {
				held[h.PlayerID] = h
			}
		}
		// Per side.
		for _, tr := range dr.Teams {
			st := out.Side[teamSide[tr.TeamID]]
			if st == nil {
				continue
			}
			best := math.MinInt
			for id, v := range totals {
				if id != tr.TeamID && v > best {
					best = v
				}
			}
			net := tr.Total - best
			st.Deals++
			st.Naturals += tr.Naturals
			st.Mixed += tr.Mixed
			st.MeldCards = append(st.MeldCards, tr.MeldCards)
			st.Canastas = append(st.Canastas, tr.Canastas)
			st.RedThrees = append(st.RedThrees, tr.RedThrees)
			st.GoingOut = append(st.GoingOut, tr.GoingOut)
			st.InHand = append(st.InHand, tr.InHand)
			st.Total = append(st.Total, tr.Total)
			st.Net = append(st.Net, net)
			kind := endExhausted
			switch {
			case dr.WentOut != "" && outTeam == tr.TeamID:
				kind = endOwn
			case dr.WentOut != "":
				kind = endOther
			}
			st.NetBy[kind] = append(st.NetBy[kind], net)
			st.InHandBy[kind] = append(st.InHandBy[kind], tr.InHand)
			if kind == endOther {
				turn := d.turns[dr.WentOut]
				for i, b := range turnBuckets {
					if turn >= b.lo && turn <= b.hi {
						st.OtherOutNet[i] = append(st.OtherOutNet[i], net)
						st.OtherOutHandPoints[i] = append(st.OtherOutHandPoints[i], tr.InHand)
					}
				}
			}
		}
		// Per seat.
		for _, p := range s.TurnOrder {
			t := out.Seat[w.label[p]]
			if t == nil {
				continue
			}
			h := held[p]
			t.SeatDeals++
			t.Turns += d.turns[p]
			t.PileTakes += d.pileTakes[p]
			t.WildsMelded += d.wilds[p]
			t.WildsLeft = append(t.WildsLeft, h.Wilds)
			if at, ok := d.opened[p]; ok {
				t.OpenTurn = append(t.OpenTurn, at)
			}
			if at, ok := d.quota[p]; ok {
				t.QuotaTurn = append(t.QuotaTurn, at)
			}
			switch {
			case dr.WentOut == p:
				t.Out++
				t.OutTurn = append(t.OutTurn, d.turns[p])
			case dr.WentOut != "":
				t.Caught++
				t.CaughtCards = append(t.CaughtCards, h.Cards)
				t.CaughtPoints = append(t.CaughtPoints, h.Points)
			default:
				t.Exhausted++
				t.ExhaustedCards = append(t.ExhaustedCards, h.Cards)
				t.ExhaustedPoints = append(t.ExhaustedPoints, h.Points)
			}
		}
	}
	return out, nil
}

// observed passes every position and move through the watcher.
type observed struct {
	bot module.Bot
	w   *watcher
}

func (o observed) Act(s module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	a, ok := o.bot.Act(s, seat, offers)
	o.w.see(s, seat.PlayerID, a, ok)
	return a, ok
}

// seating assigns labels: sides alternate by partnership (flipped on the
// second playing of a seed), and within A's side the first seat is A and the
// rest are the partner when one is given.
func seating(players []module.PlayerRef, sides [][]string, flip, partner bool) (label, side map[string]string) {
	label, side = map[string]string{}, map[string]string{}
	if len(sides) == 0 {
		for _, p := range players {
			sides = append(sides, []string{p.ID})
		}
	}
	for i, ids := range sides {
		s := labelB
		if (i%2 == 0) != flip {
			s = labelA
		}
		for j, id := range ids {
			side[id] = s
			label[id] = s
			if s == labelA && j > 0 && partner {
				label[id] = labelP
			}
		}
	}
	return label, side
}

// play plays one match and returns its report, plus A's side's outcome minus
// B's.
func play(g learn.Game, seats int, variation string, seed int64, budget int, flip bool,
	bots map[string]learn.Contender, partner bool) (*report, float64, learn.PlayStats, error) {
	players := learn.Players(seats)
	cfg := g.Config(seats, variation)
	label, side := seating(players, module.SidesOf(g.Module(), cfg, players), flip, partner)
	w := newWatcher(label, side)
	final, st, err := learn.PlayOut(g.Module(), cfg, players, seed, budget, func(id string) (module.Bot, module.Skill) {
		c := bots[label[id]]
		return observed{bot: c.Bot, w: w}, c.Skill
	})
	if err != nil || st.Stalled {
		return nil, 0, st, err
	}
	rep, err := w.finish(final)
	if err != nil {
		return nil, 0, st, err
	}
	var sumA, sumB float64
	var nA, nB int
	for _, p := range players {
		v, err := g.Outcome(final, p.ID)
		if err != nil {
			return nil, 0, st, err
		}
		if side[p.ID] == labelA {
			sumA, nA = sumA+v, nA+1
		} else {
			sumB, nB = sumB+v, nB+1
		}
	}
	diff := 0.0
	if nA > 0 && nB > 0 {
		diff = sumA/float64(nA) - sumB/float64(nB)
	}
	return rep, diff, st, nil
}

// run plays seeds [first, first+n), each twice with the sides swapped.
func run(g learn.Game, seats int, variation string, bots map[string]learn.Contender, partner bool,
	first int64, n, budget int) (*report, []float64, int, int, error) {
	total := newReport()
	samples := make([]float64, n)
	valid := make([]bool, n)
	var mu sync.Mutex
	var illegal, stalls int
	var firstErr error
	jobs := make(chan int)
	var wg sync.WaitGroup
	for i := 0; i < runtime.GOMAXPROCS(0); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				seed := first + int64(i)
				sample, ok := 0.0, true
				for _, flip := range []bool{false, true} {
					rep, diff, st, err := play(g, seats, variation, seed, budget, flip, bots, partner)
					mu.Lock()
					illegal += st.Illegal
					if err != nil && firstErr == nil {
						firstErr = fmt.Errorf("seed %d: %w", seed, err)
					}
					if st.Stalled {
						stalls++
						ok = false
					} else if rep != nil {
						total.add(rep)
					}
					mu.Unlock()
					sample += diff / 2
				}
				samples[i], valid[i] = sample, ok
			}
		}()
	}
	for i := 0; i < n; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	var out []float64
	for i, ok := range valid {
		if ok {
			out = append(out, samples[i])
		}
	}
	return total, out, illegal, stalls, firstErr
}

func main() {
	variation := flag.String("variation", "classic", "variation")
	seats := flag.Int("seats", 2, "seats at the table")
	a := flag.String("a", "hard", "contender A")
	b := flag.String("b", "closer", "contender B")
	partner := flag.String("partner", "", "A's partner at a partnership table (default: A)")
	first := flag.Int64("first", 1_000_000, "first seed")
	seeds := flag.Int("seeds", 100, "seeds, each played in both seatings")
	budget := flag.Int("actions", 50_000, "action budget per match")
	flag.Parse()

	g, err := learn.Lookup("canasta")
	if err != nil {
		fail(err)
	}
	lg, ok := g.(learn.Game)
	if !ok {
		fail(fmt.Errorf("canasta is not a learn.Game"))
	}
	bots := map[string]learn.Contender{}
	for label, spec := range map[string]string{labelA: *a, labelB: *b} {
		c, err := learn.ParseContender(g, spec)
		if err != nil {
			fail(err)
		}
		bots[label] = c
	}
	bots[labelP] = bots[labelA]
	if *partner != "" {
		c, err := learn.ParseContender(g, *partner)
		if err != nil {
			fail(err)
		}
		bots[labelP] = c
	}
	rep, samples, illegal, stalls, err := run(lg, *seats, *variation, bots, *partner != "", *first, *seeds, *budget)
	if err != nil {
		fail(err)
	}
	names := map[string]string{labelA: *a, labelP: *partner, labelB: *b}
	printReport(os.Stdout, rep, samples, names, *seats, *variation, illegal, stalls)
	if illegal+stalls > 0 {
		os.Exit(1)
	}
}

func printReport(w io.Writer, rep *report, samples []float64, names map[string]string, seats int, variation string, illegal, stalls int) {
	m, se := meanErr(samples)
	fmt.Fprintf(w, "canasta %s %d seats: A=%s", variation, seats, names[labelA])
	if names[labelP] != "" {
		fmt.Fprintf(w, " (partner %s)", names[labelP])
	}
	fmt.Fprintf(w, ", B=%s; %d seeds x 2 seatings (illegal %d, stalls %d)\n", names[labelB], len(samples), illegal, stalls)
	fmt.Fprintf(w, "match: A side minus B side %+.0f ± %.0f points\n", m, se)
	t := rep.Table
	fmt.Fprintf(w, "table: %d deals, %s deals/match, exhausted %.1f%%, table turns/deal %s, table turns to go out %s (median %.0f)\n\n",
		t.Deals, msInt(t.DealsMatch), pct(t.Exhausted, t.Deals), msInt(t.Turns), msInt(t.OutTurns), median(t.OutTurns))

	fmt.Fprintln(w, "per seat and deal")
	fmt.Fprintf(w, "%-3s %6s %12s %12s %6s %13s %7s %9s %9s %11s %12s %13s %7s %11s %12s\n",
		"", "deals", "openTurn", "quotaTurn", "out%", "outOnTurn", "turns/d", "takes/d", "wildsMld", "wildsLeft", "caught%", "caughtCards", "caughtPts", "exhCards", "exhPts")
	for _, l := range []string{labelA, labelP, labelB} {
		s := rep.Seat[l]
		if s.SeatDeals == 0 {
			continue
		}
		d := float64(s.SeatDeals)
		fmt.Fprintf(w, "%-3s %6d %12s %12s %6.1f %13s %7.1f %9.2f %9.2f %11s %12.1f %13s %7s %11s %12s\n",
			l, s.SeatDeals, msInt(s.OpenTurn), msInt(s.QuotaTurn), pct(s.Out, s.SeatDeals), msInt(s.OutTurn), float64(s.Turns)/d, float64(s.PileTakes)/d,
			float64(s.WildsMelded)/d, msInt(s.WildsLeft), pct(s.Caught, s.SeatDeals), msInt(s.CaughtCards), meanStr(s.CaughtPoints),
			msInt(s.ExhaustedCards), msInt(s.ExhaustedPoints))
	}

	fmt.Fprintln(w, "\nper side and deal (score breakdown)")
	fmt.Fprintf(w, "%-3s %6s %9s %8s %14s %14s %12s %12s %13s %14s %14s\n",
		"", "deals", "natural/d", "mixed/d", "melds", "canastaBonus", "redThrees", "goingOut", "minusHand", "total", "net")
	for _, l := range []string{labelA, labelB} {
		s := rep.Side[l]
		if s.Deals == 0 {
			continue
		}
		d := float64(s.Deals)
		fmt.Fprintf(w, "%-3s %6d %9.2f %8.2f %14s %14s %12s %12s %13s %14s %14s\n",
			l, s.Deals, float64(s.Naturals)/d, float64(s.Mixed)/d, msInt(s.MeldCards), msInt(s.Canastas),
			msInt(s.RedThrees), msInt(s.GoingOut), msInt(s.InHand), msInt(s.Total), msInt(s.Net))
	}

	fmt.Fprintln(w, "\nper side, by how the deal ended: share, net (own total minus the other side's), hand penalty")
	for _, l := range []string{labelA, labelB} {
		s := rep.Side[l]
		if s.Deals == 0 {
			continue
		}
		for k := 0; k < endKinds; k++ {
			fmt.Fprintf(w, "%-3s %-10s %5.1f%%  net %14s  hand %13s\n", l, endNames[k],
				pct(len(s.NetBy[k]), s.Deals), msInt(s.NetBy[k]), msInt(s.InHandBy[k]))
		}
	}

	fmt.Fprintln(w, "\nper side, deals the other side went out of, by the out-goer's own turn")
	for _, l := range []string{labelA, labelB} {
		s := rep.Side[l]
		if s.Deals == 0 {
			continue
		}
		for i, bk := range turnBuckets {
			fmt.Fprintf(w, "%-3s turn %-6s %5.1f%% of deals  net %14s  hand %13s\n", l, bk.name,
				pct(len(s.OtherOutNet[i]), s.Deals), msInt(s.OtherOutNet[i]), msInt(s.OtherOutHandPoints[i]))
		}
	}
}

func pct(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return 100 * float64(n) / float64(d)
}

func toFloats(xs []int) []float64 {
	out := make([]float64, len(xs))
	for i, x := range xs {
		out[i] = float64(x)
	}
	return out
}

// msInt is mean±standard error, or "-" for nothing.
func msInt(xs []int) string {
	if len(xs) == 0 {
		return "-"
	}
	m, se := meanErr(toFloats(xs))
	if math.Abs(m) >= 100 {
		return fmt.Sprintf("%.0f±%.0f", m, se)
	}
	return fmt.Sprintf("%.2f±%.2f", m, se)
}

func meanStr(xs []int) string {
	if len(xs) == 0 {
		return "-"
	}
	m, _ := meanErr(toFloats(xs))
	return fmt.Sprintf("%.0f", m)
}

func meanErr(xs []float64) (mean, stderr float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	for _, x := range xs {
		mean += x
	}
	mean /= float64(len(xs))
	if len(xs) < 2 {
		return mean, 0
	}
	v := 0.0
	for _, x := range xs {
		v += (x - mean) * (x - mean)
	}
	return mean, math.Sqrt(v / float64(len(xs)-1) / float64(len(xs)))
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
	fmt.Fprintln(os.Stderr, "canastabehave:", err)
	os.Exit(2)
}
