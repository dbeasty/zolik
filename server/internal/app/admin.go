package app

import (
	"github.com/go-chi/chi/v5"

	"zolik/server/internal/admin"
	"zolik/server/internal/buildinfo"
	"zolik/server/internal/lobby"
)

// RegisterAdminRoutes mounts the operator console.
//
// It is a method of its own, called from a *second* http.Server on a port
// nginx does not proxy and Compose publishes to host loopback only — never
// from RegisterRoutes. That separation is the load-bearing security control in
// this feature, and it is worth being explicit about why it is a separate
// listener rather than a guarded route group on the public one.
//
// The production vhost is a single catch-all:
//
//	location / { proxy_pass http://127.0.0.1:8090; }
//
// which is deliberate and right for the game — the server knows which paths
// are API and which are the web client, so nginx enumerates nothing and cannot
// drift out of step with the router. The consequence is that **every route the
// public router registers is on the internet by construction**. A console
// mounted there would be one guard bug away from the world.
//
// The obvious alternative — `location /admin { deny all; }` — is a negative
// patch on a positive-by-default catch-all, and it has to be exactly as clever
// as every path that reaches the same handler: /admin, /Admin, //admin,
// /./admin, and whatever route the console gains next year. A route that was
// never registered on the listener needs no rule and cannot be outflanked by a
// path that normalises differently in nginx than it does in chi.
//
// So: the public router does not know these routes exist, and
// TestPublicRouterHasNoAdminRoutes is what keeps that true against a future
// tidy-up that "unifies the routers".
//
// The guard is the second layer, and the tunnel is not a substitute for it —
// see internal/admin.
func (a *App) RegisterAdminRoutes(r chi.Router) {
	guard := admin.NewGuard(a.userRepo, a.cfg.AdminEmails, admin.PasswordLogin{
		Username: a.cfg.AdminUsername,
		Hash:     a.cfg.AdminPasswordHash,
	})
	guard.SetMetrics(a.metrics)

	version, _ := buildinfo.Resolved()

	admin.NewHandlers(admin.Deps{
		Guard:         guard,
		Reports:       a.reporter,
		Live:          a.hub.Registry(),
		WaitingRoomID: lobby.RoomID,
		// Handed over as a closure so package admin never imports admission:
		// the console renders whatever this returns and knows nothing about
		// capacity gates.
		Capacity: func() any { return a.admission.Snapshot() },
		Version:  version,
	}).RegisterRoutes(r)
}

// AdminConsoleConfigured reports whether any way into the console exists.
//
// Used by main to decide whether to start the second listener at all. A
// deployment that configured no door gets no listener rather than a port
// answering 401 — there is nothing behind it to reach, and a closed port is a
// clearer statement than an open one that always says no.
func (a *App) AdminConsoleConfigured() bool {
	return len(a.cfg.AdminEmails) > 0 ||
		(a.cfg.AdminUsername != "" && a.cfg.AdminPasswordHash != "")
}
