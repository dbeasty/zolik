package rules

import "testing"

func TestMeldNoContribution_Game1RejectsRun(t *testing.T) {
	p := "p1"
	st := baseActiveState(1, p)
	st.Hands[p] = []string{"5H", "6H", "7H", "8H", "2S"}

	_, _, _, err := ValidateMeldAction(st, p, []string{"5H", "6H", "7H", "8H"})
	if err == nil {
		t.Fatal("expected run rejected in round 1 (needs sets only)")
	}
	re, ok := err.(RulesError)
	if !ok || re.Code != ErrMeldNoContribution {
		t.Fatalf("expected MELD_NO_CONTRIBUTION, got %v", err)
	}
}

func TestMeldNoContribution_Game2RejectsExtraRun(t *testing.T) {
	p := "p1"
	st := baseActiveState(2, p)
	st.Hands[p] = []string{"5S", "6S", "7S", "8S", "2C"}

	st, _, _, err := ValidateMeldAction(st, p, []string{"5S", "6S", "7S", "8S"})
	if err != nil {
		t.Fatalf("first run should contribute: %v", err)
	}
	if st.RoundReqMet[p] {
		t.Fatal("round req should not be met with only a run in round 2")
	}

	st.Hands[p] = []string{"5D", "6D", "7D", "8D", "2C"}
	_, _, _, err = ValidateMeldAction(st, p, []string{"5D", "6D", "7D", "8D"})
	if err == nil {
		t.Fatal("expected second run rejected when set still needed")
	}
	re, ok := err.(RulesError)
	if !ok || re.Code != ErrMeldNoContribution {
		t.Fatalf("expected MELD_NO_CONTRIBUTION, got %v", err)
	}
}

func TestMeldContributes_AllowedAfterRoundReqMet(t *testing.T) {
	p := "p1"
	st := baseActiveState(1, p)
	st.RoundReqMet[p] = true
	st.Hands[p] = []string{"5H", "6H", "7H", "8H", "2S"}

	_, _, _, err := ValidateMeldAction(st, p, []string{"5H", "6H", "7H", "8H"})
	if err != nil {
		t.Fatalf("extra meld allowed after round req met: %v", err)
	}
}

func TestHandPenalty_AceNaturalInHandFragment(t *testing.T) {
	hand := []string{"AS", "2S", "KH"}
	got := HandPenaltyTotal(hand, ProfileContinental)
	// AS=1 (with 2S), 2S=2, KH=10
	if got != 13 {
		t.Fatalf("expected ace low fragment scoring, got %d", got)
	}
}

func TestHandPenalty_AceExtendsTableRun(t *testing.T) {
	hand := []string{"AH"}
	melds := [][]string{{"TH", "JH", "QH", "KH"}}
	got := HandPenaltyTotalWithMelds(hand, melds, ProfileContinental)
	if got != 1 {
		t.Fatalf("expected ace extending run = 1, got %d", got)
	}
}
