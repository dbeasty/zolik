package auth_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"zolik/server/internal/auth"
	"zolik/server/internal/db"
	"zolik/server/internal/identity"
	"zolik/server/internal/models"
	"zolik/server/internal/stats"
)

// These are end-to-end tests of the HTTP surface in this package, run against
// a real MongoDB rather than a mock — the behaviour that matters here (unique
// indexes resolving a sign-in race, a conditional update making a code
// single-use, an upsert making claiming idempotent) is exactly the behaviour
// a mock would have to fake, which would test the fake rather than the code.
//
// They need a reachable, unauthenticated MongoDB (the docker-compose dev
// stack's default). Point ZOLIK_TEST_MONGO_URI elsewhere, or leave it unset
// to use the standard dev compose mapping (localhost:27018). Any environment
// without one skips these — see newTestHarness.

func testMongoURI() string {
	if v := strings.TrimSpace(mongoURIFromEnv()); v != "" {
		return v
	}
	return "mongodb://127.0.0.1:27018"
}

// testHarness is one isolated deployment of the auth handlers: its own
// throw-away database, its own fake OIDC provider, and a mailer that captures
// what would have been sent instead of sending it.
type testHarness struct {
	t       *testing.T
	server  *httptest.Server
	handler *auth.Handlers
	mongo   *db.Mongo
	stats   *stats.Repository
	mailer  *capturingMailer
	oidc    *fakeOIDC
}

func newTestHarness(t *testing.T) *testHarness {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(testMongoURI()))
	if err != nil {
		t.Skipf("could not build a mongo client: %v", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		t.Skipf("no reachable mongo at %s (set ZOLIK_TEST_MONGO_URI, or start the dev compose stack): %v",
			testMongoURI(), err)
	}

	dbName := fmt.Sprintf("zolik_authtest_%d", time.Now().UnixNano())
	m := &db.Mongo{Client: client, DB: client.Database(dbName)}
	if err := m.EnsureIndexes(ctx); err != nil {
		t.Fatalf("ensuring indexes: %v", err)
	}
	t.Cleanup(func() {
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer dropCancel()
		_ = m.DB.Drop(dropCtx)
		_ = client.Disconnect(dropCtx)
	})

	oidc := newFakeOIDC(t)
	mailer := &capturingMailer{}
	statsRepo := stats.NewRepository(m)

	h := auth.NewHandlers(auth.Deps{
		Mongo:                m,
		Providers:            identity.NewRegistry(oidc.provider(t)),
		Mailer:               mailer,
		Claimer:              stats.NewClaimer(statsRepo),
		PublicBaseURL:        "http://test.local",
		AllowedReturnURLs:    []string{"app://return"},
		AppName:              "TestApp",
		TestEndpointsEnabled: true,
	})

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	return &testHarness{t: t, server: srv, handler: h, mongo: m, stats: statsRepo, mailer: mailer, oidc: oidc}
}

// --- tiny HTTP client helpers ---

type apiResponse struct {
	status int
	body   map[string]any
	raw    string
}

func (h *testHarness) do(method, path, bearer string, body any) apiResponse {
	h.t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			h.t.Fatalf("marshalling request body: %v", err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, h.server.URL+path, reader)
	if err != nil {
		h.t.Fatalf("building request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := h.server.Client().Do(req)
	if err != nil {
		h.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	out := apiResponse{status: resp.StatusCode, raw: string(raw)}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out.body)
	}
	return out
}

// doNoRedirect issues a request through a client that does not follow
// redirects, so the callback's Location header can be inspected directly.
func (h *testHarness) doNoRedirect(method, path string) *http.Response {
	h.t.Helper()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	req, err := http.NewRequest(method, h.server.URL+path, nil)
	if err != nil {
		h.t.Fatalf("building request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		h.t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

func (r apiResponse) str(key string) string {
	v, _ := r.body[key].(string)
	return v
}

func (r apiResponse) num(key string) float64 {
	v, _ := r.body[key].(float64)
	return v
}

func (r apiResponse) boolean(key string) bool {
	v, _ := r.body[key].(bool)
	return v
}

// --- a mailer that captures instead of sending ---

type capturingMailer struct {
	mu   sync.Mutex
	sent []auth.Mail
}

func (m *capturingMailer) Send(_ context.Context, mail auth.Mail) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, mail)
	return nil
}

var sixDigits = regexp.MustCompile(`\b(\d{6})\b`)

// lastCodeFor returns the six-digit code from the most recent mail sent to an
// address, so a test can complete the round trip exactly as the player would
// — by reading their inbox — without reaching into the database.
func (m *capturingMailer) lastCodeFor(t *testing.T, to string) string {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := len(m.sent) - 1; i >= 0; i-- {
		if !strings.EqualFold(m.sent[i].To, to) {
			continue
		}
		match := sixDigits.FindStringSubmatch(m.sent[i].Text)
		if match == nil {
			t.Fatalf("mail to %s carried no six-digit code:\n%s", to, m.sent[i].Text)
		}
		return match[1]
	}
	t.Fatalf("no mail was sent to %s", to)
	return ""
}

func (m *capturingMailer) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sent)
}

// --- a fake OIDC provider, self-contained per test ---

type fakeOIDC struct {
	*httptest.Server
	key     *rsa.PrivateKey
	kid     string
	issuer  string
	idToken string
}

func newFakeOIDC(t *testing.T) *fakeOIDC {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating a test signing key: %v", err)
	}
	f := &fakeOIDC{key: key, kid: "test-key"}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 f.issuer,
			"authorization_endpoint": f.issuer + "/authorize",
			"token_endpoint":         f.issuer + "/token",
			"jwks_uri":               f.issuer + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		pub := f.key.Public().(*rsa.PublicKey)
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "kid": f.kid, "use": "sig", "alg": "RS256",
			"n": base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
		}}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"id_token": f.idToken})
	})
	f.Server = httptest.NewServer(mux)
	f.issuer = f.Server.URL
	t.Cleanup(f.Close)
	return f
}

func (f *fakeOIDC) sign(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	if _, ok := claims["iss"]; !ok {
		claims["iss"] = f.issuer
	}
	if _, ok := claims["aud"]; !ok {
		claims["aud"] = "fake-client"
	}
	if _, ok := claims["exp"]; !ok {
		claims["exp"] = time.Now().Add(time.Hour).Unix()
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = f.kid
	signed, err := tok.SignedString(f.key)
	if err != nil {
		t.Fatalf("signing a test id token: %v", err)
	}
	return signed
}

func (f *fakeOIDC) provider(t *testing.T) identity.Provider {
	t.Helper()
	p := identity.NewOIDCProvider(identity.OIDCConfig{
		ID: "fake", DisplayName: "Fake", Issuer: f.issuer,
		ClientID: "fake-client", HTTPClient: f.Client(),
	})
	if p == nil {
		t.Fatal("fake provider did not configure")
	}
	return p
}

// mongoURIFromEnv keeps the single environment lookup in one place.
func mongoURIFromEnv() string {
	return os.Getenv("ZOLIK_TEST_MONGO_URI")
}

// ---------------------------------------------------------------------------
// providers
// ---------------------------------------------------------------------------

func TestListProvidersReportsGuestEmailAndConfiguredOAuth(t *testing.T) {
	h := newTestHarness(t)
	res := h.do(http.MethodGet, "/auth/providers", "", nil)
	if res.status != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.status, res.raw)
	}
	list, _ := res.body["providers"].([]any)
	ids := map[string]bool{}
	for _, p := range list {
		m := p.(map[string]any)
		ids[m["id"].(string)] = true
	}
	for _, want := range []string{"guest", "email", "fake"} {
		if !ids[want] {
			t.Errorf("providers = %v, missing %q", ids, want)
		}
	}
}

// ---------------------------------------------------------------------------
// guest sessions
// ---------------------------------------------------------------------------

func TestGuestSessionMintsADurableIDAndReuseAccumulatesItsHistory(t *testing.T) {
	h := newTestHarness(t)

	first := h.do(http.MethodPost, "/auth/guest", "", map[string]any{"guestName": "Alice"})
	if first.status != http.StatusOK {
		t.Fatalf("first guest login: status %d body %s", first.status, first.raw)
	}
	guestID := first.str("guestId")
	if len(guestID) != 32 {
		t.Fatalf("guestId = %q, want a 32-character id", guestID)
	}
	if first.num("claimableMatches") != 0 {
		t.Errorf("claimableMatches = %v on a brand-new device, want 0", first.body["claimableMatches"])
	}

	// A finished match lands against this device's guest id, exactly as a
	// real game completing would record it.
	h.seedGuestMatch(guestID)

	second := h.do(http.MethodPost, "/auth/guest", "", map[string]any{
		"guestName": "Alice", "guestId": guestID,
	})
	if second.status != http.StatusOK {
		t.Fatalf("second guest login: status %d body %s", second.status, second.raw)
	}
	if second.str("guestId") != guestID {
		t.Errorf("guestId changed across sessions: %q -> %q; history would become unreachable",
			guestID, second.str("guestId"))
	}
	if second.num("claimableMatches") != 1 {
		t.Errorf("claimableMatches = %v after one finished match, want 1", second.body["claimableMatches"])
	}
}

func TestGuestSessionRejectsAForeignSuppliedID(t *testing.T) {
	// A client-supplied id that is not shaped like one this server mints must
	// not be trusted outright — otherwise the field would let a caller name
	// an arbitrary key that reaches match records and BSON map keys.
	h := newTestHarness(t)
	res := h.do(http.MethodPost, "/auth/guest", "", map[string]any{
		"guestName": "Eve", "guestId": "$where(function(){return true})",
	})
	if res.status != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.status, res.raw)
	}
	if res.str("guestId") == "$where(function(){return true})" {
		t.Fatal("an unsanitised, attacker-supplied guest id was accepted verbatim")
	}
	if len(res.str("guestId")) != 32 {
		t.Errorf("guestId = %q, want a freshly minted 32-character id", res.str("guestId"))
	}
}

// seedGuestMatch inserts a finished match's record directly, standing in for
// a real game completing and stats.Recorder writing it. It records the exact
// shape production writes: a guest subject with this device's id, findable by
// its key.
func (h *testHarness) seedGuestMatch(guestID string) {
	h.t.Helper()
	now := time.Now().UTC()
	guestKey := (stats.Subject{Kind: stats.SubjectGuest, ID: guestID}).Key()
	m := stats.MatchResult{
		GameID:       bson.NewObjectID(),
		RulesProfile: "zolik_classic",
		StartedAt:    now.Add(-time.Hour),
		CompletedAt:  now,
		DealsPlayed:  1,
		Composition:  stats.Composition{Players: 2, Guests: 1, AIs: 1, AIDifficulties: []string{"easy"}},
		Participants: []stats.Standing{
			{PlayerID: "p0", Subject: stats.Subject{Kind: stats.SubjectGuest, ID: guestID, Name: "Guest"}, Name: "Guest", Total: 5, Rank: 1, Won: true},
			{PlayerID: "p1", Subject: stats.Subject{Kind: stats.SubjectAI, ID: "easy", Name: "Bot"}, Name: "Bot", Total: 40, Rank: 2},
		},
		SubjectKeys:    []string{guestKey, "ai:easy"},
		WinnerPlayerID: "p0",
		RecordedAt:     now,
	}
	if _, err := h.stats.InsertMatch(context.Background(), m); err != nil {
		h.t.Fatalf("seeding a guest match: %v", err)
	}
}

// ---------------------------------------------------------------------------
// passwordless email
// ---------------------------------------------------------------------------

func TestEmailSignInEndToEndCreatesAnAccountAndIsStableOnReturn(t *testing.T) {
	h := newTestHarness(t)
	email := "player@example.com"

	if res := h.do(http.MethodPost, "/auth/email/start", "", map[string]any{"email": email}); res.status != http.StatusOK {
		t.Fatalf("email/start: status %d body %s", res.status, res.raw)
	}
	code := h.mailer.lastCodeFor(t, email)

	first := h.do(http.MethodPost, "/auth/email/verify", "", map[string]any{"email": email, "code": code})
	if first.status != http.StatusOK {
		t.Fatalf("first verify: status %d body %s", first.status, first.raw)
	}
	if !first.boolean("created") {
		t.Error("created = false on a brand-new address, want true")
	}
	userID := first.str("userId")
	if userID == "" {
		t.Fatal("no userId in the response")
	}

	// Returning: a fresh code, same address, must land on the same account
	// rather than minting a second one.
	h.do(http.MethodPost, "/auth/email/start", "", map[string]any{"email": email})
	code2 := h.mailer.lastCodeFor(t, email)
	second := h.do(http.MethodPost, "/auth/email/verify", "", map[string]any{"email": email, "code": code2})
	if second.status != http.StatusOK {
		t.Fatalf("second verify: status %d body %s", second.status, second.raw)
	}
	if second.boolean("created") {
		t.Error("created = true on a returning address, want false")
	}
	if second.str("userId") != userID {
		t.Errorf("userId = %s on return, want the same account %s", second.str("userId"), userID)
	}
}

func TestEmailVerifyRejectsAWrongCode(t *testing.T) {
	h := newTestHarness(t)
	email := "wrongcode@example.com"
	h.do(http.MethodPost, "/auth/email/start", "", map[string]any{"email": email})
	h.mailer.lastCodeFor(t, email) // drain — the real code is deliberately not used

	res := h.do(http.MethodPost, "/auth/email/verify", "", map[string]any{"email": email, "code": "000000"})
	if res.status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 for a wrong code", res.status)
	}
}

func TestEmailCodeIsSingleUse(t *testing.T) {
	h := newTestHarness(t)
	email := "singleuse@example.com"
	h.do(http.MethodPost, "/auth/email/start", "", map[string]any{"email": email})
	code := h.mailer.lastCodeFor(t, email)

	first := h.do(http.MethodPost, "/auth/email/verify", "", map[string]any{"email": email, "code": code})
	if first.status != http.StatusOK {
		t.Fatalf("first redemption: status %d body %s", first.status, first.raw)
	}
	second := h.do(http.MethodPost, "/auth/email/verify", "", map[string]any{"email": email, "code": code})
	if second.status != http.StatusUnauthorized {
		t.Fatalf("replaying a spent code: status = %d, want 401", second.status)
	}
}

func TestEmailStartDoesNotRevealWhetherTheAddressHasAnAccount(t *testing.T) {
	h := newTestHarness(t)
	known := "known@example.com"
	unknown := "unknown@example.com"

	h.do(http.MethodPost, "/auth/email/start", "", map[string]any{"email": known})
	h.do(http.MethodPost, "/auth/email/verify", "", map[string]any{
		"email": known, "code": h.mailer.lastCodeFor(t, known),
	})

	knownRes := h.do(http.MethodPost, "/auth/email/start", "", map[string]any{"email": known})
	unknownRes := h.do(http.MethodPost, "/auth/email/start", "", map[string]any{"email": unknown})
	if knownRes.status != unknownRes.status || knownRes.raw != unknownRes.raw {
		t.Errorf("responses differ between a known and unknown address:\n  known:   %d %s\n  unknown: %d %s",
			knownRes.status, knownRes.raw, unknownRes.status, unknownRes.raw)
	}
}

func TestDevLastCodeReturnsWhatWasActuallyMailed(t *testing.T) {
	h := newTestHarness(t)
	email := "dev-endpoint@example.com"
	h.do(http.MethodPost, "/auth/email/start", "", map[string]any{"email": email})
	wantCode := h.mailer.lastCodeFor(t, email)

	res := h.do(http.MethodGet, "/auth/dev/last-code?email="+email, "", nil)
	if res.status != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.status, res.raw)
	}
	if res.str("code") != wantCode {
		t.Errorf("code = %q, want the code actually mailed (%q)", res.str("code"), wantCode)
	}

	// The dev endpoint doesn't consume the code — it's a read of what the
	// mailer sent, not a substitute redemption path. The real code must
	// still work through the real verify endpoint afterwards.
	verify := h.do(http.MethodPost, "/auth/email/verify", "", map[string]any{"email": email, "code": wantCode})
	if verify.status != http.StatusOK {
		t.Fatalf("verify after reading the dev code: status %d body %s", verify.status, verify.raw)
	}
}

func TestDevLastCode404sForAnAddressWithNoMailSent(t *testing.T) {
	h := newTestHarness(t)
	res := h.do(http.MethodGet, "/auth/dev/last-code?email=never-mailed@example.com", "", nil)
	if res.status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for an address nothing was ever sent to", res.status)
	}
}

func TestDevLastCodeIsUnavailableWhenTestEndpointsAreOff(t *testing.T) {
	// Mirrors debugState's own gating test in spirit: a deployment that never
	// opted into test endpoints must not expose a way to read live sign-in
	// codes back out of the server.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client, err := mongo.Connect(options.Client().ApplyURI(testMongoURI()))
	if err != nil {
		t.Skipf("could not build a mongo client: %v", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		t.Skipf("mongo unreachable: %v", err)
	}
	m := &db.Mongo{Client: client, DB: client.Database(fmt.Sprintf("zolik_authtest_gated_%d", time.Now().UnixNano()))}
	if err := m.EnsureIndexes(ctx); err != nil {
		t.Fatalf("ensuring indexes: %v", err)
	}
	t.Cleanup(func() {
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer dropCancel()
		_ = m.DB.Drop(dropCtx)
		_ = client.Disconnect(dropCtx)
	})

	mailer := &capturingMailer{}
	h := auth.NewHandlers(auth.Deps{Mongo: m, Mailer: mailer, AppName: "Gated"})
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	local := &testHarness{t: t, server: srv, mailer: mailer}
	local.do(http.MethodPost, "/auth/email/start", "", map[string]any{"email": "gated@example.com"})

	res := local.do(http.MethodGet, "/auth/dev/last-code?email=gated@example.com", "", nil)
	if res.status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 — the route must not even be reachable", res.status)
	}
}

func TestSigningInAsAGuestClaimsTheirMatchHistory(t *testing.T) {
	h := newTestHarness(t)

	guest := h.do(http.MethodPost, "/auth/guest", "", map[string]any{"guestName": "Bob"})
	guestID := guest.str("guestId")
	guestAccess := guest.str("accessToken")
	h.seedGuestMatch(guestID)
	h.seedGuestMatch(guestID)

	email := "bob@example.com"
	h.do(http.MethodPost, "/auth/email/start", "", map[string]any{"email": email})
	code := h.mailer.lastCodeFor(t, email)

	// The guest's own access token rides along, which is what tells the
	// server to fold this device's history into the new account.
	res := h.do(http.MethodPost, "/auth/email/verify", guestAccess, map[string]any{"email": email, "code": code})
	if res.status != http.StatusOK {
		t.Fatalf("verify with guest claim: status %d body %s", res.status, res.raw)
	}
	if res.num("claimedMatches") != 2 {
		t.Errorf("claimedMatches = %v, want 2", res.body["claimedMatches"])
	}

	userID := res.str("userId")
	ps, err := h.stats.FindPlayerStats(context.Background(), (stats.Subject{Kind: stats.SubjectUser, ID: userID}).Key())
	if err != nil {
		t.Fatalf("the account has no lifetime record after claiming: %v", err)
	}
	if ps.Overall.Matches != 2 {
		t.Errorf("account's lifetime matches = %d, want 2 (rebuilt from the claimed history)", ps.Overall.Matches)
	}
	if ps.Overall.Wins != 2 {
		t.Errorf("account's lifetime wins = %d, want 2", ps.Overall.Wins)
	}
}

// ---------------------------------------------------------------------------
// OAuth: native ID token
// ---------------------------------------------------------------------------

func TestNativeTokenSignInCreatesAnAccountFromClaims(t *testing.T) {
	h := newTestHarness(t)
	raw := h.oidc.sign(t, jwt.MapClaims{
		"sub": "native-subject-1", "email": "native@example.com",
		"email_verified": true, "name": "Native Player",
	})

	res := h.do(http.MethodPost, "/auth/oauth/fake/token", "", map[string]any{"idToken": raw})
	if res.status != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.status, res.raw)
	}
	if res.str("username") != "Native Player" {
		t.Errorf("username = %q, want the name from the token's claims", res.str("username"))
	}
	if !res.boolean("created") {
		t.Error("created = false, want true for a brand-new subject")
	}
	if res.str("accessToken") == "" || res.str("refreshToken") == "" {
		t.Error("a sign-in must return usable tokens")
	}
}

func TestNativeTokenRejectsAForgedSignature(t *testing.T) {
	h := newTestHarness(t)
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating a second key: %v", err)
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": h.oidc.issuer, "aud": "fake-client", "sub": "forged",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	tok.Header["kid"] = h.oidc.kid
	signed, err := tok.SignedString(other)
	if err != nil {
		t.Fatalf("signing: %v", err)
	}

	res := h.do(http.MethodPost, "/auth/oauth/fake/token", "", map[string]any{"idToken": signed})
	if res.status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 for a token signed with the wrong key", res.status)
	}
}

func TestNativeTokenLinksToTheSignedInAccountRatherThanCreatingOne(t *testing.T) {
	h := newTestHarness(t)

	base := h.do(http.MethodPost, "/auth/oauth/fake/token", "", map[string]any{
		"idToken": h.oidc.sign(t, jwt.MapClaims{"sub": "base-account", "email": "base@example.com"}),
	})
	baseUserID := base.str("userId")
	baseAccess := base.str("accessToken")

	link := h.do(http.MethodPost, "/auth/oauth/fake/token", baseAccess, map[string]any{
		"idToken": h.oidc.sign(t, jwt.MapClaims{"sub": "second-identity", "email": "second@example.com"}),
		"link":    true,
	})
	if link.status != http.StatusOK {
		t.Fatalf("linking: status %d body %s", link.status, link.raw)
	}
	if !link.boolean("linked") {
		t.Error("linked = false, want true")
	}
	if link.str("accessToken") != "" {
		t.Error("a pure link response must not mint a second session")
	}
	if link.str("userId") != baseUserID {
		t.Errorf("linked identity's userId = %s, want it to report the same account %s",
			link.str("userId"), baseUserID)
	}

	ids := h.do(http.MethodGet, "/auth/identities", baseAccess, nil)
	list, _ := ids.body["identities"].([]any)
	if len(list) < 2 {
		t.Fatalf("identities = %v, want at least the two just attached", list)
	}
}

func TestLinkingAnIdentityAlreadyLinkedElsewhereIsRefused(t *testing.T) {
	h := newTestHarness(t)

	owner := h.do(http.MethodPost, "/auth/oauth/fake/token", "", map[string]any{
		"idToken": h.oidc.sign(t, jwt.MapClaims{"sub": "contested", "email": "owner@example.com"}),
	})
	other := h.do(http.MethodPost, "/auth/oauth/fake/token", "", map[string]any{
		"idToken": h.oidc.sign(t, jwt.MapClaims{"sub": "someone-else", "email": "other@example.com"}),
	})

	res := h.do(http.MethodPost, "/auth/oauth/fake/token", other.str("accessToken"), map[string]any{
		"idToken": h.oidc.sign(t, jwt.MapClaims{"sub": "contested", "email": "owner@example.com"}),
		"link":    true,
	})
	if res.status != http.StatusConflict {
		t.Fatalf("status = %d, want 409 when linking an identity that already belongs elsewhere", res.status)
	}
	_ = owner
}

// ---------------------------------------------------------------------------
// OAuth: browser redirect flow
// ---------------------------------------------------------------------------

func TestBrowserOAuthFlowEndToEnd(t *testing.T) {
	h := newTestHarness(t)

	start := h.do(http.MethodPost, "/auth/oauth/fake/start", "", map[string]any{"returnTo": "app://return"})
	if start.status != http.StatusOK {
		t.Fatalf("start: status %d body %s", start.status, start.raw)
	}
	authURL, err := url.Parse(start.str("authorizationUrl"))
	if err != nil {
		t.Fatalf("parsing authorizationUrl: %v", err)
	}
	state := authURL.Query().Get("state")
	if state == "" {
		t.Fatal("authorizationUrl carried no state")
	}
	// The nonce is only known once /start has generated it — it rides in the
	// authorization URL exactly as a real client would read it before sending
	// the person to the provider. The provider's ID token has to echo it back
	// or VerifyIDToken correctly refuses the token (see
	// TestVerifyIDTokenRejectsBadTokens in the identity package).
	nonce := authURL.Query().Get("nonce")
	if nonce == "" {
		t.Fatal("authorizationUrl carried no nonce")
	}
	h.oidc.idToken = h.oidc.sign(t, jwt.MapClaims{
		"sub": "browser-subject", "email": "browser@example.com", "name": "Browser Player",
		"nonce": nonce,
	})

	// Simulate the provider's redirect back to our callback.
	resp := h.doNoRedirect(http.MethodGet, "/auth/oauth/fake/callback?state="+url.QueryEscape(state)+"&code=provider-code")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("callback status = %d, want a redirect. body: %s", resp.StatusCode, body)
	}
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("parsing redirect Location: %v", err)
	}
	if got := loc.Scheme + "://" + loc.Host; got != "app://return" {
		t.Errorf("redirected to %q, want the app's returnTo", loc.String())
	}
	exchangeCode := loc.Query().Get("code")
	if exchangeCode == "" {
		t.Fatalf("redirect carried no exchange code: %s", loc.String())
	}

	exchange := h.do(http.MethodPost, "/auth/oauth/exchange", "", map[string]any{"code": exchangeCode})
	if exchange.status != http.StatusOK {
		t.Fatalf("exchange: status %d body %s", exchange.status, exchange.raw)
	}
	if exchange.str("username") != "Browser Player" {
		t.Errorf("username = %q, want Browser Player", exchange.str("username"))
	}
	if exchange.str("accessToken") == "" {
		t.Error("exchange did not return an access token")
	}

	// The exchange code is single-use: a replay must fail rather than hand
	// out a second, indistinguishable session.
	replay := h.do(http.MethodPost, "/auth/oauth/exchange", "", map[string]any{"code": exchangeCode})
	if replay.status == http.StatusOK {
		t.Error("replaying a spent exchange code succeeded")
	}
}

func TestBrowserOAuthCallbackRejectsAnUnknownState(t *testing.T) {
	// No flow was ever started with this state — the CSRF check must refuse
	// it rather than silently completing something.
	h := newTestHarness(t)
	resp := h.doNoRedirect(http.MethodGet, "/auth/oauth/fake/callback?state=never-issued&code=x")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for an unrecognised state", resp.StatusCode)
	}
}

func TestOAuthStartRejectsAnUndeclaredReturnURL(t *testing.T) {
	// This is the open-redirect guard: the callback appends a code that is
	// exchangeable for a session, so returnTo must be on the allow-list.
	h := newTestHarness(t)
	res := h.do(http.MethodPost, "/auth/oauth/fake/start", "", map[string]any{
		"returnTo": "https://attacker.example/steal",
	})
	if res.status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for a returnTo outside the allow-list", res.status)
	}
}

// ---------------------------------------------------------------------------
// claiming, identities, unlinking
// ---------------------------------------------------------------------------

func TestExplicitClaimGuestEndpointMovesHistoryAndRetiresTheGuestSession(t *testing.T) {
	h := newTestHarness(t)

	guest := h.do(http.MethodPost, "/auth/guest", "", map[string]any{"guestName": "Carol"})
	guestID := guest.str("guestId")
	guestRefresh := guest.str("refreshToken")
	h.seedGuestMatch(guestID)

	account := h.do(http.MethodPost, "/auth/oauth/fake/token", "", map[string]any{
		"idToken": h.oidc.sign(t, jwt.MapClaims{"sub": "carol-account", "email": "carol@example.com"}),
	})

	claim := h.do(http.MethodPost, "/auth/claim-guest", account.str("accessToken"),
		map[string]any{"guestRefreshToken": guestRefresh})
	if claim.status != http.StatusOK {
		t.Fatalf("claim: status %d body %s", claim.status, claim.raw)
	}
	if claim.num("claimedMatches") != 1 {
		t.Errorf("claimedMatches = %v, want 1", claim.body["claimedMatches"])
	}

	// The guest session that was just claimed must no longer work — it has
	// nothing left to be a session *for*.
	refreshAfter := h.do(http.MethodPost, "/auth/refresh", "", map[string]any{"refreshToken": guestRefresh})
	if refreshAfter.status != http.StatusUnauthorized {
		t.Errorf("refreshing the claimed guest session: status = %d, want 401", refreshAfter.status)
	}
}

func TestClaimGuestRequiresASignedInAccount(t *testing.T) {
	h := newTestHarness(t)
	guest := h.do(http.MethodPost, "/auth/guest", "", map[string]any{"guestName": "Dana"})

	res := h.do(http.MethodPost, "/auth/claim-guest", guest.str("accessToken"),
		map[string]any{"guestRefreshToken": guest.str("refreshToken")})
	if res.status != http.StatusForbidden {
		t.Fatalf("a guest claiming as itself: status = %d, want 403", res.status)
	}
}

func TestUnlinkingTheLastSignInMethodIsRefused(t *testing.T) {
	h := newTestHarness(t)
	account := h.do(http.MethodPost, "/auth/oauth/fake/token", "", map[string]any{
		"idToken": h.oidc.sign(t, jwt.MapClaims{"sub": "only-method", "email": "only@example.com"}),
	})
	access := account.str("accessToken")

	res := h.do(http.MethodDelete, "/auth/identities/fake", access, nil)
	if res.status != http.StatusConflict {
		t.Fatalf("status = %d, want 409 — this is the account's only way back in", res.status)
	}

	// It really is still there afterwards.
	ids := h.do(http.MethodGet, "/auth/identities", access, nil)
	list, _ := ids.body["identities"].([]any)
	if len(list) != 1 {
		t.Fatalf("identities = %v, want the refused unlink to have changed nothing", list)
	}
}

func TestUnlinkingAfterAddingASecondMethodSucceeds(t *testing.T) {
	h := newTestHarness(t)
	account := h.do(http.MethodPost, "/auth/oauth/fake/token", "", map[string]any{
		"idToken": h.oidc.sign(t, jwt.MapClaims{"sub": "first-method", "email": "first@example.com"}),
	})
	access := account.str("accessToken")

	h.do(http.MethodPost, "/auth/email/start", "", map[string]any{"email": "second@example.com"})
	code := h.mailer.lastCodeFor(t, "second@example.com")
	h.do(http.MethodPost, "/auth/email/verify", access, map[string]any{
		"email": "second@example.com", "code": code,
	})

	res := h.do(http.MethodDelete, "/auth/identities/fake", access, nil)
	if res.status != http.StatusOK {
		t.Fatalf("unlink with a second method present: status %d body %s", res.status, res.raw)
	}
}

// ---------------------------------------------------------------------------
// legacy username/password (kept for the SSH/TUI client)
// ---------------------------------------------------------------------------

func TestLegacyRegisterAndLoginRoundTrip(t *testing.T) {
	h := newTestHarness(t)
	reg := h.do(http.MethodPost, "/auth/register", "", map[string]any{
		"username": "legacyplayer", "password": "correct horse battery staple",
	})
	if reg.status != http.StatusOK {
		t.Fatalf("register: status %d body %s", reg.status, reg.raw)
	}

	login := h.do(http.MethodPost, "/auth/login", "", map[string]any{
		"username": "legacyplayer", "password": "correct horse battery staple",
	})
	if login.status != http.StatusOK {
		t.Fatalf("login: status %d body %s", login.status, login.raw)
	}
	if login.str("userId") != reg.str("userId") {
		t.Errorf("login userId = %s, want the registered account %s", login.str("userId"), reg.str("userId"))
	}

	bad := h.do(http.MethodPost, "/auth/login", "", map[string]any{
		"username": "legacyplayer", "password": "wrong",
	})
	if bad.status != http.StatusUnauthorized {
		t.Errorf("wrong password: status = %d, want 401", bad.status)
	}
}

func TestLegacyRegisterRejectsADuplicateUsername(t *testing.T) {
	h := newTestHarness(t)
	h.do(http.MethodPost, "/auth/register", "", map[string]any{"username": "dupe", "password": "aaaaaaaa"})
	res := h.do(http.MethodPost, "/auth/register", "", map[string]any{"username": "dupe", "password": "bbbbbbbb"})
	if res.status == http.StatusOK {
		t.Fatal("registering an already-taken username succeeded")
	}
}

// ---------------------------------------------------------------------------
// session lifecycle
// ---------------------------------------------------------------------------

func TestRefreshMigratesAPreExistingGuestSessionOntoADurableID(t *testing.T) {
	// A session written before guest ids existed carries neither a UserID nor
	// a GuestID. Refreshing it must not treat that as an error: it should
	// mint a guest id on the spot so the session gains a durable identity
	// rather than staying unattributable forever, and keep it stable on every
	// refresh after that.
	h := newTestHarness(t)
	ctx := context.Background()
	legacyToken := "legacy-refresh-token-0123456789abcdef"
	_, err := h.mongo.Collections().Sessions.InsertOne(ctx, models.Session{
		Token:     legacyToken,
		GuestName: "OldGuest",
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("seeding a legacy guest session: %v", err)
	}

	first := h.do(http.MethodPost, "/auth/refresh", "", map[string]any{"refreshToken": legacyToken})
	if first.status != http.StatusOK {
		t.Fatalf("first refresh: status %d body %s", first.status, first.raw)
	}
	if !first.boolean("isGuest") {
		t.Error("isGuest = false for a legacy guest session")
	}
	mintedID := first.str("userId")
	if len(mintedID) != 32 {
		t.Fatalf("userId = %q, want a freshly minted 32-character guest id", mintedID)
	}

	second := h.do(http.MethodPost, "/auth/refresh", "", map[string]any{
		"refreshToken": first.str("refreshToken"),
	})
	if second.status != http.StatusOK {
		t.Fatalf("second refresh: status %d body %s", second.status, second.raw)
	}
	if second.str("userId") != mintedID {
		t.Errorf("userId = %s on the next refresh, want the same minted id %s", second.str("userId"), mintedID)
	}
}

func TestRefreshRotatesTheTokenAndRetiresTheOldOne(t *testing.T) {
	h := newTestHarness(t)
	guest := h.do(http.MethodPost, "/auth/guest", "", map[string]any{"guestName": "Rotator"})
	oldRefresh := guest.str("refreshToken")

	rotated := h.do(http.MethodPost, "/auth/refresh", "", map[string]any{"refreshToken": oldRefresh})
	if rotated.status != http.StatusOK {
		t.Fatalf("refresh: status %d body %s", rotated.status, rotated.raw)
	}
	if rotated.str("refreshToken") == oldRefresh {
		t.Error("the refresh token did not rotate")
	}

	reuse := h.do(http.MethodPost, "/auth/refresh", "", map[string]any{"refreshToken": oldRefresh})
	if reuse.status != http.StatusUnauthorized {
		t.Errorf("reusing a rotated-away refresh token: status = %d, want 401", reuse.status)
	}
}

func TestLogoutInvalidatesTheRefreshToken(t *testing.T) {
	h := newTestHarness(t)
	guest := h.do(http.MethodPost, "/auth/guest", "", map[string]any{"guestName": "Logout"})
	refresh := guest.str("refreshToken")

	if res := h.do(http.MethodPost, "/auth/logout", "", map[string]any{"refreshToken": refresh}); res.status != http.StatusOK {
		t.Fatalf("logout: status %d body %s", res.status, res.raw)
	}
	after := h.do(http.MethodPost, "/auth/refresh", "", map[string]any{"refreshToken": refresh})
	if after.status != http.StatusUnauthorized {
		t.Errorf("refreshing after logout: status = %d, want 401", after.status)
	}
}

// ---------------------------------------------------------------------------
// concurrency: the property the unique index exists to guarantee
// ---------------------------------------------------------------------------

func TestConcurrentFirstSignInsForTheSameIdentityResolveToOneAccount(t *testing.T) {
	// Two requests racing to sign in as a brand-new identity for the first
	// time must not create two accounts — the loser has to detect the
	// collision and land on the winner's account instead. This is
	// auth.Accounts.resolve's retry path, and it is only exercisable under
	// genuine concurrency: a sequential test cannot provoke it.
	h := newTestHarness(t)
	raw := h.oidc.sign(t, jwt.MapClaims{"sub": "race-subject", "email": "race@example.com"})

	const n = 8
	results := make([]apiResponse, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			results[i] = h.do(http.MethodPost, "/auth/oauth/fake/token", "", map[string]any{"idToken": raw})
		}(i)
	}
	wg.Wait()

	seen := map[string]bool{}
	for i, res := range results {
		if res.status != http.StatusOK {
			t.Fatalf("attempt %d: status %d body %s", i, res.status, res.raw)
		}
		seen[res.str("userId")] = true
	}
	if len(seen) != 1 {
		t.Fatalf("%d concurrent sign-ins for one identity produced %d distinct accounts: %v",
			n, len(seen), seen)
	}
}
