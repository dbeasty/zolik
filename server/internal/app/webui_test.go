package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/go-chi/chi/v5"
)

const acceptHTML = "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"

// webRouter is the real route table with a stand-in bundle behind it. The
// bundle is fake; the routing is not, which is the whole point — every
// collision this file is about is a collision between the client's URLs and
// the server's.
func webRouter(t *testing.T) chi.Router {
	t.Helper()

	a := offlineApp(t)
	a.SetWebUI(fstest.MapFS{
		"index.html":                   {Data: []byte("<!doctype html><title>zolik</title>")},
		"stats.html":                   {Data: []byte("<!doctype html><title>stats</title>")},
		"_expo/static/js/entry-ab1.js": {Data: []byte("console.log(1)")},
	})

	r := chi.NewRouter()
	a.RegisterRoutes(r)
	return r
}

func get(t *testing.T, r chi.Router, url, accept string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Accept", accept)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func servedTheApp(rec *httptest.ResponseRecorder) bool {
	return rec.Code == http.StatusOK && strings.Contains(rec.Body.String(), "<title>zolik</title>")
}

// TestSignInPagesAreReachableInABrowser is the regression this whole
// arrangement was built for.
//
// The client has screens at /auth/login, /auth/register and /auth/guest. The
// server registers all three as POST endpoints. A browser opening one of those
// URLs — a shared link, a bookmark, a hard refresh mid-sign-in — sends a GET,
// which chi matches on path, finds no GET handler for, and answers 405. The
// player saw "Method Not Allowed" instead of the sign-in page, and the old
// nginx vhost had the identical bug one layer out because it forwarded all of
// /auth/ to the API.
//
// These are the three paths where the method, not the path, is what falls
// through — so they exercise MethodNotAllowed rather than NotFound.
func TestSignInPagesAreReachableInABrowser(t *testing.T) {
	r := webRouter(t)

	for _, url := range []string{"/auth/login", "/auth/register", "/auth/guest"} {
		rec := get(t, r, url, acceptHTML)
		if !servedTheApp(rec) {
			t.Errorf("GET %s = %d %q; a browser opening this URL must get the app",
				url, rec.Code, strings.TrimSpace(rec.Body.String()))
		}
	}
}

// TestClientRoutesReachTheApp covers the paths the API never claimed, which
// fall through NotFound. /match/{id} matters most: it is the link players send
// each other, and it exists only as a client-side route — no prerendered file
// corresponds to it, so it can only work by falling back to index.html.
func TestClientRoutesReachTheApp(t *testing.T) {
	r := webRouter(t)

	for _, url := range []string{
		"/",
		"/match/6a1f2c",
		"/lobby/games",
		"/lobby/table",
		"/scoring",
		"/legal/terms",
		"/settings",
	} {
		if rec := get(t, r, url, acceptHTML); !servedTheApp(rec) {
			t.Errorf("GET %s = %d; want the app", url, rec.Code)
		}
	}
}

// TestAPIRoutesOutrankTheClient is the other half. The fallback must never
// take a path the API registered, or an endpoint becomes an HTML page and its
// callers get a parse error somewhere unrelated.
func TestAPIRoutesOutrankTheClient(t *testing.T) {
	r := webRouter(t)

	// Answered by the health group, not the client.
	if rec := get(t, r, "/healthz", acceptHTML); rec.Body.String() != "ok" {
		t.Errorf("GET /healthz = %d %q; want the API's own answer", rec.Code, rec.Body.String())
	}

	// Behind auth middleware, which rejects a missing token before any
	// handler or database is reached. A 401 proves the API claimed the path;
	// the app would have answered 200 with a page.
	if rec := get(t, r, "/users/me", acceptHTML); rec.Code != http.StatusUnauthorized {
		t.Errorf("GET /users/me = %d; want 401 from the API, not a page", rec.Code)
	}
}

// TestPrerenderedRouteBeatsTheFallback checks that a route Expo prerendered is
// served as its own file rather than as index.html. Both render, so getting
// this wrong is invisible in a browser — and it costs the prerendered HTML,
// which is the reason app.json sets output to "static" at all.
func TestPrerenderedRouteBeatsTheFallback(t *testing.T) {
	rec := get(t, webRouter(t), "/stats", acceptHTML)
	if !strings.Contains(rec.Body.String(), "<title>stats</title>") {
		t.Errorf("GET /stats served %q; want the prerendered stats page", strings.TrimSpace(rec.Body.String()))
	}
}

// TestBundleAssetsIgnoreAccept pins the ordering inside serveWebUI. A browser
// fetching the JavaScript bundle sends `Accept: */*`, so a fallback keyed on
// text/html before checking for the file would 404 every asset the page needs
// — a blank screen, with a 200 on the HTML and nothing wrong in the log.
func TestBundleAssetsIgnoreAccept(t *testing.T) {
	rec := get(t, webRouter(t), "/_expo/static/js/entry-ab1.js", "*/*")

	if rec.Code != http.StatusOK {
		t.Fatalf("GET the JS bundle = %d; want 200 regardless of Accept", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Errorf("bundle Cache-Control = %q; fingerprinted assets should be held", got)
	}
}

// TestAPIErrorsStayAPIErrors keeps a mistyped or unauthorised API call
// answering like an API. Returning a web page to a JSON client turns a clear
// 404 into a parse failure in the caller, which is how the missing `lobby`
// prefix in the old nginx vhost stayed hidden.
func TestAPIErrorsStayAPIErrors(t *testing.T) {
	r := webRouter(t)

	rec := get(t, r, "/users/mee", "application/json")
	if rec.Code != http.StatusNotFound || servedTheApp(rec) {
		t.Errorf("GET /users/mee with a JSON Accept = %d; want a plain 404", rec.Code)
	}

	// Writes never get a page: a POST to a path that does not exist is a bug
	// in the caller, and answering it with 200 and HTML hides it.
	req := httptest.NewRequest(http.MethodPost, "/auth/does-not-exist", nil)
	req.Header.Set("Accept", acceptHTML)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound || servedTheApp(rec) {
		t.Errorf("POST to a missing path = %d; want a plain 404, never the app", rec.Code)
	}
}

// TestNoBundleLeavesTheAPIAlone is the development and test build: dist/ holds
// only its .gitkeep, so there is no client to serve and the router must behave
// exactly as it did before any of this existed.
func TestNoBundleLeavesTheAPIAlone(t *testing.T) {
	a := offlineApp(t) // no SetWebUI: a.web is nil, as in every bare &App{}
	r := chi.NewRouter()
	a.RegisterRoutes(r)

	if rec := get(t, r, "/", acceptHTML); rec.Code != http.StatusNotFound {
		t.Errorf("GET / with no bundle = %d; want 404", rec.Code)
	}
	if rec := get(t, r, "/auth/login", acceptHTML); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /auth/login with no bundle = %d; want the API's own 405", rec.Code)
	}
	if rec := get(t, r, "/healthz", acceptHTML); rec.Body.String() != "ok" {
		t.Error("the health route stopped answering when no bundle is present")
	}
}
