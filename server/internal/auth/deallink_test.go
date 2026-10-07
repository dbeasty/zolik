package auth

import (
	"strings"
	"testing"
)

func TestADealLinkHidesTheMatchAndCannotBeForged(t *testing.T) {
	const match = "6ac6aa5b429c8d4653a4bf78"
	token := SealDealLink(match)
	if token == "" {
		t.Fatal("no token")
	}
	if strings.Contains(strings.ToLower(token), match) {
		t.Fatal("the token names the match")
	}
	if got := OpenDealLink(token); got != match {
		t.Fatalf("opened to %q", got)
	}
	if SealDealLink(match) == token {
		t.Fatal("two links to one deal are identical")
	}
	// Any change to the token is refused.
	b := []byte(token)
	b[len(b)/2] ^= 1
	if got := OpenDealLink(string(b)); got != "" {
		t.Fatalf("a tampered token opened to %q", got)
	}
	for _, bad := range []string{"", "x", match, "not-a-token-at-all"} {
		if got := OpenDealLink(bad); got != "" {
			t.Fatalf("%q opened to %q", bad, got)
		}
	}
	if SealDealLink("not-hex") != "" {
		t.Fatal("sealed something that is not a match id")
	}
}
