package canasta

import (
	"testing"

	"zolik/server/internal/module"
)

// Two decks are in play, so a hand can hold two cards with the same name. An
// offer that lists the cards it would take has to say so, or the client — which
// counts what it is holding — cannot send a move the engine would accept.
//
// The lay-off after a pile take is where this was found and where it bites
// hardest: the take drops a batch of one rank into a hand all at once, right
// beside the meld it just made.
func TestLayOffOffersEveryCopyItWouldTake(t *testing.T) {
	raw := twoHanded(func(s *GameState) {
		s.Teams[0].HasMelded = true
		// A seven on top and two more buried, both of them the same card.
		s.DiscardPile = []string{"7H", "KD", "7H", "9S", "7H"}
		s.Hands["p1"] = []string{"7C", "7S", "8C", "9C", "TD"}
	})

	raw, code := apply(t, raw, "p1", module.Action{Verb: VerbTakePile, Cards: []string{"7C", "7S"}})
	if code != "" {
		t.Fatalf("take_pile refused: %s", code)
	}
	s := mustDecode(t, raw)
	meld := s.Teams[0].Melds[0]

	held := 0
	for _, c := range s.Hands["p1"] {
		if c == "7H" {
			held++
		}
	}
	if held != 2 {
		t.Fatalf("setup: expected two 7H in hand, got %d (%v)", held, s.Hands["p1"])
	}

	offers, err := New().LegalActions(raw, "p1")
	if err != nil {
		t.Fatalf("LegalActions: %v", err)
	}
	var offered []string
	for _, o := range offers {
		if o.Verb == VerbLayOff && o.Target != nil && o.Target.MeldID == meld.ID {
			offered = o.Source.Cards
		}
	}
	count := 0
	for _, c := range offered {
		if c == "7H" {
			count++
		}
	}
	if count != held {
		t.Errorf("hand holds %d×7H, offer advertises %d (%v)", held, count, offered)
	}

	// And the move the offer now describes is one the engine takes.
	if _, code := apply(t, raw, "p1", module.Action{
		Verb: VerbLayOff, Cards: []string{"7H", "7H"}, Target: meld.ID,
	}); code != "" {
		t.Errorf("engine refused the two-copy lay-off: %s", code)
	}
}

// A run takes each card once — the same card cannot extend a sequence twice —
// so its extension list is the one lay-off list that stays duplicate-free.
func TestRunExtensionsStayDistinct(t *testing.T) {
	r := resolveVariation("samba")
	if !r.Sequences {
		t.Skip("no sequence variation to check")
	}
	m := &Meld{ID: "t0-run", TeamID: 0, Kind: meldRun, Suit: "H", Cards: []string{"5H", "6H", "7H"}}
	if m.kind() != meldRun {
		t.Skipf("meld did not read as a run: %+v", m)
	}
	got := layOffCards(r, []string{"8H", "8H", "4H"}, m)
	eights := 0
	for _, c := range got {
		if c == "8H" {
			eights++
		}
	}
	if eights != 1 {
		t.Errorf("a run offered %d copies of 8H, want 1 (%v)", eights, got)
	}
}

// Three black threes are three black threes whether or not two of them are the
// same card — the going-out meld has to be offered either way.
func TestBlackThreeCandidateCountsCopies(t *testing.T) {
	got := blackThreeCandidate([]string{"3C", "3C", "3S", "AH"})
	if len(got) != 3 {
		t.Errorf("got %v, want three black threes", got)
	}
	if err := validateBlackThreeMeld(got); err != nil {
		t.Errorf("the candidate is not a legal meld: %v", err)
	}
}
