package marias

import (
	"testing"

	"zolik/server/internal/module"
	"zolik/server/internal/tricks"
)

func licitState(t *testing.T) *GameState {
	t.Helper()
	return newStateVariation(t, variationLicit, nil)
}

func newStateVariation(t *testing.T, variation string, opts module.Options) *GameState {
	t.Helper()
	raw, err := New().NewMatch(module.MatchConfig{Variation: variation, Options: opts}, players, 42)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}
	s, err := decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func bidAct(rung string) module.Action {
	return module.Action{OfferID: OfferBid, Verb: VerbBid, Params: map[string]string{"rung": rung}}
}

var (
	holdAct = module.Action{OfferID: OfferHold, Verb: VerbHold}
	passAct = module.Action{OfferID: OfferAuctionPass, Verb: VerbPass}
)

func TestLicitDealsTenEachAndATalonNobodySees(t *testing.T) {
	s := licitState(t)
	for _, p := range s.Players {
		if len(s.Hands[p]) != 10 {
			t.Errorf("%s holds %d, want 10", p, len(s.Hands[p]))
		}
	}
	forhont := s.chooser()
	middle := s.next(forhont)
	zadak := s.next(middle)
	if len(s.Talon) != 2 || s.Phase != phaseAuction || s.Current != zadak || s.Holder != forhont || s.Waiting != middle {
		t.Fatalf("talon %d, phase %s, current %s, holder %s, waiting %s", len(s.Talon), s.Phase, s.Current, s.Holder, s.Waiting)
	}
	raw, _ := encode(s)
	for _, p := range s.Players {
		vm, _ := New().View(raw, p)
		for _, z := range vm.Zones {
			if z.ID == talonZoneID && len(z.Cards) > 0 {
				t.Errorf("%s sees the talon during the auction", p)
			}
		}
	}
}

func TestTheAuction(t *testing.T) {
	s := licitState(t)
	f, m, z := s.chooser(), s.next(s.chooser()), s.next(s.next(s.chooser()))

	s = must(t, s, z, bidAct("1"))
	s = must(t, s, f, holdAct)
	refused(t, s, z, bidAct("1"), ErrBidTooLow)
	s = must(t, s, z, bidAct("3"))
	// The forhont drops; the middle player takes his place and holds.
	s = must(t, s, f, passAct)
	if s.Holder != m || s.Current != m {
		t.Fatalf("after the forhont passed: holder %s, current %s", s.Holder, s.Current)
	}
	s = must(t, s, m, passAct)
	if s.Declarer != z || s.Rung != 3 || len(s.Hands[z]) != 12 || s.Phase != phaseAnnounce {
		t.Fatalf("zadák should win at sto: declarer %s rung %d cards %d phase %s", s.Declarer, s.Rung, len(s.Hands[z]), s.Phase)
	}
}

func TestNobodyBidsAndTheForhontPlaysSedma(t *testing.T) {
	s := licitState(t)
	f, m, z := s.chooser(), s.next(s.chooser()), s.next(s.next(s.chooser()))
	s = must(t, s, z, passAct)
	if s.Bidder != m || s.Current != m {
		t.Fatalf("after the zadák passed the middle should bid: bidder %s current %s", s.Bidder, s.Current)
	}
	s = must(t, s, m, passAct)
	if s.Declarer != f || s.Rung != 1 {
		t.Fatalf("declarer %s at rung %d, want the forhont at 1", s.Declarer, s.Rung)
	}
}

// announcing is a licitovaný deal won by p1 at the given rung with hand.
func announcing(t *testing.T, rung int, hand []string) *GameState {
	s := licitState(t)
	s.Hands["p1"] = hand
	s.Declarer, s.Rung, s.Phase, s.Current = "p1", rung, phaseAnnounce, "p1"
	s.Holder, s.Bidder, s.Waiting, s.Talon = "", "", "", nil
	return s
}

var licitHand = []string{"7H", "8H", "AH", "KC", "QC", "AC", "TC", "8C", "9S", "8S", "7S", "JD"}

func announce(offer string, params map[string]string) module.Action {
	return module.Action{OfferID: offer, Verb: VerbAnnounce, Params: params}
}

func TestTheContractIsAtOrAboveTheRungWon(t *testing.T) {
	s := announcing(t, 3, licitHand)
	refused(t, s, "p1", announce(OfferLSedma, map[string]string{"trump": "H"}), ErrBelowTheBid)
	// Sto is rung 3 in any suit, so it answers a won sto.
	must(t, s, "p1", announce(OfferLSto, map[string]string{"trump": "C"}))

	s = announcing(t, 4, licitHand)
	refused(t, s, "p1", announce(OfferLSto, map[string]string{"trump": "C"}), ErrBelowTheBid)
	must(t, s, "p1", announce(OfferLSto, map[string]string{"trump": "H"})) // sto červených, rung 5
}

func TestPlainSedmaNeedsItsSevenButHigherContractsMayBluff(t *testing.T) {
	s := announcing(t, 1, licitHand)
	refused(t, s, "p1", announce(OfferLSedma, map[string]string{"trump": "D"}), ErrNoSeven)
	must(t, s, "p1", announce(OfferLSedma, map[string]string{"trump": "H"}))
	// Sto a sedma in diamonds: no seven, no marriage — allowed, and lost.
	must(t, s, "p1", announce(OfferLStoSedma, map[string]string{"trump": "D"}))
}

func TestDveSedmyNeedsTwoSuits(t *testing.T) {
	s := announcing(t, 9, licitHand)
	refused(t, s, "p1", announce(OfferLDveSedmy, map[string]string{"trump": "H", "helper": "H"}), ErrHelperIsTrump)
	s = must(t, s, "p1", announce(OfferLDveSedmy, map[string]string{"trump": "H", "helper": "S"}))
	if s.Game != gameDveSedmy || s.Trump != "H" || s.Helper != "S" {
		t.Fatalf("contract %s trumps %s helper %s", s.Game, s.Trump, s.Helper)
	}
	// Neither announced seven may go to the talon.
	refused(t, s, "p1", module.Action{OfferID: OfferTalon, Verb: VerbDiscard, Cards: []string{"7S", "8C"}}, ErrSevenInTalon)
	s = must(t, s, "p1", module.Action{OfferID: OfferTalon, Verb: VerbDiscard, Cards: []string{"8C", "JD"}})
	if s.Phase != phaseFlek {
		t.Fatalf("after the talon the phase is %s, want the doubling round", s.Phase)
	}
	refused(t, s, s.Current, module.Action{OfferID: OfferProtiSedma, Verb: VerbProti}, ErrProtiNotNow)
}

func TestOmylFoldsPlainSedma(t *testing.T) {
	s := announcing(t, 1, licitHand)
	deal := s.Deal
	s = must(t, s, "p1", module.Action{OfferID: OfferOmyl, Verb: VerbFold})
	if len(s.Rounds) != 1 || s.Deal != deal+1 {
		t.Fatal("omyl did not settle the deal")
	}
	if d := s.Rounds[0].Deltas; d["p1"] != -12 || d["p2"] != 6 || d["p3"] != 6 {
		t.Fatalf("omyl paid %+v, want 6 to each defender", d)
	}
	refused(t, announcing(t, 3, licitHand), "p1", module.Action{OfferID: OfferOmyl, Verb: VerbFold}, ErrNoOmyl)
}

func TestTheForhontLeadsUnlessBetlOrDurch(t *testing.T) {
	s := announcing(t, 1, licitHand)
	s.Game, s.Trump = gameHra, "H"
	if s.firstLeader() != s.chooser() {
		t.Error("a trump game is led by the forhont")
	}
	s.Game = gameBetl
	if s.firstLeader() != "p1" {
		t.Error("betl is led by the declarer")
	}
}

func TestTheHelperSevenWaitsForTheTrickBeforeLast(t *testing.T) {
	s := playState(t, map[string][]string{"p1": {"7S", "9S", "7H"}, "p2": {"8S", "AD", "KD"}, "p3": {"TS", "QD", "JD"}})
	s.Variation, s.Game, s.Trump, s.Helper = variationLicit, gameDveSedmy, "H", "S"
	if got := s.legal("p1"); len(got) != 1 || got[0] != "9S" {
		t.Fatalf("legal = %v, want only 9S while three cards remain", got)
	}
}

func dveSedmyEnded(t *testing.T, prev, last []tricks.Play) *GameState {
	s := playState(t, map[string][]string{"p1": nil, "p2": nil, "p3": nil})
	s.Variation, s.Game, s.Trump, s.Helper = variationLicit, gameDveSedmy, "C", "S"
	s.PrevTrick, s.LastTrick = prev, last
	return s
}

func TestDveSedmySettles(t *testing.T) {
	made := dveSedmyEnded(t,
		[]tricks.Play{{Seat: 0, Card: "7S"}, {Seat: 1, Card: "8D"}, {Seat: 2, Card: "9D"}},
		[]tricks.Play{{Seat: 0, Card: "7C"}, {Seat: 1, Card: "8H"}, {Seat: 2, Card: "9H"}})
	if p := part(t, made, partGame); !p.ForDeclarer || p.Units != 40 {
		t.Errorf("dvě sedmy made: %+v", p)
	}
	beaten := dveSedmyEnded(t,
		[]tricks.Play{{Seat: 0, Card: "7S"}, {Seat: 1, Card: "8S"}, {Seat: 2, Card: "9D"}},
		[]tricks.Play{{Seat: 0, Card: "7C"}, {Seat: 1, Card: "8H"}, {Seat: 2, Card: "9H"}})
	if p := part(t, beaten, partGame); p.ForDeclarer {
		t.Errorf("helper seven beaten, still made: %+v", p)
	}
}

func TestNoDealPaysMoreThanTheLimit(t *testing.T) {
	s := dveSedmyEnded(t,
		[]tricks.Play{{Seat: 0, Card: "7S"}, {Seat: 1, Card: "8D"}, {Seat: 2, Card: "9D"}},
		[]tricks.Play{{Seat: 0, Card: "7H"}, {Seat: 1, Card: "8D"}, {Seat: 2, Card: "9D"}})
	s.Trump, s.Fleks = "H", map[string]int{partGame: 4} // 40 × 16 × 2 hearts = 1280
	s.settle()
	if d := s.Rounds[0].Deltas; d["p2"] != -payLimit || d["p1"] != 2*payLimit {
		t.Fatalf("paid %+v, want %d from each defender", d, payLimit)
	}
}

func TestSettlementCountsTheFirstMarriageForSto(t *testing.T) {
	s := ended(t, "C", map[string]int{"p1": 60, "p2": 30}, nil)
	s.Game = gameSto
	// A plain marriage first, then the trump one: only the first counts, so
	// 60 + 20 = 80, twenty short, against no defenders' marriages.
	s.Announced = []Marriage{{Player: "p1", Suit: "S"}, {Player: "p1", Suit: "C"}}
	s.Marriages = map[string][]string{"p1": {"S", "C"}}
	if p := part(t, s, partGame); p.ForDeclarer || p.Units != 8 {
		t.Errorf("sto with its first marriage plain: %+v", p)
	}
}

func TestLicitMatchesPlayOutLegally(t *testing.T) {
	for _, skill := range []module.Skill{module.SkillEasy, module.SkillMedium, module.SkillHard} {
		for seed := int64(1); seed <= 10; seed++ {
			playMatchVariation(t, variationLicit, seed, map[string]module.Skill{"p1": skill, "p2": skill, "p3": skill})
		}
	}
}
