package auth_test

import (
	"context"
	"net/http"
	"testing"
)

func (h *testHarness) mailCount() int {
	h.mailer.mu.Lock()
	defer h.mailer.mu.Unlock()
	return len(h.mailer.sent)
}

func registerWithEmail(t *testing.T, h *testHarness, username, email, password string) apiResponse {
	t.Helper()
	res := h.do(http.MethodPost, "/auth/register", "", map[string]any{
		"username": username, "email": email, "password": password,
	})
	if res.status != http.StatusOK {
		t.Fatalf("register: status %d body %s", res.status, res.raw)
	}
	return res
}

func TestPasswordResetSetsANewPasswordAndEndsOldSessions(t *testing.T) {
	h := newTestHarness(t)
	reg := registerWithEmail(t, h, "forgetful", "Forgetful@Example.com", "old password")

	start := h.do(http.MethodPost, "/auth/password/forgot", "", map[string]any{"email": "forgetful@example.com"})
	if start.status != http.StatusOK {
		t.Fatalf("forgot: status %d body %s", start.status, start.raw)
	}
	code := h.mailer.lastCodeFor(t, "forgetful@example.com")

	res := h.do(http.MethodPost, "/auth/password/reset", "", map[string]any{
		"email": "forgetful@example.com", "code": code, "password": "new password",
	})
	if res.status != http.StatusOK {
		t.Fatalf("reset: status %d body %s", res.status, res.raw)
	}
	if res.str("userId") != reg.str("userId") {
		t.Fatalf("reset signed in %s, want the account %s", res.str("userId"), reg.str("userId"))
	}

	if old := h.do(http.MethodPost, "/auth/login", "", map[string]any{"username": "forgetful", "password": "old password"}); old.status != http.StatusUnauthorized {
		t.Errorf("the old password still works: status %d", old.status)
	}
	if fresh := h.do(http.MethodPost, "/auth/login", "", map[string]any{"username": "forgetful", "password": "new password"}); fresh.status != http.StatusOK {
		t.Errorf("the new password does not work: status %d body %s", fresh.status, fresh.raw)
	}

	// The session from before the reset is gone.
	if again := h.do(http.MethodPost, "/auth/refresh", "", map[string]any{"refreshToken": reg.str("refreshToken")}); again.status == http.StatusOK {
		t.Error("a session from before the reset still refreshes")
	}

	// The code proved the address, so a later code sign-in finds this account
	// instead of making a second one and taking the address away.
	u, err := h.store.FindUserByID(context.Background(), reg.str("userId"))
	if err != nil {
		t.Fatal(err)
	}
	if !u.EmailVerified {
		t.Error("the reset did not mark the address verified")
	}
	h.do(http.MethodPost, "/auth/email/start", "", map[string]any{"email": "forgetful@example.com"})
	signIn := h.do(http.MethodPost, "/auth/email/verify", "", map[string]any{
		"email": "forgetful@example.com", "code": h.mailer.lastCodeFor(t, "forgetful@example.com"),
	})
	if signIn.str("userId") != reg.str("userId") {
		t.Errorf("code sign-in after the reset landed in %s, want %s", signIn.str("userId"), reg.str("userId"))
	}
}

func TestPasswordResetCodeIsSingleUse(t *testing.T) {
	h := newTestHarness(t)
	registerWithEmail(t, h, "once", "once@example.com", "old password")
	h.do(http.MethodPost, "/auth/password/forgot", "", map[string]any{"email": "once@example.com"})
	code := h.mailer.lastCodeFor(t, "once@example.com")

	first := h.do(http.MethodPost, "/auth/password/reset", "", map[string]any{"email": "once@example.com", "code": code, "password": "first"})
	if first.status != http.StatusOK {
		t.Fatalf("reset: status %d body %s", first.status, first.raw)
	}
	second := h.do(http.MethodPost, "/auth/password/reset", "", map[string]any{"email": "once@example.com", "code": code, "password": "second"})
	if second.status != http.StatusUnauthorized || second.str("code") != "CODE_INVALID" {
		t.Errorf("a spent code reset again: status %d body %s", second.status, second.raw)
	}
}

func TestPasswordResetSaysNothingAboutWhichAddressesAreAccounts(t *testing.T) {
	h := newTestHarness(t)
	registerWithEmail(t, h, "real", "real@example.com", "pw")
	// An account with no password: the email-code sign-in kind.
	h.do(http.MethodPost, "/auth/email/start", "", map[string]any{"email": "codeonly@example.com"})
	h.do(http.MethodPost, "/auth/email/verify", "", map[string]any{
		"email": "codeonly@example.com", "code": h.mailer.lastCodeFor(t, "codeonly@example.com"),
	})

	before := h.mailCount()
	real := h.do(http.MethodPost, "/auth/password/forgot", "", map[string]any{"email": "real@example.com"})
	if h.mailCount() != before+1 {
		t.Fatal("the password account was not mailed a code")
	}
	for _, addr := range []string{"nobody@example.com", "codeonly@example.com"} {
		before := h.mailCount()
		res := h.do(http.MethodPost, "/auth/password/forgot", "", map[string]any{"email": addr})
		if res.status != real.status || res.raw != real.raw {
			t.Errorf("%s: answered %d %s, a real account %d %s", addr, res.status, res.raw, real.status, real.raw)
		}
		if h.mailCount() != before {
			t.Errorf("%s: mail was sent to an address with no password account", addr)
		}
	}
}

func TestSignInAndResetCodesDoNotCrossOver(t *testing.T) {
	h := newTestHarness(t)
	reg := registerWithEmail(t, h, "crossed", "crossed@example.com", "old password")

	// A sign-in code does not reset a password.
	h.do(http.MethodPost, "/auth/email/start", "", map[string]any{"email": "crossed@example.com"})
	signInCode := h.mailer.lastCodeFor(t, "crossed@example.com")
	res := h.do(http.MethodPost, "/auth/password/reset", "", map[string]any{"email": "crossed@example.com", "code": signInCode, "password": "stolen"})
	if res.status != http.StatusUnauthorized {
		t.Errorf("a sign-in code reset the password: status %d", res.status)
	}

	// A reset code does not sign anybody in.
	h.do(http.MethodPost, "/auth/password/forgot", "", map[string]any{"email": "crossed@example.com"})
	resetCode := h.mailer.lastCodeFor(t, "crossed@example.com")
	res = h.do(http.MethodPost, "/auth/email/verify", "", map[string]any{"email": "crossed@example.com", "code": resetCode})
	if res.status != http.StatusUnauthorized {
		t.Errorf("a reset code signed in: status %d body %s", res.status, res.raw)
	}
	if login := h.do(http.MethodPost, "/auth/login", "", map[string]any{"username": "crossed", "password": "old password"}); login.status != http.StatusOK || login.str("userId") != reg.str("userId") {
		t.Errorf("the password changed: status %d", login.status)
	}
}

func TestPasswordResetRefusesAnEmptyPasswordWithoutSpendingTheCode(t *testing.T) {
	h := newTestHarness(t)
	registerWithEmail(t, h, "blank", "blank@example.com", "pw")
	h.do(http.MethodPost, "/auth/password/forgot", "", map[string]any{"email": "blank@example.com"})
	code := h.mailer.lastCodeFor(t, "blank@example.com")

	res := h.do(http.MethodPost, "/auth/password/reset", "", map[string]any{"email": "blank@example.com", "code": code, "password": ""})
	if res.status != http.StatusBadRequest || res.str("code") != "PASSWORD_REQUIRED" {
		t.Fatalf("empty password: status %d body %s", res.status, res.raw)
	}
	if ok := h.do(http.MethodPost, "/auth/password/reset", "", map[string]any{"email": "blank@example.com", "code": code, "password": "real"}); ok.status != http.StatusOK {
		t.Errorf("the code did not survive the refusal: status %d body %s", ok.status, ok.raw)
	}
}
