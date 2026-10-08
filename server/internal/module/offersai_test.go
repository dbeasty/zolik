package module_test

import (
	"testing"

	"zolik/server/internal/canasta"
	"zolik/server/internal/holdem"
	"zolik/server/internal/lastcard"
	"zolik/server/internal/marias"
	"zolik/server/internal/module"
	"zolik/server/internal/prsi"
	"zolik/server/internal/sedma"
	"zolik/server/internal/snaps"
	"zolik/server/internal/zolikmod"
)

// "AI" is offered by the games that ship a trained network and by no other:
// at the rest it was a setup choice their heuristic answered as Medium.
func TestOnlyGamesWithAModelOfferAI(t *testing.T) {
	for _, m := range []module.GameModule{zolikmod.New(), canasta.New(), holdem.New()} {
		if !module.OffersAI(m.Descriptor()) {
			t.Errorf("%s ships a model and does not offer AI", m.Descriptor().ID)
		}
	}
	for _, m := range []module.GameModule{prsi.New(), marias.New(), lastcard.New(), sedma.New(), snaps.New()} {
		if module.OffersAI(m.Descriptor()) {
			t.Errorf("%s ships no model and offers AI", m.Descriptor().ID)
		}
	}
}
