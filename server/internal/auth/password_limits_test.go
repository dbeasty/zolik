package auth_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"zolik/server/internal/auth"
)

// Before the limiters, /auth/login answered every guess at full speed, each
// one a bcrypt comparison, and /auth/register hashed for anybody who asked.

func TestLoginGuessesAtOneAccountAreThrottled(t *testing.T) {
	h := newTestHarness(t) // test endpoints on: the per-account ceiling still applies
	if reg := h.do(http.MethodPost, "/auth/register", "", map[string]any{
		"username": "guessme", "password": "the right password",
	}); reg.status != http.StatusOK {
		t.Fatalf("register: %d %s", reg.status, reg.raw)
	}
	limited := 0
	for i := 0; i < 30; i++ {
		res := h.do(http.MethodPost, "/auth/login", "", map[string]any{"username": "guessme", "password": fmt.Sprint("guess", i)})
		if res.status == http.StatusTooManyRequests {
			if res.str("code") != "RATE_LIMITED" {
				t.Fatalf("throttled login code = %q, want RATE_LIMITED", res.str("code"))
			}
			limited++
		}
	}
	if limited == 0 {
		t.Fatal("thirty wrong passwords in a row were all answered")
	}
	// The right password is refused too while throttled: otherwise the
	// ceiling would only slow down guesses that were wrong anyway.
	if res := h.do(http.MethodPost, "/auth/login", "", map[string]any{"username": "guessme", "password": "the right password"}); res.status != http.StatusTooManyRequests {
		t.Fatalf("login while throttled: status %d, want 429", res.status)
	}
	// The SSH door shares the ceiling.
	if _, err := h.handler.LoginSession(context.Background(), "guessme", "the right password"); !errors.Is(err, auth.ErrTooManyAttempts) {
		t.Fatalf("SSH login while throttled: %v, want ErrTooManyAttempts", err)
	}
	// Somebody else's account is unaffected.
	if res := h.do(http.MethodPost, "/auth/login", "", map[string]any{"username": "someoneelse", "password": "x"}); res.status != http.StatusUnauthorized {
		t.Fatalf("another account: status %d, want 401", res.status)
	}
}

func TestLoginGuessesFromOneClientAreThrottled(t *testing.T) {
	h := newTestHarnessWith(t, func(d *auth.Deps) { d.TestEndpointsEnabled = false })
	limited := false
	for i := 0; i < 80 && !limited; i++ {
		res := h.do(http.MethodPost, "/auth/login", "", map[string]any{"username": fmt.Sprint("spray", i), "password": "x"})
		limited = res.status == http.StatusTooManyRequests
	}
	if !limited {
		t.Fatal("eighty guesses across eighty names from one client were all answered")
	}
}

func TestRegisteringFromOneClientIsThrottled(t *testing.T) {
	h := newTestHarnessWith(t, func(d *auth.Deps) { d.TestEndpointsEnabled = false })
	limited := false
	for i := 0; i < 15 && !limited; i++ {
		res := h.do(http.MethodPost, "/auth/register", "", map[string]any{"username": fmt.Sprint("bulk", i), "password": "a password"})
		if res.status == http.StatusTooManyRequests {
			limited = true
		} else if res.status != http.StatusOK {
			t.Fatalf("register %d: %d %s", i, res.status, res.raw)
		}
	}
	if !limited {
		t.Fatal("fifteen registrations from one client in a row were all accepted")
	}
}
