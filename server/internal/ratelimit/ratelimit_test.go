package ratelimit

import (
	"net/http/httptest"
	"testing"
	"time"
)

// One key running out of allowance must not silence everybody else.
func TestTheCeilingIsPerKey(t *testing.T) {
	l := New(time.Hour, 2)
	now := time.Now()

	for i := 0; i < 2; i++ {
		if !l.Allow("10.0.0.1", now) {
			t.Fatalf("attempt %d from the first key was refused early", i+1)
		}
	}
	if l.Allow("10.0.0.1", now) {
		t.Error("the first key went over its ceiling")
	}
	if !l.Allow("10.0.0.2", now) {
		t.Error("a second key was throttled by the first key's history")
	}
}

// Allowance has to come back, or one busy hour bans someone for good.
func TestTheCeilingExpires(t *testing.T) {
	l := New(time.Hour, 1)
	start := time.Now()

	if !l.Allow("k", start) {
		t.Fatal("the first attempt was refused")
	}
	if l.Allow("k", start.Add(time.Minute)) {
		t.Error("a second attempt inside the window was allowed")
	}
	if !l.Allow("k", start.Add(time.Hour+time.Minute)) {
		t.Error("the allowance never came back after the window passed")
	}
}

// A few wrong guesses before the right one must not count against whatever
// the caller does next.
func TestResetForgetsAKey(t *testing.T) {
	l := New(time.Hour, 2)
	now := time.Now()

	l.Allow("k", now)
	l.Allow("k", now)
	if l.Allow("k", now) {
		t.Fatal("the ceiling did not hold before the reset")
	}
	l.Reset("k")
	if !l.Allow("k", now) {
		t.Error("Reset did not restore the allowance")
	}
}

func TestClientIPPrefersTheForwardedChainsFirstEntry(t *testing.T) {
	req := httptest.NewRequest("POST", "/", nil)
	req.RemoteAddr = "10.9.9.9:5555"
	if got := ClientIP(req); got != "10.9.9.9" {
		t.Errorf("with no header: got %q, want 10.9.9.9", got)
	}

	req.Header.Set("X-Forwarded-For", "203.0.113.7, 70.41.3.18")
	if got := ClientIP(req); got != "203.0.113.7" {
		t.Errorf("with a chain: got %q, want 203.0.113.7", got)
	}

	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	if got := ClientIP(req); got != "203.0.113.9" {
		t.Errorf("with a single entry: got %q, want 203.0.113.9", got)
	}
}
