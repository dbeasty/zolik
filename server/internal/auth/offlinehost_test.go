package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"

	"zolik/server/internal/models"
)

// memSessions is a SessionRepository in a map. An offline host's sessions are
// the one part of this flow that has to be stored, and storing them in a
// database here would test the database rather than the seating.
type memSessions struct {
	mu       sync.Mutex
	sessions map[string]models.Session
}

func newMemSessions() *memSessions {
	return &memSessions{sessions: map[string]models.Session{}}
}

var _ SessionRepository = (*memSessions)(nil)

func (m *memSessions) CreateGuestSession(ctx context.Context, token, guestName string, ttl time.Duration) error {
	now := time.Now().UTC()
	return m.CreateSession(ctx, models.Session{Token: token, GuestName: guestName, CreatedAt: now, ExpiresAt: now.Add(ttl)})
}

func (m *memSessions) CreateSession(_ context.Context, s models.Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[s.Token] = s
	return nil
}

func (m *memSessions) FindByToken(_ context.Context, token string) (models.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[token]
	if !ok {
		return models.Session{}, ErrNotFound
	}
	return s, nil
}

func (m *memSessions) SetGuestID(_ context.Context, token, guestID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sessions[token]
	s.GuestID = guestID
	m.sessions[token] = s
	return nil
}

func (m *memSessions) Retire(_ context.Context, token, replacedBy string, until time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sessions[token]
	s.ReplacedBy, s.ExpiresAt = replacedBy, until
	m.sessions[token] = s
	return nil
}

func (m *memSessions) DeleteByToken(_ context.Context, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, token)
	return nil
}

// cachedCloudKeys is the copy of the cloud's keys a phone holds: taken while
// the process was still the cloud, and read back afterwards from a document
// rather than from the keyring, which is what an offline host actually has.
func cachedCloudKeys(t *testing.T) PublicKeys {
	t.Helper()
	raw, err := json.Marshal(LocalJWKS())
	if err != nil {
		t.Fatal(err)
	}
	return NewJWKSCache(JWKSCacheConfig{Path: filepath.Join(t.TempDir(), "jwks.json"), Bundled: raw})
}

// offlineHost is a phone hosting a table: the local routes, a node key of its
// own, and a copy of the cloud's keys but no cloud.
type offlineHost struct {
	t        *testing.T
	router   chi.Router
	sessions *memSessions
}

func newOfflineHost(t *testing.T, keys PublicKeys) *offlineHost {
	t.Helper()
	sessions := newMemSessions()
	h := NewHandlers(Deps{Sessions: sessions, OfflineKeys: keys})
	r := chi.NewRouter()
	h.RegisterLocalRoutes(r)
	return &offlineHost{t: t, router: r, sessions: sessions}
}

func (o *offlineHost) post(path string, body map[string]any) *httptest.ResponseRecorder {
	o.t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		o.t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	o.router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw)))
	return rec
}

// The whole of item three, from the cloud's signature to a seat: a phone with
// no internet seats a real account as user:<hex>, on the strength of a pass
// and the keys it cached earlier.
func TestOfflineHostSeatsAnAccountFromAPass(t *testing.T) {
	newSigningKey(t, "https://zolik.test")
	pass, err := CreateOfflinePass(testUserID, "alice", 1, OfflinePassTTL)
	if err != nil {
		t.Fatal(err)
	}
	cloudKeys := cachedCloudKeys(t)

	// From here on this process is the phone, not the cloud: it holds a node
	// key and no cloud key at all, exactly as an embedded host does.
	resetKeys(t)
	newNodeKey(t)
	host := newOfflineHost(t, cloudKeys)

	rec := host.post("/auth/offline-pass", map[string]any{"offlinePass": pass})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		UserID       string `json:"userId"`
		Username     string `json:"username"`
		IsGuest      bool   `json:"isGuest"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.UserID != "user:"+testUserID {
		t.Errorf("seated as %q, want user:<hex>", out.UserID)
	}
	if out.IsGuest {
		t.Error("the account was seated as a guest")
	}
	if out.Username != "alice" {
		t.Errorf("username %q, want alice", out.Username)
	}

	// The access token is this host's own, signed with its node key, and it
	// names the account rather than a guest id.
	claims, err := ParseAccessClaims(out.AccessToken)
	if err != nil {
		t.Fatalf("the host cannot read the token it just issued: %v", err)
	}
	if claims.Subject != "user:"+testUserID || claims.IsGuest {
		t.Fatalf("token seats %q (guest=%v)", claims.Subject, claims.IsGuest)
	}

	// The local session may not outlive the pass that justified it.
	session, err := host.sessions.FindByToken(context.Background(), out.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if session.UserID != "user:"+testUserID {
		t.Errorf("session records %q", session.UserID)
	}
	if session.ExpiresAt.After(time.Now().Add(OfflinePassTTL + time.Minute)) {
		t.Errorf("the session outlives the pass: expires %v", session.ExpiresAt)
	}
}

func TestOfflineHostRefusesAPassItCannotCheck(t *testing.T) {
	newSigningKey(t, "https://zolik.test")
	signer := activeSigner()
	now := time.Now().UTC()

	expired, err := signEdDSA(signer, OfflinePassClaims{
		Username: "alice",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   testUserID,
			Audience:  jwt.ClaimStrings{AudienceOffline},
			ExpiresAt: jwt.NewNumericDate(now.Add(-time.Hour)),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	wrongAudience, err := CreateAccessToken(testUserID, "alice", false, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	cloudKeys := cachedCloudKeys(t)

	resetKeys(t)
	newNodeKey(t)

	for name, pass := range map[string]string{
		"expired":        expired,
		"wrong audience": wrongAudience,
		"nonsense":       "not.a.token",
		"empty":          "",
	} {
		t.Run(name, func(t *testing.T) {
			host := newOfflineHost(t, cloudKeys)
			rec := host.post("/auth/offline-pass", map[string]any{"offlinePass": pass})
			if rec.Code == http.StatusOK {
				t.Fatalf("a %s pass seated a player", name)
			}
			if len(host.sessions.sessions) != 0 {
				t.Fatalf("a %s pass created a session anyway", name)
			}
		})
	}
}

// A host that has never had the cloud's keys cannot check anything, and says
// so rather than seating whoever asks.
func TestOfflineHostWithNoKeysSeatsNobody(t *testing.T) {
	newSigningKey(t, "https://zolik.test")
	pass, err := CreateOfflinePass(testUserID, "alice", 1, OfflinePassTTL)
	if err != nil {
		t.Fatal(err)
	}

	resetKeys(t)
	newNodeKey(t)
	host := newOfflineHost(t, nil)

	if rec := host.post("/auth/offline-pass", map[string]any{"offlinePass": pass}); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d, want 503", rec.Code)
	}
}

// A guest seated by a host with a node key leaves with a receipt, which is
// what makes the seat reconcilable with an account later.
func TestGuestSeatComesWithAReceipt(t *testing.T) {
	resetKeys(t)
	pub := newNodeKey(t)
	host := newOfflineHost(t, nil)

	rec := host.post("/auth/guest", map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		GuestID     string `json:"guestId"`
		SeatReceipt string `json:"seatReceipt"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.SeatReceipt == "" {
		t.Fatal("a host with a node key handed out no receipt")
	}
	claims, err := VerifySeatReceipt(out.SeatReceipt, pub)
	if err != nil {
		t.Fatalf("the receipt does not verify against the node that issued it: %v", err)
	}
	if claims.GuestID != out.GuestID {
		t.Fatalf("the receipt is for %q, the guest is %q", claims.GuestID, out.GuestID)
	}
}

// A host with no node key hands out none, and the seat is exactly what it was
// before receipts existed.
func TestGuestSeatWithoutANodeKeyHasNoReceipt(t *testing.T) {
	t.Setenv("JWT_ACCESS_SECRET", "test_secret")
	resetKeys(t)
	host := newOfflineHost(t, nil)

	rec := host.post("/auth/guest", map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if _, ok := decodeJSON(t, rec)["seatReceipt"]; ok {
		t.Fatal("a host with no node key claimed to have signed a receipt")
	}
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// Refreshing an offline seat keeps it, and does not extend it: the pass is
// what bounds the seat, and a host that rolled it forward every fifteen
// minutes would have turned a thirty day credential into a permanent one.
func TestOfflineSeatIsNotExtendedByRefreshing(t *testing.T) {
	newSigningKey(t, "https://zolik.test")
	pass, err := CreateOfflinePass(testUserID, "alice", 1, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	cloudKeys := cachedCloudKeys(t)

	resetKeys(t)
	newNodeKey(t)
	host := newOfflineHost(t, cloudKeys)

	seated := decodeJSON(t, host.post("/auth/offline-pass", map[string]any{"offlinePass": pass}))
	first, _ := seated["refreshToken"].(string)
	if first == "" {
		t.Fatalf("no session: %v", seated)
	}
	before, err := host.sessions.FindByToken(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}

	refreshed := decodeJSON(t, host.post("/auth/refresh", map[string]any{"refreshToken": first}))
	next, _ := refreshed["refreshToken"].(string)
	if next == "" {
		t.Fatalf("the seat could not be refreshed: %v", refreshed)
	}
	if subject, _ := refreshed["userId"].(string); subject != "user:"+testUserID {
		t.Errorf("refreshed as %q, want the seated account", subject)
	}
	after, err := host.sessions.FindByToken(context.Background(), next)
	if err != nil {
		t.Fatal(err)
	}
	if after.ExpiresAt.After(before.ExpiresAt) {
		t.Fatalf("the seat now expires at %v, having expired at %v", after.ExpiresAt, before.ExpiresAt)
	}
}
