package holdem

import (
	"testing"

	"zolik/server/internal/module"
)

func cashMatch(t *testing.T, players ...module.PlayerRef) module.State {
	t.Helper()
	raw, err := New().NewMatch(module.MatchConfig{Options: module.Options{OptFormat: FormatCash}}, players, 7)
	if err != nil {
		t.Fatalf("new match: %v", err)
	}
	return raw
}

func people(ids ...string) []module.PlayerRef {
	out := make([]module.PlayerRef, 0, len(ids))
	for _, id := range ids {
		out = append(out, module.PlayerRef{ID: id, Name: id})
	}
	return out
}

func hasOffer(offers []module.ActionOffer, verb string) *module.ActionOffer {
	for i := range offers {
		if offers[i].Verb == verb {
			return &offers[i]
		}
	}
	return nil
}

// A tournament is played for the chips: nobody gets up with theirs, and
// nobody is offered to.
func TestNobodyLeavesATournament(t *testing.T) {
	raw, err := New().NewMatch(module.MatchConfig{}, people("p1", "p2", "p3"), 7)
	if err != nil {
		t.Fatal(err)
	}
	if _, code := apply(t, raw, "p2", module.Action{Verb: module.VerbLeave}); code != ErrLeaveTournament {
		t.Fatalf("leaving a tournament: code %q, want %s", code, ErrLeaveTournament)
	}
	offers, _ := New().LegalActions(raw, "p2")
	if hasOffer(offers, module.VerbLeave) != nil {
		t.Error("a tournament offers to leave")
	}
}

// At a cash table a player can get up whenever they like — here off turn,
// mid-hand — with what is in front of them. What they put in this hand
// stays in the pot, and the hand goes on without them.
func TestLeavingACashTableMidHand(t *testing.T) {
	raw := cashMatch(t, people("p1", "p2", "p3")...)
	s := mustDecode(t, raw)
	onTurn := s.Seats[s.Current].PlayerID
	var leaver string
	for _, st := range s.Seats {
		if st.PlayerID != onTurn {
			leaver = st.PlayerID
			break
		}
	}
	before := *s.seat(leaver)

	offers, _ := New().LegalActions(raw, leaver)
	o := hasOffer(offers, module.VerbLeave)
	if o == nil || !o.Enabled || !o.Manual {
		t.Fatalf("leave offer = %+v, want an enabled, manual one off turn", o)
	}
	if offers[len(offers)-1].Verb != module.VerbLeave {
		t.Error("leave is not the last offer")
	}

	raw, code := apply(t, raw, leaver, module.Action{Verb: module.VerbLeave})
	if code != "" {
		t.Fatalf("leave refused: %s", code)
	}
	s = mustDecode(t, raw)
	gone := s.seat(leaver)
	if !gone.Left || !gone.Out || !gone.Folded {
		t.Fatalf("leaver = %+v, want left, out and folded", gone)
	}
	if gone.Stack != before.Stack {
		t.Errorf("stack went %d -> %d; leaving moves no chips", before.Stack, gone.Stack)
	}
	if s.Status != "active" || s.Seats[s.Current].PlayerID != onTurn {
		t.Errorf("the hand did not go on with %s to act: status %s, current %d", onTurn, s.Status, s.Current)
	}
	if _, code := apply(t, raw, leaver, module.Action{Verb: module.VerbLeave}); code != ErrAlreadyLeft {
		t.Errorf("leaving twice: code %q, want %s", code, ErrAlreadyLeft)
	}
	for _, id := range order(s) {
		if id == leaver {
			t.Error("a seat that got up is still waited on between hands")
		}
	}
}

// Heads-up, the other player leaving ends the match, and the standings rank
// the chips each walked away with.
func TestTheLastOpponentLeavingEndsTheMatch(t *testing.T) {
	raw := cashMatch(t, people("p1", "p2")...)
	s := mustDecode(t, raw)
	leaver := s.Seats[s.Current].PlayerID
	raw, code := apply(t, raw, leaver, module.Action{Verb: module.VerbLeave})
	if code != "" {
		t.Fatalf("leave refused: %s", code)
	}
	// The hand is over and the showdown is up; going on ends the match.
	s = mustDecode(t, raw)
	for s.Status == "active" && s.Break.Open {
		var stayer string
		for _, id := range order(s) {
			stayer = id
		}
		raw, code = apply(t, raw, stayer, module.Action{Verb: module.VerbContinue})
		if code != "" {
			t.Fatalf("continue refused: %s", code)
		}
		s = mustDecode(t, raw)
	}
	if s.Status != "completed" {
		t.Fatalf("status = %q, want completed with one player left", s.Status)
	}
	standings, _ := New().Standings(raw)
	if len(standings) != 2 {
		t.Fatalf("standings = %+v, want both seats — the leaver's chips are their result", standings)
	}
}

// A table where every person has got up ends, rather than leaving the bots to
// play on to nobody.
func TestATableWithOnlyBotsLeftEnds(t *testing.T) {
	raw := cashMatch(t, module.PlayerRef{ID: "ann", Name: "Ann"}, module.PlayerRef{ID: "b1", IsAI: true}, module.PlayerRef{ID: "b2", IsAI: true})
	raw, code := apply(t, raw, "ann", module.Action{Verb: module.VerbLeave})
	if code != "" {
		t.Fatalf("leave refused: %s", code)
	}
	s := mustDecode(t, raw)
	// Play the hand out between the bots, folding where it can.
	for i := 0; i < 50 && s.Status == "active"; i++ {
		if s.Break.Open {
			for _, id := range order(s) {
				raw, _ = apply(t, raw, id, module.Action{Verb: module.VerbContinue})
			}
		} else {
			id := s.Seats[s.Current].PlayerID
			if raw2, c := apply(t, raw, id, module.Action{Verb: VerbCheck}); c == "" {
				raw = raw2
			} else {
				raw, _ = apply(t, raw, id, module.Action{Verb: VerbFold})
			}
		}
		s = mustDecode(t, raw)
	}
	if s.Status != "completed" {
		t.Fatalf("status = %q after the last person left, want completed", s.Status)
	}
}

// Leaving during the showdown counts as going on: the table is not left
// waiting for somebody who is not there.
func TestLeavingAtTheShowdownDoesNotHoldItUp(t *testing.T) {
	raw := cashMatch(t, people("p1", "p2", "p3")...)
	s := mustDecode(t, raw)
	// Fold round to the showdown.
	for i := 0; i < 5 && !s.Break.Open; i++ {
		raw, _ = apply(t, raw, s.Seats[s.Current].PlayerID, module.Action{Verb: VerbFold})
		s = mustDecode(t, raw)
	}
	if !s.Break.Open {
		t.Fatal("never reached the showdown")
	}
	ids := order(s)
	raw, _ = apply(t, raw, ids[0], module.Action{Verb: module.VerbContinue})
	raw, _ = apply(t, raw, ids[1], module.Action{Verb: module.VerbContinue})
	raw, code := apply(t, raw, ids[2], module.Action{Verb: module.VerbLeave})
	if code != "" {
		t.Fatalf("leave at the showdown refused: %s", code)
	}
	s = mustDecode(t, raw)
	if s.Break.Open {
		t.Error("the showdown is still waiting after the last player to go on left")
	}
	if s.Status != "active" || s.HandNumber != 2 {
		t.Errorf("status %q hand %d, want the next hand dealt to the two still seated", s.Status, s.HandNumber)
	}
}
