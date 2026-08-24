package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/mail"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"zolik/server/internal/identity"
	"zolik/server/internal/models"
)

// Passwordless email sign-in.
//
// The person types an address, we mail a six-digit code, they type it back.
// There is no password, so there is nothing to forget, reset, reuse across
// sites, or leak — which removes the entire password-reset flow along with the
// class of bugs that lives in it. The code is the credential, and it is
// deliberately short-lived, single-use, rate-limited and rate-capped on
// guesses, because a six-digit secret is only safe when all four hold.
const (
	// codeTTL is how long a mailed code stays valid. Long enough to switch
	// apps and find the mail, short enough that a code left in an inbox is not
	// a standing key to the account.
	codeTTL = 10 * time.Minute
	// maxCodeAttempts is how many wrong guesses burn a code. With a million
	// possibilities and five tries, guessing succeeds about once every 200,000
	// attempts — and each attempt needs a fresh code request, which is itself
	// throttled.
	maxCodeAttempts = 5
	// maxCodesPerWindow throttles requests per address, which is what stops
	// this endpoint being used to mail-bomb somebody else's inbox.
	maxCodesPerWindow = 5
	codeRateWindow    = 15 * time.Minute
)

// ErrTooManyCodes is returned when an address has requested too many codes.
var ErrTooManyCodes = errors.New("too many sign-in codes requested for this address")

// ErrInvalidCode covers every failed redemption — wrong, expired, already
// used, or never issued.
//
// One error for all four is intentional: distinguishing them tells an attacker
// which addresses have codes outstanding, and tells them when to stop
// guessing. The person who genuinely mistyped a digit is told to request a new
// code, which is the right advice in all four cases anyway.
var ErrInvalidCode = errors.New("that code is not valid — request a new one")

// EmailAuth issues and redeems one-time sign-in codes.
type EmailAuth struct {
	store  *Store
	mailer Mailer
	// appName is the product name used in the mail body.
	appName string
}

func NewEmailAuth(store *Store, mailer Mailer, appName string) *EmailAuth {
	if appName == "" {
		appName = "Žolíky"
	}
	return &EmailAuth{store: store, mailer: mailer, appName: appName}
}

// StartSignIn mails a fresh code to an address.
//
// It reports nothing about whether the address has an account here. Answering
// that question would turn this endpoint into a membership oracle that anyone
// can query, so a first-time address and a returning player get identical
// responses, and the account is created later — at redemption — if needed.
func (e *EmailAuth) StartSignIn(ctx context.Context, email, requestIP string) error {
	email = NormalizeEmail(email)
	if !ValidEmail(email) {
		return errors.New("that does not look like an email address")
	}

	recent, err := e.store.CountRecentLoginCodes(ctx, email, time.Now().UTC().Add(-codeRateWindow))
	if err != nil {
		return err
	}
	if recent >= maxCodesPerWindow {
		return ErrTooManyCodes
	}

	code, err := generateCode()
	if err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	if err := e.store.InsertLoginCode(ctx, models.LoginCode{
		Email:     email,
		CodeHash:  string(hash),
		CreatedAt: now,
		ExpiresAt: now.Add(codeTTL),
		RequestIP: requestIP,
	}); err != nil {
		return err
	}

	return e.mailer.Send(ctx, Mail{
		To:      email,
		Subject: fmt.Sprintf("%s sign-in code: %s", e.appName, code),
		Text: fmt.Sprintf(
			"Your %s sign-in code is:\n\n    %s\n\n"+
				"It expires in %d minutes and can be used once.\n\n"+
				"If you did not ask to sign in, you can ignore this message — "+
				"nobody can use this code without it.\n",
			e.appName, code, int(codeTTL.Minutes())),
	})
}

// VerifyCode redeems a code and returns the identity claims it proves.
//
// A successful redemption proves exactly one thing — that whoever typed the
// code can read mail at that address — which is precisely what an identity
// provider asserts, so the result is the same identity.Claims a Google sign-in
// produces and flows into the same Accounts.SignIn. Email is not a special
// case anywhere past this function.
func (e *EmailAuth) VerifyCode(ctx context.Context, email, code string) (identity.Claims, error) {
	email = NormalizeEmail(email)
	code = strings.TrimSpace(code)
	if email == "" || code == "" {
		return identity.Claims{}, ErrInvalidCode
	}

	rec, err := e.store.LatestLoginCode(ctx, email)
	if err != nil {
		// Includes "no code was ever requested for this address".
		return identity.Claims{}, ErrInvalidCode
	}
	if rec.ConsumedAt != nil || rec.Attempts >= maxCodeAttempts {
		return identity.Claims{}, ErrInvalidCode
	}

	if err := bcrypt.CompareHashAndPassword([]byte(rec.CodeHash), []byte(code)); err != nil {
		// Count the miss before answering, so a burst of parallel guesses is
		// still counted even though each one is told the same thing.
		_ = e.store.RecordCodeAttempt(ctx, rec.ID)
		return identity.Claims{}, ErrInvalidCode
	}
	// Consuming is conditional on the code still being unspent, so two
	// simultaneous correct submissions produce one sign-in, not two.
	if err := e.store.ConsumeLoginCode(ctx, rec.ID); err != nil {
		return identity.Claims{}, ErrInvalidCode
	}

	return identity.Claims{
		Provider: models.IdentityProviderEmail,
		// The address is the subject: for this provider it *is* the stable id,
		// and it is the only thing the code proved.
		Subject:       email,
		Email:         email,
		EmailVerified: true,
	}, nil
}

// generateCode returns a uniformly random six-digit code, zero-padded.
//
// crypto/rand rather than math/rand, and rejection-free modulo-safe sampling
// via Int: this is a credential, so a predictable or skewed generator is a
// vulnerability rather than an inelegance.
func generateCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// ValidEmail applies a deliberately loose check: a parseable address with a
// dot-bearing domain.
//
// Stricter validation rejects real addresses, and the only test that actually
// matters is whether a code sent there arrives — which the flow performs
// anyway. This exists to catch typos and to keep obvious junk out of the
// mailer, not to adjudicate RFC 5322.
func ValidEmail(email string) bool {
	if len(email) > 254 || strings.ContainsAny(email, " \r\n\t") {
		return false
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return false
	}
	_, domain, ok := strings.Cut(email, "@")
	return ok && strings.Contains(domain, ".") && !strings.HasSuffix(domain, ".")
}
