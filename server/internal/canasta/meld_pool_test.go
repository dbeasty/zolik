package canasta

import (
	"testing"

	"zolik/server/internal/module"
)

// A meld offer names two different things — what a press sends, and what a
// person may pick — and for a long time it named only the first for both.
//
// The bug that produced this file: in Samba, a hand holding three jacks and a
// two could not lay JJ2. The engine had no objection at all; the offer listed
// the three jacks as the cards a selection may draw from, the client checked
// the selection against that list, and the two was not on it. Every test below
// is a case where the submission and the pool are legitimately different, and
// the reason the difference has to be said out loud rather than inferred.

// meldPoolTable is a Samba side already out, so nothing here is about opening.
func meldPoolTable(hand []string) module.State {
	return sambaTable(func(s *GameState) {
		s.Phase = phaseMeld
		s.Teams[0].HasMelded = true
		s.Teams[0].MeldSeq = 1
		s.Teams[0].Melds = []Meld{sevenOf("Q")}
		s.Hands["p1"] = hand
	})
}

func meldOffer(t *testing.T, raw module.State, rank string) *module.ActionOffer {
	t.Helper()
	offers, err := New().LegalActions(raw, "p1")
	if err != nil {
		t.Fatalf("LegalActions: %v", err)
	}
	o := offerByID(offers, OfferLayMeld+":"+rank)
	if o == nil {
		t.Fatalf("no lay_meld offer for %s; got %v", rank, offerIDs(offers))
	}
	if o.Source == nil {
		t.Fatalf("lay_meld:%s has no source selector", rank)
	}
	return o
}

func offerIDs(offers []module.ActionOffer) []string {
	out := make([]string, 0, len(offers))
	for _, o := range offers {
		out = append(out, o.ID)
	}
	return out
}

// The reported bug, at the size it was reported: three jacks and a two, and
// the two has to be pickable even though the jacks did not need it.
func TestWildIsOfferedToAGroupThatDoesNotNeedIt(t *testing.T) {
	hand := []string{"JS", "JD", "JH", "2C", "9H", "5S"}
	raw := meldPoolTable(hand)
	o := meldOffer(t, raw, "J")

	if !contains(o.Source.Cards, "2C") {
		t.Errorf("pool is %v — the two is pickable and has to be listed", o.Source.Cards)
	}
	// Four cards to pick from, and the engine takes all four, so a selection
	// may reach that far.
	if o.Source.MaxCards != 4 {
		t.Errorf("maxCards is %d, want 4 (three jacks and a wild)", o.Source.MaxCards)
	}
	// The submission stays the jacks: a press should not spend a wild the
	// meld does not need, and neither should a bot.
	if got := o.Source.Submit; len(got) != 3 || containsWild(got) {
		t.Errorf("submit is %v, want the three jacks and no wild", got)
	}

	// And the engine agrees about both melds the pool now permits.
	for _, try := range [][]string{{"JS", "JD", "2C"}, {"JS", "JD", "JH", "2C"}} {
		if _, code := apply(t, raw, "p1", module.Action{
			Verb: VerbLayMeld, Cards: try,
		}); code != "" {
			t.Errorf("engine refused %v with %s", try, code)
		}
	}
}

// A wild is a wild. The engine cannot tell one two from another and neither can
// a player, so naming one copy and not the others made the rest dead cards.
func TestEveryWildCopyIsOffered(t *testing.T) {
	hand := []string{"JS", "JD", "2H", "2S", "JOKER1", "9H"}
	o := meldOffer(t, meldPoolTable(hand), "J")

	for _, wild := range []string{"2H", "2S", "JOKER1"} {
		if !contains(o.Source.Cards, wild) {
			t.Errorf("pool is %v — %s is a wild in this hand and may be picked", o.Source.Cards, wild)
		}
	}
}

// Whichever copy a player picks, the engine takes it — which is the fact the
// offer above is only relaying.
func TestAnyWildCopyIsAccepted(t *testing.T) {
	hand := []string{"JS", "JD", "2H", "2S", "JOKER1", "9H"}
	raw := meldPoolTable(hand)
	for _, wild := range []string{"2H", "2S", "JOKER1"} {
		if _, code := apply(t, raw, "p1", module.Action{
			Verb: VerbLayMeld, Cards: []string{"JS", "JD", wild},
		}); code != "" {
			t.Errorf("JJ%s was refused with %s", wild, code)
		}
	}
}

// The bound is a count, and the count is Samba's: two wilds at most and twice
// as many naturals as wilds, so four jacks reach six cards and no further —
// even with a third wild sitting in the hand.
func TestMaxCardsStopsAtTheWildLimit(t *testing.T) {
	hand := []string{"JS", "JD", "JH", "JC", "2C", "2D", "JOKER1", "9H", "5S"}
	o := meldOffer(t, meldPoolTable(hand), "J")

	if len(o.Source.Cards) != 7 {
		t.Errorf("pool is %v, want the four jacks and the three wilds", o.Source.Cards)
	}
	if o.Source.MaxCards != 6 {
		t.Errorf("maxCards is %d, want 6 — four naturals carry two wilds and no more", o.Source.MaxCards)
	}
}

// And the limits themselves stay the engine's. A pool is a list of cards a
// player may reach for, not a promise that every handful of them is a meld —
// so the two ways of overspending wilds are refused by code, each pointing at
// the rule it breaks, rather than being quietly unpickable.
func TestOverspendingWildsIsRefusedByTheEngine(t *testing.T) {
	hand := []string{"JS", "JD", "JH", "JC", "2C", "2D", "JOKER1", "9H", "5S"}

	t.Run("more wilds than the variation allows", func(t *testing.T) {
		if _, code := apply(t, meldPoolTable(hand), "p1", module.Action{
			Verb:  VerbLayMeld,
			Cards: []string{"JS", "JD", "JH", "JC", "2C", "2D", "JOKER1"},
		}); code != ErrTooManyWilds {
			t.Errorf("three wilds was %q, want %s", code, ErrTooManyWilds)
		}
	})

	t.Run("too few naturals to carry them", func(t *testing.T) {
		if _, code := apply(t, meldPoolTable(hand), "p1", module.Action{
			Verb: VerbLayMeld, Cards: []string{"JS", "JD", "JH", "2C", "2D"},
		}); code != ErrNotEnoughNaturals {
			t.Errorf("three jacks carrying two wilds was %q, want %s",
				code, ErrNotEnoughNaturals)
		}
	})
}

// The bound is the engine's whole answer, not just its opinion about wilds. A
// hand that would be emptied by the biggest legal-looking meld cannot lay it,
// because the side has not the canastas to go out — and because the maximum is
// asked rather than computed, it comes back already knowing that.
func TestMaxCardsRespectsWhatTheTurnWouldLeave(t *testing.T) {
	// Seven cards and nothing else: laying six leaves one, and discarding it
	// is going out, which this side may not do on one canasta.
	hand := []string{"JS", "JD", "JH", "JC", "2C", "2D", "JOKER1"}
	o := meldOffer(t, meldPoolTable(hand), "J")

	if o.Source.MaxCards != 5 {
		t.Errorf("maxCards is %d, want 5 — a sixth card would strand the player",
			o.Source.MaxCards)
	}
}

// Samba's group canasta is not closed by its seventh card, so a hand holding
// eight of a rank may lay eight. The offer used to cut the pool at seven.
func TestGroupPoolIsNotCutAtSevenWhereTheCanastaStaysOpen(t *testing.T) {
	hand := []string{"KS", "KD", "KH", "KC", "KS", "KD", "KH", "KC", "9H"}
	o := meldOffer(t, meldPoolTable(hand), "K")

	if len(o.Source.Cards) != 8 {
		t.Errorf("pool is %v, want all eight kings", o.Source.Cards)
	}
	if o.Source.MaxCards != 8 {
		t.Errorf("maxCards is %d, want 8 — a Samba group keeps taking cards", o.Source.MaxCards)
	}
}

// Canasta's does close at seven, and the same offer has to say so — the pool
// widening is not a licence to offer a variation a meld it does not have.
func TestClassicGroupStillStopsAtSeven(t *testing.T) {
	raw := twoHanded(func(s *GameState) {
		s.Phase = phaseMeld
		s.Teams[0].HasMelded = true
		s.Teams[0].MeldSeq = 1
		s.Hands["p1"] = []string{"KS", "KD", "KH", "KC", "KS", "KD", "KH", "KC", "9H"}
	})
	o := meldOffer(t, raw, "K")

	if o.Source.MaxCards != canastaSize {
		t.Errorf("maxCards is %d, want %d — a Canasta group closes at seven",
			o.Source.MaxCards, canastaSize)
	}
	if _, code := apply(t, raw, "p1", module.Action{
		Verb:  VerbLayMeld,
		Cards: []string{"KS", "KD", "KH", "KC", "KS", "KD", "KH", "KC"},
	}); code != ErrMeldTooLarge {
		t.Errorf("eight kings in Classic was %q, want %s", code, ErrMeldTooLarge)
	}
}

// A two-natural group has to spend a wild to exist at all, and that one has
// always worked. Kept so the padding it depends on cannot be lost to a
// refactor of the pool beside it.
func TestTwoNaturalsStillSubmitWithAWild(t *testing.T) {
	o := meldOffer(t, meldPoolTable([]string{"JS", "JD", "2C", "9H", "5S"}), "J")

	if got := o.Source.Submit; len(got) != 3 || !containsWild(got) {
		t.Errorf("submit is %v, want two jacks and a wild", got)
	}
	if o.Source.MinCards != 3 {
		t.Errorf("minCards is %d, want 3", o.Source.MinCards)
	}
}

// A sequence takes no wilds, so its pool is its cards and nothing has changed
// for it.
func TestSequenceOfferHasNoWiderPool(t *testing.T) {
	raw := sambaTable(func(s *GameState) {
		s.Phase = phaseMeld
		s.Teams[0].HasMelded = true
		s.Teams[0].MeldSeq = 1
		s.Teams[0].Melds = []Meld{sevenOf("Q")}
		s.Hands["p1"] = []string{"5H", "6H", "7H", "2C", "9S"}
	})
	offers, err := New().LegalActions(raw, "p1")
	if err != nil {
		t.Fatal(err)
	}
	o := offerByID(offers, OfferLayMeld+":run:H5")
	if o == nil {
		t.Fatalf("no sequence offer; got %v", offerIDs(offers))
	}
	if containsWild(o.Source.Cards) {
		t.Errorf("sequence pool is %v — a run never takes a wild", o.Source.Cards)
	}
	if o.Source.MaxCards != 3 || o.Source.MinCards != 3 {
		t.Errorf("sequence bounds are %d..%d, want 3..3",
			o.Source.MinCards, o.Source.MaxCards)
	}
}

func contains(cards []string, card string) bool {
	for _, c := range cards {
		if c == card {
			return true
		}
	}
	return false
}
