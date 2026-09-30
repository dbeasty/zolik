package admin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/crypto/bcrypt"

	"zolik/server/internal/auth"
	"zolik/server/internal/metrics"
	"zolik/server/internal/models"
)

type harness struct {
	router  http.Handler
	users   *fakeUsers
	live    *fakeLive
	reports *fakeReports
	counts  *countingSink
	admin   models.User
	other   models.User
}

// testPassword is what the harness's password login accepts. A fixture, not a
// credential: it guards nothing outside this file.
const testPassword = "console-password-1"

// hashFor bcrypts at the minimum cost, because these tests hash on nearly
// every case and cost 12 would make the package take minutes.
func hashFor(t *testing.T, password string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hashing: %v", err)
	}
	return string(h)
}

func newHarness(t *testing.T, adminEmails ...string) (*harness, PasswordLogin) {
	t.Helper()
	login := PasswordLogin{Username: "operator", Hash: hashFor(t, testPassword)}
	return newHarnessWith(t, login, adminEmails...), login
}

func newHarnessWith(t *testing.T, passwordLogin PasswordLogin, adminEmails ...string) *harness {
	t.Helper()
	t.Setenv("JWT_ACCESS_SECRET", "test_secret")

	adminUser := models.User{
		ID: bson.NewObjectID(), Username: "boss",
		Email: "boss@example.com", EmailVerified: true,
	}
	other := models.User{
		ID: bson.NewObjectID(), Username: "player",
		Email: "player@example.com", EmailVerified: true,
	}
	users := &fakeUsers{byID: map[string]models.User{
		adminUser.ID.Hex(): adminUser,
		other.ID.Hex():     other,
	}}

	guard := NewGuard(users, adminEmails, passwordLogin)
	counts := &countingSink{counts: map[string]int64{}}
	guard.SetMetrics(counts)

	live := &fakeLive{}
	reports := &fakeReports{}

	r := chi.NewRouter()
	NewHandlers(Deps{
		Guard:         guard,
		Reports:       reports,
		Live:          live,
		WaitingRoomID: "waiting",
		Version:       "test",
	}).RegisterRoutes(r)

	return &harness{
		router: r, users: users, live: live, reports: reports,
		counts: counts, admin: adminUser, other: other,
	}
}

// as issues a request carrying a player access token for u — the allow-list
// door.
func (h *harness) as(t *testing.T, u models.User, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	token, err := auth.CreateAccessToken(u.ID.Hex(), u.Username, false, time.Minute)
	if err != nil {
		t.Fatalf("minting a token: %v", err)
	}
	return h.withToken(t, token, method, path, body)
}

// withToken issues a request carrying an arbitrary bearer token.
func (h *harness) withToken(t *testing.T, token, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decoding %q: %v", rec.Body.String(), err)
	}
	return out
}

/* ------------------------------------------------------------------- fakes */

type fakeUsers struct{ byID map[string]models.User }

func (f *fakeUsers) FindByID(_ context.Context, id bson.ObjectID) (models.User, error) {
	u, ok := f.byID[id.Hex()]
	if !ok {
		return models.User{}, context.Canceled
	}
	return u, nil
}

type fakeLive struct {
	rooms, conns int
	waiting      int
}

func (f *fakeLive) Totals() (int, int)   { return f.rooms, f.conns }
func (f *fakeLive) CountRoom(string) int { return f.waiting }

type fakeReports struct {
	report metrics.Report
	err    error
	// last records what the handler asked for, so the tests can assert on the
	// defaults it chose.
	lastFrom, lastTo time.Time
	lastBucket       metrics.Bucket
}

func (f *fakeReports) Build(_ context.Context, from, to time.Time, bucket metrics.Bucket) (metrics.Report, error) {
	f.lastFrom, f.lastTo, f.lastBucket = from, to, bucket
	if f.err != nil {
		return metrics.Report{}, f.err
	}
	return f.report, nil
}

type countingSink struct{ counts map[string]int64 }

func (c *countingSink) Add(name string, n int64) { c.counts[name] += n }
func (c *countingSink) SeePlayer(string)         {}
