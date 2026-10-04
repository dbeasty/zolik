package learn

import (
	"math"
	"testing"
)

func TestWinModel(t *testing.T) {
	m := &WinModel{Features: 7, Layers: []WinModelLay{
		{W: [][]float64{{0, 0, 1, 0, 0, 0, 0}, {0, 0, -1, 0, 0, 0, 0}}, B: []float64{0, 0}},
		{W: [][]float64{{2, -2}}, B: []float64{0}},
	}}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	if p := m.P(MatchScore{Target: 5000}); math.Abs(p-0.5) > 1e-12 {
		t.Fatalf("level score: %v", p)
	}
	ahead := m.P(MatchScore{Own: 3000, Best: 1000, Target: 5000})
	want := 1 / (1 + math.Exp(-(2*math.Tanh(0.4) - 2*math.Tanh(-0.4))))
	if math.Abs(ahead-want) > 1e-12 {
		t.Fatalf("ahead: %v, want %v", ahead, want)
	}
	if p := m.P(MatchScore{Own: 100, Best: 6000, Target: 5000, Over: true, Won: 1}); p != 1 {
		t.Fatalf("a finished match is its result, got %v", p)
	}
	bad := &WinModel{Features: 7, Layers: []WinModelLay{{W: [][]float64{{1, 2}}, B: []float64{0}}}}
	if bad.Validate() == nil {
		t.Fatal("a layer of the wrong width validated")
	}
	if _, err := LoadWinModel([]byte(`{"features":7,"layers":[{"w":[[0,0,1,0,0,0,0]],"b":[0]}],"k":3}`)); err != nil {
		t.Fatal(err)
	}
}
