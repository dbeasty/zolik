package rules

import "testing"

// TestStartMatch_DealsAccordingToTheRuleset checks the opening deal honours
// the ruleset rather than a hardcoded size, and leaves the table in the state
// the first player expects.
func TestStartMatch_DealsAccordingToTheRuleset(t *testing.T) {
	for _, cfg := range []RulesConfig{ProfileContinental, ProfileZolikClassic} {
		t.Run(cfg.Profile, func(t *testing.T) {
			st, err := StartMatch(cfg, []string{"p1", "p2", "p3"}, 4242, "")
			if err != nil {
				t.Fatalf("StartMatch: %v", err)
			}
			for _, pid := range st.TurnOrder {
				if got := len(st.Hands[pid]); got != cfg.DealSize {
					t.Fatalf("%s was dealt %d cards, want the ruleset's DealSize %d", pid, got, cfg.DealSize)
				}
				if st.RoundReqMet[pid] {
					t.Fatalf("%s should not start the deal already down", pid)
				}
			}
			if st.GameNumber != 1 {
				t.Fatalf("first deal should be GameNumber 1, got %d", st.GameNumber)
			}
			if st.Round != 1 {
				t.Fatalf("first lap should be Round 1, got %d", st.Round)
			}
			if st.Phase != PhaseDraw {
				t.Fatalf("the deal should open in the draw phase, got %q", st.Phase)
			}
			if st.CurrentTurn != "p1" || st.DealStarterID != "p1" {
				t.Fatalf("p1 should lead and mark the lap, got turn=%q starter=%q", st.CurrentTurn, st.DealStarterID)
			}
			if len(st.DiscardPile) != 1 {
				t.Fatalf("the deal should turn exactly one card onto the discard pile, got %v", st.DiscardPile)
			}
			// Every card is accounted for: hands + the turned discard + what
			// is left in the draw pile equals the full deck.
			dealt := len(st.TurnOrder)*cfg.DealSize + len(st.DiscardPile) + len(st.DrawPile)
			if want := len(BuildDeck(len(st.TurnOrder))); dealt != want {
				t.Fatalf("cards went missing: accounted for %d of %d", dealt, want)
			}
		})
	}
}

// TestStartMatch_IsDeterministicForASeed guards the property replay and
// reproduction depend on: the same seed deals the same cards.
func TestStartMatch_IsDeterministicForASeed(t *testing.T) {
	a, err := StartMatch(ProfileZolikClassic, []string{"p1", "p2"}, 99, "")
	if err != nil {
		t.Fatalf("StartMatch: %v", err)
	}
	b, err := StartMatch(ProfileZolikClassic, []string{"p1", "p2"}, 99, "")
	if err != nil {
		t.Fatalf("StartMatch: %v", err)
	}
	for _, pid := range []string{"p1", "p2"} {
		for i := range a.Hands[pid] {
			if a.Hands[pid][i] != b.Hands[pid][i] {
				t.Fatalf("same seed dealt different hands to %s: %v vs %v", pid, a.Hands[pid], b.Hands[pid])
			}
		}
	}
}

// TestStartMatch_RejectsAnEmptyTable keeps the guard that used to live in
// DealHand reachable from the new entry point.
func TestStartMatch_RejectsAnEmptyTable(t *testing.T) {
	if _, err := StartMatch(ProfileZolikClassic, nil, 1, ""); err == nil {
		t.Fatalf("expected StartMatch to reject a match with no players")
	}
}

// TestStartMatch_AndStartNextGame_ProduceTheSameShape is the regression guard
// for the duplicated deal setup: the opening deal was hand-rolled in the REST
// handler with a slightly different field set from StartNextGame's, so the two
// paths could (and did) drift. Both now funnel through dealNewGame, and this
// pins the invariants that must hold identically either way.
func TestStartMatch_AndStartNextGame_ProduceTheSameShape(t *testing.T) {
	cfg := ProfileZolikClassic
	first, err := StartMatch(cfg, []string{"p1", "p2"}, 7, "")
	if err != nil {
		t.Fatalf("StartMatch: %v", err)
	}

	// Take the opening state through to a later deal the other way round.
	later := first
	later.Melds["p1"] = [][]string{{"5C", "6C", "7C"}}
	later.MeldMeta["p1"] = []MeldInfo{{MeldID: "meld_1", Type: MeldRun, OwnerID: "p1"}}
	later.RoundReqMet["p1"] = true
	later.NextMeldSeq = 1
	later.MeldsLaidThisTurn = 2
	later.DiscardDrawnCardPendingMeld = "9H"
	later.LastLayOff = &LayOffSnapshot{PlayerID: "p1"}
	later.TurnMeldSnapshot = &TurnMeldSnapshot{PlayerID: "p1"}

	next, err := StartNextGame(later, "p2")
	if err != nil {
		t.Fatalf("StartNextGame: %v", err)
	}

	// Per-deal state is wiped identically by both paths.
	checks := []struct {
		name       string
		start, nxt interface{}
	}{
		{"phase", first.Phase, next.Phase},
		{"round", first.Round, next.Round},
		{"nextMeldSeq", first.NextMeldSeq, next.NextMeldSeq},
		{"meldsLaidThisTurn", first.MeldsLaidThisTurn, next.MeldsLaidThisTurn},
		{"pendingMeldCard", first.DiscardDrawnCardPendingMeld, next.DiscardDrawnCardPendingMeld},
		{"reshuffleCount", first.ReshuffleCount, next.ReshuffleCount},
		{"handSize", len(first.Hands["p1"]), len(next.Hands["p1"])},
		{"discardPileSize", len(first.DiscardPile), len(next.DiscardPile)},
	}
	for _, c := range checks {
		if c.start != c.nxt {
			t.Fatalf("%s differs between the opening deal and a later one: %v vs %v", c.name, c.start, c.nxt)
		}
	}
	for _, pid := range next.TurnOrder {
		if len(next.Melds[pid]) != 0 || len(next.MeldMeta[pid]) != 0 {
			t.Fatalf("%s kept melds across the deal boundary", pid)
		}
		if next.RoundReqMet[pid] {
			t.Fatalf("%s stayed down across the deal boundary", pid)
		}
	}
	// Stale same-turn undo snapshots must not survive into a fresh deal —
	// they point at melds that no longer exist.
	if next.LastLayOff != nil || next.LastMeldLaid != nil || next.TurnMeldSnapshot != nil {
		t.Fatalf("undo snapshots leaked into the next deal: layOff=%v meldLaid=%v turn=%v",
			next.LastLayOff, next.LastMeldLaid, next.TurnMeldSnapshot)
	}
	if next.DealStarterID != "p2" || next.CurrentTurn != "p2" {
		t.Fatalf("the next deal should be led by p2, got turn=%q starter=%q", next.CurrentTurn, next.DealStarterID)
	}
}
