package admin

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed ui
var uiFS embed.FS

// uiHandler serves the admin console's static files.
//
// The console is embedded into the binary rather than built and deployed
// separately: it is a few hundred lines with no build step, and shipping it
// inside the server means there is no second artifact that can be a different
// version from the API it talks to.
func uiHandler() http.Handler {
	sub, err := fs.Sub(uiFS, "ui")
	if err != nil {
		// Only reachable if the embed directive above stops matching, which
		// is a build-time mistake rather than a runtime condition.
		panic(err)
	}
	files := http.StripPrefix("/admin", http.FileServer(http.FS(sub)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The console is entirely self-contained, so it can afford the
		// strictest policy there is: no inline script, no inline style, and
		// nothing loaded from anywhere but this origin. That is why the CSS
		// and JS are separate files rather than inlined into the page.
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; form-action 'none'; frame-ancestors 'none'; base-uri 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		// The console holds an admin token in local storage; keeping it out of
		// shared caches and out of search indexes both matter.
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Robots-Tag", "noindex, nofollow")
		files.ServeHTTP(w, r)
	})
}
