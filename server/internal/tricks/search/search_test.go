package search

import (
	"math/rand"
	"sort"
	"testing"
	"time"

	"zolik/server/internal/tricks"
)

const trumpOrder tricks.Order = "ATKQJ987"

func points(c string) int {
	switch tricks.Rank(c) {
	case 'A', 'T':
		return 10
	}
	return 0
}

// cardPoints values a trick as Mariáš does: aces and tens, and ten for the
// last, to seat 0's side (seat 0 alone against 1 and 2).
func cardPoints(plays []tricks.Play, winner, left int) (int, bool) {
	if winner != 0 {
		return 0, false
	}
	v := 0
	for _, p := range plays {
		v += points(p.Card)
	}
	if left == 0 {
		v += 10
	}
	return v, false
}

// brute is plain minimax on tricks.Legal — slow, obvious, and the reference
// the solver has to agree with.
func brute(p Problem, hands [][]string, trick []tricks.Play, leader int) int {
	n := len(hands)
	seat := (leader + len(trick)) % n
	legal := tricks.Legal(hands[seat], tricks.Trick{Plays: trick}, p.Trump, p.Order, p.Policy)
	for _, r := range p.Reserves {
		if r.Seat == seat && len(hands[seat]) > r.UntilLeft && len(legal) > 1 {
			var kept []string
			for _, c := range legal {
				if c != r.Card {
					kept = append(kept, c)
				}
			}
			legal = kept
		}
	}
	best := inf
	if p.Maximizer[seat] {
		best = -inf
	}
	for _, c := range legal {
		h := append([][]string(nil), hands...)
		h[seat] = without(hands[seat], c)
		t := append(append([]tricks.Play(nil), trick...), tricks.Play{Seat: seat, Card: c})
		var v int
		for _, pr := range p.Pairs {
			if pr.First == c && contains(h[seat], pr.Second) {
				if p.Maximizer[seat] {
					v += pr.Value
				} else {
					v -= pr.Value
				}
			}
		}
		if len(t) < n {
			v += brute(p, h, t, leader)
		} else {
			w, _ := tricks.Trick{Plays: t}.Winning(p.Trump, p.Order)
			left := len(h[0])
			val, stop := p.TrickValue(t, w.Seat, left)
			v += val
			if !stop && left > 0 {
				v += brute(p, h, nil, w.Seat)
			}
		}
		if p.Maximizer[seat] {
			best = max(best, v)
		} else {
			best = min(best, v)
		}
	}
	return best
}

func without(hand []string, c string) []string {
	var out []string
	for _, x := range hand {
		if x != c {
			out = append(out, x)
		}
	}
	return out
}

func contains(hand []string, c string) bool {
	for _, x := range hand {
		if x == c {
			return true
		}
	}
	return false
}

func deck() []string {
	var out []string
	for _, s := range "HDCS" {
		for _, r := range trumpOrder {
			out = append(out, string(r)+string(s))
		}
	}
	return out
}

func randomProblem(r *rand.Rand, tricksLeft int) Problem {
	d := deck()
	r.Shuffle(len(d), func(i, j int) { d[i], d[j] = d[j], d[i] })
	hands := [][]string{d[:tricksLeft], d[tricksLeft : 2*tricksLeft], d[2*tricksLeft : 3*tricksLeft]}
	trump := []byte{'H', 'D', 'C', 'S', tricks.NoTrump}[r.Intn(5)]
	return Problem{
		Hands:      hands,
		Leader:     r.Intn(3),
		Trump:      trump,
		Order:      trumpOrder,
		Policy:     tricks.FollowBeatTrump,
		Maximizer:  []bool{true, false, false},
		TrickValue: cardPoints,
		CardClass: func(c string) int {
			if r := tricks.Rank(c); r == 'K' || r == 'Q' {
				return 1000 + int(r)*8 + int(tricks.Suit(c))
			}
			return points(c)
		},
	}
}

func TestSolverAgreesWithBruteForce(t *testing.T) {
	r := rand.New(rand.NewSource(3))
	for i := 0; i < 300; i++ {
		p := randomProblem(r, 1+r.Intn(4))
		// Marriages in every suit, as Mariáš scores them.
		for _, su := range "HDCS" {
			p.Pairs = append(p.Pairs, Pair{First: "Q" + string(su), Second: "K" + string(su), Value: 20})
		}
		if r.Intn(3) == 0 {
			// Hold seat 0's lowest trump back, as a seven announced.
			if p.Trump != tricks.NoTrump {
				p.Reserves = []Reserve{{Seat: 0, Card: p.Hands[0][0], UntilLeft: 1}}
			}
		}
		got := Solve(p)
		seat := p.Leader
		legal := tricks.Legal(p.Hands[seat], tricks.Trick{}, p.Trump, p.Order, p.Policy)
		for _, c := range legal {
			if _, ok := got.Values[c]; !ok && !reserved(p, seat, c, legal) {
				t.Fatalf("problem %d: no value for legal %s", i, c)
			}
		}
		for c, v := range got.Values {
			h := append([][]string(nil), p.Hands...)
			h[seat] = without(p.Hands[seat], c)
			want := brute(p, h, []tricks.Play{{Seat: seat, Card: c}}, p.Leader)
			// The root card is played outside brute's own loop, so its
			// marriage, if it makes one, is scored here.
			for _, pr := range p.Pairs {
				if pr.First == c && contains(h[seat], pr.Second) {
					if p.Maximizer[seat] {
						want += pr.Value
					} else {
						want -= pr.Value
					}
				}
			}
			if v != want {
				t.Fatalf("problem %d (%v, trump %c): %s is worth %d, brute force says %d", i, p.Hands, p.Trump, c, v, want)
			}
		}
	}
}

func reserved(p Problem, seat int, c string, legal []string) bool {
	for _, r := range p.Reserves {
		if r.Seat == seat && r.Card == c && len(p.Hands[seat]) > r.UntilLeft && len(legal) > 1 {
			return true
		}
	}
	return false
}

func TestAStoppedContractEndsThePlay(t *testing.T) {
	// Betl: seat 0 must take no trick; the first one taken ends it at -100.
	// Led, the ace wins at once. The seven loses to the 9 of spades, and
	// the queen of diamonds then led lets seat 0 throw the ace away.
	p := Problem{
		Hands:     [][]string{{"AH", "7S"}, {"8S", "KD"}, {"9S", "QD"}},
		Leader:    0,
		Trump:     tricks.NoTrump,
		Order:     "AKQJT987",
		Policy:    tricks.FollowBeatTrump,
		Maximizer: []bool{true, false, false},
		TrickValue: func(_ []tricks.Play, winner, _ int) (int, bool) {
			if winner == 0 {
				return -100, true
			}
			return 0, false
		},
	}
	got := Solve(p)
	if got.Values["AH"] != -100 || got.Values["7S"] != 0 {
		t.Fatalf("values %v: leading the ace loses the betl, the seven does not", got.Values)
	}
}

// A full deal must solve in a time a bot can afford per sample.
func TestAFullDealSolvesQuickly(t *testing.T) {
	r := rand.New(rand.NewSource(9))
	var times []time.Duration
	for i := 0; i < 5; i++ {
		p := randomProblem(r, 10)
		p.Trump = 'H'
		start := time.Now()
		res := Solve(p)
		times = append(times, time.Since(start))
		t.Logf("deal %d: %d nodes in %v", i, res.Nodes, times[i])
	}
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
	if times[len(times)/2] > 2*time.Second {
		t.Errorf("median full solve %v", times[len(times)/2])
	}
}
