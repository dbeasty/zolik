package admin

import (
	"context"
	"encoding/json"
	"errors"
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
	"zolik/server/internal/feedback"
	"zolik/server/internal/models"
	"zolik/server/internal/stats"
	userrepo "zolik/server/internal/user"
)

/* ------------------------------------------------------------------- fakes */

type fakeUsers struct {
	byID    map[string]models.User
	updates map[string]bson.M
	deleted []string
}

func (f *fakeUsers) FindByID(_ context.Context, id bson.ObjectID) (models.User, error) {
	u, ok := f.byID[id.Hex()]
	if !ok {
		return models.User{}, errors.New("not found")
	}
	return u, nil
}

func (f *fakeUsers) UpdateByID(_ context.Context, id bson.ObjectID, update bson.M) error {
	if f.updates == nil {
		f.updates = map[string]bson.M{}
	}
	f.updates[id.Hex()] = update
	return nil
}

func (f *fakeUsers) DeleteByID(_ context.Context, id bson.ObjectID) error {
	f.deleted = append(f.deleted, id.Hex())
	delete(f.byID, id.Hex())
	return nil
}

func (f *fakeUsers) ListUsers(_ context.Context, _ userrepo.Query) ([]models.User, error) {
	out := []models.User{}
	for _, u := range f.byID {
		out = append(out, u)
	}
	return out, nil
}

func (f *fakeUsers) CountUsers(_ context.Context, _ userrepo.Query) (int64, error) {
	return int64(len(f.byID)), nil
}

func (f *fakeUsers) CountUsersSeenSince(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}

type fakeIdentities struct {
	list    []models.Identity
	cleared []string
}

func (f *fakeIdentities) ListIdentities(_ context.Context, _ string) ([]models.Identity, error) {
	return f.list, nil
}

func (f *fakeIdentities) DeleteIdentitiesForUser(_ context.Context, userID string) (int64, error) {
	f.cleared = append(f.cleared, userID)
	return int64(len(f.list)), nil
}

type fakeSessions struct {
	revoked []string
}

func (f *fakeSessions) DeleteByUserID(_ context.Context, userID string) (int64, error) {
	f.revoked = append(f.revoked, userID)
	return 2, nil
}

func (f *fakeSessions) CountActiveSessions(_ context.Context, _ time.Time) (int64, int64, error) {
	return 3, 4, nil
}

type fakeUsage struct{}

func (fakeUsage) UsageSummary(_ context.Context, days int) (stats.Usage, error) {
	return stats.Usage{TotalMatches: 7, ByDay: make([]stats.DayUsage, days)}, nil
}

// fakeLive models the registry's real shape: rooms includes the waiting room
// alongside any match rooms.
type fakeLive struct {
	rooms   int
	conns   int
	waiting int
}

func (f fakeLive) Totals() (int, int) { return f.rooms, f.conns }

func (f fakeLive) CountRoom(roomID string) int {
	if roomID == testWaitingRoom {
		return f.waiting
	}
	return 0
}

const testWaitingRoom = "__lobby__"

type fakeFeedback struct {
	reports []feedback.Report
	updates map[string]bson.M
	deleted []string
	lastQ   feedback.Query
}

func (f *fakeFeedback) List(_ context.Context, q feedback.Query) ([]feedback.Report, error) {
	f.lastQ = q
	return f.reports, nil
}

func (f *fakeFeedback) Count(_ context.Context, q feedback.Query) (int64, error) {
	f.lastQ = q
	return int64(len(f.reports)), nil
}

func (f *fakeFeedback) CountByStatus(context.Context) (map[string]int64, error) {
	return map[string]int64{feedback.StatusNew: 2, feedback.StatusOpen: 1, feedback.StatusResolved: 0}, nil
}

func (f *fakeFeedback) Update(_ context.Context, id bson.ObjectID, set bson.M) error {
	if f.updates == nil {
		f.updates = map[string]bson.M{}
	}
	f.updates[id.Hex()] = set
	return nil
}

func (f *fakeFeedback) Delete(_ context.Context, id bson.ObjectID) error {
	f.deleted = append(f.deleted, id.Hex())
	return nil
}

/* ------------------------------------------------------------------ harness */

type harness struct {
	router     http.Handler
	users      *fakeUsers
	identities *fakeIdentities
	sessions   *fakeSessions
	live       *fakeLive
	feedback   *fakeFeedback
	admin      models.User
	other      models.User
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

func newHarness(t *testing.T, adminEmails ...string) *harness {
	t.Helper()
	return newHarnessWith(t, PasswordLogin{}, adminEmails...)
}

// newHarnessWith builds a console with a password login configured as well.
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
	identities := &fakeIdentities{list: []models.Identity{{Provider: "google"}}}
	sessions := &fakeSessions{}

	if len(adminEmails) == 0 {
		adminEmails = []string{"boss@example.com"}
	}
	live := &fakeLive{}
	reports := &fakeFeedback{}
	h := NewHandlers(Deps{
		Guard:         NewGuard(users, adminEmails, passwordLogin),
		Users:         users,
		Identities:    identities,
		Sessions:      sessions,
		Usage:         fakeUsage{},
		Live:          live,
		Feedback:      reports,
		WaitingRoomID: testWaitingRoom,
	})
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	return &harness{
		router: r, users: users, identities: identities, sessions: sessions,
		live: live, feedback: reports, admin: adminUser, other: other,
	}
}

// as issues a request signed as the given user.
func (h *harness) as(t *testing.T, u models.User, method, path string, body string) *httptest.ResponseRecorder {
	t.Helper()
	token, err := auth.CreateAccessToken(u.ID.Hex(), u.Username, false, time.Minute)
	if err != nil {
		t.Fatalf("minting a token: %v", err)
	}
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Authorization", "Bearer "+token)
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

/* -------------------------------------------------------------- guard tests */

// An unconfigured deployment must not have an open admin API. This is the
// single most consequential default in the package: getting it backwards would
// hand account deletion to anyone who can sign in.
func TestNoConfiguredAdminsDeniesEveryone(t *testing.T) {
	h := newHarness(t, "")
	if rec := h.as(t, h.admin, "GET", "/admin/api/session", ""); rec.Code != http.StatusForbidden {
		t.Errorf("an unconfigured console admitted a caller: got %d, want 403", rec.Code)
	}
}

func TestNonAdminIsDenied(t *testing.T) {
	h := newHarness(t)
	if rec := h.as(t, h.other, "GET", "/admin/api/session", ""); rec.Code != http.StatusForbidden {
		t.Errorf("a signed-in non-admin got in: got %d, want 403", rec.Code)
	}
}

// Matching an allow-listed address on an *unverified* account would let anyone
// claim an administrator's address at sign-up and inherit the console with it.
func TestUnverifiedAdminEmailIsDenied(t *testing.T) {
	h := newHarness(t)
	unverified := h.admin
	unverified.EmailVerified = false
	h.users.byID[unverified.ID.Hex()] = unverified

	if rec := h.as(t, unverified, "GET", "/admin/api/session", ""); rec.Code != http.StatusForbidden {
		t.Errorf("an unverified address matched the allow-list: got %d, want 403", rec.Code)
	}
}

func TestAdminIsAdmitted(t *testing.T) {
	h := newHarness(t)
	rec := h.as(t, h.admin, "GET", "/admin/api/session", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("the configured admin was refused: got %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	if got := decode(t, rec)["email"]; got != "boss@example.com" {
		t.Errorf("session reported email %v, want boss@example.com", got)
	}
}

func TestAllowListIgnoresCaseAndSpacing(t *testing.T) {
	h := newHarness(t, "  BOSS@Example.COM  ")
	if rec := h.as(t, h.admin, "GET", "/admin/api/session", ""); rec.Code != http.StatusOK {
		t.Errorf("an allow-list entry differing in case/spacing was not matched: got %d, want 200", rec.Code)
	}
}

func TestGuestTokenIsDenied(t *testing.T) {
	t.Setenv("JWT_ACCESS_SECRET", "test_secret")
	h := newHarness(t)
	token, err := auth.CreateAccessToken(h.admin.ID.Hex(), h.admin.Username, true, time.Minute)
	if err != nil {
		t.Fatalf("minting a guest token: %v", err)
	}
	req := httptest.NewRequest("GET", "/admin/api/session", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("a guest token reached the console: got %d, want 403", rec.Code)
	}
}

/* ------------------------------------------------------------ delete tests */

func TestDeleteUserCascades(t *testing.T) {
	h := newHarness(t)
	target := h.other.ID.Hex()

	rec := h.as(t, h.admin, "DELETE", "/admin/api/users/"+target, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("deleting failed: got %d (%s)", rec.Code, rec.Body.String())
	}

	// Identities must go, or their (provider, subject) unique index keeps
	// pointing at an account that no longer exists and locks the person out of
	// ever signing up again.
	if len(h.identities.cleared) != 1 || h.identities.cleared[0] != target {
		t.Errorf("identities not cleared for the deleted account: %v", h.identities.cleared)
	}
	if len(h.sessions.revoked) != 1 || h.sessions.revoked[0] != target {
		t.Errorf("sessions not revoked for the deleted account: %v", h.sessions.revoked)
	}
	if len(h.users.deleted) != 1 || h.users.deleted[0] != target {
		t.Errorf("account not deleted: %v", h.users.deleted)
	}
}

func TestDeletingYourselfIsRefused(t *testing.T) {
	h := newHarness(t)
	rec := h.as(t, h.admin, "DELETE", "/admin/api/users/"+h.admin.ID.Hex(), "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("self-deletion was allowed: got %d, want 409", rec.Code)
	}
	if len(h.users.deleted) != 0 {
		t.Errorf("self-deletion still reached the repository: %v", h.users.deleted)
	}
}

func TestDeleteUnknownUserIs404(t *testing.T) {
	h := newHarness(t)
	rec := h.as(t, h.admin, "DELETE", "/admin/api/users/"+bson.NewObjectID().Hex(), "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("deleting a missing account: got %d, want 404", rec.Code)
	}
}

/* ---------------------------------------------------------- password tests */

// The generated password must be the one actually stored, and the change must
// invalidate existing sessions — a reset that leaves the old sessions alive
// locks nobody out, which is the entire point of resetting.
func TestGeneratedPasswordIsStoredAndRevokesSessions(t *testing.T) {
	h := newHarness(t)
	target := h.other.ID.Hex()

	rec := h.as(t, h.admin, "POST", "/admin/api/users/"+target+"/password", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("resetting failed: got %d (%s)", rec.Code, rec.Body.String())
	}
	body := decode(t, rec)
	password, _ := body["password"].(string)
	if password == "" {
		t.Fatal("no generated password was returned")
	}

	update := h.users.updates[target]
	hash, _ := update["passwordHash"].(string)
	if hash == "" {
		t.Fatal("no password hash was written")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		t.Errorf("the stored hash does not match the password handed to the admin: %v", err)
	}
	if len(h.sessions.revoked) != 1 || h.sessions.revoked[0] != target {
		t.Errorf("a password reset did not revoke sessions: %v", h.sessions.revoked)
	}
}

func TestSuppliedPasswordIsUsed(t *testing.T) {
	h := newHarness(t)
	target := h.other.ID.Hex()

	rec := h.as(t, h.admin, "POST", "/admin/api/users/"+target+"/password", `{"password":"correct horse battery"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("resetting failed: got %d (%s)", rec.Code, rec.Body.String())
	}
	if _, leaked := decode(t, rec)["password"]; leaked {
		t.Error("the response echoed a password the caller already knows")
	}
	hash, _ := h.users.updates[target]["passwordHash"].(string)
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("correct horse battery")); err != nil {
		t.Errorf("the supplied password was not what got stored: %v", err)
	}
}

func TestShortPasswordIsRejected(t *testing.T) {
	h := newHarness(t)
	target := h.other.ID.Hex()

	rec := h.as(t, h.admin, "POST", "/admin/api/users/"+target+"/password", `{"password":"short"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("a short password was accepted: got %d, want 400", rec.Code)
	}
	if _, written := h.users.updates[target]; written {
		t.Error("a rejected password still reached the repository")
	}
}

/* ------------------------------------------------------------- roster tests */

// models.User hides PasswordHash from JSON, but the roster builds its own
// shape — this is what keeps the omission true if that shape is ever edited.
func TestRosterNeverLeaksPasswordHash(t *testing.T) {
	h := newHarness(t)
	withHash := h.other
	withHash.PasswordHash = "$2a$12$averysecretbcrypthashvalue"
	h.users.byID[withHash.ID.Hex()] = withHash

	rec := h.as(t, h.admin, "GET", "/admin/api/users", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("listing failed: got %d (%s)", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "bcrypthash") {
		t.Errorf("the roster leaked a password hash: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"hasPassword":true`) {
		t.Error("the roster did not report that the account has a password")
	}
}

func TestRosterFlagsAdmins(t *testing.T) {
	h := newHarness(t)
	rec := h.as(t, h.admin, "GET", "/admin/api/users", "")

	var body struct {
		Users []userRow `json:"users"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	for _, row := range body.Users {
		want := row.Email == "boss@example.com"
		if row.IsAdmin != want {
			t.Errorf("%s: isAdmin=%v, want %v", row.Email, row.IsAdmin, want)
		}
	}
}

/* ----------------------------------------------------------------- usage */

// The lobby's waiting room shares the connection registry with real matches
// under a reserved id. Counting rooms naively reports an idle server as having
// a game in progress, which is what this pins.
func TestWaitingRoomIsNotCountedAsAMatch(t *testing.T) {
	cases := []struct {
		name                     string
		rooms, conns, waiting    int
		wantMatches, wantWaiting int
		wantConnections          int
	}{
		{"idle server with people waiting", 1, 3, 3, 0, 3, 3},
		{"waiting room plus two matches", 3, 9, 2, 2, 2, 9},
		{"matches only", 2, 6, 0, 2, 0, 6},
		{"nothing at all", 0, 0, 0, 0, 0, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			*h.live = fakeLive{rooms: tc.rooms, conns: tc.conns, waiting: tc.waiting}

			rec := h.as(t, h.admin, "GET", "/admin/api/usage", "")
			if rec.Code != http.StatusOK {
				t.Fatalf("usage failed: got %d (%s)", rec.Code, rec.Body.String())
			}
			live := decode(t, rec)["live"].(map[string]any)

			if got := int(live["instanceMatches"].(float64)); got != tc.wantMatches {
				t.Errorf("instanceMatches = %d, want %d", got, tc.wantMatches)
			}
			if got := int(live["instanceWaiting"].(float64)); got != tc.wantWaiting {
				t.Errorf("instanceWaiting = %d, want %d", got, tc.wantWaiting)
			}
			if got := int(live["instanceConnections"].(float64)); got != tc.wantConnections {
				t.Errorf("instanceConnections = %d, want %d", got, tc.wantConnections)
			}
		})
	}
}

/* -------------------------------------------------------------- feedback */

func TestFeedbackRequiresAdmin(t *testing.T) {
	h := newHarness(t)
	for _, tc := range []struct {
		method, path string
	}{
		{"GET", "/admin/api/feedback"},
		{"PATCH", "/admin/api/feedback/" + bson.NewObjectID().Hex()},
		{"DELETE", "/admin/api/feedback/" + bson.NewObjectID().Hex()},
	} {
		if rec := h.as(t, h.other, tc.method, tc.path, `{}`); rec.Code != http.StatusForbidden {
			t.Errorf("%s %s: got %d, want 403", tc.method, tc.path, rec.Code)
		}
	}
}

func TestFeedbackListPassesFiltersThrough(t *testing.T) {
	h := newHarness(t)
	rec := h.as(t, h.admin, "GET", "/admin/api/feedback?status=open&kind=bug&limit=5&skip=10", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200 (%s)", rec.Code, rec.Body.String())
	}

	got := h.feedback.lastQ
	want := feedback.Query{Status: "open", Kind: "bug", Limit: 5, Skip: 10}
	if got != want {
		t.Errorf("query = %+v, want %+v", got, want)
	}
	if _, ok := decode(t, rec)["counts"]; !ok {
		t.Error("the response carries no per-status counts for the filter labels")
	}
}

// An unrecognised filter must be refused, not ignored. Quietly returning
// everything would read to the operator as "nothing matches" — the opposite of
// what actually happened.
func TestUnknownFeedbackFilterIsRefused(t *testing.T) {
	h := newHarness(t)
	for _, path := range []string{
		"/admin/api/feedback?status=banana",
		"/admin/api/feedback?kind=banana",
	} {
		if rec := h.as(t, h.admin, "GET", path, ""); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: got %d, want 400", path, rec.Code)
		}
	}
}

func TestFeedbackTriage(t *testing.T) {
	h := newHarness(t)
	id := bson.NewObjectID().Hex()

	rec := h.as(t, h.admin, "PATCH", "/admin/api/feedback/"+id, `{"status":"resolved","note":"fixed in 1.1"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	set := h.feedback.updates[id]
	if set["status"] != feedback.StatusResolved {
		t.Errorf("status = %v, want resolved", set["status"])
	}
	if set["note"] != "fixed in 1.1" {
		t.Errorf("note = %v, want the submitted note", set["note"])
	}
}

func TestFeedbackRejectsUnknownStatus(t *testing.T) {
	h := newHarness(t)
	id := bson.NewObjectID().Hex()

	rec := h.as(t, h.admin, "PATCH", "/admin/api/feedback/"+id, `{"status":"banana"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400", rec.Code)
	}
	if _, written := h.feedback.updates[id]; written {
		t.Error("a rejected status still reached the repository")
	}
}

func TestFeedbackDelete(t *testing.T) {
	h := newHarness(t)
	id := bson.NewObjectID().Hex()

	if rec := h.as(t, h.admin, "DELETE", "/admin/api/feedback/"+id, ""); rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	if len(h.feedback.deleted) != 1 || h.feedback.deleted[0] != id {
		t.Errorf("deleted = %v, want [%s]", h.feedback.deleted, id)
	}
}

/* ----------------------------------------------------------------- ui tests */

// The console is what holds the admin token, so its headers are part of the
// security story rather than decoration.
func TestConsoleIsServedWithAStrictPolicy(t *testing.T) {
	h := newHarness(t)
	req := httptest.NewRequest("GET", "/admin/", nil)
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("the console did not load: got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Žolíky admin") {
		t.Error("the console page did not render")
	}
	csp := rec.Header().Get("Content-Security-Policy")
	for _, want := range []string{"default-src 'self'", "frame-ancestors 'none'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP %q is missing %q", csp, want)
		}
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("the console is cacheable, but it carries an admin token")
	}
}

// The console must be reachable without a token — it is what the sign-in form
// lives on — while the API behind it must not be.
func TestConsoleIsPublicButApiIsNot(t *testing.T) {
	h := newHarness(t)
	for _, path := range []string{"/admin/", "/admin/app.js", "/admin/styles.css"} {
		rec := httptest.NewRecorder()
		h.router.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: got %d, want 200", path, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, httptest.NewRequest("GET", "/admin/api/users", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("the API answered an unauthenticated request: got %d, want 401", rec.Code)
	}
}
