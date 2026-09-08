package admin

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"zolik/server/internal/metrics"
)

func sampleReport() metrics.Report {
	rate := 0.8
	return metrics.Report{
		From: "2026-09-01", To: "2026-09-02", Bucket: metrics.BucketDay, Timezone: "UTC",
		Buckets: []metrics.Period{
			{
				Label: "2026-09-01", Start: "2026-09-01", End: "2026-09-01",
				Matches: metrics.MatchCounts{Created: 5, Started: 4, Completed: 3, Abandoned: 1, NeverStarted: 1, CompletionRate: &rate},
				Players: metrics.PlayerCounts{Distinct: 7},
				Users:   metrics.UserCounts{Registered: 2, Guests: 4},
			},
			{
				Label: "2026-09-02", Start: "2026-09-02", End: "2026-09-02", Partial: true,
				Matches: metrics.MatchCounts{Created: 1},
				Players: metrics.PlayerCounts{Distinct: 2, IsFloor: true},
			},
		},
		Totals: metrics.Totals{
			Matches: metrics.MatchCounts{Created: 6, Started: 4, Completed: 3, Abandoned: 1, CompletionRate: &rate},
			Players: metrics.PlayerCounts{Distinct: 8, IsFloor: true},
			Ops:     metrics.OpsCounts{Boots: 3, UncleanBoots: 1},
		},
	}
}

func signedIn(t *testing.T, h *harness, login PasswordLogin) string {
	t.Helper()
	token, err := login.mintToken(time.Now().UTC())
	if err != nil {
		t.Fatalf("minting: %v", err)
	}
	return token
}

// Everything behind the guard must be behind the guard. This is the cheap
// check that a route added to the authenticated group did not accidentally get
// registered next to /login instead.
func TestReportingRoutesRequireAuth(t *testing.T) {
	h, _ := newHarness(t)
	for _, path := range []string{"/admin/api/report", "/admin/api/status", "/admin/api/session"} {
		if rec := h.withToken(t, "", "GET", path, ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s without a token: got %d, want 401", path, rec.Code)
		}
	}
}

func TestReportDefaultsToThirtyDaysByDay(t *testing.T) {
	h, login := newHarness(t)
	h.reports.report = sampleReport()

	rec := h.withToken(t, signedIn(t, h, login), "GET", "/admin/api/report", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
	}

	if h.reports.lastBucket != metrics.BucketDay {
		t.Errorf("bucket = %q, want day", h.reports.lastBucket)
	}
	// Thirty days inclusive is twenty-nine days back, which is the off-by-one
	// worth pinning: "the last 30 days" that returns 31 buckets is wrong in a
	// way nobody notices until they compare two months.
	span := h.reports.lastTo.Sub(h.reports.lastFrom)
	if span != 29*24*time.Hour {
		t.Errorf("default range spans %v, want 29 days (30 inclusive)", span)
	}
}

func TestReportHonoursTheRequestedRange(t *testing.T) {
	h, login := newHarness(t)
	h.reports.report = sampleReport()

	rec := h.withToken(t, signedIn(t, h, login), "GET",
		"/admin/api/report?from=2026-08-01&to=2026-08-31&bucket=week", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
	if h.reports.lastBucket != metrics.BucketWeek {
		t.Errorf("bucket = %q, want week", h.reports.lastBucket)
	}
	if got := h.reports.lastFrom.Format(metrics.DayFormat); got != "2026-08-01" {
		t.Errorf("from = %s, want 2026-08-01", got)
	}
	if got := h.reports.lastTo.Format(metrics.DayFormat); got != "2026-08-31" {
		t.Errorf("to = %s, want 2026-08-31", got)
	}
}

// A malformed date is the default, not a 400: the console's own controls
// cannot produce one, so a bad value is somebody experimenting with the URL.
func TestReportFallsBackOnAMalformedDate(t *testing.T) {
	h, login := newHarness(t)
	h.reports.report = sampleReport()

	rec := h.withToken(t, signedIn(t, h, login), "GET", "/admin/api/report?from=last-tuesday", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
}

func TestReportRendersJSON(t *testing.T) {
	h, login := newHarness(t)
	h.reports.report = sampleReport()

	rec := h.withToken(t, signedIn(t, h, login), "GET", "/admin/api/report", "")
	var got metrics.Report
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if got.Timezone != "UTC" {
		t.Errorf("timezone = %q — the screen must say which day it means", got.Timezone)
	}
	if len(got.Buckets) != 2 {
		t.Fatalf("got %d buckets, want 2", len(got.Buckets))
	}
	if !got.Buckets[1].Partial {
		t.Error("the partial bucket lost its flag on the way out")
	}
	if !got.Totals.Players.IsFloor {
		t.Error("the distinct-player floor did not survive serialisation")
	}
}

func TestReportCSV(t *testing.T) {
	h, login := newHarness(t)
	h.reports.report = sampleReport()

	rec := h.withToken(t, signedIn(t, h, login), "GET", "/admin/api/report?format=csv", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("content-type = %q, want text/csv", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "2026-09-01") {
		t.Errorf("content-disposition = %q, want the range in the filename", cd)
	}

	rows, err := csv.NewReader(strings.NewReader(rec.Body.String())).ReadAll()
	if err != nil {
		t.Fatalf("parsing the csv: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want a header and two buckets", len(rows))
	}
	if rows[1][0] != "2026-09-01" {
		t.Errorf("first row label = %q", rows[1][0])
	}
	// Empty rather than 0: a spreadsheet averaging a column of rates must not
	// be handed a zero for a bucket in which nothing resolved.
	rateCol := indexOf(rows[0], "completion_rate")
	if rateCol < 0 {
		t.Fatal("no completion_rate column")
	}
	if rows[2][rateCol] != "" {
		t.Errorf("an unresolved bucket exported a rate of %q, want empty", rows[2][rateCol])
	}
}

func indexOf(row []string, name string) int {
	for i, v := range row {
		if v == name {
			return i
		}
	}
	return -1
}

func TestReportSurfacesAStoreFailureAsA500(t *testing.T) {
	h, login := newHarness(t)
	h.reports.err = errors.New("the database is away")

	rec := h.withToken(t, signedIn(t, h, login), "GET", "/admin/api/report", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("got %d, want 500", rec.Code)
	}
	// The operator gets "could not build the report"; the detail goes to the
	// log. A database error echoed to the browser describes the deployment to
	// whoever is reading.
	if strings.Contains(rec.Body.String(), "database is away") {
		t.Error("the store's error text was echoed to the client")
	}
}

// The waiting room is a room in the connection registry like any other, so
// without discounting it an idle server with one person in the lobby reports a
// game in progress.
func TestStatusDoesNotCountTheWaitingRoomAsAMatch(t *testing.T) {
	h, login := newHarness(t)
	h.live.rooms = 3
	h.live.conns = 9
	h.live.waiting = 2

	rec := h.withToken(t, signedIn(t, h, login), "GET", "/admin/api/status", "")
	body := decode(t, rec)
	live, ok := body["live"].(map[string]any)
	if !ok {
		t.Fatalf("no live block in %v", body)
	}
	if live["matches"] != float64(2) {
		t.Errorf("matches = %v, want 2 (3 rooms less the waiting room)", live["matches"])
	}
	if live["waiting"] != float64(2) {
		t.Errorf("waiting = %v, want 2", live["waiting"])
	}
}

func TestStatusWithNothingHappening(t *testing.T) {
	h, login := newHarness(t)

	rec := h.withToken(t, signedIn(t, h, login), "GET", "/admin/api/status", "")
	body := decode(t, rec)
	live := body["live"].(map[string]any)
	// Not -1: an empty waiting room is not registered at all, so there is
	// nothing to discount.
	if live["matches"] != float64(0) {
		t.Errorf("matches = %v, want 0", live["matches"])
	}
}

// Sign-ins are counted because a console meant to be unreachable from the
// internet has no business being knocked on.
func TestSignInsAreCounted(t *testing.T) {
	h, _ := newHarness(t)

	postLogin(t, h, `{"username":"operator","password":"wrong"}`)
	postLogin(t, h, `{"username":"operator","password":"`+testPassword+`"}`)

	if got := h.counts.counts[metrics.AdminLoginDenied]; got != 1 {
		t.Errorf("admin.login.denied = %d, want 1", got)
	}
	if got := h.counts.counts[metrics.AdminLoginOK]; got != 1 {
		t.Errorf("admin.login.ok = %d, want 1", got)
	}
}
