package auth

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/models"
)

// Password reset for username/password accounts that gave an address.
//
// The legacy register form takes an optional email, and until this existed an
// account whose owner forgot the password was simply lost. The reset is built
// from the same one-time code as email sign-in — same TTL, same guess cap,
// same per-address throttle — but issued for its own purpose, so a reset code
// cannot be spent as a sign-in code, nor a sign-in code as a reset.
//
// Redeeming a reset code also proves the address, so the account's email is
// marked verified. That matters beyond tidiness: an unverified address is
// released to whoever next proves it, so without this the owner's own later
// email sign-in would make them a second account and take the address off
// this one.

// StartPasswordReset mails a reset code to an address if a password account
// holds it.
//
// It reports the same success whether or not one does, for the reason
// StartSignIn does: anything else answers "is this address an account here?"
// for anybody who asks. An address with no password account gets no mail at
// all, so the endpoint cannot be used to send mail to strangers.
func (e *EmailAuth) StartPasswordReset(ctx context.Context, email, requestIP string) error {
	email = NormalizeEmail(email)
	if !ValidEmail(email) {
		return ErrInvalidEmail
	}
	u, err := e.store.FindUserByEmail(ctx, email)
	if errors.Is(err, ErrNotFound) || (err == nil && u.PasswordHash == "") {
		return nil
	}
	if err != nil {
		return err
	}

	code, err := e.issueCode(ctx, email, requestIP, models.LoginCodePurposePasswordReset)
	if err != nil {
		return err
	}
	return e.mailer.Send(ctx, Mail{
		To:      email,
		Subject: fmt.Sprintf("%s password reset code: %s", e.appName, code),
		Text: fmt.Sprintf(
			"Your %s password reset code for the account %q is:\n\n    %s\n\n"+
				"It expires in %d minutes and can be used once.\n\n"+
				"If you did not ask to reset your password, you can ignore this "+
				"message — your password stays as it is.\n",
			e.appName, u.Username, code, int(codeTTL.Minutes())),
	})
}

// CompletePasswordReset redeems a reset code and sets the account's new
// password, returning the account.
//
// The code is checked before the account is looked up again, so a wrong code
// and an address with no account fail identically, with ErrInvalidCode.
func (e *EmailAuth) CompletePasswordReset(ctx context.Context, email, code, newPassword string) (models.User, error) {
	email = NormalizeEmail(email)
	if err := e.redeemCode(ctx, email, code, models.LoginCodePurposePasswordReset); err != nil {
		return models.User{}, err
	}
	// Looked up again rather than trusted from the request: the address could
	// have changed hands in the ten minutes the code was live.
	u, err := e.store.FindUserByEmail(ctx, email)
	if errors.Is(err, ErrNotFound) || (err == nil && u.PasswordHash == "") {
		return models.User{}, ErrInvalidCode
	}
	if err != nil {
		return models.User{}, err
	}

	hash, err := hashPassword(newPassword, 12)
	if err != nil {
		return models.User{}, err
	}
	if err := e.store.UpdateUser(ctx, u.ID, bson.M{
		"passwordHash":  hash,
		"emailVerified": true,
	}); err != nil {
		return models.User{}, err
	}
	u.PasswordHash = hash
	u.EmailVerified = true
	return u, nil
}
