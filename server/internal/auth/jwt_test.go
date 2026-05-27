package auth

import "testing"

func TestSubjectFromToken_DevFallback(t *testing.T) {
	subj, err := SubjectFromToken("dev:player123")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if subj != "player123" {
		t.Fatalf("expected player123 got %s", subj)
	}
}

