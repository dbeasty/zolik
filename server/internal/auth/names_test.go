package auth

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCleanDisplayName(t *testing.T) {
	cases := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"an ordinary name is untouched", "Ann", 24, "Ann"},
		{"accents and other scripts survive", "Žofie Čapková", 24, "Žofie Čapková"},
		{"emoji survive", "🂡 Ace 🃁", 24, "🂡 Ace 🃁"},
		{"a zero-width joiner inside a sequence is kept", "👩\u200d💻", 24, "👩\u200d💻"},
		{"a Persian name keeps its non-joiner", "می\u200cخواهم", 24, "می\u200cخواهم"},
		{"the ends are trimmed", "  Ann  ", 24, "Ann"},
		{"runs of whitespace collapse to one space", "Ann \t\n  Lee", 24, "Ann Lee"},
		{"nothing but spaces is nobody", "     ", 24, ""},
		{"nothing but zero-width characters is nobody", "\u200b\u200b\u2060\ufeff", 24, ""},
		{"nothing but joiners is nobody", "\u200d\u200c", 24, ""},
		{"a NUL is dropped", "a\x00b", 24, "ab"},
		{"control characters are dropped", "a\x07b\x1bc", 24, "abc"},
		{"a right-to-left override cannot reverse what follows", "\u202eevil", 24, "evil"},
		{"bidi isolates and embeddings are dropped too", "a\u2066b\u2069c\u202ad\u202c", 24, "abcd"},
		{"line and paragraph separators are only spaces", "a\u2028b\u2029c", 24, "a b c"},
		{"invalid UTF-8 is dropped, not passed on", "ab\xffcd\xc3", 24, "abcd"},
		{"a name is cut at the limit, in characters not bytes", strings.Repeat("ž", 40), 24, strings.Repeat("ž", 24)},
		{"a ten-thousand-character name is cut", strings.Repeat("x", 10_000), 24, strings.Repeat("x", 24)},
		{"the cut does not leave a trailing space", "abcdefghij klmnopqrstuvwxyz", 11, "abcdefghij"},
		{"markup is left alone: it is shown as text, never parsed", "<b>hi</b>", 24, "<b>hi</b>"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := CleanDisplayName(c.in, c.max)
			if got != c.want {
				t.Fatalf("CleanDisplayName(%q, %d) = %q, want %q", c.in, c.max, got, c.want)
			}
			if !utf8.ValidString(got) {
				t.Fatalf("result %q is not valid UTF-8", got)
			}
			if utf8.RuneCountInString(got) > c.max {
				t.Fatalf("result has %d runes, over the limit %d", utf8.RuneCountInString(got), c.max)
			}
			if again := CleanDisplayName(got, c.max); again != got {
				t.Fatalf("not idempotent: %q then %q", got, again)
			}
		})
	}
}

func TestAPasswordLongerThanBcryptReadsStillCountsInFull(t *testing.T) {
	long := strings.Repeat("a", 80) + "X"
	hash, err := hashPassword(long, 4)
	if err != nil {
		t.Fatalf("a long password must hash, not error: %v", err)
	}
	if !passwordMatches(hash, long) {
		t.Fatal("the password it was made from must match")
	}
	// bcrypt alone would read only the first 72 bytes and accept this.
	if passwordMatches(hash, strings.Repeat("a", 80)+"Y") {
		t.Fatal("a password differing past byte 72 must not match")
	}
	if passwordMatches(hash, strings.Repeat("a", 72)) {
		t.Fatal("a prefix of the password must not match")
	}
}

func TestAShortPasswordHashesTheWayItAlwaysDid(t *testing.T) {
	// Every account made before long passwords were allowed holds a bcrypt hash
	// of the raw password; the helper must verify those untouched.
	for _, pw := range []string{"x", "correct horse battery staple", strings.Repeat("p", 72)} {
		hash, err := hashPassword(pw, 4)
		if err != nil {
			t.Fatal(err)
		}
		if string(bcryptInput(pw)) != pw {
			t.Fatalf("a password of %d bytes was altered before hashing", len(pw))
		}
		if !passwordMatches(hash, pw) {
			t.Fatalf("%d-byte password does not match its own hash", len(pw))
		}
	}
}

func TestAnAbsurdlyLongPasswordNeverMatches(t *testing.T) {
	hash, _ := hashPassword("short", 4)
	if passwordMatches(hash, strings.Repeat("a", MaxPasswordBytes+1)) {
		t.Fatal("an over-long password must be refused before it is hashed")
	}
}
