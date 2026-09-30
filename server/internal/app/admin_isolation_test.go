package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// TestPublicRouterHasNoAdminRoutes is the regression test this whole feature
// rests on.
//
// The production vhost proxies one catch-all to the public listener, by
// design — the server knows which paths are API and which are the web client,
// so nginx enumerates nothing and cannot drift. The consequence is that every
// route the public router registers is on the internet by construction. The
// console is therefore registered on a *different* router, served on a
// loopback-bound port that nginx does not proxy.
//
// That property is invisible in the code unless somebody goes looking, and the
// tempting refactor — "why are there two routers? let me unify them" — would
// silently publish an admin API. This test is what makes that refactor fail
// loudly instead.
func TestPublicRouterHasNoAdminRoutes(t *testing.T) {
	a := offlineApp(t)

	public := chi.NewRouter()
	a.RegisterRoutes(public)

	paths := []string{
		"/admin",
		"/admin/",
		"/admin/api/login",
		"/admin/api/report",
		"/admin/api/status",
		"/admin/api/session",
		"/admin/app.js",
		"/admin/styles.css",
	}

	for _, path := range paths {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			req := httptest.NewRequest(method, path, nil)
			rec := httptest.NewRecorder()
			public.ServeHTTP(rec, req)

			// 404 or 405 — anything the SPA fallback would give an unknown
			// path. What must never happen is 200, 401 or 403: each of those
			// is the console answering, and 401 in particular announces that
			// there is something here to sign in to.
			switch rec.Code {
			case http.StatusOK, http.StatusUnauthorized, http.StatusForbidden:
				t.Errorf("%s %s: got %d — the admin console is reachable on the PUBLIC listener",
					method, path, rec.Code)
			}
			if strings.Contains(strings.ToLower(rec.Body.String()), "operator console") {
				t.Errorf("%s %s: the public listener served the console page", method, path)
			}
		}
	}
}

// And the other half: the admin router must actually carry them, so the test
// above cannot pass simply because the console was never wired up.
func TestAdminRouterHasTheConsole(t *testing.T) {
	a := offlineApp(t)

	adminRouter := chi.NewRouter()
	a.RegisterAdminRoutes(adminRouter)

	req := httptest.NewRequest(http.MethodGet, "/admin/api/report", nil)
	rec := httptest.NewRecorder()
	adminRouter.ServeHTTP(rec, req)

	// 401, because no token was sent. That is the console answering, which is
	// exactly what must not happen on the public router.
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401 — the admin router should carry a guarded report route", rec.Code)
	}
}

// A deployment that configured no way in gets no listener at all, rather than
// a port that answers 401 forever. This is the default an operator who has not
// thought about the console lands on, so it is the one worth pinning.
func TestNoConsoleWithoutADoor(t *testing.T) {
	a := offlineApp(t)
	if a.AdminConsoleConfigured() {
		t.Error("an unconfigured deployment reports a console as available")
	}
}
