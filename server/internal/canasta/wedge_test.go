package canasta

import (
	"testing"

	"zolik/server/internal/module"
)

// The turn that froze game 6aaa157d0079d0b3a6624b3a, and the two things that
// now keep one from freezing again.
//
// A Samba side captured the pile with a pair of fives, putting fifteen points
// on the table towards an opening it had not made yet, and then found the
// engine refusing every meld it tried — including the three that between them
// were worth more than it still needed. The turn had one legal move in it,
// undo_take_pile, and the bot driving the seat took it and then took the pile
// again, and again, until the runtime gave up and the table stopped for
// everybody at it.
//
// The position below is the same fault one floor band lower, reproduced from
// self-play: a side on 3075 needing 120, fifteen of it laid by the capture. The
// production game was the 150 band, which is only a bigger version of the same
// arithmetic.
//
// Two faults, and both are below. reachableValue said seventy where a hundred
// and ten was there to be had, which is what made a legal opening illegal; and
// the capture stopped being undoable the moment anything was laid on top of it,
// which is what left an unopened side with cards it could not pick back up.

// The bound that refused the opening.
//
// The hand holds two jacks, two queens, three tens and two deuces, and the side
// has three fives on the table. The three melds a player would actually lay —
// jacks with one deuce, queens with the other, tens on their own — are worth a
// hundred and ten. reachableValue counted seventy, because its first pass was a
// lay-off and a lay-off will take both deuces onto the fives for twenty points
// apiece, leaving nothing to hold either pair together.
func TestAWildIsNotSpentOnALayOffWhenAMeldNeedsIt(t *testing.T) {
	r := resolveVariation("samba")
	team := &Team{ID: 0, Score: 3075, Melds: []Meld{{
		ID: "t0-5", TeamID: 0, Kind: meldSet, Rank: "5", Cards: []string{"5D", "5S", "5D"},
	}}}
	hand := []string{"JH", "TD", "2D", "JC", "QS", "8D", "3S", "KH", "TH", "9S", "2C", "TS", "QD", "7H"}

	// 40 + 40 + 30: jacks and a deuce, queens and a deuce, three tens.
	const achievable = 110
	if got := reachableValue(r, hand, team); got < achievable {
		t.Errorf("reachableValue = %d, want at least %d — the hand can lay that much", got, achievable)
	}

	// And the number is what the floor is actually measured against, so the
	// opening this hand can reach is one the engine now accepts.
	if floor := r.meldFloor(team.Score); 15+reachableValue(r, hand, team) < floor {
		t.Errorf("a side that can reach the %d floor is still being told it cannot "+
			"(15 laid, %d reachable)", floor, reachableValue(r, hand, team))
	}
}

// The same hand, driven through the engine rather than through the bound: every
// meld in it goes down, and the turn ends in a discard instead of in an undo.
func TestTheCapturedTurnCanStillOpenTheAccount(t *testing.T) {
	m := New()
	raw := twoHanded(func(s *GameState) {
		s.Variation = "samba"
		s.Rules = rulesPtr("samba")
		s.HandSize, s.TargetScore, s.CanastasToGoOut = 15, 10000, 2
		s.Phase = phaseMeld
		s.Teams[0].Score, s.Teams[1].Score = 3075, 2260
		s.Teams[0].Melds = []Meld{{
			ID: "t0-5", TeamID: 0, Kind: meldSet, Rank: "5", Cards: []string{"5D", "5S", "5D"},
		}}
		s.LaidThisTurn = 15
		s.TookPileThisTurn = true
		s.Hands["p1"] = []string{"JH", "TD", "2D", "JC", "QS", "8D", "3S", "KH", "TH", "9S", "2C", "TS", "QD", "7H"}
	})

	for _, cards := range [][]string{
		{"JC", "JH", "2C"},
		{"QD", "QS", "2D"},
		{"TD", "TH", "TS"},
	} {
		next, _, err := m.Apply(raw, "p1", module.Action{Verb: VerbLayMeld, Cards: cards})
		if err != nil {
			t.Fatalf("laying %v was refused: %v", cards, err)
		}
		raw = next
	}

	s, err := decode(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !s.Teams[0].HasMelded {
		t.Fatalf("the side laid %d against a floor of 150 and is still not open", s.LaidThisTurn)
	}
	// The whole point of opening: the turn can now be ended the ordinary way.
	if _, _, err := m.Apply(raw, "p1", module.Action{Verb: VerbDiscard, Cards: []string{"8D"}}); err != nil {
		t.Errorf("an opened side could not discard to end its turn: %v", err)
	}
}

// Undoing has to reach the beginning of the turn, or it does not reach
// anywhere.
//
// A side that has not opened cannot discard, so a turn that took the pile and
// then melded its way to just under the floor has only one way out: put it all
// back. The capture used to fall out of the undo window the instant anything
// was laid on top of it, which left exactly that turn holding cards it could
// not pick up and no move it could make.
func TestATurnUnwindsAllTheWayBackToTheCapture(t *testing.T) {
	m := New()
	raw := twoHanded(func(s *GameState) {
		s.Variation = "samba"
		s.Rules = rulesPtr("samba")
		s.HandSize, s.TargetScore, s.CanastasToGoOut = 15, 10000, 2
		s.Teams[0].Score = 3075
		s.Hands["p1"] = []string{
			"5S", "5D",
			"JH", "TD", "2D", "JC", "QS", "8D", "3S", "KH", "TH", "9S", "2C", "TS", "QD", "7H",
		}
		s.DiscardPile = []string{"9C", "5D"}
	})
	before, err := decode(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	raw = mustApply(t, raw, module.Action{Verb: VerbTakePile, Cards: []string{"5S", "5D"}})
	raw = mustApply(t, raw, module.Action{Verb: VerbLayMeld, Cards: []string{"JC", "JH", "2C"}})

	// Out of order: the meld is sitting on top of the capture, and putting the
	// hand back as it was would hand back the cards that meld is made of.
	if _, _, err := m.Apply(raw, "p1", module.Action{Verb: VerbUndoTakePile}); err == nil {
		t.Fatal("the capture was undone with a meld still standing on top of it")
	} else if code := module.CodeOf(err); code != ErrUndoMeldsFirst {
		t.Fatalf("undoing out of order gave %q, want %q", code, ErrUndoMeldsFirst)
	}

	// In order, it unwinds the whole way.
	raw = mustApply(t, raw, module.Action{Verb: VerbUndoLayMeld})
	raw = mustApply(t, raw, module.Action{Verb: VerbUndoTakePile})

	after, err := decode(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got, want := after.Hands["p1"], before.Hands["p1"]; !sameCards(got, want) {
		t.Errorf("hand came back as %v, want %v", got, want)
	}
	if got, want := after.DiscardPile, before.DiscardPile; !sameCards(got, want) {
		t.Errorf("pile came back as %v, want %v", got, want)
	}
	if len(after.Teams[0].Melds) != 0 {
		t.Errorf("melds left on the table after unwinding: %v", after.Teams[0].Melds)
	}
	if after.LaidThisTurn != 0 || after.Teams[0].HasMelded {
		t.Errorf("laidThisTurn=%d hasMelded=%v, want a turn that has laid nothing",
			after.LaidThisTurn, after.Teams[0].HasMelded)
	}
	if after.Phase != phaseDraw {
		t.Errorf("phase=%q, want %q — a turn back at its beginning can draw instead",
			after.Phase, phaseDraw)
	}
	// Which is the move that makes this a way out rather than a circle.
	if _, _, err := m.Apply(raw, "p1", module.Action{Verb: VerbDraw}); err != nil {
		t.Errorf("a turn wound back to its start could not draw: %v", err)
	}
}

// The runtime's replay guard reads this flag off the offer rather than off the
// verb's spelling — see module.ActionOffer.Undo and match.botTurn. An undo that
// forgot to declare itself would put the oscillation straight back.
func TestTheUndoOffersSayTheyAreUndos(t *testing.T) {
	m := New()
	raw := twoHanded(func(s *GameState) {
		s.Variation = "samba"
		s.Rules = rulesPtr("samba")
		s.HandSize, s.TargetScore, s.CanastasToGoOut = 15, 10000, 2
		s.Hands["p1"] = []string{
			"5S", "5D",
			"JH", "TD", "2D", "JC", "QS", "8D", "3S", "KH", "TH", "9S", "2C", "TS", "QD", "7H",
		}
		s.DiscardPile = []string{"9C", "5D"}
	})
	raw = mustApply(t, raw, module.Action{Verb: VerbTakePile, Cards: []string{"5S", "5D"}})
	raw = mustApply(t, raw, module.Action{Verb: VerbLayMeld, Cards: []string{"JC", "JH", "2C"}})

	offers, err := m.LegalActions(raw, "p1")
	if err != nil {
		t.Fatalf("LegalActions: %v", err)
	}
	for _, id := range []string{OfferUndoTakePile, OfferUndoLayMeld} {
		o := module.FindOffer(offers, id)
		if o == nil {
			t.Errorf("%s is not on offer in a turn that has both to take back", id)
			continue
		}
		if !o.Undo {
			t.Errorf("%s does not declare itself an undo", id)
		}
	}
	for _, o := range offers {
		if o.Undo && o.Verb != VerbUndoTakePile && o.Verb != VerbUndoLayMeld && o.Verb != VerbUndoLayOff {
			t.Errorf("offer %q declares itself an undo but is not one", o.ID)
		}
	}
}

// mustApply is engine_test.go's apply with the refusal turned into a failure —
// every action below is one the turn is supposed to be able to make.
func mustApply(t *testing.T, raw module.State, a module.Action) module.State {
	t.Helper()
	next, code := apply(t, raw, "p1", a)
	if code != "" {
		t.Fatalf("%s %v was refused: %s", a.Verb, a.Cards, code)
	}
	return next
}

func rulesPtr(variation string) *ruleset {
	r := resolveVariation(variation)
	return &r
}

func sameCards(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]int{}
	for _, c := range a {
		seen[c]++
	}
	for _, c := range b {
		seen[c]--
		if seen[c] < 0 {
			return false
		}
	}
	return true
}
