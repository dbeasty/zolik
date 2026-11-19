package auth_test

import (
	"net/http"
	"testing"
)

// A player with an email account opened "sign in with a username instead",
// typed their address, could not sign in (the account has no password), and
// created an account from the same screen. That second account then played
// and won while the real one's record stood still. Registering with an address
// that already has an account is refused and points at email sign-in.
func TestRegisteringWithAnAddressThatHasAnAccountIsRefused(t *testing.T) {
	h := newTestHarness(t)
	addr := "player@example.com"

	h.do(http.MethodPost, "/auth/email/start", "", map[string]any{"email": addr})
	code := h.mailer.lastCodeFor(t, addr)
	owner := h.do(http.MethodPost, "/auth/email/verify", "", map[string]any{"email": addr, "code": code})
	if owner.status != http.StatusOK {
		t.Fatalf("email sign-in: status %d body %s", owner.status, owner.raw)
	}

	for _, tc := range []struct {
		name string
		body map[string]any
	}{
		{"address as the username and the address", map[string]any{"username": addr, "email": addr, "password": "a password"}},
		{"address as the username only", map[string]any{"username": "Player@Example.com", "password": "a password"}},
		{"address only", map[string]any{"username": "someone", "email": " PLAYER@example.com ", "password": "a password"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := h.do(http.MethodPost, "/auth/register", "", tc.body)
			if res.status != http.StatusConflict || res.str("code") != "EMAIL_HAS_ACCOUNT" {
				t.Fatalf("status %d code %q, want 409 EMAIL_HAS_ACCOUNT (body %s)", res.status, res.str("code"), res.raw)
			}
		})
	}

	// An address nobody holds still registers, as the legacy form always has.
	if res := h.do(http.MethodPost, "/auth/register", "", map[string]any{
		"username": "fresh@example.com", "email": "fresh@example.com", "password": "a password",
	}); res.status != http.StatusOK {
		t.Fatalf("a free address: status %d body %s", res.status, res.raw)
	}
}
