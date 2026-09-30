package auth

import (
	"testing"
	"time"
)

// A "dev:<playerId>" token used to be taken at its word, so anyone could open
// a match socket as any player and read their hand.
func TestSubjectFromToken_RejectsDevToken(t *testing.T) {
	for _, tok := range []string{"dev:player123", "dev:", " dev:player123 "} {
		if subj, err := SubjectFromToken(tok); err == nil {
			t.Errorf("SubjectFromToken(%q) = %q, want an error", tok, subj)
		}
	}
}

func TestSubjectFromToken_AcceptsSignedToken(t *testing.T) {
	t.Setenv("JWT_ACCESS_SECRET", "test_secret")
	tok, err := CreateAccessToken("player123", "alice", false, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	subj, err := SubjectFromToken(tok)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if subj != "player123" {
		t.Fatalf("expected player123 got %s", subj)
	}
}

func TestSubjectFromToken_RejectsTokenSignedWithAnotherKey(t *testing.T) {
	t.Setenv("JWT_ACCESS_SECRET", "attacker_key")
	tok, err := CreateAccessToken("player123", "alice", false, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("JWT_ACCESS_SECRET", "server_key")
	if _, err := SubjectFromToken(tok); err == nil {
		t.Fatal("accepted a token signed with a different key")
	}
}

func TestCheckAccessSecret(t *testing.T) {
	cases := []struct {
		secret string
		local  bool
		ok     bool
	}{
		{"", true, true},
		{"", false, false},
		{"   ", false, false},
		{devAccessSecret, false, false},
		{"REPLACE_ON_FIRST_DEPLOY", false, false},
		{"a-real-private-value", false, true},
	}
	for _, c := range cases {
		t.Setenv("JWT_ACCESS_SECRET", c.secret)
		err := CheckAccessSecret(c.local)
		if (err == nil) != c.ok {
			t.Errorf("secret=%q local=%v: err=%v, want ok=%v", c.secret, c.local, err, c.ok)
		}
	}
}
