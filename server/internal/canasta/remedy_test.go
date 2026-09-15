package canasta

import (
	"strings"
	"testing"

	"zolik/server/internal/module"
)

// What a refused player actually reads.
//
// The rule-index check next door proves every code points somewhere real; it
// cannot prove the answer is any use. These do: they take the refusals a
// player meets most, and assert the offer carries a sentence that names the
// move to make instead — and, where one exists, the id of the control that
// makes it.
//
// WRONG_PHASE is the case worth two tests rather than one. It is the same code
// at both ends of a turn and it has to say opposite things: before the draw,
// draw; after it, meld or discard. A single test would pass on an
// implementation that always said one of them.

func offerByID(offers []module.ActionOffer, id string) *module.ActionOffer {
	for i := range offers {
		if offers[i].ID == id {
			return &offers[i]
		}
	}
	return nil
}

func TestRefusedBeforeDrawing_IsToldToDraw(t *testing.T) {
	m := New()
	// A hand that could meld, stopped only by not having drawn yet.
	raw := twoHanded(func(s *GameState) {
		s.Phase = phaseDraw
		s.Hands["p1"] = []string{"KH", "KD", "KS", "4C"}
	})

	offers, err := m.LegalActions(raw, "p1")
	if err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{OfferLayMeld, OfferDiscard} {
		o := offerByID(offers, id)
		if o == nil {
			t.Fatalf("no %s offer at all", id)
		}
		if o.Enabled {
			t.Fatalf("%s was enabled before the draw", id)
		}
		if o.WhyNot != ErrWrongPhase {
			t.Fatalf("%s refused with %q, want %s", id, o.WhyNot, ErrWrongPhase)
		}
		if o.Remedy == nil {
			t.Fatalf("%s: refused with a bare code and no way out", id)
		}
		if got := o.Remedy.LabelKey; got != "canasta.remedy.drawOrTakePile" {
			t.Errorf("%s: remedy %q, want the one that says to draw", id, got)
		}
		// The point of the whole feature: a working control under the
		// sentence, not an instruction to go and find one.
		if o.RemedyOfferID != OfferDraw {
			t.Errorf("%s: remedy points at %q, want the draw offer", id, o.RemedyOfferID)
		}
		if len(o.RuleIDs) == 0 {
			t.Errorf("%s: no written rule behind the refusal", id)
		}
	}
}

func TestRefusedAfterDrawing_IsToldToMeldOrDiscard(t *testing.T) {
	m := New()
	raw := twoHanded(func(s *GameState) {
		s.Phase = phaseMeld
		s.Hands["p1"] = []string{"KH", "KD", "KS", "4C"}
	})

	offers, err := m.LegalActions(raw, "p1")
	if err != nil {
		t.Fatal(err)
	}

	draw := offerByID(offers, OfferDraw)
	if draw == nil || draw.Enabled {
		t.Fatalf("draw should be refused once the player has drawn: %+v", draw)
	}
	if draw.Remedy == nil {
		t.Fatal("drawing twice was refused with a bare code and no way out")
	}
	if got := draw.Remedy.LabelKey; got != "canasta.remedy.meldOrDiscard" {
		t.Errorf("remedy %q, want the one that says to meld or discard", got)
	}
	if draw.RemedyOfferID != OfferDiscard && !strings.HasPrefix(draw.RemedyOfferID, OfferLayMeld) {
		t.Errorf("remedy points at %q, want the discard or a meld", draw.RemedyOfferID)
	}
}

// The gap, not the floor. A side twenty points short should be told twenty,
// because the alternative is a player doing the subtraction the server has
// already done — and getting it wrong, since the floor moves with their score.
func TestOpeningRefusal_NamesThePointsStillMissing(t *testing.T) {
	m := New()
	raw := twoHanded(func(s *GameState) {
		s.Phase = phaseMeld
		// Below the classic 50 floor, with 30 already laid this turn and a
		// hand that cannot reach the rest.
		s.Teams[0].Score = 0
		s.LaidThisTurn = 30
		s.Hands["p1"] = []string{"4C", "4D"}
	})

	offers, err := m.LegalActions(raw, "p1")
	if err != nil {
		t.Fatal(err)
	}
	discard := offerByID(offers, OfferDiscard)
	if discard == nil || discard.Enabled {
		t.Fatalf("a discard that leaves the side short should be refused: %+v", discard)
	}
	if discard.WhyNot != ErrInitialMeldNotMet {
		t.Fatalf("refused with %q, want %s", discard.WhyNot, ErrInitialMeldNotMet)
	}
	if discard.Remedy == nil {
		t.Fatal("no remedy on the opening refusal")
	}
	if got := discard.Remedy.Params["n"]; got != 20 {
		t.Errorf("told the player they are %v short; the floor is 50 and 30 is down, so it is 20", got)
	}
}

// The commonest refusal in the game: the capture control, greyed out because
// the hand holds no matching pair.
//
// It answered MELD_TOO_SMALL — "a meld needs more cards than that" — under a
// button that melds nothing, on most turns of most deals. Its own code now,
// and a remedy that names what the capture would cost.
func TestCaptureWithoutAPair_SaysWhatThePileCosts(t *testing.T) {
	m := New()
	raw := twoHanded(func(s *GameState) {
		s.Phase = phaseDraw
		s.DiscardPile = []string{"KH"}
		// Nothing that matches the king, so no capture is available.
		s.Hands["p1"] = []string{"4C", "5D", "6S"}
	})

	offers, err := m.LegalActions(raw, "p1")
	if err != nil {
		t.Fatal(err)
	}
	take := offerByID(offers, OfferTakePile)
	if take == nil || take.Enabled {
		t.Fatalf("a capture with no matching pair should be refused: %+v", take)
	}
	if take.WhyNot != ErrCaptureNeedsTwo {
		t.Fatalf("refused with %q, want %s — a code about melds is the wrong "+
			"sentence under the button that takes the pile", take.WhyNot, ErrCaptureNeedsTwo)
	}
	if take.Remedy == nil {
		t.Fatal("no remedy on the refusal a player meets most")
	}
	if got := take.Remedy.Params["card"]; got != "KH" {
		t.Errorf("remedy names %v; it should name the card the pair has to match", got)
	}
	if take.RemedyOfferID != OfferDraw {
		t.Errorf("remedy points at %q, want the draw", take.RemedyOfferID)
	}
}

// Nobody else's turn refusals get a remedy, because there is nothing to press
// — but they still get the rule, which is what says whose turn it is and why.
func TestWaitingForYourTurn_GetsTheRuleAndNoFalseRemedy(t *testing.T) {
	m := New()
	raw := twoHanded(nil)

	offers, err := m.LegalActions(raw, "p2")
	if err != nil {
		t.Fatal(err)
	}
	if len(offers) == 0 {
		t.Fatal("a waiting player was sent no offers at all")
	}
	for _, o := range offers {
		if o.WhyNot != ErrNotYourTurn {
			t.Fatalf("%s refused with %q, want %s", o.ID, o.WhyNot, ErrNotYourTurn)
		}
		if len(o.RuleIDs) == 0 {
			t.Errorf("%s: nothing explains whose turn it is", o.ID)
		}
		if o.Remedy != nil {
			t.Errorf("%s: offered a remedy (%s) for something the player cannot fix",
				o.ID, o.Remedy.LabelKey)
		}
	}
}
