package cardinfer

import (
	"math"
	"math/rand"
	"testing"
	"time"
)

func zolikSpec(minRun int) Spec {
	return Spec{Copies: 2, Jokers: 4, MinSet: 3, MinRun: minRun, AceLow: true, DistinctSuitSets: true}
}

func canastaSpec() Spec {
	sp := Spec{Copies: 2, Jokers: 4, MinSet: 3, WildDeuces: true}
	sp.Unmeldable[1] = true // threes
	return sp
}

func s(cards ...string) []int { return Slots(cards) }

func rowSum(row *[NumSlots]float64) float64 {
	t := 0.0
	for _, v := range row {
		t += v
	}
	return t
}

func TestSlotRoundTrip(t *testing.T) {
	for k := 0; k < NumSlots; k++ {
		if got := Slot(SlotCard(k)); got != k {
			t.Fatalf("slot %d -> %q -> %d", k, SlotCard(k), got)
		}
	}
	if Slot("JOKER2") != JokerSlot || Slot("") != -1 || Slot("1X") != -1 {
		t.Fatal("special cards")
	}
}

// The case the task was written for: a seat that took 9♦ and 10♦ off the
// pile holds them, and wants the 8♦ and the J♦.
func TestTookNineTenOfDiamonds(t *testing.T) {
	for _, minRun := range []int{3, 4} {
		o := Observation{
			Spec: zolikSpec(minRun),
			Mine: s("2H", "5C", "KS", "KH", "7C", "4D", "QH", "3S", "6H", "AC", "8S"),
			Seen: s("4C"),
			Seats: []Seat{
				{HandSize: 11, Held: s("9D", "TD")},
				{HandSize: 11},
			},
		}
		var e Estimate
		Infer(&o, &e)
		d9, d10 := Slot("9D"), Slot("TD")
		if e.Hold[0][d9] < 1 || e.Hold[0][d10] < 1 {
			t.Fatalf("minRun %d: P(holds 9D)=%.2f P(holds TD)=%.2f, want >= 1", minRun, e.Hold[0][d9], e.Hold[0][d10])
		}
		w8, wJ := e.Want[0][Slot("8D")], e.Want[0][Slot("JD")]
		base := e.Want[1][Slot("8D")]
		t.Logf("minRun %d: want 8D %.2f JD %.2f; a seat with no record wants 8D %.2f", minRun, w8, wJ, base)
		floor := 0.9
		if minRun == 4 {
			floor = 0.45
		}
		if w8 < floor || wJ < floor {
			t.Errorf("minRun %d: want 8D %.2f, JD %.2f; want both >= %.2f", minRun, w8, wJ, floor)
		}
		if w8 < base+0.25 {
			t.Errorf("minRun %d: 8D wanted %.2f, barely above a blank seat's %.2f", minRun, w8, base)
		}
		// A card nowhere near: the 4 of spades is not one it is building.
		if far := e.Want[0][Slot("4S")]; far > w8/2 {
			t.Errorf("4S wanted %.2f, near the 8D's %.2f", far, w8)
		}
	}
}

func TestDiscardAndPassLowerHolding(t *testing.T) {
	o := Observation{
		Spec: zolikSpec(4),
		Mine: s("2H", "5C", "KS"),
		Seats: []Seat{
			{HandSize: 10, Discards: s("7H", "7C"), Passes: []Pass{{Slot: Slot("QD")}}},
			{HandSize: 10},
		},
	}
	var e Estimate
	Infer(&o, &e)
	for _, c := range []string{"7S", "7D", "QS"} {
		k := Slot(c)
		if e.Hold[0][k] >= e.Hold[1][k] {
			t.Errorf("%s: discarder holds %.3f, blank seat %.3f; want lower", c, e.Hold[0][k], e.Hold[1][k])
		}
	}
	if e.Want[0][Slot("7S")] >= e.Want[1][Slot("7S")] {
		t.Errorf("discarder wants 7S %.3f >= blank %.3f", e.Want[0][Slot("7S")], e.Want[1][Slot("7S")])
	}
}

func TestStrongPassIsStronger(t *testing.T) {
	weak := Observation{Spec: canastaSpec(), Seats: []Seat{{HandSize: 11, Passes: []Pass{{Slot: Slot("KH")}}}, {HandSize: 11}}}
	strong := weak
	strong.Seats = []Seat{{HandSize: 11, Passes: []Pass{{Slot: Slot("KH"), Strong: true}}}, {HandSize: 11}}
	var a, b Estimate
	Infer(&weak, &a)
	Infer(&strong, &b)
	k := Slot("KS")
	if !(b.Hold[0][k] < a.Hold[0][k] && a.Hold[0][k] < a.Hold[1][k]) {
		t.Errorf("KS held: strong pass %.3f, weak pass %.3f, none %.3f", b.Hold[0][k], a.Hold[0][k], a.Hold[1][k])
	}
}

func TestExtendsIsCertainWant(t *testing.T) {
	o := Observation{Spec: canastaSpec(), Seats: []Seat{{HandSize: 8}}}
	o.Seats[0].Extends[Slot("KH")] = true
	var e Estimate
	Infer(&o, &e)
	if e.Want[0][Slot("KH")] != 1 {
		t.Fatalf("want %.2f", e.Want[0][Slot("KH")])
	}
	if e.Want[0][Slot("3S")] != 0 {
		t.Fatalf("a three is never meld material in Canasta, want %.2f", e.Want[0][Slot("3S")])
	}
	if e.Want[0][Slot("2C")] != 1 || e.Want[0][JokerSlot] != 1 {
		t.Fatal("wilds are wanted by everybody")
	}
}

// Normalisation: the expected cards per seat equal its hand size, and no card
// is expected in more copies than are unseen.
func TestNormalisation(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	for trial := 0; trial < 200; trial++ {
		o, _ := randomObservation(rng)
		var e Estimate
		Infer(&o, &e)
		for i := 0; i < e.N; i++ {
			if got := rowSum(&e.Hold[i]); math.Abs(got-float64(o.Seats[i].HandSize)) > 1e-6 {
				t.Fatalf("trial %d seat %d: expected %.6f cards, hand size %d", trial, i, got, o.Seats[i].HandSize)
			}
			for k := 0; k < NumSlots; k++ {
				if e.Want[i][k] < 0 || e.Want[i][k] > 1 || math.IsNaN(e.Want[i][k]) {
					t.Fatalf("want out of range: %v", e.Want[i][k])
				}
			}
		}
		for k := 0; k < NumSlots; k++ {
			unknown := 0.0
			for i := 0; i < e.N; i++ {
				unknown += e.Hold[i][k]
			}
			if unknown > e.Unseen[k]+countIn(o, k)+1e-3 {
				t.Fatalf("trial %d: %s expected %.3f in hands, %v unseen", trial, SlotCard(k), unknown, e.Unseen[k])
			}
		}
	}
}

func countIn(o Observation, k int) float64 {
	n := 0.0
	for _, st := range o.Seats {
		for _, h := range st.Held {
			if h == k {
				n++
			}
		}
	}
	return n
}

func TestDeterministic(t *testing.T) {
	rng := rand.New(rand.NewSource(9))
	for trial := 0; trial < 20; trial++ {
		o, _ := randomObservation(rng)
		var a, b Estimate
		Infer(&o, &a)
		Infer(&o, &b)
		if a != b {
			t.Fatal("two calls on one observation differ")
		}
	}
}

// randomObservation deals a real two-pack deck at four seats and builds the
// observer's view of it, with some public history.
func randomObservation(rng *rand.Rand) (Observation, [][]int) {
	var deck []int
	for c := 0; c < 2; c++ {
		for k := 0; k < JokerSlot; k++ {
			deck = append(deck, k)
		}
	}
	for j := 0; j < 4; j++ {
		deck = append(deck, JokerSlot)
	}
	rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
	o := Observation{Spec: zolikSpec(3 + rng.Intn(2))}
	hands := make([][]int, 4)
	at := 0
	for i := range hands {
		n := 5 + rng.Intn(10)
		hands[i] = deck[at : at+n]
		at += n
	}
	o.Mine = hands[0]
	seen := 3 + rng.Intn(20)
	o.Seen = deck[at : at+seen]
	at += seen
	for i := 1; i < 4; i++ {
		st := Seat{HandSize: len(hands[i])}
		for _, k := range hands[i] {
			if rng.Intn(4) == 0 {
				st.Held = append(st.Held, k)
			}
		}
		for j := 0; j < rng.Intn(6); j++ {
			st.Discards = append(st.Discards, o.Seen[rng.Intn(len(o.Seen))])
		}
		if rng.Intn(2) == 0 {
			st.Passes = append(st.Passes, Pass{Slot: o.Seen[0], Strong: rng.Intn(2) == 0})
		}
		st.Down = rng.Intn(2) == 0
		st.Extends[rng.Intn(JokerSlot)] = true
		o.Seats = append(o.Seats, st)
	}
	return o, hands
}

func BenchmarkInfer(b *testing.B) {
	rng := rand.New(rand.NewSource(1))
	o, _ := randomObservation(rng)
	var e Estimate
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Infer(&o, &e)
	}
}

// The budget the bots and the encoders are built around.
func TestInferIsCheap(t *testing.T) {
	if raceEnabled {
		t.Skip("timing under -race is meaningless")
	}
	res := testing.Benchmark(BenchmarkInfer)
	per := time.Duration(res.NsPerOp())
	t.Logf("Infer: %v per call, %d allocs", per, res.AllocsPerOp())
	if per > 100*time.Microsecond {
		t.Errorf("Infer takes %v, budget 100µs", per)
	}
}
