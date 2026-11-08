package pinochle

import (
	"testing"

	"zolik/server/internal/module"
	"zolik/server/internal/tricks"
)

func TestTheDeal(t *testing.T) {
	for _, v := range allVariations {
		s := newState(t, v, nil)
		tb := s.table()
		total := len(s.Widow) + len(s.Stock)
		if s.TurnUp != "" {
			total++
		}
		for _, p := range s.Players {
			if len(s.Hands[p]) != tb.handSize {
				t.Errorf("%s: %s holds %d, want %d", v, p, len(s.Hands[p]), tb.handSize)
			}
			total += len(s.Hands[p])
		}
		if total != len(tb.buildDeck()) {
			t.Errorf("%s: %d cards on the table, want %d", v, total, len(tb.buildDeck()))
		}
		if len(s.Widow) != tb.widow {
			t.Errorf("%s: widow of %d, want %d", v, len(s.Widow), tb.widow)
		}
		switch v {
		case VarTwoHanded:
			if s.Phase != phasePlay || s.Trump == "" || len(s.Stock) != 23 {
				t.Errorf("two-handed opens in %s, trump %q, stock %d", s.Phase, s.Trump, len(s.Stock))
			}
			if s.Current != s.next(s.dealer()) {
				t.Errorf("two-handed: %s leads, want the non-dealer", s.Current)
			}
		default:
			if s.Phase != phaseAuction || s.Current != s.next(s.dealer()) {
				t.Errorf("%s opens in %s with %s, want the auction from the dealer's left", v, s.Phase, s.Current)
			}
		}
	}
}

func TestWrongSeatCountIsRefused(t *testing.T) {
	if _, err := New().NewMatch(module.MatchConfig{Variation: VarCutthroat}, refs(4), 1); module.CodeOf(err) != ErrWrongPlayers {
		t.Fatalf("four at a cutthroat table: %v", err)
	}
}

// auctionAt is an auction with p4 dealing, so p1 calls first.
func auctionAt(t *testing.T, opts module.Options) *GameState {
	s := newState(t, VarPartnership, opts)
	s.FirstDealer, s.Deal = 3, 0
	s.Current = "p1"
	return s
}

func TestTheAuction(t *testing.T) {
	s := auctionAt(t, nil)
	refused(t, s, "p1", bid("240"), ErrBidTooLow)
	refused(t, s, "p1", bid("255"), ErrBidStep)
	refused(t, s, "p1", bid("lots"), ErrNotANumber)
	refused(t, s, "p2", bid("250"), ErrNotYourTurn)
	s = must(t, s, "p1", bid("250"))
	refused(t, s, "p2", bid("250"), ErrBidTooLow)
	s = must(t, s, "p2", bid("300"))
	s = must(t, s, "p3", passAction)
	s = must(t, s, "p4", passAction)
	if s.Current != "p1" {
		t.Fatalf("after two passes %s is on, want p1", s.Current)
	}
	s = must(t, s, "p1", passAction)
	if s.Phase != phaseTrump || s.Bidder != "p2" || s.Bid != 300 || s.Current != "p2" {
		t.Fatalf("auction ended in %s to %s at %d (%s on)", s.Phase, s.Bidder, s.Bid, s.Current)
	}
}

func TestAPassIsFinal(t *testing.T) {
	s := auctionAt(t, nil)
	s = must(t, s, "p1", passAction)
	s = must(t, s, "p2", bid("250"))
	s = must(t, s, "p3", bid("260"))
	s = must(t, s, "p4", passAction)
	if s.Current != "p2" {
		t.Fatalf("p1 passed and was asked again: %s on", s.Current)
	}
	s.Current = "p1"
	refused(t, s, "p1", bid("300"), ErrAlreadyPassed)
}

func TestEveryonePassingSticksTheDealer(t *testing.T) {
	s := auctionAt(t, nil)
	for _, p := range []string{"p1", "p2", "p3", "p4"} {
		s = must(t, s, p, passAction)
	}
	if !s.Stuck || s.Bidder != "p4" || s.Bid != 250 || s.Phase != phaseTrump {
		t.Fatalf("stuck=%v bidder=%s bid=%d phase=%s", s.Stuck, s.Bidder, s.Bid, s.Phase)
	}
}

func TestEveryonePassingCanRedeal(t *testing.T) {
	s := auctionAt(t, module.Options{OptAllPass: AllPassRedeal})
	for _, p := range []string{"p1", "p2", "p3", "p4"} {
		s = must(t, s, p, passAction)
	}
	if s.Deal != 1 || len(s.Rounds) != 1 || !s.Rounds[0].Redealt || s.Phase != phaseAuction {
		t.Fatalf("deal %d rounds %v phase %s", s.Deal, s.Rounds, s.Phase)
	}
	for _, side := range s.sides() {
		if s.Scores[side] != 0 {
			t.Fatalf("a redeal scored: %v", s.Scores)
		}
	}
}

func TestNamingTrumpsLaysTheMeld(t *testing.T) {
	s := auctionAt(t, nil)
	s = must(t, s, "p1", bid("250"))
	for _, p := range []string{"p2", "p3", "p4"} {
		s = must(t, s, p, passAction)
	}
	refused(t, s, "p1", trumpAction("X"), ErrNoSuchSuit)
	s = must(t, s, "p1", trumpAction("S"))
	if s.Phase != phasePlay || s.Current != "p1" || s.Trump != "S" {
		t.Fatalf("after trumps: phase %s, %s on, trump %s", s.Phase, s.Current, s.Trump)
	}
	for _, p := range s.Players {
		if got, want := s.Melds[p].Total, BestMeld(s.Hands[p], 'S', s.table()).Total; got != want {
			t.Errorf("%s meld %d, want %d", p, got, want)
		}
	}
}

func TestPartnersPassCards(t *testing.T) {
	s := auctionAt(t, module.Options{OptPass: 3})
	s = must(t, s, "p1", bid("250"))
	for _, p := range []string{"p2", "p3", "p4"} {
		s = must(t, s, p, passAction)
	}
	s = must(t, s, "p1", trumpAction("H"))
	if s.Phase != phasePassIn || s.Current != "p3" {
		t.Fatalf("after trumps: %s with %s on, want p3 to pass", s.Phase, s.Current)
	}
	give := s.Hands["p3"][:2]
	refused(t, s, "p3", module.Action{OfferID: OfferGive, Verb: VerbGive, Cards: give}, ErrGiveCount)
	give = append([]string(nil), s.Hands["p3"][:3]...)
	s = must(t, s, "p3", module.Action{OfferID: OfferGive, Verb: VerbGive, Cards: give})
	if len(s.Hands["p1"]) != 15 || len(s.Hands["p3"]) != 9 || s.Current != "p1" || s.Phase != phasePassBack {
		t.Fatalf("after the pass in: %d / %d, %s on in %s", len(s.Hands["p1"]), len(s.Hands["p3"]), s.Current, s.Phase)
	}
	back := append([]string(nil), s.Hands["p1"][:3]...)
	s = must(t, s, "p1", module.Action{OfferID: OfferGive, Verb: VerbGive, Cards: back})
	if len(s.Hands["p1"]) != 12 || len(s.Hands["p3"]) != 12 || s.Phase != phasePlay {
		t.Fatalf("after the pass back: %d / %d in %s", len(s.Hands["p1"]), len(s.Hands["p3"]), s.Phase)
	}
}

func TestCutthroatWidowAndBury(t *testing.T) {
	s := newState(t, VarCutthroat, nil)
	s.FirstDealer, s.Deal = 2, 0 // p3 deals, p1 bids first
	s.Current = "p1"
	widow := append([]string(nil), s.Widow...)
	s = must(t, s, "p1", bid("250"))
	s = must(t, s, "p2", passAction)
	s = must(t, s, "p3", passAction)
	if len(s.Hands["p1"]) != 18 || !hasCards(s.Hands["p1"], widow) {
		t.Fatalf("the bidder did not take the widow: %d cards", len(s.Hands["p1"]))
	}
	vm, _ := New().View(mustEncode(t, s), "p2")
	if z := zoneByID(vm, widowZoneID); z == nil || len(z.Cards) != 3 {
		t.Fatalf("the widow is not shown to the table while trumps are named: %+v", z)
	}
	s = must(t, s, "p1", trumpAction("H"))
	bury := append([]string(nil), s.Hands["p1"][len(s.Hands["p1"])-3:]...)
	refused(t, s, "p1", module.Action{OfferID: OfferBury, Verb: VerbBury, Cards: bury[:2]}, ErrBuryCount)
	s = must(t, s, "p1", module.Action{OfferID: OfferBury, Verb: VerbBury, Cards: bury})
	if len(s.Hands["p1"]) != 15 || len(s.Buried) != 3 || s.Phase != phasePlay {
		t.Fatalf("after the bury: %d cards, %d buried, %s", len(s.Hands["p1"]), len(s.Buried), s.Phase)
	}
	vm, _ = New().View(mustEncode(t, s), "p2")
	if z := zoneByID(vm, buriedZoneID); z == nil || len(z.Cards) != 0 || z.Count != 3 {
		t.Fatalf("another player sees the bury: %+v", z)
	}
	vm, _ = New().View(mustEncode(t, s), "p1")
	if z := zoneByID(vm, buriedZoneID); z == nil || len(z.Cards) != 3 {
		t.Fatalf("the bidder cannot see their own bury: %+v", z)
	}
}

func TestFollowingBeatingAndTrumping(t *testing.T) {
	hands := map[string][]string{
		"p1": {"KH", "AD", "9C"},
		"p2": {"AH", "9H", "9S"},
		"p3": {"TS", "AC", "QD"},
		"p4": {"AS", "9S", "JC"},
	}
	s := playingAt(t, "S", hands)
	s = must(t, s, "p1", card("KH"))
	refused(t, s, "p2", card("9S"), ErrMustFollow)
	refused(t, s, "p2", card("9H"), ErrMustBeat)
	s = must(t, s, "p2", card("AH"))
	// p3 has no heart: a trump is owed.
	refused(t, s, "p3", card("AC"), ErrMustTrump)
	s = must(t, s, "p3", card("TS"))
	// p4 has no heart either, and must overtrump with the ace.
	refused(t, s, "p4", card("9S"), ErrMustBeat)
	s = must(t, s, "p4", card("AS"))
	if s.LastTrick == nil || s.Leader != "p4" || s.Tricks["1"] != 1 {
		t.Fatalf("trick went to %s (tricks %v)", s.Leader, s.Tricks)
	}
	if s.Taken["1"] != 40 {
		t.Errorf("the trick held %d, want 40 (K, A, T, A)", s.Taken["1"])
	}
}

func TestTheFirstOfTwoIdenticalCardsWins(t *testing.T) {
	hands := map[string][]string{
		"p1": {"AD", "9C"},
		"p2": {"AD", "9H"},
		"p3": {"9D", "AC"},
		"p4": {"JD", "QH"},
	}
	s := playingAt(t, "S", hands)
	s = must(t, s, "p1", card("AD"))
	s = must(t, s, "p2", card("AD"))
	s = must(t, s, "p3", card("9D"))
	s = must(t, s, "p4", card("JD"))
	if s.Leader != "p1" {
		t.Fatalf("the second ace of diamonds took the trick: %s leads", s.Leader)
	}
	w, _ := tricks.Trick{Plays: s.LastTrick}.Winning('S', tableOf(VarPartnership).order())
	if w.Seat != 0 {
		t.Fatalf("Winning names seat %d", w.Seat)
	}
}

func TestScoringAHand(t *testing.T) {
	s := newState(t, VarPartnership, nil)
	s.Bidder, s.Bid = "p1", 250
	s.Melds = map[string]Meld{"p1": {Total: 100}, "p3": {Total: 50}, "p2": {Total: 60}, "p4": {Total: 0}}
	made := func(taken0, tricks0, taken1, tricks1 int) (map[string]int, bool) {
		_, d, ok := s.handDeltas(map[string]int{"0": taken0, "1": taken1}, map[string]int{"0": tricks0, "1": tricks1},
			map[string]int{"0": tricks0, "1": tricks1}, false)
		return d, ok
	}
	if d, ok := made(110, 5, 140, 7); !ok || d["0"] != 260 || d["1"] != 200 {
		t.Errorf("made: %v %v, want 260 and 200", ok, d)
	}
	if d, ok := made(90, 4, 160, 8); ok || d["0"] != -250 || d["1"] != 220 {
		t.Errorf("set: %v %v, want -250 and 220", ok, d)
	}
	// A side that takes no trick does not score its meld.
	if d, _ := made(250, 12, 0, 0); d["1"] != 0 {
		t.Errorf("a trickless side scored its meld: %v", d)
	}
}

func TestMeldNeedsACounter(t *testing.T) {
	s := newState(t, VarPartnership, module.Options{OptMeldNeeds: MeldNeedsCounter})
	s.Bidder, s.Bid = "p1", 250
	s.Melds = map[string]Meld{"p2": {Total: 60}}
	_, d, _ := s.handDeltas(map[string]int{"0": 250, "1": 0}, map[string]int{"0": 11, "1": 1},
		map[string]int{"0": 24, "1": 0}, false)
	if d["1"] != 0 {
		t.Errorf("a trick with no counter in it saved the meld: %v", d)
	}
	s.MeldNeeds = MeldNeedsTrick
	_, d, _ = s.handDeltas(map[string]int{"0": 250, "1": 0}, map[string]int{"0": 11, "1": 1},
		map[string]int{"0": 24, "1": 0}, false)
	if d["1"] != 60 {
		t.Errorf("under the trick rule the meld should count: %v", d)
	}
}

func TestThrowingIn(t *testing.T) {
	hands := map[string][]string{
		"p1": {"9C", "9D"}, "p2": {"KC", "QC"}, "p3": {"9H", "9S"}, "p4": {"JC", "JH"},
	}
	s := playingAt(t, "S", hands) // p2 melds a club marriage
	refused(t, s, "p2", module.Action{OfferID: OfferThrowIn, Verb: VerbThrowIn}, ErrNotYourTurn)
	s = must(t, s, "p1", module.Action{OfferID: OfferThrowIn, Verb: VerbThrowIn})
	if s.Scores["0"] != -250 || s.Scores["1"] != 20 {
		t.Fatalf("after a throw-in: %v", s.Scores)
	}
	if r := s.Rounds[len(s.Rounds)-1]; !r.ThrownIn {
		t.Fatalf("the record does not say thrown in: %+v", r)
	}
}

func TestNoThrowInOnceTheLeadIsMade(t *testing.T) {
	hands := map[string][]string{
		"p1": {"9C", "9D"}, "p2": {"KC", "QC"}, "p3": {"9H", "9D"}, "p4": {"JC", "JH"},
	}
	s := playingAt(t, "S", hands)
	s = must(t, s, "p1", card("9C"))
	s = must(t, s, "p2", card("KC"))
	s = must(t, s, "p3", card("9H"))
	s = must(t, s, "p4", card("JC"))
	s.Current = "p1"
	refused(t, s, "p1", module.Action{OfferID: OfferThrowIn, Verb: VerbThrowIn}, ErrTooLate)
}

func TestTheMatchEndsAtTheTarget(t *testing.T) {
	s := newState(t, VarPartnership, nil)
	s.Scores = map[string]int{"0": 1400, "1": 1450}
	s.Bidder = "p1"
	s.Bid = 150
	s.Melds = map[string]Meld{}
	s.Taken = map[string]int{"0": 150, "1": 100}
	s.Tricks = map[string]int{"0": 6, "1": 6}
	s.Counters = map[string]int{"0": 15, "1": 10}
	s.endHand(false)
	// Both sides are over 1500; the bidding side goes out first.
	if s.Status != "completed" || len(s.Winners) != 2 || s.side(s.Winners[0]) != "0" {
		t.Fatalf("status %s, winners %v, scores %v", s.Status, s.Winners, s.Scores)
	}
	// Side 1 has more points, but the scoreboard must agree with the rule.
	standings, _ := New().Standings(mustEncode(t, s))
	for _, st := range standings {
		if wantWon := s.side(st.PlayerID) == "0"; st.Won != wantWon || (st.Rank == 1) != wantWon {
			t.Errorf("%s: rank %d won %v (score %d)", st.PlayerID, st.Rank, st.Won, st.Score)
		}
	}
}

func TestTwoHandedMeldAndDraw(t *testing.T) {
	s := newState(t, VarTwoHanded, nil)
	s.FirstDealer, s.Deal = 1, 0 // p2 deals, p1 leads
	s.Trump, s.TurnUp = "H", "AH"
	s.Stock = []string{"JC", "9D", "KD"}
	s.Hands = map[string][]string{
		"p1": {"KS", "QS", "JD", "9H", "TC"},
		"p2": {"9C", "QD", "TD", "AC", "JS"},
	}
	s.Leader, s.Current, s.Phase = "p1", "p1", phasePlay
	s.Used = map[string]Used{"p1": {}, "p2": {}}
	s.MeldScore = map[string]int{}
	// Any card may be played while the stock lasts.
	s = must(t, s, "p1", card("TC"))
	s = must(t, s, "p2", card("9C")) // p1's ten takes it
	if s.Phase != phaseMeld || s.Current != "p1" {
		t.Fatalf("after the trick: %s, %s on", s.Phase, s.Current)
	}
	raw := mustEncode(t, s)
	offers, _ := New().LegalActions(raw, "p1")
	ids := map[string]bool{}
	for _, o := range offers {
		if o.Enabled {
			ids[o.ID] = true
		}
	}
	for _, want := range []string{"meld.pinochle", "meld.marriage.S", OfferExchange, OfferDix, OfferDraw} {
		if !ids[want] {
			t.Errorf("missing offer %s (have %v)", want, ids)
		}
	}
	s = must(t, s, "p1", module.Action{OfferID: OfferExchange, Verb: VerbMeld})
	if s.TurnUp != "9H" || !hasCards(s.Hands["p1"], []string{"AH"}) || s.MeldScore["0"] != 10 {
		t.Fatalf("the exchange: turn-up %s, meld %v", s.TurnUp, s.MeldScore)
	}
	// Melding draws: winner first.
	if len(s.Hands["p1"]) != 5 || len(s.Hands["p2"]) != 5 || len(s.Stock) != 1 {
		t.Fatalf("after drawing: %d / %d, stock %d", len(s.Hands["p1"]), len(s.Hands["p2"]), len(s.Stock))
	}
	if s.Phase != phasePlay || s.Current != "p1" {
		t.Fatalf("after the meld: %s, %s on", s.Phase, s.Current)
	}
}

func TestTwoHandedStockRunsOut(t *testing.T) {
	s := newState(t, VarTwoHanded, nil)
	s.FirstDealer, s.Deal = 1, 0
	s.Trump, s.TurnUp = "H", "KH"
	s.Stock = []string{"JC"}
	s.Hands = map[string][]string{"p1": {"AC", "9S"}, "p2": {"TC", "9C"}}
	s.Leader, s.Current, s.Phase = "p1", "p1", phasePlay
	s.Used = map[string]Used{"p1": {}, "p2": {}}
	s.MeldScore = map[string]int{}
	s = must(t, s, "p1", card("AC"))
	s = must(t, s, "p2", card("9C"))
	s = must(t, s, "p1", module.Action{OfferID: OfferDraw, Verb: VerbDraw})
	if !s.Late || len(s.Stock) != 0 || s.TurnUp != "" {
		t.Fatalf("late %v, stock %v, turn-up %q", s.Late, s.Stock, s.TurnUp)
	}
	if !hasCards(s.Hands["p1"], []string{"JC"}) || !hasCards(s.Hands["p2"], []string{"KH"}) {
		t.Fatalf("the winner should draw the stock, the loser the trump: %v", s.Hands)
	}
	// Now the follow rules bind: p1 leads the jack of clubs, and p2 must beat.
	s = must(t, s, "p1", card("JC"))
	refused(t, s, "p2", card("KH"), ErrMustFollow)
}

func TestFinishedNamesAPartnership(t *testing.T) {
	m := New()
	raw, _ := m.NewMatch(cfgOf(VarPartnership, nil), refs(4), 7)
	final, res, err := module.PlayWithOffers(m, raw, refs(4), module.DriverOptions{MaxActions: 30000, Prefer: []string{VerbPass, VerbTrump, VerbPlay}})
	if err != nil || !res.Finished {
		t.Fatalf("%v finished=%v", err, res.Finished)
	}
	_, winners, _ := m.Finished(final)
	if len(winners) != 2 {
		t.Fatalf("winners %v", winners)
	}
	s, _ := decode(final)
	if s.side(winners[0]) != s.side(winners[1]) {
		t.Fatalf("winners on two sides: %v", winners)
	}
}

func mustEncode(t *testing.T, s *GameState) module.State {
	t.Helper()
	raw, err := encode(s)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func zoneByID(vm module.ViewModel, id string) *module.Zone {
	for i := range vm.Zones {
		if vm.Zones[i].ID == id {
			return &vm.Zones[i]
		}
	}
	return nil
}
