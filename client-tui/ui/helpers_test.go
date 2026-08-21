package ui

import (
	"strings"
	"testing"

	"zolik/client-tui/api"
)

// The header used to be built from the profile *name* plus this package's own
// transcription of the Continental contract table. It now reads the ruleset
// and contract the server resolved, so these tests are expressed the same
// way: by the shape of the match, not by its name.

var continentalRules = api.ResolvedRules{
	Profile:        "continental",
	DealSize:       12,
	MinSetSize:     3,
	MinRunSize:     4,
	FixedDealCount: 7,
	MatchEndMode:   "after_deals",
}

var classicRules = api.ResolvedRules{
	Profile:        "zolik_classic",
	DealSize:       13,
	MinSetSize:     3,
	MinRunSize:     3,
	FixedDealCount: 0,
	MatchEndMode:   "at_score",
	TargetScore:    200,
}

// TestDealHeaderLabel_FixedLengthMatchNamesTheDealAndContract keeps the header
// that is correct for a match with a fixed deal count and a per-deal required
// combination.
func TestDealHeaderLabel_FixedLengthMatchNamesTheDealAndContract(t *testing.T) {
	got := dealHeaderLabel(continentalRules, api.Contract{Runs: 2}, 3)
	if want := "Game 3 of 7: Two runs"; got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

// TestDealHeaderLabel_ScoreLimitedOmitsDealCountAndContract is the regression
// guard for a header that read "Game N of 7: <contract>" for every profile. A
// score-limited match has no fixed deal count, and going down needs only one
// joker-free run rather than a per-deal combination — so both halves of that
// header were wrong.
func TestDealHeaderLabel_ScoreLimitedOmitsDealCountAndContract(t *testing.T) {
	got := dealHeaderLabel(classicRules, api.Contract{RequireCleanRun: true}, 3)
	if want := "Deal 3"; got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
	if strings.Contains(got, "of 7") {
		t.Fatalf("a score-limited match has no fixed deal count, but the header claims one: %q", got)
	}
}

// TestDealHeaderLabel_ScoreLimitedPastDealSeven is the case that made the old
// header visibly wrong in play: a score-limited match keeps dealing, so deal 9
// is perfectly normal and "Game 9 of 7" is nonsense.
func TestDealHeaderLabel_ScoreLimitedPastDealSeven(t *testing.T) {
	if got := dealHeaderLabel(classicRules, api.Contract{RequireCleanRun: true}, 9); got != "Deal 9" {
		t.Fatalf("want %q, got %q", "Deal 9", got)
	}
}

// TestDealHeaderLabel_NoRulesYetDoesNotInventAContract makes the safe choice
// the default: before the first state arrives (or against a server that sends
// no ruleset) the header states only what it knows.
func TestDealHeaderLabel_NoRulesYetDoesNotInventAContract(t *testing.T) {
	got := dealHeaderLabel(api.ResolvedRules{}, api.Contract{}, 2)
	if got != "Deal 2" {
		t.Fatalf("want the neutral %q, got %q", "Deal 2", got)
	}
}

// TestDealHeaderLabel_DescribesAProfileItHasNeverSeen is the extensibility
// claim made checkable: a five-deal variation this client has no knowledge of
// is labelled correctly, because every fact in the label came from the server.
func TestDealHeaderLabel_DescribesAProfileItHasNeverSeen(t *testing.T) {
	kalooki := api.ResolvedRules{
		Profile:        "kalooki_house",
		FixedDealCount: 5,
		MatchEndMode:   "after_deals",
	}
	got := dealHeaderLabel(kalooki, api.Contract{Sets: 1, Runs: 2}, 5)
	if want := "Game 5 of 5: One set, Two runs"; got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestContractLabel(t *testing.T) {
	for _, tc := range []struct {
		name     string
		contract api.Contract
		want     string
	}{
		{"two sets", api.Contract{Sets: 2}, "Two sets"},
		{"one of each", api.Contract{Sets: 1, Runs: 1}, "One set, One run"},
		{"three runs", api.Contract{Runs: 3}, "Three runs"},
		{"clean run only", api.Contract{RequireCleanRun: true}, "Any mix of sets and runs, one run joker-free"},
		{"counted plus clean", api.Contract{Sets: 1, Runs: 1, RequireCleanRun: true}, "One set, One run (one run joker-free)"},
		{"no requirement", api.Contract{}, "Any valid meld"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := contractLabel(tc.contract); got != tc.want {
				t.Fatalf("want %q, got %q", tc.want, got)
			}
		})
	}
}
