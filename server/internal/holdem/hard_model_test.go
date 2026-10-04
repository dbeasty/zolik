package holdem

import (
	"testing"

	"zolik/server/internal/learn"
	"zolik/server/internal/learn/models"
)

// The model this binary ships for holdem's Hard seats must fit the encoder on
// this branch. One that does not would never play — HardBot falls back — but
// the console would offer a switch that does nothing, and a changed encoder
// is exactly the regression this is here to catch before a release does.
func TestShippedHardModelFits(t *testing.T) {
	b, info, ok := models.Hard("holdem")
	if !ok || len(b) == 0 {
		t.Fatal("no Hard model shipped for holdem")
	}
	p, err := learn.LoadEmbedded("holdem", b)
	if err != nil {
		t.Fatalf("loading the shipped model: %v", err)
	}
	g := learnGame{}
	if !p.Fits(g) {
		t.Fatalf("shipped model is %dx%d; this encoder is %dx%d",
			p.Net().StateDim, p.Net().CandDim, g.StateDim(), g.CandDim())
	}
	if name := p.Net().Game; name != "" && name != g.Name() {
		t.Fatalf("shipped model was trained for %q", name)
	}
	if _, ok := learn.HardBot(g, b, bot{}).(learn.NetBot); !ok {
		t.Fatal("HardBot does not seat the shipped model")
	}
	if info.SHA256 == "" || info.Benchmark == "" || info.SourceRun == "" {
		t.Errorf("metadata incomplete: %+v", info)
	}
	t.Logf("holdem: %s, %dx%d", info.SHA256[:12], p.Net().StateDim, p.Net().CandDim)
}
