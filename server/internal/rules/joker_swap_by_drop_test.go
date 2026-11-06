package rules

import (
	"slices"
	"testing"
)

// Dropping the joker's own card onto a run is a swap, and a swap grows
// neither end of the run. The end check refused it at both ends — including
// the end LegalActions had just told the client to drop on — so on a phone,
// where a drop is the only way to play a card onto a meld, a joker in a run
// could never be bought back. Reported from a real game: a K♦ that would not
// go onto the opponent's run where its joker stood for the king.
func TestLayOffSwapsTheJokerAtTheEndTheHintNames(t *testing.T) {
	for name, meld := range map[string][]string{
		"joker as the high end": {"JD", "QD", "JOKER1"},
		"joker in the middle":   {"QD", "JOKER1", "AD"},
		"long run":              {"9D", "TD", "JD", "QD", "JOKER1"},
	} {
		t.Run(name, func(t *testing.T) {
			st := classicState("me")
			st.Hands["me"] = []string{"KD", "5S", "7H"}
			st.Melds["p2"] = [][]string{append([]string(nil), meld...)}
			st.MeldMeta["p2"] = []MeldInfo{{MeldID: "m1", Type: MeldRun, OwnerID: "p2", WildCount: 1}}
			st.Melds["me"] = [][]string{{"3C", "4C", "5C"}}
			st.MeldMeta["me"] = []MeldInfo{{MeldID: "m0", Type: MeldRun, OwnerID: "me"}}
			st.RoundReqMet["me"], st.RoundReqMet["p2"] = true, true

			off := FindOffer(LegalActions(st, "me"), LayOffOfferID("m1"))
			if off == nil || !off.Enabled {
				t.Fatalf("expected an enabled lay-off onto the run, got %+v", off)
			}
			var positions []string
			found := false
			for _, p := range off.Source.Placements {
				if p.Card == "KD" {
					positions, found = p.Positions, true
				}
			}
			if !found {
				t.Fatalf("KD is not offered onto %v", meld)
			}
			for _, pos := range append(positions, "front", "end", "") {
				ns, err := ValidateLayOff(cloneState(st), "me", "m1", []string{"KD"}, pos)
				if err != nil {
					t.Fatalf("dropping KD at %q (hinted %v): %v", pos, positions, err)
				}
				got := ns.Melds["p2"][0]
				if !slices.Contains(got, "KD") || slices.Contains(got, "JOKER1") {
					t.Fatalf("at %q: expected KD in the joker's place, meld is %v", pos, got)
				}
				if !slices.Contains(ns.Hands["me"], "JOKER1") {
					t.Fatalf("at %q: expected the joker in hand, hand is %v", pos, ns.Hands["me"])
				}
			}
		})
	}
}

// The end check still holds for a card that really does grow the run.
func TestLayOffStillRefusesTheWrongEndForAnExtension(t *testing.T) {
	st := classicState("me")
	st.Hands["me"] = []string{"8D", "5S", "7H"}
	st.Melds["p2"] = [][]string{{"9D", "TD", "JD"}}
	st.MeldMeta["p2"] = []MeldInfo{{MeldID: "m1", Type: MeldRun, OwnerID: "p2"}}
	st.RoundReqMet["me"], st.RoundReqMet["p2"] = true, true
	_, err := ValidateLayOff(st, "me", "m1", []string{"8D"}, "end")
	if re, ok := err.(RulesError); !ok || re.Code != ErrWrongRunEnd {
		t.Fatalf("expected WRONG_RUN_END for 8D dropped on the high end, got %v", err)
	}
}
