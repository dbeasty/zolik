package webui

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func bundle() fstest.MapFS {
	return fstest.MapFS{
		"index.html":                   {Data: []byte("<!doctype html><title>index</title>")},
		"stats.html":                   {Data: []byte("<!doctype html><title>stats</title>")},
		"lobby/games.html":             {Data: []byte("<!doctype html><title>games</title>")},
		"legal/terms/index.html":       {Data: []byte("<!doctype html><title>terms</title>")},
		"_expo/static/js/entry-ab1.js": {Data: []byte("console.log(1)")},
		"favicon.ico":                  {Data: []byte("\x00")},
	}
}

// TestLookupMirrorsTryFiles pins the three-candidate resolution against the
// nginx directive it replaces: `try_files $uri $uri/ /index.html`. Expo's
// static export puts a route at any of the three shapes depending on whether
// it has children, so resolving only one of them serves a blank page for the
// others.
func TestLookupMirrorsTryFiles(t *testing.T) {
	h := NewHandler(bundle())

	for _, tc := range []struct {
		url  string
		want string
	}{
		{"/", "index.html"},                                               // the root
		{"/favicon.ico", "favicon.ico"},                                   // $uri, exact
		{"/stats", "stats.html"},                                          // $uri.html, a prerendered leaf route
		{"/lobby/games", "lobby/games.html"},                              // the same, nested
		{"/legal/terms", "legal/terms/index.html"},                        // $uri/, a route with children
		{"/_expo/static/js/entry-ab1.js", "_expo/static/js/entry-ab1.js"}, // the bundle itself
	} {
		got, ok := h.Lookup(tc.url)
		if !ok || got != tc.want {
			t.Errorf("Lookup(%q) = %q, %v; want %q, true", tc.url, got, ok, tc.want)
		}
	}
}

// TestLookupMissesClientOnlyRoutes covers the paths that exist only inside the
// running app. They must miss, so the caller falls through to index.html —
// /match/{id} is the one every player reaches by link.
func TestLookupMissesClientOnlyRoutes(t *testing.T) {
	h := NewHandler(bundle())

	for _, url := range []string{"/match/abc123", "/nothing/here", "/stats/deeper"} {
		if got, ok := h.Lookup(url); ok {
			t.Errorf("Lookup(%q) = %q, true; want a miss so index.html serves it", url, got)
		}
	}
}

// TestLookupCannotEscapeTheBundle is the traversal guard. A hit here would
// serve the container's filesystem to anyone who asks.
func TestLookupCannotEscapeTheBundle(t *testing.T) {
	h := NewHandler(bundle())

	for _, url := range []string{
		"/../etc/passwd",
		"/../../../../etc/passwd",
		"/_expo/../../etc/passwd",
	} {
		if got, ok := h.Lookup(url); ok {
			t.Errorf("Lookup(%q) resolved to %q — it must not escape the bundle", url, got)
		}
	}
}

// TestNoBundleIsUnavailable pins the development build: dist/ holds nothing but
// its .gitkeep, so nothing is served and nothing panics.
func TestNoBundleIsUnavailable(t *testing.T) {
	h := NewHandler(nil)

	if h.Available() {
		t.Error("a handler with no bundle reports itself available")
	}
	if _, ok := h.Lookup("/"); ok {
		t.Error("a handler with no bundle resolved a path")
	}

	// A nil *Handler is the shape offlineApp and every other bare &App{} has.
	var nilHandler *Handler
	if nilHandler.Available() {
		t.Error("a nil handler reports itself available")
	}
}

// TestEmbeddedIsEmptyInDevelopment guards the .gitkeep arrangement itself: if
// someone commits a built bundle, or drops the placeholder and breaks the
// embed, this says so.
func TestEmbeddedIsEmptyInDevelopment(t *testing.T) {
	if Embedded() != nil {
		t.Error("a bundle is compiled into the test binary — server/internal/webui/dist " +
			"should hold only .gitkeep in the repository; the real bundle arrives at image build time")
	}
}

// TestFingerprintedAssetsAreImmutable covers the two cache policies. Getting
// them the wrong way round is invisible in testing and expensive in
// production: browsers pinned to a stale index.html run the previous release
// until they are hard-refreshed.
func TestFingerprintedAssetsAreImmutable(t *testing.T) {
	h := NewHandler(bundle())

	rec := httptest.NewRecorder()
	h.ServeFile(rec, httptest.NewRequest(http.MethodGet, "/_expo/static/js/entry-ab1.js", nil),
		"_expo/static/js/entry-ab1.js")
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Errorf("fingerprinted asset Cache-Control = %q, want it held forever", got)
	}

	rec = httptest.NewRecorder()
	h.ServeIndex(rec, httptest.NewRequest(http.MethodGet, "/lobby/table", nil))
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("index.html Cache-Control = %q, want no-cache", got)
	}
}

// TestWantsHTMLOnlyForBrowserReads pins the guard that keeps a mistyped API
// call answering 404 instead of returning a page the caller cannot parse.
func TestWantsHTMLOnlyForBrowserReads(t *testing.T) {
	html := "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"

	for _, tc := range []struct {
		method string
		accept string
		want   bool
	}{
		{http.MethodGet, html, true},
		{http.MethodHead, html, true},
		{http.MethodPost, html, false},   // a write to a path that does not exist is a bug
		{http.MethodDelete, html, false}, // and so is this
		{http.MethodGet, "application/json", false},
		{http.MethodGet, "*/*", false},
		{http.MethodGet, "", false},
	} {
		req := httptest.NewRequest(tc.method, "/anything", nil)
		req.Header.Set("Accept", tc.accept)
		if got := WantsHTML(req); got != tc.want {
			t.Errorf("WantsHTML(%s, Accept: %q) = %v, want %v", tc.method, tc.accept, got, tc.want)
		}
	}
}
