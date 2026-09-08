package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"

	tuissH "zolik/client-tui/ssh"
	tuiui "zolik/client-tui/ui"
	"zolik/server/internal/app"
	"zolik/server/internal/buildinfo"
	"zolik/server/internal/obs"
	"zolik/server/internal/tuiauth"
)

func main() {
	_ = godotenv.Load()

	cfg := app.LoadConfig()

	// Logging first, before anything that might want to log. slog.SetDefault
	// also re-points the standard log package, so every log.Printf below and
	// throughout the server becomes a structured record without being edited.
	obs.Setup(obs.Options{Local: cfg.IsLocal(), Level: cfg.LogLevel})

	log.Printf("storage engine: %s (FEATURE_FLAG_DB_ENGINE)", cfg.DBEngine)
	a, err := app.New(cfg)
	if err != nil {
		log.Fatalf("init app: %v", err)
	}
	defer a.Close(context.Background())

	r := chi.NewRouter()
	// Bearer-token auth (no cookies), so a wide-open CORS policy carries no
	// CSRF risk here and keeps every client origin (web, LAN devices, etc.)
	// working without per-deployment configuration.
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	a.RegisterRoutes(r)

	if a.Hub() != nil {
		log.Printf("ws hub ready (instance=%s, redis=%v)", a.Hub().InstanceID(), a.Hub().RedisEnabled())
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	// The operator console, on a listener of its own.
	//
	// This is the security control the console rests on, so it is worth being
	// plain about what each part of it does. The public vhost proxies one
	// catch-all to the port above, by design, which means anything registered
	// on that router is on the internet. These routes are registered on a
	// different router, served on a different port, and that port is bound to
	// loopback — so it is unreachable from another machine even before nginx
	// is considered, and nginx is not configured to reach it either. An
	// operator gets in with `ssh -L`, using the key that already deploys this.
	//
	// Bound to loopback rather than simply unpublished by Compose because the
	// two together are what make an SSH tunnel work: Compose maps
	// 127.0.0.1:8091 on the host to this port in the container, and this bind
	// is what stops the same port being reachable on any other interface if
	// the process is ever run outside a container.
	//
	// Not started at all when no sign-in method is configured. A closed port
	// is a clearer statement than one that answers 401 forever, and it means a
	// deployment that has not thought about the console does not expose one.
	adminSrv := startAdminListener(cfg, a)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Counter flushing, the boot record, and the abandon reaper. All
	// best-effort: none of it may stop the server serving.
	a.Start(ctx)

	var sshSrv *tuissH.Server
	if cfg.SSHEnabled {
		serverURL := "http://127.0.0.1:" + cfg.Port
		var err error
		sshSrv, err = tuissH.Start(ctx, tuissH.Config{
			Addr:         tuissH.AddrFromPort(cfg.SSHPort),
			HostKeyPath:  cfg.SSHHostKeyPath,
			AllowAllKeys: cfg.SSHAllowAllKeys,
		}, tuissH.Deps{
			ServerURL: serverURL,
			Auth:      &tuiauth.Adapter{Auth: a.Auth()},
			Build:     tuiBuild(),
		})
		if err != nil {
			// The SSH terminal client is another door onto the same game, not
			// the game. Refusing to start at all because that door will not
			// open takes every web player down over a feature none of them
			// are using — which is exactly what happened when a deployment
			// stopped passing SSH_HOST_KEY_PATH and the default landed
			// somewhere the container's unprivileged user cannot write:
			// one fatal, restart, fatal, for as long as the host would keep
			// trying.
			//
			// Logged loudly rather than swallowed. Somebody who set
			// SSH_ENABLED=true meant it, and needs to be told they did not
			// get it; everybody else needs the server to still be serving.
			log.Printf("ssh server: NOT started: %v — the game server is unaffected", err)
			sshSrv = nil
		}
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Before cancel() and before Close: this is what records that the process
	// exited on purpose, and the next start reads its absence as a crash. A
	// slow database teardown must not be what turns a clean deploy into a
	// reported crash.
	a.Stop(shutdownCtx)

	cancel()
	if sshSrv != nil {
		_ = sshSrv
	}
	if adminSrv != nil {
		_ = adminSrv.Shutdown(shutdownCtx)
	}
	_ = srv.Shutdown(shutdownCtx)
}

// startAdminListener serves the operator console on the loopback-only admin
// port, or returns nil when no console is configured.
func startAdminListener(cfg app.Config, a *app.App) *http.Server {
	if !a.AdminConsoleConfigured() {
		log.Printf("admin console: off (set ADMIN_USERNAME and ADMIN_PASSWORD_HASH to enable)")
		return nil
	}

	adminRouter := chi.NewRouter()
	// No CORS. The console is served from the same origin it calls, and the
	// wide-open policy the game uses exists for LAN devices and native
	// clients — neither of which reaches this port.
	a.RegisterAdminRoutes(adminRouter)

	addr := net.JoinHostPort(cfg.AdminBind, cfg.AdminPort)
	srv := &http.Server{
		// Always an explicit interface, never the http default of every
		// interface. Which one is Config.AdminBind's job: 127.0.0.1 on a bare
		// host, where this line is the only thing standing between the
		// console and the network, and 0.0.0.0 in the container, where the
		// published-port binding is that thing instead and a loopback bind
		// here would only make the console unreachable. See Config.AdminBind.
		Addr:              addr,
		Handler:           adminRouter,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		// The tunnel in this line targets the *host's* loopback, which is
		// where the published port lands — not addr, which is the interface
		// inside the container and is nobody's destination.
		log.Printf("admin console on %s (reach it with: ssh -N -L %s:127.0.0.1:%s <host>)",
			addr, cfg.AdminPort, cfg.AdminPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Not fatal. A console that cannot bind is a console nobody can
			// read; it is not a reason to stop serving games.
			log.Printf("admin console: NOT started: %v — the game server is unaffected", err)
		}
	}()
	return srv
}

// tuiBuild is the server's own build identity, handed to the SSH TUI as
// plain data — see client-tui/ui.Build's doc comment for why.
func tuiBuild() tuiui.Build {
	version, commit := buildinfo.Resolved()
	return tuiui.Build{Version: version, Commit: commit}
}
