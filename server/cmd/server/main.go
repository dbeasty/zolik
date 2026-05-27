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
	"github.com/joho/godotenv"

	tuissH "zolik/client-tui/ssh"
	"zolik/server/internal/app"
	"zolik/server/internal/tuiauth"
)

func main() {
	_ = godotenv.Load()

	cfg := app.LoadConfig()
	a, err := app.New(cfg)
	if err != nil {
		log.Fatalf("init app: %v", err)
	}
	defer a.Close(context.Background())

	r := chi.NewRouter()
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
		})
		if err != nil {
			log.Fatalf("ssh server: %v", err)
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

