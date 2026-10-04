package zolikmod

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"zolik/server/internal/learn"
	"zolik/server/internal/learn/models"
)

// TestNarrowedV2ChoosesAsOnItsOwnEncoder is what lets the shipped zolik-v2
// model (encoder 900/62) sit in Hard seats on today's encoder (1161/64): the
// card inference was appended to the state and to every candidate, so
// learn.Narrowed hands the network the prefix it was trained on.
//
// The claim is checked against the build the model was trained for, not
// argued. testdata/zolik-v2-choices.json.gz was recorded on the
// claude/zolik-adapter-v2 branch (v2choices_test.go, unchanged, run there):
// eight matches across classic two-seat, floor-35 four-seat and continental
// three-seat, the network in one seat and the heuristic in the others, and
// for every step the fingerprint of the network's input and the move it
// chose. Replayed here through the narrowed game, every decision must see
// the same bits and make the same choice.
func TestNarrowedV2ChoosesAsOnItsOwnEncoder(t *testing.T) {
	b, _, ok := models.Hard("zolik")
	if !ok {
		t.Fatal("no shipped zolik model")
	}
	fx := readV2Fixture(t, "testdata/zolik-v2-choices.json.gz")
	sum := sha256.Sum256(b)
	if got := hex.EncodeToString(sum[:]); got != fx.Model {
		t.Skipf("the shipped model (%s) is not the one the fixture was recorded with (%s)", got[:12], fx.Model[:12])
	}
	p, err := learn.LoadEmbedded("zolik", b)
	if err != nil {
		t.Fatal(err)
	}
	if fx.Encoder != [2]int{p.Net().StateDim, p.Net().CandDim} {
		t.Fatalf("fixture recorded on a %v encoder, model is %d/%d", fx.Encoder, p.Net().StateDim, p.Net().CandDim)
	}
	g, ok := p.GameFor(learnGame{})
	if !ok {
		t.Fatal("the shipped model does not narrow onto this encoder")
	}
	if g.StateDim() == (learnGame{}).StateDim() {
		t.Fatal("expected a narrowed game: the fixture is for the encoder before the card inference")
	}
	if testing.Short() {
		// Two matches, one per table size the shipped model is benchmarked
		// at, keep the short suite short.
		fx.Matches = []v2Match{fx.Matches[0], fx.Matches[3]}
	}
	asked, steps := v2Check(t, g, p, fx)
	if asked < 100 {
		t.Fatalf("only %d network decisions replayed", asked)
	}
	t.Logf("%d steps replayed; the network chose identically at all %d of its decisions", steps, asked)
}
