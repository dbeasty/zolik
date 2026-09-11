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

// A hand with four queens may lay three of them. The offer used to demand
// exactly four — it declared the candidate's size as its minimum — so every
// shorter selection sat unready on screen and the meld zone stayed dark.
func TestAGroupWillSettleForFewerCardsThanItOffers(t *testing.T) {
	raw := twoHanded(func(s *GameState) {
		s.Phase = phaseMeld
		s.Teams[0].HasMelded = true
		s.Hands["p1"] = []string{"QH", "QS", "QD", "QC", "8C", "9C", "TD"}
	})

	offers, err := New().LegalActions(raw, "p1")
	if err != nil {
		t.Fatalf("LegalActions: %v", err)
	}
	var queens *module.Selector
	for _, o := range offers {
		if o.Verb == VerbLayMeld && o.Source != nil && len(o.Source.Cards) > 0 && rankOf(o.Source.Cards[0]) == "Q" {
			queens = o.Source
		}
	}
	if queens == nil {
		t.Fatal("no meld offer for the queens")
	}
	if queens.MinCards != minMeldSize {
		t.Errorf("minCards=%d, want %d — a three-queen meld is legal here", queens.MinCards, minMeldSize)
	}
	if queens.MaxCards != 4 {
		t.Errorf("maxCards=%d, want 4", queens.MaxCards)
	}
	// The press still lays all four: the minimum says what a player may do,
	// Submit says what a button does.
	if len(queens.Submit) != 4 {
		t.Errorf("submit=%v, want all four queens", queens.Submit)
	}
	if _, code := apply(t, raw, "p1", module.Action{
		Verb: VerbLayMeld, Cards: queens.Cards[:queens.MinCards],
	}); code != "" {
		t.Errorf("the engine refused the minimum the offer advertised: %s", code)
	}
}

// The minimum is not a property of the cards: before a partnership opens, five
// eights are legal only all five together, because three of them are twenty
// short of the floor and the two left over cannot get there. So the offer says
// five — and would be wrong to assume the meld minimum.
func TestTheMinimumFollowsTheTableNotTheCards(t *testing.T) {
	raw := twoHanded(func(s *GameState) {
		s.Phase = phaseMeld
		s.Teams[0].HasMelded = false
		s.Teams[0].Score = 0 // a floor of 50; five eights make exactly that
		s.Hands["p1"] = []string{"8C", "8D", "8H", "8S", "8C", "9C", "TD"}
	})

	offers, err := New().LegalActions(raw, "p1")
	if err != nil {
		t.Fatalf("LegalActions: %v", err)
	}
	found := false
	for _, o := range offers {
		if o.Verb != VerbLayMeld || o.Source == nil || len(o.Source.Cards) == 0 || rankOf(o.Source.Cards[0]) != "8" {
			continue
		}
		found = true
		if o.Source.MinCards != 5 {
			t.Errorf("minCards=%d, want 5 — a shorter meld does not reach the floor", o.Source.MinCards)
		}
		// Whatever it says the minimum is, the engine has to accept it.
		if _, code := apply(t, raw, "p1", module.Action{
			Verb: VerbLayMeld, Cards: o.Source.Cards[:o.Source.MinCards],
		}); code != "" {
			t.Errorf("offer advertised minCards=%d but the engine refused it: %s", o.Source.MinCards, code)
		}
		// And the shorter one really is refused, or this table proves nothing.
		if _, code := apply(t, raw, "p1", module.Action{
			Verb: VerbLayMeld, Cards: o.Source.Cards[:minMeldSize],
		}); code != ErrInitialMeldNotMet {
			t.Errorf("three eights were refused with %q, want %s", code, ErrInitialMeldNotMet)
		}
	}
	if !found {
		t.Fatal("no meld offer for the eights")
	}
}

// The whole reason the driver had to change: a bot reading these offers must
// still lay the biggest meld, not the smallest one it is now told it may lay.
func TestABotStillLaysTheWholeCandidate(t *testing.T) {
	raw := twoHanded(func(s *GameState) {
		s.Phase = phaseMeld
		s.Teams[0].HasMelded = true
		s.Hands["p1"] = []string{"QH", "QS", "QD", "QC", "8C", "9C", "TD"}
	})
	offers, err := New().LegalActions(raw, "p1")
	if err != nil {
		t.Fatalf("LegalActions: %v", err)
	}
	a, ok := module.ChooseAction(offers, []string{VerbLayMeld})
	if !ok {
		t.Fatal("the driver found nothing to do")
	}
	if a.Verb != VerbLayMeld || len(a.Cards) != 4 {
		t.Errorf("driver chose %s %v, want all four queens", a.Verb, a.Cards)
	}
	if _, code := apply(t, raw, "p1", a); code != "" {
		t.Errorf("the driver's own choice was refused: %s", code)
	}
}
