package holdem

import (
	"testing"

	"zolik/server/internal/module"
)

// What a refused player actually reads.
//
// Poker's refusals are all about a number, and the number is exactly what the
// code leaves out: "you can't check" and "a raise has to be at least the last
// one" are both true and neither says what to do. These assert the figure is
// in the sentence, and that it is the right figure — a remedy that names a
// number the engine would reject is worse than no remedy at all.

func offerByID(offers []module.ActionOffer, id string) *module.ActionOffer {
	for i := range offers {
		if offers[i].ID == id {
			return &offers[i]
		}
	}
	return nil
}

func TestCheckRefusal_NamesWhatIsOwed(t *testing.T) {
	m := New()
	// A seat facing a bet of 60 having put up the big blind of 20: forty owed.
	raw := table(2, func(s *GameState) {
		s.CurrentBet = 60
		s.MinRaise = 40
		s.Seats[0].Bet = 20
		s.Seats[1].Bet = 60
		s.Current = 0
	})

	offers, err := m.LegalActions(raw, "p1")
	if err != nil {
		t.Fatal(err)
	}
	check := offerByID(offers, OfferCheck)
	if check == nil || check.Enabled {
		t.Fatalf("checking into a bet should be refused: %+v", check)
	}
	if check.Remedy == nil {
		t.Fatal("refused with a bare code and no figure")
	}
	if got := check.Remedy.Params["n"]; got != 40 {
		t.Errorf("told the player %v is owed; the bet is 60 against their 20, so it is 40", got)
	}
	if check.RemedyOfferID != OfferCall {
		t.Errorf("remedy points at %q, want the call", check.RemedyOfferID)
	}
	if len(check.RuleIDs) == 0 {
		t.Error("nothing explains when a check is allowed")
	}
}

// The figure a too-small raise is told is the one the engine would actually
// accept, checked by submitting it. A remedy naming a number that is itself
// refused would send the player round the loop a second time.
func TestRaiseRefusal_NamesAFigureTheEngineTakes(t *testing.T) {
	m := New()
	raw := table(3, func(s *GameState) {
		s.CurrentBet = 100
		s.MinRaise = 80
		s.Seats[0].Bet = 20
		s.Seats[0].Stack = 980
		s.Seats[1].Bet = 100
		s.Seats[2].Bet = 100
		s.Current = 0
	})

	// A raise the engine refuses as too small, asked for directly: the offer
	// list only ever probes the legal minimum, so this is the path a player
	// takes by dragging a slider rather than pressing a button.
	_, _, err := m.Apply(raw, "p1", module.Action{
		Verb: VerbRaise, Params: map[string]string{ParamAmount: "120"},
	})
	if code := module.CodeOf(err); code != ErrRaiseTooSmall {
		t.Fatalf("a 120 raise over a 100 bet with an 80 minimum gave %q, want %s", code, ErrRaiseTooSmall)
	}

	s, err := decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	minTo, _ := raiseRange(s, &s.Seats[0])
	if minTo != 180 {
		t.Fatalf("minimum raise is %d, want 180 (100 + 80)", minTo)
	}
	// The remedy would name minTo; the point of the test is that naming it is
	// safe, so the same figure goes back through the engine.
	if _, _, err := m.Apply(raw, "p1", module.Action{
		Verb: VerbRaise, Params: map[string]string{ParamAmount: "180"},
	}); err != nil {
		t.Errorf("the figure the remedy names was itself refused: %v", err)
	}
}

// A seat that cannot get above the bet even all in has no raise coming back
// this street, so it is told to call all in rather than to raise more.
func TestShortStack_IsToldToCallAllIn(t *testing.T) {
	m := New()
	raw := table(3, func(s *GameState) {
		s.CurrentBet = 500
		s.MinRaise = 500
		s.Seats[0].Bet = 0
		s.Seats[0].Stack = 300
		s.Seats[1].Bet = 500
		s.Seats[2].Bet = 500
		s.Current = 0
	})

	offers, err := m.LegalActions(raw, "p1")
	if err != nil {
		t.Fatal(err)
	}
	raise := offerByID(offers, OfferRaise)
	if raise == nil || raise.Enabled {
		t.Fatalf("a seat that cannot clear the bet should not be offered a raise: %+v", raise)
	}
	if raise.WhyNot != ErrCannotRaise {
		t.Fatalf("refused with %q, want %s", raise.WhyNot, ErrCannotRaise)
	}
	if raise.Remedy == nil || raise.Remedy.LabelKey != "holdem.remedy.callAllInOrFold" {
		t.Fatalf("remedy is %+v, want the one that says to call all in", raise.Remedy)
	}
	if got := raise.Remedy.Params["n"]; got != 300 {
		t.Errorf("told the player to call %v; their whole stack is 300", got)
	}
}
