package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"zolik/server/internal/auth"
	"zolik/server/internal/models"
)

// loginHarness builds a console whose password door is open.
func loginHarness(t *testing.T, adminEmails ...string) (*harness, PasswordLogin) {
	t.Helper()
	login := PasswordLogin{Username: "operator", Hash: hashFor(t, testPassword)}
	return newHarnessWith(t, login, adminEmails...), login
}

func postLogin(t *testing.T, h *harness, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/admin/api/login", strings.NewReader(body))
	req.RemoteAddr = "192.0.2.10:1234"
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

// withToken issues a request bearing a console token.
func withToken(t *testing.T, h *harness, token, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

// The point of the whole feature: a console you can get into with nothing but
// configuration, when mail delivery is not working.
func TestPasswordSignInReachesTheConsole(t *testing.T) {
	h, _ := loginHarness(t)

	rec := postLogin(t, h, `{"username":"operator","password":"`+testPassword+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("sign-in failed: got %d (%s)", rec.Code, rec.Body.String())
	}
	var body struct {
		Token    string `json:"token"`
		Username string `json:"username"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if body.Token == "" {
		t.Fatal("no token was issued")
	}

	got := withToken(t, h, body.Token, "GET", "/admin/api/session")
	if got.Code != http.StatusOK {
		t.Fatalf("the console refused its own token: got %d (%s)", got.Code, got.Body.String())
	}
	session := decode(t, got)
	if session["username"] != "operator" {
		t.Errorf("session username = %v, want operator", session["username"])
	}
	if session["viaPassword"] != true {
		t.Error("the session does not report that it came through the password door")
	}
}

// A password administrator can do the job, not merely see the front page.
func TestPasswordAdminHasFullAccess(t *testing.T) {
	h, login := loginHarness(t)
	token, err := login.mintToken(time.Now().UTC())
	if err != nil {
		t.Fatalf("minting: %v", err)
	}

	for _, path := range []string{"/admin/api/users", "/admin/api/usage", "/admin/api/feedback"} {
		if rec := withToken(t, h, token, "GET", path); rec.Code != http.StatusOK {
			t.Errorf("%s: got %d, want 200", path, rec.Code)
		}
	}
}

func TestPasswordSignInRefusals(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"a wrong password", `{"username":"operator","password":"nope-nope-nope"}`, http.StatusUnauthorized},
		{"a wrong username", `{"username":"someone","password":"` + testPassword + `"}`, http.StatusUnauthorized},
		{"both wrong", `{"username":"someone","password":"nope"}`, http.StatusUnauthorized},
		{"an empty body", `{}`, http.StatusUnauthorized},
		{"malformed json", `{`, http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := loginHarness(t)
			if rec := postLogin(t, h, tc.body); rec.Code != tc.want {
				t.Errorf("got %d, want %d (%s)", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

// A wrong username and a wrong password must be indistinguishable, or the
// endpoint tells an attacker when they have guessed the username.
func TestRefusalDoesNotRevealWhichHalfWasWrong(t *testing.T) {
	h, _ := loginHarness(t)

	wrongPassword := postLogin(t, h, `{"username":"operator","password":"nope-nope-nope"}`)
	wrongUsername := postLogin(t, h, `{"username":"someone","password":"`+testPassword+`"}`)

	if wrongPassword.Body.String() != wrongUsername.Body.String() {
		t.Errorf("the two refusals differ:\n  wrong password: %q\n  wrong username: %q",
			wrongPassword.Body.String(), wrongUsername.Body.String())
	}
}

// Unconfigured means the door does not exist — not that it stands open.
func TestPasswordSignInIsOffUnlessConfigured(t *testing.T) {
	cases := []struct {
		name  string
		login PasswordLogin
	}{
		{"nothing configured", PasswordLogin{}},
		{"a username with no hash", PasswordLogin{Username: "operator"}},
		{"a hash with no username", PasswordLogin{Hash: "$2a$04$abcdefghijklmnopqrstuv"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarnessWith(t, tc.login)
			rec := postLogin(t, h, `{"username":"operator","password":"`+testPassword+`"}`)
			if rec.Code != http.StatusNotFound {
				t.Errorf("got %d, want 404", rec.Code)
			}
		})
	}
}

// A console token is signed with a key derived from the password hash, so
// changing the password must invalidate every token already out there.
func TestChangingThePasswordInvalidatesIssuedTokens(t *testing.T) {
	h, login := loginHarness(t)
	token, err := login.mintToken(time.Now().UTC())
	if err != nil {
		t.Fatalf("minting: %v", err)
	}
	if rec := withToken(t, h, token, "GET", "/admin/api/session"); rec.Code != http.StatusOK {
		t.Fatalf("the token did not work to begin with: got %d", rec.Code)
	}

	rotated := newHarnessWith(t, PasswordLogin{Username: "operator", Hash: hashFor(t, "a-different-password")})
	if rec := withToken(t, rotated, token, "GET", "/admin/api/session"); rec.Code == http.StatusOK {
		t.Error("a token minted under the old password still works")
	}
}

// The console token must not be a player token, in either direction: it is
// signed with a different key precisely so neither can be traded for the other.
func TestConsoleTokensAndPlayerTokensAreNotInterchangeable(t *testing.T) {
	h, login := loginHarness(t, "boss@example.com")

	consoleToken, err := login.mintToken(time.Now().UTC())
	if err != nil {
		t.Fatalf("minting: %v", err)
	}
	// A console token is not a valid player access token.
	if _, err := authParseAccessClaims(consoleToken); err == nil {
		t.Error("a console token parsed as a player access token")
	}
	// And a player token is not a console token — the allow-list is what lets
	// this one in, not the password door.
	if _, err := login.parseToken(playerToken(t, h.admin)); err == nil {
		t.Error("a player access token parsed as a console token")
	}
}

// Guessing has to get expensive, since this password is the console's only key.
func TestSignInAttemptsAreThrottled(t *testing.T) {
	h, _ := loginHarness(t)

	for i := 0; i < loginLimit; i++ {
		if rec := postLogin(t, h, `{"username":"operator","password":"wrong"}`); rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: got %d, want 401", i+1, rec.Code)
		}
	}
	rec := postLogin(t, h, `{"username":"operator","password":"`+testPassword+`"}`)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("past the ceiling: got %d, want 429", rec.Code)
	}
}

// Fumbling the password a few times before getting it right must not leave the
// next sign-in on a short fuse.
func TestASuccessfulSignInClearsTheAttemptCount(t *testing.T) {
	h, _ := loginHarness(t)

	for i := 0; i < loginLimit-1; i++ {
		postLogin(t, h, `{"username":"operator","password":"wrong"}`)
	}
	if rec := postLogin(t, h, `{"username":"operator","password":"`+testPassword+`"}`); rec.Code != http.StatusOK {
		t.Fatalf("the right password was refused: got %d", rec.Code)
	}
	if rec := postLogin(t, h, `{"username":"operator","password":"wrong"}`); rec.Code != http.StatusUnauthorized {
		t.Errorf("after a success the count did not reset: got %d, want 401", rec.Code)
	}
}

// The console reads this before anyone is signed in, so it must be public —
// and must say only whether each door exists.
func TestMethodsIsPublic(t *testing.T) {
	h, _ := loginHarness(t, "boss@example.com")

	req := httptest.NewRequest("GET", "/admin/api/methods", nil)
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	body := decode(t, rec)
	if body["password"] != true || body["email"] != true {
		t.Errorf("methods = %v, want both doors reported open", body)
	}
	if strings.Contains(rec.Body.String(), "operator") {
		t.Error("methods leaked the administrator's username")
	}
}

func TestMethodsReportsWhatIsActuallyConfigured(t *testing.T) {
	h := newHarnessWith(t, PasswordLogin{})
	req := httptest.NewRequest("GET", "/admin/api/methods", nil)
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)

	body := decode(t, rec)
	if body["password"] != false {
		t.Error("an unconfigured password door is reported as available")
	}
}

// A hash that is not a bcrypt hash at all must refuse everything rather than
// accidentally matching.
func TestAMalformedHashAcceptsNothing(t *testing.T) {
	h := newHarnessWith(t, PasswordLogin{Username: "operator", Hash: "not-a-bcrypt-hash"})

	for _, password := range []string{"", "not-a-bcrypt-hash", testPassword} {
		body, _ := json.Marshal(map[string]string{"username": "operator", "password": password})
		if rec := postLogin(t, h, string(body)); rec.Code != http.StatusUnauthorized {
			t.Errorf("password %q: got %d, want 401", password, rec.Code)
		}
	}
}

// Thin wrappers so the interchangeability test reads in terms of the two token
// kinds rather than the packages they happen to come from.
func authParseAccessClaims(token string) (any, error) {
	return auth.ParseAccessClaims(token)
}

func playerToken(t *testing.T, u models.User) string {
	t.Helper()
	token, err := auth.CreateAccessToken(u.ID.Hex(), u.Username, false, time.Minute)
	if err != nil {
		t.Fatalf("minting a player token: %v", err)
	}
	return token
}
