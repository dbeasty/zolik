package learn

import (
	"math"
	"math/rand"
	"testing"

	"zolik/server/internal/module"
)

func randomDense(rnd *rand.Rand, in, out int) Dense {
	d := Dense{In: in, Out: out, W: make([]float32, in*out), B: make([]float32, out)}
	for i := range d.W {
		d.W[i] = float32(rnd.NormFloat64())
	}
	for i := range d.B {
		d.B[i] = float32(rnd.NormFloat64())
	}
	return d
}

// Policy.Logits sums the embedding's half of the first scorer layer once for
// all candidates; the numbers must be Net.Logits's to the bit, at every scorer
// depth, since the trainer's parity test is pinned against Net.
func TestPolicyLogitsAreNetLogits(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	for _, scorer := range [][]int{{1}, {9, 1}, {7, 5, 1}} {
		n := &Net{StateDim: 11, CandDim: 6}
		n.Trunk = []Dense{randomDense(rnd, 11, 13), randomDense(rnd, 13, 8)}
		in := 8 + 6
		for _, w := range scorer {
			n.Scorer = append(n.Scorer, randomDense(rnd, in, w))
			in = w
		}
		n.Value = []Dense{randomDense(rnd, 8, 1)}
		if err := n.check(); err != nil {
			t.Fatal(err)
		}
		p := NewPolicy(n)
		for trial := 0; trial < 50; trial++ {
			obs := make([]float32, 11)
			for i := range obs {
				obs[i] = float32(rnd.NormFloat64())
			}
			cands := make([][]float32, rnd.Intn(12))
			for i := range cands {
				cands[i] = make([]float32, 6)
				for j := range cands[i] {
					cands[i][j] = float32(rnd.NormFloat64())
				}
			}
			want := n.Logits(n.Embed(obs), cands)
			got := p.Logits(obs, cands)
			for i := range want {
				if math.Float32bits(got[i]) != math.Float32bits(want[i]) {
					t.Fatalf("scorer %v candidate %d: policy %v, net %v", scorer, i, got[i], want[i])
				}
			}
		}
	}
}

func TestLoadEmbeddedParsesOnce(t *testing.T) {
	model, err := preferThird().Marshal()
	if err != nil {
		t.Fatal(err)
	}
	a, err := LoadEmbedded("nim", model)
	if err != nil {
		t.Fatal(err)
	}
	b, err := LoadEmbedded("nim", model)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatal("the same bytes loaded twice")
	}
	// Other bytes under the same name are another model, not the first one.
	other := append([]byte(nil), model...)
	c, err := LoadEmbedded("nim", other)
	if err != nil {
		t.Fatal(err)
	}
	if c == a {
		t.Fatal("different bytes under one name returned the first model")
	}
	if _, err := LoadEmbedded("broken", []byte("not a model")); err == nil {
		t.Fatal("a broken model loaded")
	}
	if allocs := testing.AllocsPerRun(100, func() { _, _ = LoadEmbedded("nim", model) }); allocs > 1 {
		t.Fatalf("a cached load allocates %v times", allocs)
	}
}

func TestHardBot(t *testing.T) {
	model, err := preferThird().Marshal()
	if err != nil {
		t.Fatal(err)
	}
	s, _ := nim{}.NewMatch(module.MatchConfig{}, Players(2), 3) // pile 13: perfect takes 1
	offers, _ := nim{}.LegalActions(s, "p0")
	seat := module.BotSeat{PlayerID: "p0"}

	bot := HardBot(nimGame{}, model, perfect{})
	nb, ok := bot.(NetBot)
	if !ok || nb.Policy == nil {
		t.Fatalf("a good model gave %T", bot)
	}
	if again := HardBot(nimGame{}, model, perfect{}).(NetBot); again.Policy != nb.Policy {
		t.Fatal("two HardBots on the same bytes hold different policies")
	}
	if a, _ := bot.Act(s, seat, offers); a.OfferID != "take3" {
		t.Fatalf("played %+v, want the network's favourite", a)
	}

	wrong := preferThird()
	wrong.Game = "canasta"
	wrongBytes, _ := wrong.Marshal()
	for name, b := range map[string][]byte{"empty": nil, "broken": []byte("ZLNET1\nxx"), "other game": wrongBytes} {
		if _, ok := HardBot(nimGame{}, b, perfect{}).(perfect); !ok {
			t.Errorf("%s: did not fall back", name)
		}
	}
}

// The forward pass alone, at the trainer's sizes (trunk 256x256, scorer 128)
// and a Canasta-like dozen candidates: Net.Logits, which runs the whole
// scorer per candidate, against Policy.Logits, which sums the embedding's
// share of the first scorer layer once.
func BenchmarkLogits(b *testing.B) {
	rnd := rand.New(rand.NewSource(1))
	const stateDim, candDim = 400, 40
	n := &Net{StateDim: stateDim, CandDim: candDim,
		Trunk:  []Dense{randomDense(rnd, stateDim, 256), randomDense(rnd, 256, 256)},
		Scorer: []Dense{randomDense(rnd, 256+candDim, 128), randomDense(rnd, 128, 1)},
		Value:  []Dense{randomDense(rnd, 256, 128), randomDense(rnd, 128, 1)},
	}
	obs := make([]float32, stateDim)
	cands := make([][]float32, 12)
	for i := range cands {
		cands[i] = make([]float32, candDim)
	}
	b.Run("net", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			n.Logits(n.Embed(obs), cands)
		}
	})
	p := NewPolicy(n)
	b.Run("policy", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			p.Logits(obs, cands)
		}
	})
}
