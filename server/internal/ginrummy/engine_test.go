package ginrummy

import (
	"encoding/json"
	"testing"

	"zolik/server/internal/module"
)

func newMatch(t *testing.T, seed int64, cfg module.MatchConfig) module.State {
	t.Helper()
	st, err := New().NewMatch(cfg, []module.PlayerRef{{ID: "p1", Name: "p1"}, {ID: "p2", Name: "p2"}}, seed)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}
	return st
}

func stateOf(t *testing.T, raw module.State) *GameState {
	t.Helper()
	s, err := decode(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return s
}

// withState builds a hand-crafted position: a ready-to-discard turn for p1
// with a controllable hand, stock and knock limit, so knock/dead-hand/lay-off
// behavior can be pinned exactly rather than hoped for out of a real deal.
func withState(t *testing.T, mut func(*GameState)) module.State {
	t.Helper()
	s := &GameState{
		Status:      "active",
		Players:     []string{"p1", "p2"},
		Dealer:      "p2",
		Current:     "p1",
		Phase:       phaseDiscard,
		Hands:       map[string][]string{"p1": {}, "p2": {}},
		Stock:       []string{"2C", "3C", "4C"},
		DiscardPile: []string{"9S"},
		Scores:      map[string]int{"p1": 0, "p2": 0},
		HandsWon:    map[string]int{},
		TargetScore: 100,
		KnockLimit:  10,
		LineBonuses: true,
	}
	mut(s)
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

func apply(t *testing.T, raw module.State, playerID string, a module.Action) (module.State, error) {
	t.Helper()
	next, _, err := New().Apply(raw, playerID, a)
	return next, err
}

// --- the upcard dance --------------------------------------------------

func TestNewMatch_DealsTenEachAndOneUpcard(t *testing.T) {
	s := stateOf(t, newMatch(t, 7, module.MatchConfig{}))
	if len(s.Hands["p1"]) != 10 || len(s.Hands["p2"]) != 10 {
		t.Fatalf("expected 10 cards each, got %d and %d", len(s.Hands["p1"]), len(s.Hands["p2"]))
	}
	if len(s.DiscardPile) != 1 {
		t.Fatalf("expected exactly one upcard, got %d", len(s.DiscardPile))
	}
	if len(s.Stock) != 52-10-10-1 {
		t.Fatalf("stock = %d, want %d", len(s.Stock), 52-10-10-1)
	}
	nonDealer := other(s.Players, s.Dealer)
	if s.Current != nonDealer || s.Phase != phaseUpcardNonDealer {
		t.Errorf("expected non-dealer %s to decide the upcard first, got current=%s phase=%s", nonDealer, s.Current, s.Phase)
	}
}

func TestUpcardDance_NonDealerTakesIt(t *testing.T) {
	raw := newMatch(t, 3, module.MatchConfig{})
	s := stateOf(t, raw)
	nonDealer := other(s.Players, s.Dealer)
	up := s.DiscardPile[0]

	next, err := apply(t, raw, nonDealer, module.Action{OfferID: OfferDrawDiscard, Verb: VerbDraw})
	if err != nil {
		t.Fatalf("take upcard: %v", err)
	}
	s2 := stateOf(t, next)
	if len(s2.DiscardPile) != 0 {
		t.Errorf("discard pile should be empty after the upcard is taken, got %v", s2.DiscardPile)
	}
	if !hasCard(s2.Hands[nonDealer], up) {
		t.Errorf("%s should hold the upcard %s", nonDealer, up)
	}
	if len(s2.Hands[nonDealer]) != 11 {
		t.Errorf("expected 11 cards after taking the upcard, got %d", len(s2.Hands[nonDealer]))
	}
	if s2.Phase != phaseDiscard || s2.Current != nonDealer {
		t.Errorf("expected %s to discard next, got current=%s phase=%s", nonDealer, s2.Current, s2.Phase)
	}
}

func TestUpcardDance_BothDeclineForcesNonDealerToStock(t *testing.T) {
	raw := newMatch(t, 11, module.MatchConfig{})
	s := stateOf(t, raw)
	nonDealer := other(s.Players, s.Dealer)

	raw, err := apply(t, raw, nonDealer, module.Action{OfferID: OfferPassUpcard, Verb: VerbPass})
	if err != nil {
		t.Fatalf("non-dealer pass: %v", err)
	}
	s2 := stateOf(t, raw)
	if s2.Phase != phaseUpcardDealer || s2.Current != s.Dealer {
		t.Fatalf("expected the dealer's turn to decide, got current=%s phase=%s", s2.Current, s2.Phase)
	}

	raw, err = apply(t, raw, s.Dealer, module.Action{OfferID: OfferPassUpcard, Verb: VerbPass})
	if err != nil {
		t.Fatalf("dealer pass: %v", err)
	}
	s3 := stateOf(t, raw)
	if s3.Phase != phaseDraw || s3.Current != nonDealer || !s3.ForcedStockDraw {
		t.Fatalf("expected a forced stock draw for %s, got current=%s phase=%s forced=%v",
			nonDealer, s3.Current, s3.Phase, s3.ForcedStockDraw)
	}

	// Taking the discard here would be taking the very card both players just
	// refused — it must be refused.
	if _, err := apply(t, raw, nonDealer, module.Action{OfferID: OfferDrawDiscard, Verb: VerbDraw}); err == nil {
		t.Error("expected the declined upcard to stay unavailable")
	} else if module.CodeOf(err) != ErrUpcardDeclined {
		t.Errorf("expected %s, got %v", ErrUpcardDeclined, err)
	}

	raw, err = apply(t, raw, nonDealer, module.Action{OfferID: OfferDrawStock, Verb: VerbDraw})
	if err != nil {
		t.Fatalf("forced stock draw: %v", err)
	}
	s4 := stateOf(t, raw)
	if len(s4.Hands[nonDealer]) != 11 || s4.Phase != phaseDiscard {
		t.Fatalf("expected %s to hold 11 and be discarding, got %d cards phase=%s", nonDealer, len(s4.Hands[nonDealer]), s4.Phase)
	}
	if s4.ForcedStockDraw {
		t.Error("the forced flag should clear once the stock draw happens")
	}
}

// --- knocking ------------------------------------------------------------

func TestKnock_BelowTheLimitIsAccepted(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Hands["p1"] = []string{
			"5H", "5D", "5C", // set
			"7S", "8S", "9S", // run
			"AH", "2D", // deadwood 1+2 = 3, kept
			"KH", // discarded
		}
		s.KnockLimit = 10
	})
	next, err := apply(t, raw, "p1", module.Action{Verb: VerbKnock, Cards: []string{"KH"}})
	if err != nil {
		t.Fatalf("knock: %v", err)
	}
	s := stateOf(t, next)
	if s.Knocker != "p1" {
		t.Fatalf("expected p1 to be recorded as the knocker")
	}
	if s.KnockerDeadwood != 3 {
		t.Fatalf("expected deadwood 3, got %d", s.KnockerDeadwood)
	}
	if s.Phase != phaseLayoff || s.Current != "p2" {
		t.Fatalf("expected the defender's lay-off phase, got current=%s phase=%s", s.Current, s.Phase)
	}
	if hasCard(s.DiscardPile, "KH") {
		t.Error("a knock's discard is laid face down, not onto the visible pile")
	}
}

func TestKnock_AboveTheLimitIsRefused(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Hands["p1"] = []string{"AH", "3D", "5C", "7S", "9H", "JD", "KC", "2S", "4H", "6D", "8C"}
		s.KnockLimit = 10
	})
	if _, err := apply(t, raw, "p1", module.Action{Verb: VerbKnock, Cards: []string{"8C"}}); err == nil {
		t.Fatal("expected a knock this far above the limit to be refused")
	} else if module.CodeOf(err) != ErrDeadwoodTooHigh {
		t.Errorf("expected %s, got %v", ErrDeadwoodTooHigh, err)
	}
}

func TestKnock_ZeroDeadwoodIsGinAndSkipsLayoff(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Hands["p1"] = []string{
			"2H", "2D", "2C", "2S",
			"6H", "7H", "8H", "9H",
			"TH", "JH", "QH",
		}
		s.Hands["p2"] = []string{"AH", "3D", "5C", "7S", "9D", "JC", "2S", "4H", "6D", "8S"}
		s.KnockLimit = 10
	})
	next, err := apply(t, raw, "p1", module.Action{Verb: VerbKnock, Cards: []string{"QH"}})
	if err != nil {
		t.Fatalf("gin: %v", err)
	}
	s := stateOf(t, next)
	if len(s.Rounds) != 1 || s.Rounds[0].Kind != "gin" {
		t.Fatalf("expected a gin to end the hand immediately, got %+v", s.Rounds)
	}
	if s.Phase == phaseLayoff {
		t.Error("gin must not pass through a lay-off phase")
	}
}

// --- the dead hand -------------------------------------------------------

func TestDeadHand_StockDownToOneAfterDiscardWithNoKnock(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Hands["p1"] = []string{"AH", "3D", "5C", "7S", "9H", "JD", "KC", "2S", "4H", "6D", "8C"}
		s.Stock = []string{"9C"} // one card left after this discard
		s.KnockLimit = 0         // nothing in hand can knock
		s.Dealer = "p2"
	})
	next, err := apply(t, raw, "p1", module.Action{Verb: VerbDiscard, Cards: []string{"8C"}})
	if err != nil {
		t.Fatalf("discard: %v", err)
	}
	s := stateOf(t, next)
	if len(s.Rounds) != 1 || s.Rounds[0].Kind != "dead" {
		t.Fatalf("expected a dead hand, got %+v", s.Rounds)
	}
	if s.Scores["p1"] != 0 || s.Scores["p2"] != 0 {
		t.Errorf("a dead hand must not score: %+v", s.Scores)
	}
	if s.Dealer != "p2" {
		t.Errorf("the same dealer redeals after a dead hand, got %s", s.Dealer)
	}
}

func TestDiscard_DealerAlternatesOnAScoredHand(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Hands["p1"] = []string{
			"2H", "2D", "2C", "2S",
			"6H", "7H", "8H", "9H",
			"TH", "JH", "QH",
		}
		s.Hands["p2"] = []string{"AH", "3D", "5C", "7S", "9D", "JC", "2S", "4H", "6D", "8S"}
		s.Dealer = "p2"
		s.LineBonuses = false
	})
	next, err := apply(t, raw, "p1", module.Action{Verb: VerbKnock, Cards: []string{"QH"}})
	if err != nil {
		t.Fatalf("gin: %v", err)
	}
	s := stateOf(t, next)
	if s.Dealer != "p1" {
		t.Errorf("expected the dealer to alternate to p1, got %s", s.Dealer)
	}
}

// --- lay-off ---------------------------------------------------------------

func TestLayOff_ExtendsASetAndReducesDeadwood(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Phase = phaseLayoff
		s.Current = "p2"
		s.Knocker = "p1"
		s.KnockerDeadwood = 3
		s.KnockerMelds = []Meld{{ID: "m0", Kind: "set", Cards: []string{"5C", "5D", "5H"}}}
		s.Hands["p2"] = []string{"5S", "9C"}
	})
	next, err := apply(t, raw, "p2", module.Action{Verb: VerbLayOff, Target: "m0", Cards: []string{"5S"}})
	if err != nil {
		t.Fatalf("lay off: %v", err)
	}
	s := stateOf(t, next)
	if hasCard(s.Hands["p2"], "5S") {
		t.Error("the laid-off card should leave the defender's hand")
	}
	if !hasCard(s.KnockerMelds[0].Cards, "5S") {
		t.Error("the laid-off card should join the meld")
	}
	if s.Phase != phaseLayoff {
		t.Error("the defender may lay off more than one card in the same phase")
	}
}

func TestLayOff_ACardThatDoesNotExtendIsRefused(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Phase = phaseLayoff
		s.Current = "p2"
		s.Knocker = "p1"
		s.KnockerMelds = []Meld{{ID: "m0", Kind: "run", Cards: []string{"5H", "6H", "7H"}}}
		s.Hands["p2"] = []string{"9C"}
	})
	if _, err := apply(t, raw, "p2", module.Action{Verb: VerbLayOff, Target: "m0", Cards: []string{"9C"}}); err == nil {
		t.Fatal("expected a non-extending card to be refused")
	} else if module.CodeOf(err) != ErrCardDoesNotExtendMeld {
		t.Errorf("expected %s, got %v", ErrCardDoesNotExtendMeld, err)
	}
}

func TestLayOff_OnlyTheDefenderMayAct(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Phase = phaseLayoff
		s.Current = "p2"
		s.Knocker = "p1"
		s.KnockerMelds = []Meld{{ID: "m0", Kind: "set", Cards: []string{"5C", "5D", "5H"}}}
	})
	if _, err := apply(t, raw, "p1", module.Action{Verb: VerbFinishLayoff}); err == nil {
		t.Fatal("the knocker must not be able to act during the lay-off phase")
	}
}

func TestFinishLayoff_EndsTheHand(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Phase = phaseLayoff
		s.Current = "p2"
		s.Knocker = "p1"
		s.KnockerDeadwood = 3
		s.KnockerMelds = []Meld{{ID: "m0", Kind: "set", Cards: []string{"5C", "5D", "5H"}}}
		s.Hands["p2"] = []string{"AH", "3D", "5S", "7S", "9H", "JD", "KC", "2S", "4H", "6D"}
	})
	next, err := apply(t, raw, "p2", module.Action{Verb: VerbFinishLayoff})
	if err != nil {
		t.Fatalf("finish_layoff: %v", err)
	}
	s := stateOf(t, next)
	if len(s.Rounds) != 1 {
		t.Fatalf("expected the hand to be scored, got %+v", s.Rounds)
	}
}

// --- apply does not mutate the caller's bytes -------------------------------

func TestApply_RefusalLeavesTheCallersStateUntouched(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Hands["p1"] = []string{"AH"}
	})
	before := append(module.State(nil), raw...)
	if _, err := apply(t, raw, "p1", module.Action{Verb: VerbDiscard, Cards: []string{"9Z"}}); err == nil {
		t.Fatal("expected a refusal for a card not in hand")
	}
	if string(raw) != string(before) {
		t.Error("a refused action must not mutate the caller's bytes")
	}
}
