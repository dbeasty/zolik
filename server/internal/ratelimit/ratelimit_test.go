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

func TestClientIPReadsTheEntryOurProxyAdded(t *testing.T) {
	req := httptest.NewRequest("POST", "/", nil)
	req.RemoteAddr = "10.9.9.9:5555"
	if got := ClientIP(req); got != "10.9.9.9" {
		t.Errorf("with no header: got %q, want 10.9.9.9", got)
	}

	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	if got := ClientIP(req); got != "203.0.113.9" {
		t.Errorf("with a single entry: got %q, want 203.0.113.9", got)
	}

	// A client-sent entry ahead of the one the proxy appended is the client's
	// own word; keying on it would let a caller pick a fresh identity each
	// request.
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 203.0.113.7")
	if got := ClientIP(req); got != "203.0.113.7" {
		t.Errorf("with a spoofed chain: got %q, want 203.0.113.7", got)
	}

	req.Header.Set("X-Forwarded-For", "6.6.6.6,7.7.7.7 ,  2001:db8::7 ")
	if got := ClientIP(req); got != "2001:db8::7" {
		t.Errorf("with a longer chain: got %q, want 2001:db8::7", got)
	}
}

func TestClientIPSpoofedEntriesDoNotChangeTheKey(t *testing.T) {
	seen := map[string]bool{}
	for _, spoof := range []string{"1.1.1.1", "2.2.2.2", "9.9.9.9, 8.8.8.8"} {
		req := httptest.NewRequest("POST", "/", nil)
		req.Header.Set("X-Forwarded-For", spoof+", 198.51.100.4")
		seen[ClientIP(req)] = true
	}
	if len(seen) != 1 || !seen["198.51.100.4"] {
		t.Errorf("spoofed prefixes produced keys %v, want only 198.51.100.4", seen)
	}
}

func TestClientIPFallsBackToTheRemoteAddr(t *testing.T) {
	cases := map[string]string{
		"10.9.9.9:5555":      "10.9.9.9",
		"[2001:db8::1]:443":  "2001:db8::1",
		"[::1]:8090":         "::1",
		"[fe80::1%en0]:1234": "fe80::1%en0",
		"ble-peer:0":         "ble-peer",
		"no-port-at-all":     "no-port-at-all",
	}
	for remote, want := range cases {
		req := httptest.NewRequest("POST", "/", nil)
		req.RemoteAddr = remote
		if got := ClientIP(req); got != want {
			t.Errorf("RemoteAddr %q: got %q, want %q", remote, got, want)
		}
	}

	// An empty or blank header is no header at all.
	req := httptest.NewRequest("POST", "/", nil)
	req.RemoteAddr = "[2001:db8::2]:1"
	req.Header.Set("X-Forwarded-For", " ")
	if got := ClientIP(req); got != "2001:db8::2" {
		t.Errorf("with a blank header: got %q, want 2001:db8::2", got)
	}
}
