package auth

import (
	"strconv"
	"strings"
	"testing"
)

func TestNormalizeEmail(t *testing.T) {
	cases := map[string]string{
		"  Player@Example.COM ": "player@example.com",
		"already@lower.com":     "already@lower.com",
		"":                      "",
	}
	for in, want := range cases {
		if got := NormalizeEmail(in); got != want {
			t.Errorf("NormalizeEmail(%q) = %q, want %q", in, got, want)
		}
	}
}

// Normalisation stops at case and whitespace on purpose: stripping dots or
// +tags is a Gmail rule, and applying it everywhere merges genuinely different
// mailboxes into one account.
func TestNormalizeEmailDoesNotRewriteTheLocalPart(t *testing.T) {
	for _, addr := range []string{"first.last@example.com", "player+zolik@example.com"} {
		if got := NormalizeEmail(addr); got != addr {
			t.Errorf("NormalizeEmail(%q) = %q — the local part must be left alone", addr, got)
		}
	}
}

func TestValidEmail(t *testing.T) {
	valid := []string{
		"player@example.com",
		"first.last@sub.example.co.uk",
		"player+tag@example.com",
		"číslo@příklad.cz",
	}
	for _, addr := range valid {
		if !ValidEmail(NormalizeEmail(addr)) {
			t.Errorf("ValidEmail(%q) = false, want true", addr)
		}
	}

	invalid := []string{
		"",
		"nobody",
		"no@domain",                          // no dot in the domain
		"no@domain.",                         // trailing dot
		"two@@example.com",                   // malformed
		"spaces in@name.com",                 // unquoted spaces
		"a@b.com\r\nBcc: victim@example.com", // header injection attempt
		"Name <player@example.com>",          // a display name is not an address
		strings.Repeat("a", 250) + "@example.com",
	}
	for _, addr := range invalid {
		if ValidEmail(addr) {
			t.Errorf("ValidEmail(%q) = true, want false", addr)
		}
	}
}

func TestGenerateCodeIsSixDigits(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		code, err := generateCode()
		if err != nil {
			t.Fatalf("generateCode: %v", err)
		}
		if len(code) != 6 {
			t.Fatalf("code %q is %d characters, want 6 — a short code is a weaker secret", code, len(code))
		}
		if _, err := strconv.Atoi(code); err != nil {
			t.Fatalf("code %q is not numeric", code)
		}
		seen[code] = true
	}
	// Not a randomness test — just a canary for a generator stuck on one
	// value, which would turn every code into the same code.
	if len(seen) < 150 {
		t.Errorf("only %d distinct codes in 200 draws — the generator looks degenerate", len(seen))
	}
}

func TestStripCRLFDefendsMailHeaders(t *testing.T) {
	got := stripCRLF("Subject line\r\nBcc: victim@example.com")
	if strings.ContainsAny(got, "\r\n") {
		t.Errorf("stripCRLF left a newline in %q — that is SMTP header injection", got)
	}
}
