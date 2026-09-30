package rules

import (
	"slices"
	"testing"
)

// The discard offer used to go dark when the card just taken off the pile was
// the first card in hand. probeDiscard tried cards in hand order, treated only
// the joker ban and CARD_NOT_IN_HAND as being about one card, and so read the
// taken card's refusal as a verdict on the verb — disabling the discard while
// every other card in hand was a legal one. A client or bot trusting Enabled
// saw no way to end the turn.
func TestDiscardOffer_TakenCardFirstInHandLeavesTheOfferEnabled(t *testing.T) {
	st := downStateHolding([]string{"QH", "4S", "3S"})
	st.DiscardTakenCard = "QH"

	d := FindOffer(LegalActions(st, "p1"), OfferDiscard)
	if d == nil {
		t.Fatal("no discard offer")
	}
	if !d.Enabled {
		t.Fatalf("discard disabled (%s) with 4S and 3S both legal", d.WhyNot)
	}
	if d.WhyNot != "" {
		t.Fatalf("enabled offer carries a reason: %s", d.WhyNot)
	}
	if got := d.Source.Cards; slices.Contains(got, "QH") || !slices.Contains(got, "4S") || !slices.Contains(got, "3S") {
		t.Fatalf("discardable cards %v, want 4S and 3S without QH", got)
	}
	want := []CardRefusal{{Card: "QH", WhyNot: ErrDiscardTakenCard}}
	if !slices.Equal(d.Source.Refused, want) {
		t.Fatalf("refused %v, want %v", d.Source.Refused, want)
	}
	if got := HeldBackTakenCard(st, "p1"); got != "QH" {
		t.Fatalf("HeldBackTakenCard = %q, want QH", got)
	}
}

// The ban yields when the taken card is all there is to discard, and then
// nothing is refused and nothing is held back.
func TestDiscardOffer_TakenCardAloneIsDiscardable(t *testing.T) {
	st := downStateHolding([]string{"QH", "JOKER1"})
	st.DiscardTakenCard = "QH"
	st.Rules = ProfileContinental

	d := FindOffer(LegalActions(st, "p1"), OfferDiscard)
	if !d.Enabled || !slices.Equal(d.Source.Cards, []string{"QH"}) {
		t.Fatalf("enabled=%v cards=%v, want QH alone discardable", d.Enabled, d.Source.Cards)
	}
	for _, r := range d.Source.Refused {
		if r.Card == "QH" {
			t.Fatalf("QH refused (%s) as the only legal discard", r.WhyNot)
		}
	}
	if got := HeldBackTakenCard(st, "p1"); got != "" {
		t.Fatalf("HeldBackTakenCard = %q, want nothing held back", got)
	}
}

// A refusal about the turn rather than the card is still the offer's: it is
// not repeated per card, and the offer stays disabled.
func TestDiscardOffer_TurnWideRefusalIsNotListedPerCard(t *testing.T) {
	st := downStateHolding([]string{"QH", "4S", "3S"})
	st.RoundReqMet["p1"] = false
	st.DiscardTakenCard = "QH"
	st.DiscardDrawnCardPendingMeld = "QH"

	d := FindOffer(LegalActions(st, "p1"), OfferDiscard)
	if d.Enabled || d.WhyNot != ErrDiscardCardNotMelded {
		t.Fatalf("enabled=%v why=%s, want disabled by %s", d.Enabled, d.WhyNot, ErrDiscardCardNotMelded)
	}
	if len(d.Source.Refused) != 0 {
		t.Fatalf("turn-wide refusal repeated per card: %v", d.Source.Refused)
	}
	if got := HeldBackTakenCard(st, "p1"); got != "" {
		t.Fatalf("HeldBackTakenCard = %q for an owed pickup, want empty", got)
	}
}
