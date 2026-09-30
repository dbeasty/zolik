package auth

import (
	"strconv"
	"strings"
	"testing"
)

// The whole point of the seed: a device that keeps no name of its own is
// greeted as the same player every time it comes back, rather than as a
// stranger.
func TestGuestNameForIsStablePerSeed(t *testing.T) {
	const seed = "0123456789abcdef0123456789abcdef"
	first := GuestNameFor(seed)
	for i := 0; i < 10; i++ {
		if got := GuestNameFor(seed); got != first {
			t.Fatalf("GuestNameFor(%q) = %q on call %d, was %q", seed, got, i, first)
		}
	}
}

// A default name that is the same word for everybody is the defect this
// replaces, so the test that matters is that different devices differ.
func TestGuestNamesSpreadAcrossTheRoster(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 500; i++ {
		seen[GuestNameFor(strings.Repeat("a", i%7)+string(rune('a'+i%26))+strconv.Itoa(i))] = true
	}
	// Five hundred ids landing on fewer than a hundred names would mean the
	// hash is collapsing, whatever the roster's size says.
	if len(seen) < 100 {
		t.Errorf("500 guest ids produced only %d distinct names", len(seen))
	}
}

// Both halves have to move. Two plain moduli of one number over two lists of
// equal length would lock them together — 64 names instead of 4096 — and the
// symptom would be a roster that looks full and behaves tiny.
func TestGuestNameVariesInBothHalves(t *testing.T) {
	adjectives, nouns := map[string]bool{}, map[string]bool{}
	for i := 0; i < 2000; i++ {
		adj, noun, ok := strings.Cut(GuestNameFor(strconv.Itoa(i)), " ")
		if !ok {
			t.Fatalf("GuestNameFor(%d) is not two words", i)
		}
		adjectives[adj] = true
		nouns[noun] = true
	}
	if len(adjectives) < len(guestAdjectives)/2 || len(nouns) < len(guestNouns)/2 {
		t.Errorf("thin coverage: %d adjectives and %d nouns out of %d and %d",
			len(adjectives), len(nouns), len(guestAdjectives), len(guestNouns))
	}
}

// An unseeded name is for the caller with nothing durable to hang one on, and
// it must still not be the same word every time.
func TestUnseededGuestNamesDiffer(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		seen[GuestNameFor("")] = true
	}
	if len(seen) < 20 {
		t.Errorf("200 unseeded names produced only %d distinct ones", len(seen))
	}
}

// Nothing in the roster may carry a character a display name cannot, and
// nothing may be long enough to need truncating on a seat tile.
func TestGuestNameRosterIsWellFormed(t *testing.T) {
	for _, word := range append(append([]string{}, guestAdjectives...), guestNouns...) {
		if word == "" || strings.TrimSpace(word) != word || strings.ContainsAny(word, " \t") {
			t.Errorf("roster word %q is not a single trimmed word", word)
		}
		if len([]rune(word)) > 10 {
			t.Errorf("roster word %q is %d characters, too long for a seat tile", word, len([]rune(word)))
		}
	}
	if dupes := duplicates(append(append([]string{}, guestAdjectives...), guestNouns...)); len(dupes) > 0 {
		// Including across the two lists: nothing should be able to draw
		// "Kite Kite". A repeat is always a slip rather than a decision.
		t.Errorf("roster repeats %v", dupes)
	}
}

func duplicates(words []string) []string {
	seen, out := map[string]bool{}, []string{}
	for _, w := range words {
		if seen[w] {
			out = append(out, w)
		}
		seen[w] = true
	}
	return out
}
