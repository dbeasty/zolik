package auth

import (
	"testing"
	"time"
)

// A match-scoped token is refused by everything that does not name its match,
// without those callers having to know such tokens exist.
func TestAMatchScopedTokenOnlyOpensItsMatch(t *testing.T) {
	scoped, err := CreateMatchScopedToken("bob", "Bob", true, "m1", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseAccessClaims(scoped); err == nil {
		t.Error("ParseAccessClaims accepted a scoped token")
	}
	if _, err := SubjectFromToken(scoped); err == nil {
		t.Error("SubjectFromToken accepted a scoped token")
	}
	if sub, err := SubjectForMatch(scoped, "m1"); err != nil || sub != "bob" {
		t.Errorf("its own match: %q %v", sub, err)
	}
	for _, other := range []string{"m2", ""} {
		if _, err := SubjectForMatch(scoped, other); err == nil {
			t.Errorf("match %q accepted a token scoped to m1", other)
		}
	}

	// An ordinary token is still good at any match.
	plain, err := CreateAccessToken("ann", "Ann", false, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if sub, err := SubjectForMatch(plain, "m1"); err != nil || sub != "ann" {
		t.Errorf("an ordinary token at a match: %q %v", sub, err)
	}
}
