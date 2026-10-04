package holdem

import (
	"fmt"
	"go/format"
	"math"
	"math/rand"
	"runtime"
	"strings"
	"sync"
)

// The solver behind pushfold_table.go.
//
// Heads-up jam-or-fold, the game Miltersen & Sørensen (AAMAS 2007) solved for
// the heads-up tournament: the small blind may only go all in or fold, and the
// big blind, facing the all-in, may only call or fold. Payoffs are chips, in
// big blinds — this module scores matches in chips, so there is no tournament
// prize structure to convert through. Every stack is the effective one, blinds
// included, with a small blind of half a big blind and no antes.
//
// Two stages, both deterministic:
//
//  1. An equity matrix over the 169 starting-hand classes, with the card
//     removal between them: for every pair of classes, every pair of combos
//     that can be dealt together is played against random boards — the same
//     number of boards for each combo pair, from a source seeded by the pair of
//     classes, so the matrix is a function of nothing but the constants here.
//     How many combo pairs can be dealt together is also the weight of that
//     pair of classes in the game.
//  2. CFR+ (Tammelin 2014) on the resulting two-decision game at each stack on
//     the grid, with linearly weighted averaging. The average strategy is the
//     answer; its exploitability is reported by the generator so a chart that
//     has not converged cannot be mistaken for one that has.
//
// The chart keeps, for each class, the deepest stack on the grid up to which
// it jams (or calls) at every grid point — "jam up to 12.5 big blinds", the
// form these charts are published in. A class that plays at some stacks and
// not at shallower ones is cut at the first gap, which is the conservative
// reading of a mixed or non-monotone row.

// pushFoldGrid is every stack the chart is solved at, in big blinds: half-blind
// steps up to twenty, where the decisions are, and whole ones beyond it to
// fifty. Beyond twenty the jam-or-fold game is not the game being played; the
// rows there exist to rank hands for the multi-way extension (pushfold.go).
func pushFoldGrid() []float64 {
	var g []float64
	for s := 1.0; s <= 20; s += 0.5 {
		g = append(g, s)
	}
	for s := 21.0; s <= 50; s++ {
		g = append(g, s)
	}
	return g
}

const (
	pushFoldBoards     = 3000 // boards per pair of classes, spread over its combo pairs
	pushFoldIterations = 3000 // CFR+ iterations per stack
	pushFoldSeed       = 20070514
	// pushFoldBeyond marks a class that still plays at the deepest stack on
	// the grid.
	pushFoldBeyond = 255
)

// handClasses are the 169 starting hands, indexed on a 13x13 grid the way
// charts print them: row the first rank, column the second, aces first; the
// diagonal is the pairs, above it suited, below it offsuit.
type handClass struct {
	hi, lo int // rank index, 0 = two .. 12 = ace
	suited bool
}

func classAt(i int) handClass {
	row, col := 12-i/13, 12-i%13
	switch {
	case row == col:
		return handClass{hi: row, lo: row}
	case row > col:
		return handClass{hi: row, lo: col, suited: true}
	default:
		return handClass{hi: col, lo: row}
	}
}

// classOf is the grid index of two hole cards.
func classOf(hole []string) int {
	a, b := codeOf(hole[0]), codeOf(hole[1])
	return classIndex(int(a>>2), int(b>>2), a&3 == b&3)
}

func classIndex(r1, r2 int, suited bool) int {
	hi, lo := r1, r2
	if lo > hi {
		hi, lo = lo, hi
	}
	if hi == lo || suited {
		return (12-hi)*13 + (12 - lo)
	}
	return (12-lo)*13 + (12 - hi)
}

func (c handClass) String() string {
	const names = "23456789TJQKA"
	out := string(names[c.hi]) + string(names[c.lo])
	switch {
	case c.hi == c.lo:
	case c.suited:
		out += "s"
	default:
		out += "o"
	}
	return out
}

// combos is every way of being dealt the class.
func (c handClass) combos() [][2]code {
	var out [][2]code
	for s1 := 0; s1 < 4; s1++ {
		for s2 := 0; s2 < 4; s2++ {
			switch {
			case c.hi == c.lo && s2 <= s1:
				continue
			case c.hi != c.lo && c.suited && s1 != s2:
				continue
			case c.hi != c.lo && !c.suited && s1 == s2:
				continue
			}
			out = append(out, [2]code{code(c.hi*4 + s1), code(c.lo*4 + s2)})
		}
	}
	return out
}

// pushFoldEquities is the 169x169 all-in equity matrix and the number of combo
// pairs behind each entry.
func pushFoldEquities() (eq, weight [169][169]float64) {
	type job struct{ i, j int }
	jobs := make(chan job)
	var wg sync.WaitGroup
	for w := 0; w < runtime.GOMAXPROCS(0); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for jb := range jobs {
				e, n := classEquity(jb.i, jb.j)
				eq[jb.i][jb.j], eq[jb.j][jb.i] = e, 1-e
				weight[jb.i][jb.j], weight[jb.j][jb.i] = n, n
			}
		}()
	}
	for i := 0; i < 169; i++ {
		for j := i; j < 169; j++ {
			jobs <- job{i, j}
		}
	}
	close(jobs)
	wg.Wait()
	return eq, weight
}

// classEquity is class i's all-in equity against class j, and how many combo
// pairs of the two can be dealt together.
func classEquity(i, j int) (float64, float64) {
	var pairs [][4]code
	for _, a := range classAt(i).combos() {
		for _, b := range classAt(j).combos() {
			if a[0] != b[0] && a[0] != b[1] && a[1] != b[0] && a[1] != b[1] {
				pairs = append(pairs, [4]code{a[0], a[1], b[0], b[1]})
			}
		}
	}
	if len(pairs) == 0 {
		return 0.5, 0
	}
	rnd := rand.New(rand.NewSource(pushFoldSeed + int64(i)*1000 + int64(j)))
	per := (pushFoldBoards + len(pairs) - 1) / len(pairs)
	var won float64
	var deck [52]code
	mine, theirs := make([]code, 7), make([]code, 7)
	for _, p := range pairs {
		n := 0
		for c := code(0); c < 52; c++ {
			if c != p[0] && c != p[1] && c != p[2] && c != p[3] {
				deck[n] = c
				n++
			}
		}
		for t := 0; t < per; t++ {
			for k := 0; k < 5; k++ {
				r := k + rnd.Intn(n-k)
				deck[k], deck[r] = deck[r], deck[k]
			}
			mine[0], mine[1] = p[0], p[1]
			theirs[0], theirs[1] = p[2], p[3]
			copy(mine[2:], deck[:5])
			copy(theirs[2:], deck[:5])
			switch a, b := score7(mine), score7(theirs); {
			case a > b:
				won++
			case a == b:
				won += 0.5
			}
		}
	}
	return won / float64(per*len(pairs)), float64(len(pairs))
}

// pushFoldSolution is the jam and call probabilities at one stack, and how far
// from an equilibrium they are, in big blinds per hand.
type pushFoldSolution struct {
	jam, call      [169]float64
	exploitability float64
}

// solvePushFold runs CFR+ on the jam-or-fold game at a stack of s big blinds.
func solvePushFold(eq, weight *[169][169]float64, s float64) pushFoldSolution {
	var rJam, rFold, rCall, rPass [169]float64
	var avgJam, avgCall [169]float64
	var jam, call [169]float64
	norm := 0.0
	strat := func(pos, neg float64) float64 {
		if pos+neg <= 0 {
			return 0.5
		}
		return pos / (pos + neg)
	}
	for it := 1; it <= pushFoldIterations; it++ {
		for i := 0; i < 169; i++ {
			jam[i] = strat(rJam[i], rFold[i])
			call[i] = strat(rCall[i], rPass[i])
		}
		// The small blind against the big blind's current calls.
		for i := 0; i < 169; i++ {
			uJam, mass := 0.0, 0.0
			for j := 0; j < 169; j++ {
				w := weight[i][j]
				uJam += w * ((1-call[j])*1 + call[j]*(eq[i][j]*2*s-s))
				mass += w
			}
			uFold := -0.5 * mass
			u := jam[i]*uJam + (1-jam[i])*uFold
			rJam[i] = math.Max(rJam[i]+uJam-u, 0)
			rFold[i] = math.Max(rFold[i]+uFold-u, 0)
		}
		// The big blind against the small blind's current jams, weighted by
		// how often each jam reaches it.
		for j := 0; j < 169; j++ {
			uCall, uPass := 0.0, 0.0
			for i := 0; i < 169; i++ {
				w := weight[i][j] * jam[i]
				uCall += w * (eq[j][i]*2*s - s)
				uPass += w * -1
			}
			u := call[j]*uCall + (1-call[j])*uPass
			rCall[j] = math.Max(rCall[j]+uCall-u, 0)
			rPass[j] = math.Max(rPass[j]+uPass-u, 0)
		}
		for i := 0; i < 169; i++ {
			avgJam[i] += float64(it) * jam[i]
			avgCall[i] += float64(it) * call[i]
		}
		norm += float64(it)
	}
	var out pushFoldSolution
	for i := 0; i < 169; i++ {
		out.jam[i] = avgJam[i] / norm
		out.call[i] = avgCall[i] / norm
	}
	out.exploitability = pushFoldExploitability(eq, weight, s, &out)
	return out
}

// pushFoldExploitability is how much the two best responses gain against the
// strategy pair, averaged, in big blinds per hand.
func pushFoldExploitability(eq, weight *[169][169]float64, s float64, sol *pushFoldSolution) float64 {
	total := 0.0
	for i := 0; i < 169; i++ {
		for j := 0; j < 169; j++ {
			total += weight[i][j]
		}
	}
	// Value of the strategy pair to the small blind.
	value, brSB, brBB := 0.0, 0.0, 0.0
	for i := 0; i < 169; i++ {
		uJam, mass := 0.0, 0.0
		for j := 0; j < 169; j++ {
			w := weight[i][j]
			uJam += w * ((1-sol.call[j])*1 + sol.call[j]*(eq[i][j]*2*s-s))
			mass += w
		}
		uFold := -0.5 * mass
		value += sol.jam[i]*uJam + (1-sol.jam[i])*uFold
		brSB += math.Max(uJam, uFold)
	}
	// The big blind's best response, as the small blind's value under it.
	for j := 0; j < 169; j++ {
		sbIfCall, sbIfPass := 0.0, 0.0
		for i := 0; i < 169; i++ {
			w := weight[i][j]
			sbIfCall += w * (sol.jam[i]*(eq[i][j]*2*s-s) + (1-sol.jam[i])*-0.5)
			sbIfPass += w * (sol.jam[i]*1 + (1-sol.jam[i])*-0.5)
		}
		brBB += math.Min(sbIfCall, sbIfPass)
	}
	return ((brSB - value) + (value - brBB)) / 2 / total
}

// pushFoldChart is the generated chart: per class, the deepest grid stack in
// half big blinds up to which it jams (or calls) at every grid point, 0 for
// never and pushFoldBeyond for still at the deepest.
type pushFoldChart struct {
	jam, call      [169]uint8
	exploitability float64 // the worst over the grid
	jamShare       map[float64]float64
	callShare      map[float64]float64
}

func generatePushFold() pushFoldChart {
	eq, weight := pushFoldEquities()
	grid := pushFoldGrid()
	sols := make([]pushFoldSolution, len(grid))
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))
	for k, s := range grid {
		wg.Add(1)
		sem <- struct{}{}
		go func(k int, s float64) {
			defer wg.Done()
			sols[k] = solvePushFold(&eq, &weight, s)
			<-sem
		}(k, s)
	}
	wg.Wait()

	out := pushFoldChart{jamShare: map[float64]float64{}, callShare: map[float64]float64{}}
	combos := func(i int) float64 { return float64(len(classAt(i).combos())) }
	for k, s := range grid {
		out.exploitability = math.Max(out.exploitability, sols[k].exploitability)
		for i := 0; i < 169; i++ {
			out.jamShare[s] += sols[k].jam[i] * combos(i) / 1326
			out.callShare[s] += sols[k].call[i] * combos(i) / 1326
		}
	}
	upTo := func(plays func(k int) bool) uint8 {
		last := uint8(0)
		for k, s := range grid {
			if !plays(k) {
				return last
			}
			last = uint8(math.Round(2 * s))
		}
		return pushFoldBeyond
	}
	for i := 0; i < 169; i++ {
		out.jam[i] = upTo(func(k int) bool { return sols[k].jam[i] >= 0.5 })
		out.call[i] = upTo(func(k int) bool { return sols[k].call[i] >= 0.5 })
	}
	return out
}

// PushFoldSource generates pushfold_table.go, and a summary of the solve for
// the generator to print. cmd/pushfold writes it; TestPushFoldTableIsGenerated
// regenerates it and requires the file to match.
func PushFoldSource() (source, summary string) {
	c := generatePushFold()
	return c.source(), c.summary()
}

func (c pushFoldChart) source() string {
	var b strings.Builder
	b.WriteString(`// Code generated by go run ./cmd/pushfold; DO NOT EDIT.

package holdem

// Heads-up jam-or-fold equilibrium (pushfold_solve.go). Each entry is the
// deepest effective stack, in half big blinds, up to which the hand jams from
// the small blind (pushFoldJam) or calls that jam from the big blind
// (pushFoldCall); 0 is never, 255 is still at fifty big blinds. Rows and
// columns run A, K, Q .. 2; suited above the diagonal, offsuit below.
`)
	for _, t := range []struct {
		name string
		v    *[169]uint8
	}{{"pushFoldJam", &c.jam}, {"pushFoldCall", &c.call}} {
		fmt.Fprintf(&b, "\nvar %s = [169]uint8{\n", t.name)
		b.WriteString("\t//  A    K    Q    J    T    9    8    7    6    5    4    3    2\n")
		for row := 0; row < 13; row++ {
			b.WriteString("\t")
			for col := 0; col < 13; col++ {
				fmt.Fprintf(&b, "%3d,", t.v[row*13+col])
				if col < 12 {
					b.WriteString(" ")
				}
			}
			fmt.Fprintf(&b, " // %c\n", "AKQJT98765432"[row])
		}
		b.WriteString("}\n")
	}
	out, err := format.Source([]byte(b.String()))
	if err != nil {
		panic(err) // the template above is broken
	}
	return string(out)
}

func (c pushFoldChart) summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "worst exploitability over the grid: %.5f big blinds per hand\n", c.exploitability)
	for _, s := range []float64{2, 5, 8, 10, 12, 15, 20} {
		fmt.Fprintf(&b, "  %4.1f BB: small blind jams %4.1f%%, big blind calls %4.1f%%\n",
			s, 100*c.jamShare[s], 100*c.callShare[s])
	}
	return b.String()
}
