// Package identity is the pluggable half of authentication: everything that
// knows how to turn "this person controls an account somewhere else" into a
// set of claims this server is willing to act on.
//
// The rest of the server never names a login provider. It asks the Registry
// for one by id, gets a Provider back, and works in Claims. Adding Apple or
// Microsoft is therefore a configuration change plus, at most, one small
// quirk hook — not a new code path through the handlers, the user model or
// the clients. That is the whole point of the split, and the reason the
// Google-specific part of this package is a config literal.
package identity

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrUnknownProvider is returned when a request names a provider that is not
// registered or not configured on this deployment. Handlers turn it into a
// 404 rather than a 500: an unconfigured provider is a deployment fact, not a
// server fault.
var ErrUnknownProvider = errors.New("unknown or unconfigured identity provider")

// Claims is one provider's assertion about a person, normalised to the only
// fields this server actually uses. Everything downstream — account lookup,
// linking, display names — reads this struct and never a provider-shaped
// payload, so a new provider cannot leak its vocabulary into the user model.
type Claims struct {
	// Provider is the registry id ("google", "apple", "microsoft", "email").
	Provider string
	// Subject is the provider's stable, opaque id for this person. It is the
	// half of the identity key that must never change: email addresses get
	// reassigned and display names get edited, subjects do not. Paired with
	// Provider it is unique across the whole system — see the identities
	// collection's unique index, which is what actually enforces it.
	Subject string
	// Email is the address the provider reports. It may be absent (an Apple
	// private-relay user who revoked sharing) and it may be unverified.
	Email string
	// EmailVerified reports whether the *provider* vouches for the address.
	// An unverified address is never used to match an existing account — that
	// is the account-takeover hole this flag exists to close.
	EmailVerified bool
	// Name is a display-name suggestion, used only when the account has none
	// yet. Never authoritative: users rename themselves.
	Name string
	// Picture is an avatar URL, if the provider offers one.
	Picture string
}

// Valid reports whether the claims are usable as an identity. A provider that
// returns no subject has told us nothing we can key an account on.
func (c Claims) Valid() bool {
	return c.Provider != "" && c.Subject != ""
}

// Provider is one way of proving who someone is.
//
// The two verification entry points exist because the same provider is
// reached differently on different platforms, and both are first-class:
//
//   - ExchangeCode backs the browser redirect flow (web, and mobile via an
//     in-app browser tab). The server holds the client secret and talks to the
//     token endpoint itself, so nothing confidential reaches the client.
//   - VerifyIDToken backs native SDK flows (Google Sign-In, Sign in with
//     Apple, MSAL), where the platform hands the app an ID token directly and
//     there is no authorization code to exchange.
//
// Both return the same Claims, so the account plumbing behind them is shared:
// there is exactly one implementation of "find or create the user for these
// claims", no matter which door the person came through.
type Provider interface {
	// ID is the stable registry key used in URLs and stored on identities.
	ID() string
	// DisplayName is the human label a client renders on the button.
	DisplayName() string
	// AuthCodeURL builds the provider's authorization URL for the redirect
	// flow. state and nonce are generated and remembered by the caller.
	AuthCodeURL(state, nonce, redirectURI string) (string, error)
	// ExchangeCode redeems an authorization code for verified claims.
	ExchangeCode(ctx context.Context, code, redirectURI, nonce string) (Claims, error)
	// VerifyIDToken verifies an ID token obtained by a native SDK. nonce may
	// be empty when the platform's flow does not carry one.
	VerifyIDToken(ctx context.Context, rawIDToken, nonce string) (Claims, error)
}

// Descriptor is the public description of a login method, served to clients so
// the sign-in screen is built from what the server actually has configured
// rather than from a hardcoded list per client. Turning Google off in one
// deployment therefore removes the button from every client at once, and
// turning Apple on later lights it up without shipping a client build.
type Descriptor struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	// Kind tells a client which flow to run: "oauth" means send the person
	// through the browser or native flow; "email" means collect an address and
	// post it to the code endpoints; "guest" needs no input at all.
	Kind string `json:"kind"`
}

// Registry holds the providers this deployment has configured.
//
// Built once at startup from configuration (see FromConfig) and read-only
// afterwards, so it needs no locking.
type Registry struct {
	providers map[string]Provider
}

// NewRegistry builds a registry from the given providers. A nil provider is
// skipped, which is what lets FromConfig write one line per provider and let
// the unconfigured ones fall away.
func NewRegistry(ps ...Provider) *Registry {
	r := &Registry{providers: map[string]Provider{}}
	for _, p := range ps {
		if p == nil {
			continue
		}
		r.providers[p.ID()] = p
	}
	return r
}

// Get returns the provider with this id.
func (r *Registry) Get(id string) (Provider, error) {
	if r == nil {
		return nil, ErrUnknownProvider
	}
	p, ok := r.providers[strings.ToLower(strings.TrimSpace(id))]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownProvider, id)
	}
	return p, nil
}

// Descriptors lists the configured OAuth-style providers, ordered by id so the
// sign-in screen does not reshuffle between requests.
func (r *Registry) Descriptors() []Descriptor {
	if r == nil {
		return nil
	}
	out := make([]Descriptor, 0, len(r.providers))
	for id, p := range r.providers {
		out = append(out, Descriptor{ID: id, DisplayName: p.DisplayName(), Kind: "oauth"})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Len reports how many providers are configured.
func (r *Registry) Len() int {
	if r == nil {
		return 0
	}
	return len(r.providers)
}
