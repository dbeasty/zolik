package auth_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

// Somebody registers a password account with an address that is not theirs.
// The legacy form never proves an address, but the address is still unique
// across accounts, so before the fix the real owner's first verified sign-in
// could not create its account and failed with a 500 ("could not resolve
// identity ... after 9 attempts").
func TestAVerifiedEmailSignInTakesTheAddressFromAnUnverifiedSquatter(t *testing.T) {
	h := newTestHarness(t)
	victim := "owner@example.com"

	squat := h.do(http.MethodPost, "/auth/register", "", map[string]any{
		"username": "squatter", "email": victim, "password": "squatters password",
	})
	if squat.status != http.StatusOK {
		t.Fatalf("register: status %d body %s", squat.status, squat.raw)
	}

	h.do(http.MethodPost, "/auth/email/start", "", map[string]any{"email": victim})
	code := h.mailer.lastCodeFor(t, victim)
	res := h.do(http.MethodPost, "/auth/email/verify", "", map[string]any{"email": victim, "code": code})
	if res.status != http.StatusOK {
		t.Fatalf("the address's owner could not sign in: status %d body %s", res.status, res.raw)
	}
	if res.str("userId") == squat.str("userId") {
		t.Fatal("the owner was signed in to the squatter's account")
	}

	ctx := context.Background()
	squatter, err := h.store.FindUserByID(ctx, squat.str("userId"))
	if err != nil {
		t.Fatal(err)
	}
	if squatter.Email != "" {
		t.Errorf("the squatter still holds %q", squatter.Email)
	}
	owner, err := h.store.FindUserByVerifiedEmail(ctx, victim)
	if err != nil {
		t.Fatalf("no account holds the proven address: %v", err)
	}
	if owner.ID.Hex() != res.str("userId") {
		t.Errorf("the proven address is on %s, want the owner %s", owner.ID.Hex(), res.str("userId"))
	}

	// The squatter keeps their account and password, just not the address.
	login := h.do(http.MethodPost, "/auth/login", "", map[string]any{
		"username": "squatter", "password": "squatters password",
	})
	if login.status != http.StatusOK {
		t.Errorf("the squatter's own login broke: status %d", login.status)
	}
}

// The same lock-out through a provider that vouches for the address.
func TestAVerifiedProviderSignInTakesTheAddressFromAnUnverifiedSquatter(t *testing.T) {
	h := newTestHarness(t)
	victim := "oauth-owner@example.com"

	squat := h.do(http.MethodPost, "/auth/register", "", map[string]any{
		"username": "oauthsquatter", "email": victim, "password": "squatters password",
	})
	if squat.status != http.StatusOK {
		t.Fatalf("register: status %d body %s", squat.status, squat.raw)
	}

	res := h.do(http.MethodPost, "/auth/oauth/fake/token", "", map[string]any{
		"idToken": h.oidc.sign(t, jwt.MapClaims{"sub": "real-owner", "email": victim, "email_verified": true}),
	})
	if res.status != http.StatusOK {
		t.Fatalf("the address's owner could not sign in: status %d body %s", res.status, res.raw)
	}
	if res.str("userId") == squat.str("userId") {
		t.Fatal("an unproven address merged the owner into the squatter's account")
	}
}

// A provider that does not vouch for the address takes nothing from anybody.
func TestAnUnverifiedProviderEmailTakesNothing(t *testing.T) {
	h := newTestHarness(t)
	addr := "held@example.com"

	first := h.do(http.MethodPost, "/auth/register", "", map[string]any{
		"username": "firstholder", "email": addr, "password": "a long password",
	})
	if first.status != http.StatusOK {
		t.Fatalf("register: status %d body %s", first.status, first.raw)
	}
	res := h.do(http.MethodPost, "/auth/oauth/fake/token", "", map[string]any{
		"idToken": h.oidc.sign(t, jwt.MapClaims{"sub": "unproven", "email": addr}),
	})
	if res.status != http.StatusOK {
		t.Fatalf("sign-in: status %d body %s", res.status, res.raw)
	}
	held, err := h.store.FindUserByID(context.Background(), first.str("userId"))
	if err != nil {
		t.Fatal(err)
	}
	if held.Email != addr {
		t.Errorf("an unverified claim took the address: holder now has %q", held.Email)
	}
}
