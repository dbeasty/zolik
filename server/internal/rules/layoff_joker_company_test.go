package rules

import "testing"

// jokerCompanyState is the reported bug, as a board: someone else's run of
// 5-6-7-8, and a hand holding the joker, the 10 and the 9.
//
// The 10 does not reach the run on its own — the 9 sits between them — but it
// reaches with company, and the player holds two cards that can be that
// company: the 9 itself, or the joker standing in the 9's place.
func jokerCompanyState(hand []string) GameState {
	return GameState{
		Status:      StatusActive,
		Rules:       continentalNoFloor(),
		GameNumber:  1,
		Round:       1,
		Phase:       PhaseMeld,
		CurrentTurn: "me",
		TurnOrder:   []string{"me", "karel"},
		Hands:       map[string][]string{"me": hand, "karel": {"2D"}},
		Melds:       map[string][][]string{"karel": {{"5C", "6C", "7C", "8C"}}},
		MeldMeta: map[string][]MeldInfo{"karel": {
			{MeldID: "m1", Type: MeldRun, OwnerID: "karel"},
		}},
		RoundReqMet: map[string]bool{"me": true, "karel": true},
		GameScores:  map[string][]int{},
		TotalScores: map[string]int{},
	}
}

// The move itself, as the engine has always seen it: the joker fills the gap
// at the 9 and the 10 lands on top of it. Nothing below changes this — it is
// here so the rest of the file is read as "the offer disagreed with the
// engine", not "the move was never legal".
func TestValidateLayOff_JokerBridgesForTheTen(t *testing.T) {
	st := jokerCompanyState([]string{"JOKER", "TC", "9C", "2H"})
	if _, err := ValidateLayOff(cloneState(st), "me", "m1", []string{"JOKER", "TC"}, ""); err != nil {
		t.Fatalf("joker + 10 onto 5-6-7-8 is a legal lay-off, got %v", err)
	}
}

// And the workaround the player is left with when the pair is refused: drop
// the joker, then drop the 10 onto the run the joker just extended. Two
// actions for one move — which is how the bug was noticed.
func TestValidateLayOff_JokerThenTenGoesCardByCard(t *testing.T) {
	st := jokerCompanyState([]string{"JOKER", "TC", "9C", "2H"})
	st, err := ValidateLayOff(cloneState(st), "me", "m1", []string{"JOKER"}, "")
	if err != nil {
		t.Fatalf("laying the joker off on its own: %v", err)
	}
	if _, err := ValidateLayOff(cloneState(st), "me", "m1", []string{"TC"}, ""); err != nil {
		t.Fatalf("laying the 10 off after the joker: %v", err)
	}
}

// The bug. The 10's placement named the 9 as its company, because the chain
// pass reaches for naturals before wilds — so a client that reads `requires`
// as the whole truth refused joker + 10, a move the engine takes, and made
// the player lay the two cards one at a time.
//
// The 9 is not wrong; it is one of two answers. What was missing was the
// other one.
func TestLayOffPlacements_AJokerIsCompanyToo(t *testing.T) {
	st := jokerCompanyState([]string{"JOKER", "TC", "9C", "2H"})

	ten := layOffPlacementFor(t, st, "m1", "TC")
	if ten == nil {
		t.Fatal("the 10 is playable with company and must be offered")
	}
	if len(ten.Requires) == 0 {
		t.Fatalf("the 10 does not reach the run alone, got requires=%v", ten.Requires)
	}
	if !placementAccepts(*ten, []string{"9C"}) {
		t.Errorf("the 9 bridges the gap and must still be accepted company, got %+v", ten)
	}
	if !placementAccepts(*ten, []string{"JOKER"}) {
		t.Errorf("the joker bridges the gap too, got requires=%v alternatives=%v",
			ten.Requires, ten.Alternatives)
	}
	// Neither companion makes the 10 sendable alone.
	if !placementNeedsCompany(*ten) {
		t.Error("the 10 must still be refused on its own")
	}
	// And the engine agrees with both readings, which is the whole point.
	for _, pair := range [][]string{{"9C", "TC"}, {"JOKER", "TC"}} {
		if _, err := ValidateLayOff(cloneState(st), "me", "m1", pair, ""); err != nil {
			t.Errorf("the engine refused %v, which the offer now lists: %v", pair, err)
		}
	}
}

// The gate the offer feeds is a client's, so state it the way the client asks
// it: given these cards picked, is this placement satisfied?
func placementAccepts(p Placement, alsoPicked []string) bool {
	picked := map[string]bool{p.Card: true}
	for _, c := range alsoPicked {
		picked[c] = true
	}
	for _, set := range append([][]string{p.Requires}, p.Alternatives...) {
		ok := true
		for _, r := range set {
			if !picked[r] {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func placementNeedsCompany(p Placement) bool { return !placementAccepts(p, nil) }

// A hand holding no natural bridge still has to work, and did before this
// change: with no 9 to prefer, the chain pass already reached for the joker.
func TestLayOffPlacements_JokerIsTheOnlyCompany(t *testing.T) {
	st := jokerCompanyState([]string{"JOKER", "TC", "2H", "3H"})
	ten := layOffPlacementFor(t, st, "m1", "TC")
	if ten == nil {
		t.Fatal("the 10 is playable with the joker and must be offered")
	}
	if !placementAccepts(*ten, []string{"JOKER"}) {
		t.Fatalf("the joker is the 10's only company, got requires=%v alternatives=%v",
			ten.Requires, ten.Alternatives)
	}
}

// A wild-free hand must be left exactly as it was: one companion set, no
// alternatives invented for it.
func TestLayOffPlacements_NoWildNoAlternatives(t *testing.T) {
	st := jokerCompanyState([]string{"TC", "9C", "2H", "3H"})
	ten := layOffPlacementFor(t, st, "m1", "TC")
	if ten == nil {
		t.Fatal("the 10 is playable with the 9 and must be offered")
	}
	if len(ten.Requires) != 1 || ten.Requires[0] != "9C" {
		t.Fatalf("the 10 needs exactly the 9, got requires=%v", ten.Requires)
	}
	if len(ten.Alternatives) != 0 {
		t.Fatalf("there is no other way to reach the run, got alternatives=%v", ten.Alternatives)
	}
}

// The invariant the client leans on to keep its run-end highlight when a
// player takes the alternative: a placement's Positions describe the gap the
// card is reaching across, and every companion set fills that same gap, so
// they all grow the same end. Swept rather than argued, because the client
// reuses one hint for every way and would point at the wrong end of the run
// if this ever stopped holding.
func TestLayOffPlacements_EveryCompanionGrowsTheSameEnd(t *testing.T) {
	cfg := continentalNoFloor()
	melds := [][]string{
		{"5C", "6C", "7C", "8C"},
		{"5C", "JOKER", "7C", "8C"},
		{"TC", "JC", "QC", "KC"},
		{"AC", "2C", "3C", "4C"},
	}
	pool := []string{"2C", "3C", "4C", "6C", "9C", "TC", "JC", "JOKER", "JOKER2"}
	checked := 0
	for _, base := range melds {
		for _, a := range pool {
			for _, b := range pool {
				for _, c := range pool {
					st := jokerCompanyState(nil)
					st.Melds["karel"] = [][]string{base}
					st.Hands["me"] = []string{a, b, c, "2H", "3H"}
					o := FindOffer(LegalActions(st, "me"), LayOffOfferID("m1"))
					if o == nil || o.Source == nil {
						continue
					}
					for _, p := range o.Source.Placements {
						for _, alt := range p.Alternatives {
							checked++
							whole := append(append(append([]string(nil), base...), alt...), p.Card)
							if got := droppableEnds(base, whole, cfg); !sameCards(got, p.Positions) {
								t.Fatalf("meld %v, %s with %v grows %v but its hint says %v (for %v)",
									base, p.Card, alt, got, p.Positions, p.Requires)
							}
						}
					}
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("swept nothing — the boards above stopped producing alternatives")
	}
}
