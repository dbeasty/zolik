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
	// Its points ride with its name, so the seat's total can be read off the table.
	if z.Groups[0].Value != "40" {
		t.Fatalf("p2's pinochle shows %q, want its 40 points", z.Groups[0].Value)
	}
	if f := seatFact(vm, "p1", "pinochle.seat.meld"); f == nil || f.Params["n"] != 20 {
		t.Fatalf("p1's meld total is not on its seat: %+v", vm.SeatOf("p1"))
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

func seatFact(vm module.ViewModel, who, key string) *module.Fact {
	seat := vm.SeatOf(who)
	if seat == nil {
		return nil
	}
	for i := range seat.Facts {
		if seat.Facts[i].LabelKey == key {
			return &seat.Facts[i]
		}
	}
	return nil
}

// TestEverySideSeesTheRunningScore: once a trick is taken, every seat carries
// its meld and its side's points taken, so a player reads the hand's score off
// the table while it is played, the other side's included, and the meld does
// not vanish once the cards go back into hand.
func TestEverySideSeesTheRunningScore(t *testing.T) {
	hands := map[string][]string{
		"p1": {"KC", "QC", "9D"}, "p2": {"QS", "JD", "9H"}, "p3": {"9C", "TD", "AH"}, "p4": {"JC", "JH", "AS"},
	}
	s := playingAt(t, "S", hands)
	s = must(t, s, "p1", card("9D"))
	s = must(t, s, "p2", card("JD"))
	s = must(t, s, "p3", card("TD"))
	s = must(t, s, "p4", card("AS"))
	if s.Taken[s.side("p4")] == 0 {
		t.Fatalf("the trick scored nothing: %+v", s.Taken)
	}
	vm, _ := New().View(mustEncode(t, s), "p1")
	for _, p := range s.Players {
		taken := seatFact(vm, p, "pinochle.seat.taken")
		if taken == nil || taken.Params["n"] != s.Taken[s.side(p)] {
			t.Errorf("%s: taken %+v, want its side's %d", p, taken, s.Taken[s.side(p)])
		}
		if meld := seatFact(vm, p, "pinochle.seat.meld"); meld == nil || meld.Params["n"] != s.Melds[p].Total {
			t.Errorf("%s: meld %+v, want %d", p, meld, s.Melds[p].Total)
		}
	}
	for _, f := range vm.Header {
		if f.LabelKey == "pinochle.header.sideTaken" {
			t.Errorf("the header repeats the seats: %+v", f)
		}
	}

	// Switched off, a player keeps only their own side's count, in the header.
	s.HideCardPoints = true
	vm, _ = New().View(mustEncode(t, s), "p1")
	for _, p := range s.Players {
		if f := seatFact(vm, p, "pinochle.seat.taken"); f != nil {
			t.Errorf("%s: taken shown with the option off: %+v", p, f)
		}
	}
	found := false
	for _, f := range vm.Header {
		found = found || f.LabelKey == "pinochle.header.sideTaken"
	}
	if !found {
		t.Errorf("with the option off the viewer lost their own count: %+v", vm.Header)
	}
}

// TestTwoHandedSeatsCarryMeldAndTaken: two-handed melds as it plays, so each
// seat's meld is what it has been credited so far.
func TestTwoHandedSeatsCarryMeldAndTaken(t *testing.T) {
	s := newState(t, VarTwoHanded, nil)
	s.MeldScore = map[string]int{s.side("p1"): 40, s.side("p2"): 0}
	s.Taken = map[string]int{s.side("p1"): 10, s.side("p2"): 20}
	vm, _ := New().View(mustEncode(t, s), "p2")
	if f := seatFact(vm, "p1", "pinochle.seat.meld"); f == nil || f.Params["n"] != 40 {
		t.Errorf("p1 meld: %+v", f)
	}
	if f := seatFact(vm, "p1", "pinochle.seat.taken"); f == nil || f.Params["n"] != 10 {
		t.Errorf("p1 taken: %+v", f)
	}
	if f := seatFact(vm, "p2", "pinochle.seat.taken"); f == nil || f.Params["n"] != 20 {
		t.Errorf("p2 taken: %+v", f)
	}
}
