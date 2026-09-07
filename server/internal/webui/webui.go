// Package webui serves the Expo web export from inside the server binary.
//
// It exists because the bundle and the server used to be deployed as two
// independent things — a directory rsynced to the host and read by nginx, and
// a container built from source — with nothing tying them to the same commit.
// A deploy that skipped the web step left one release's client talking to
// another release's API, and said so nowhere.
//
// Serving it from here makes that unrepresentable: one image, one commit, one
// version footer.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// The `all:` prefix is load-bearing, not decoration. A plain `//go:embed dist`
// skips every file and directory whose name starts with "." or "_", and Expo
// puts the entire JavaScript bundle under `_expo/static/`. Without `all:` this
// compiles, embeds the HTML, serves a page that requests a script, and 404s
// on it — a blank screen with no error anywhere on the server.
//
//go:embed all:dist
var embedded embed.FS

// Embedded is the bundle compiled into this binary, or nil when none was.
//
// Local builds and every `go test` run compile against a dist/ holding only
// its .gitkeep, because Expo has not run: that is the nil case, and the
// handler below answers nothing rather than pretending to be a web server.
func Embedded() fs.FS {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		return nil
	}
	// index.html decides it, not the directory's existence — the directory
	// always exists, because .gitkeep is in it.
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil
	}
	return sub
}

// Handler serves a static export the way nginx's `try_files $uri $uri/
// /index.html` did, which is the behaviour app.json's `"output": "static"`
// requires: expo-router prerenders one HTML file per route, and the dynamic
// ones (/match/{id}) exist only as a client-side route reached through
// index.html.
type Handler struct {
	files fs.FS
}

// NewHandler wraps a bundle. A nil fsys yields a handler that reports itself
// unavailable and serves nothing, which is the ordinary state of a
// development build.
func NewHandler(fsys fs.FS) *Handler { return &Handler{files: fsys} }

// Available reports whether a bundle was compiled in. Callers use it to decide
// between falling back to the app and returning the API's own error.
func (h *Handler) Available() bool { return h != nil && h.files != nil }

// Lookup resolves a request path to a file in the bundle, mirroring nginx's
// try_files: the path itself, then the prerendered `<path>.html`, then
// `<path>/index.html`. It reports false when the path names nothing, which is
// how a caller tells "an asset the client asked for" from "a route only the
// client knows about".
func (h *Handler) Lookup(urlPath string) (string, bool) {
	if !h.Available() {
		return "", false
	}

	// path.Clean resolves the ".." segments that would otherwise walk out of
	// the bundle. fs.FS rejects those too, so this is belt and braces — and
	// the belt, because it also normalises "//" and trailing dots before the
	// candidates below are built from them.
	clean := path.Clean("/" + urlPath)
	name := strings.TrimPrefix(clean, "/")
	if name == "" {
		name = "index.html"
	}

	for _, candidate := range []string{name, name + ".html", path.Join(name, "index.html")} {
		info, err := fs.Stat(h.files, candidate)
		if err == nil && !info.IsDir() {
			return candidate, true
		}
	}
	return "", false
}

// ServeFile writes one file from the bundle.
func (h *Handler) ServeFile(w http.ResponseWriter, r *http.Request, name string) {
	// Expo fingerprints everything under _expo/static/, so those URLs name one
	// immutable byte sequence for as long as they exist and a client should
	// never ask twice. Everything else — index.html above all — must not be
	// held, or a deploy leaves browsers running the previous release until
	// they happen to hard-refresh.
	if strings.HasPrefix(name, "_expo/static/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	http.ServeFileFS(w, r, h.files, name)
}

// ServeIndex writes the SPA entry point with a 200, for a route that exists
// only inside the client.
func (h *Handler) ServeIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFileFS(w, r, h.files, "index.html")
}

// WantsHTML reports whether this looks like a browser asking for a page, as
// opposed to a client asking for JSON.
//
// The distinction is what keeps a mistyped API call honest: /users/mee should
// answer 404, not 200 with a web page the caller cannot parse. Restricting the
// fallback to reads matters for the same reason — a POST to a path that does
// not exist is a bug in the caller, and answering it with a page hides that.
func WantsHTML(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}
