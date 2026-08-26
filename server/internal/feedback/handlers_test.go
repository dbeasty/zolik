package feedback

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/auth"
)

type fakeRepo struct {
	inserted []Report
	recent   int64
}

func (f *fakeRepo) Insert(_ context.Context, r Report) (Report, error) {
	r.ID = bson.NewObjectID()
	f.inserted = append(f.inserted, r)
	return r, nil
}

func (f *fakeRepo) List(context.Context, Query) ([]Report, error) { return nil, nil }
func (f *fakeRepo) Count(context.Context, Query) (int64, error)   { return 0, nil }
func (f *fakeRepo) CountByStatus(context.Context) (map[string]int64, error) {
	return nil, nil
}

func (f *fakeRepo) CountRecentFrom(_ context.Context, _, _ string, _ time.Time) (int64, error) {
	return f.recent, nil
}
func (f *fakeRepo) Delete(context.Context, bson.ObjectID) error         { return nil }
func (f *fakeRepo) Update(context.Context, bson.ObjectID, bson.M) error { return nil }

func newRouter(repo Repository) http.Handler {
	r := chi.NewRouter()
	NewHandlers(repo).RegisterRoutes(r)
	return r
}

// submit posts a report, optionally signed as someone.
func submit(t *testing.T, router http.Handler, body string, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/feedback", strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// A guest reporting a bug is the commonest case there is, and requiring an
// account would lose exactly the reports most worth reading.
func TestAGuestCanSendAReportWithNoSession(t *testing.T) {
	repo := &fakeRepo{}
	rec := submit(t, newRouter(repo), `{"kind":"bug","message":"the discard pile vanished"}`, "")

	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d, want 201 (%s)", rec.Code, rec.Body.String())
	}
	if len(repo.inserted) != 1 {
		t.Fatalf("stored %d reports, want 1", len(repo.inserted))
	}
	got := repo.inserted[0]
	if got.Message != "the discard pile vanished" || got.Kind != KindBug {
		t.Errorf("stored %+v, want the submitted kind and message", got)
	}
	if got.Status != StatusNew {
		t.Errorf("status = %q, want %q", got.Status, StatusNew)
	}
}

// A signed-in report must be traceable back to the account, and a guest's to
// the device — that attribution is what makes a follow-up possible at all.
func TestSessionIsRecordedOnTheReport(t *testing.T) {
	t.Setenv("JWT_ACCESS_SECRET", "test_secret")

	for _, tc := range []struct {
		name        string
		isGuest     bool
		wantUserID  string
		wantGuestID string
	}{
		{name: "signed-in", isGuest: false, wantUserID: "subject-1"},
		{name: "guest", isGuest: true, wantGuestID: "subject-1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token, err := auth.CreateAccessToken("subject-1", "kaja", tc.isGuest, time.Minute)
			if err != nil {
				t.Fatalf("minting a token: %v", err)
			}
			repo := &fakeRepo{}
			rec := submit(t, newRouter(repo), `{"kind":"idea","message":"let me sort my hand"}`, token)
			if rec.Code != http.StatusCreated {
				t.Fatalf("got %d, want 201", rec.Code)
			}

			got := repo.inserted[0]
			if got.UserID != tc.wantUserID {
				t.Errorf("userId = %q, want %q", got.UserID, tc.wantUserID)
			}
			if got.GuestID != tc.wantGuestID {
				t.Errorf("guestId = %q, want %q", got.GuestID, tc.wantGuestID)
			}
			if got.Username != "kaja" {
				t.Errorf("username = %q, want kaja", got.Username)
			}
		})
	}
}

func TestSubmissionValidation(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"an empty message is refused", `{"kind":"bug","message":"   "}`, http.StatusBadRequest},
		{"an unknown kind is refused", `{"kind":"rant","message":"hello"}`, http.StatusBadRequest},
		{"malformed json is refused", `{`, http.StatusBadRequest},
		{"a missing kind defaults rather than failing", `{"message":"hello"}`, http.StatusCreated},
		{
			"an over-long message is refused",
			`{"kind":"bug","message":"` + strings.Repeat("a", MaxMessageLen+1) + `"}`,
			http.StatusBadRequest,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if rec := submit(t, newRouter(&fakeRepo{}), tc.body, ""); rec.Code != tc.want {
				t.Errorf("got %d, want %d (%s)", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func TestMissingKindBecomesOther(t *testing.T) {
	repo := &fakeRepo{}
	submit(t, newRouter(repo), `{"message":"hello"}`, "")
	if got := repo.inserted[0].Kind; got != KindOther {
		t.Errorf("kind = %q, want %q", got, KindOther)
	}
}

// tokenFor mints an access token for a reporter with a session.
func tokenFor(t *testing.T, subject string, isGuest bool) string {
	t.Helper()
	t.Setenv("JWT_ACCESS_SECRET", "test_secret")
	token, err := auth.CreateAccessToken(subject, "kaja", isGuest, time.Minute)
	if err != nil {
		t.Fatalf("minting a token: %v", err)
	}
	return token
}

// A reporter with a session is counted in the database, so the ceiling holds
// across restarts and across instances — unlike the in-process one that covers
// anonymous reports.
func TestReportsAreThrottledPerReporter(t *testing.T) {
	repo := &fakeRepo{recent: perReporterLimit}
	rec := submit(t, newRouter(repo), `{"kind":"bug","message":"again"}`, tokenFor(t, "subject-1", false))

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("got %d, want 429", rec.Code)
	}
	if len(repo.inserted) != 0 {
		t.Error("a throttled report was stored anyway")
	}
}

// The repository throttle counts by account or guest id, so it cannot see a
// report that arrives with no session — which left the one path anybody can
// reach as the only unthrottled one. This is that hole.
func TestAnonymousReportsAreThrottledToo(t *testing.T) {
	repo := &fakeRepo{}
	router := newRouter(repo)

	for i := 0; i < perReporterLimit; i++ {
		if rec := submit(t, router, `{"kind":"other","message":"probe"}`, ""); rec.Code != http.StatusCreated {
			t.Fatalf("report %d: got %d, want 201", i+1, rec.Code)
		}
	}
	if rec := submit(t, router, `{"kind":"other","message":"one too many"}`, ""); rec.Code != http.StatusTooManyRequests {
		t.Errorf("the report past the ceiling: got %d, want 429", rec.Code)
	}
	if len(repo.inserted) != perReporterLimit {
		t.Errorf("stored %d reports, want %d", len(repo.inserted), perReporterLimit)
	}
}

func TestJustUnderTheThrottleStillGoesThrough(t *testing.T) {
	repo := &fakeRepo{recent: perReporterLimit - 1}
	rec := submit(t, newRouter(repo), `{"kind":"bug","message":"once more"}`, tokenFor(t, "subject-1", false))
	if rec.Code != http.StatusCreated {
		t.Errorf("got %d, want 201", rec.Code)
	}
}

// Context fields are labels, not prose. Clipping rather than rejecting keeps a
// client bug in one of them from costing the whole report.
func TestOverlongContextIsClippedNotRejected(t *testing.T) {
	repo := &fakeRepo{}
	body, _ := json.Marshal(map[string]string{
		"kind":     "bug",
		"message":  "something broke",
		"platform": strings.Repeat("x", maxContextLen+50),
	})
	rec := submit(t, newRouter(repo), string(body), "")

	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d, want 201", rec.Code)
	}
	if got := len([]rune(repo.inserted[0].Platform)); got != maxContextLen {
		t.Errorf("platform kept %d runes, want %d", got, maxContextLen)
	}
}

// clip counts runes, not bytes. Cutting mid-character would store an invalid
// UTF-8 sequence, and these messages are routinely not ASCII.
func TestClipDoesNotSplitMultibyteCharacters(t *testing.T) {
	got := clip(strings.Repeat("ž", 10), 4)
	if got != "žžžž" {
		t.Errorf("clip = %q, want %q", got, "žžžž")
	}
}
