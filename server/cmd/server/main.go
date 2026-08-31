package main

import (
	"context"
	"log"
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
	"zolik/server/internal/tuiauth"
)

func main() {
	_ = godotenv.Load()

	cfg := app.LoadConfig()
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
	cancel()
	if sshSrv != nil {
		_ = sshSrv
	}
	_ = srv.Shutdown(shutdownCtx)
}

// tuiBuild is the server's own build identity, handed to the SSH TUI as
// plain data — see client-tui/ui.Build's doc comment for why.
func tuiBuild() tuiui.Build {
	version, commit := buildinfo.Resolved()
	return tuiui.Build{Version: version, Commit: commit}
}

