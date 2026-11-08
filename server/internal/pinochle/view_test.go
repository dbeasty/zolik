package pinochle

import (
	"testing"

	"zolik/server/internal/module"
)

func TestViewHidesWhatIsHidden(t *testing.T) {
	for _, v := range allVariations {
		s := newState(t, v, nil)
		raw := mustEncode(t, s)
		me := s.Players[0]
		vm, err := New().View(raw, me)
		if err != nil {
			t.Fatal(err)
		}
		for _, z := range vm.Zones {
			switch {
			case z.ID == handZoneID(me):
				if len(z.Cards) != len(s.Hands[me]) {
					t.Errorf("%s: own hand shows %d of %d", v, len(z.Cards), len(s.Hands[me]))
				}
			case z.Kind == module.ZoneHand:
				if len(z.Cards) > 0 {
					t.Errorf("%s: %s is face up to %s", v, z.ID, me)
				}
			case z.ID == stockZoneID, z.ID == widowZoneID && s.Phase == phaseAuction:
				if len(z.Cards) > 0 {
					t.Errorf("%s: %s is face up", v, z.ID)
				}
			}
		}
		open, _ := New().OpenView(raw)
		for _, z := range open.Zones {
			if z.Kind == module.ZoneHand && len(z.Cards) != z.Count {
				t.Errorf("%s: the open view hides %s", v, z.ID)
			}
		}
	}
}

func TestMeldIsOnTheTableForTheFirstTrick(t *testing.T) {
	hands := map[string][]string{
		"p1": {"KC", "QC", "9D"}, "p2": {"QS", "JD", "9H"}, "p3": {"9C", "TD", "AH"}, "p4": {"JC", "JH", "AS"},
	}
	s := playingAt(t, "S", hands)
	vm, _ := New().View(mustEncode(t, s), "p3")
	z := zoneByID(vm, meldZoneID("p2"))
	if z == nil || len(z.Groups) != 1 || z.Groups[0].Kind != MeldPinochle {
		t.Fatalf("p2's pinochle is not on the table: %+v", z)
	}
	if seat := vm.SeatOf("p1"); seat == nil || len(seat.Facts) == 0 || seat.Facts[len(seat.Facts)-1].Params["n"] != 20 {
		t.Fatalf("p1's meld total is not on its seat: %+v", seat)
	}
	s = must(t, s, "p1", card("9D"))
	s = must(t, s, "p2", card("JD"))
	s = must(t, s, "p3", card("TD"))
	s = must(t, s, "p4", card("AS"))
	vm, _ = New().View(mustEncode(t, s), "p3")
	if z := zoneByID(vm, meldZoneID("p2")); z != nil {
		t.Fatalf("the meld is still out after the first trick: %+v", z)
	}
}

func TestOffersCarryTheBidRange(t *testing.T) {
	s := newState(t, VarPartnership, nil)
	offers, _ := New().LegalActions(mustEncode(t, s), s.Current)
	var bidOffer *module.ActionOffer
	for i := range offers {
		if offers[i].ID == OfferBid {
			bidOffer = &offers[i]
		}
	}
	if bidOffer == nil || !bidOffer.Enabled || len(bidOffer.Params) != 1 {
		t.Fatalf("bid offer: %+v", bidOffer)
	}
	p := bidOffer.Params[0]
	if p.Kind != module.ParamKindInt || p.Min != 250 || p.Step != 10 || p.Default != 250 {
		t.Fatalf("bid range: %+v", p)
	}
	// Another seat sees the offer, refused with a reason that points at a
	// written rule.
	other := s.next(s.Current)
	offers, _ = New().LegalActions(mustEncode(t, s), other)
	for _, o := range offers {
		if o.ID == OfferBid && (o.Enabled || o.WhyNot != ErrNotYourTurn || len(o.RuleIDs) == 0 || o.Remedy == nil) {
			t.Fatalf("someone else's bid offer: %+v", o)
		}
	}
}
