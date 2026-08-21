package ui

import (
	"strings"
	"testing"
)

// TestDealHeaderLabel_ContinentalNamesTheDealAndContract keeps the header that
// is correct for the one profile that actually has a fixed seven-deal match
// with a per-deal required combination.
func TestDealHeaderLabel_ContinentalNamesTheDealAndContract(t *testing.T) {
	got := dealHeaderLabel("continental", 3)
	if want := "Game 3 of 7: Two Runs of 4"; got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

// TestDealHeaderLabel_ClassicOmitsDealCountAndContract is the regression guard
// for a header that read "Game N of 7: <contract>" for every profile. Under
// zolik_classic the match runs to a target score with no fixed deal count, and
// going down needs only one joker-free run rather than a per-deal combination
// — so both halves of that header were wrong.
func TestDealHeaderLabel_ClassicOmitsDealCountAndContract(t *testing.T) {
	got := dealHeaderLabel("zolik_classic", 3)
	if want := "Deal 3"; got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
	if strings.Contains(got, "of 7") {
		t.Fatalf("zolik_classic has no fixed deal count, but the header claims one: %q", got)
	}
}

// TestDealHeaderLabel_ClassicPastDealSeven is the case that made the old
// header visibly wrong in play: a score-limited match keeps dealing, so deal 8
// is perfectly normal and "Game 8 of 7" is nonsense.
func TestDealHeaderLabel_ClassicPastDealSeven(t *testing.T) {
	if got := dealHeaderLabel("zolik_classic", 9); got != "Deal 9" {
		t.Fatalf("want %q, got %q", "Deal 9", got)
	}
}

// TestDealHeaderLabel_UnknownProfileDoesNotInventAContract makes the safe
// choice the default: an unrecognised profile gets the neutral label rather
// than Continental's claims.
func TestDealHeaderLabel_UnknownProfileDoesNotInventAContract(t *testing.T) {
	for _, profile := range []string{"", "custom", "canasta"} {
		got := dealHeaderLabel(profile, 2)
		if got != "Deal 2" {
			t.Fatalf("profile %q: want the neutral %q, got %q", profile, "Deal 2", got)
		}
	}
}
