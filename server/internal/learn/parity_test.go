package learn

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

// TestParityWithTrainer holds this package's forward pass to PyTorch's.
//
// testdata/parity is written by the trainer (ml/, `python -m zolik_ml.parity`):
// a model with random weights in the ZLNET1 format, some inputs, and the
// logits and value PyTorch computed for them. A model that trains well in
// Python and then plays differently here would be a bug nothing else catches —
// a transposed weight, a ReLU in the wrong place, a bias read in the wrong
// order — so the fixtures are committed and this runs without Python.
func TestParityWithTrainer(t *testing.T) {
	raw, err := os.ReadFile("testdata/parity/model.bin")
	if err != nil {
		t.Fatal(err)
	}
	n, err := LoadNet(raw)
	if err != nil {
		t.Fatal(err)
	}
	cb, err := os.ReadFile("testdata/parity/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Obs    []float32   `json:"obs"`
		Cands  [][]float32 `json:"cands"`
		Logits []float64   `json:"logits"`
		Value  float64     `json:"value"`
	}
	if err := json.Unmarshal(cb, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("no cases")
	}
	// Both sides are float32; they differ only in summation order.
	near := func(got float32, want float64) bool {
		return math.Abs(float64(got)-want) <= 1e-5*math.Max(1, math.Abs(want))
	}
	for i, c := range cases {
		if len(c.Obs) != n.StateDim {
			t.Fatalf("case %d: obs has %d, model takes %d", i, len(c.Obs), n.StateDim)
		}
		emb := n.Embed(c.Obs)
		got := n.Logits(emb, c.Cands)
		if len(got) != len(c.Logits) {
			t.Fatalf("case %d: %d logits, want %d", i, len(got), len(c.Logits))
		}
		for j := range got {
			if !near(got[j], c.Logits[j]) {
				t.Errorf("case %d logit %d: go %.7f, torch %.7f", i, j, got[j], c.Logits[j])
			}
		}
		if v := n.ValueOf(emb); !near(v, c.Value) {
			t.Errorf("case %d value: go %.7f, torch %.7f", i, v, c.Value)
		}
	}
}
