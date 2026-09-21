// Package zolikcore is the game server as the iOS and Android apps embed it.
//
// A phone that hosts a table runs this package in its own process. It is the
// same runtime as play.limidus.com, with the same modules and bots, reduced to
// what a table with no internet needs (see app.RegisterMobileRoutes). The
// client talks to it over HTTP and WebSocket exactly as it talks to the real
// server. Only the address differs.
//
// The API is shaped for `gomobile bind`: exported types, and only the
// parameter types gobind can carry (string, bool, int, error, pointers to
// exported structs). Swift sees ZolikcoreStart and Kotlin sees
// Zolikcore.start.
package zolikcore

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"zolik/server/internal/app"
)

// LANPort is where a host listens when other phones should reach it. It is
// fixed, so a QR code or a typed address stays valid across restarts, and
// uncommon, so it rarely collides. If it is taken, the host falls back to any
// free port.
const LANPort = 47800

// Host is one running embedded server. A process has at most one: see Start.
type Host struct {
	app        *app.App
	srv        *http.Server
	ln         net.Listener
	cancel     context.CancelFunc
	instanceID string
	lan        bool
	done       chan struct{}
}

var (
	mu      sync.Mutex
	current *Host
)

// Start brings the host up on dataDir, which must be a directory the app
// owns and keeps (Application Support on iOS, filesDir on Android). With lan
// false it listens on loopback only, which is enough for a player alone
// against bots. With lan true it listens on every interface, so other phones
// in the room can join.
//
// Calling Start again while a host is running returns that host if lan
// matches, and an error if it does not. JS reloads, a second screen and a
// quick re-tap all land here, and none of them should get a second database
// on the same directory.
func Start(dataDir string, lan bool) (*Host, error) {
	mu.Lock()
	defer mu.Unlock()
	if current != nil {
		if current.lan == lan {
			return current, nil
		}
		return nil, fmt.Errorf("a host is already running with lan=%v; stop it first", current.lan)
	}
	if dataDir == "" {
		return nil, errors.New("dataDir is required")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("data dir: %w", err)
	}
	id, err := loadIdentity(dataDir)
	if err != nil {
		return nil, err
	}

	a, err := app.NewMobile(dataDir, id.AccessSecret, id.RefreshSecret)
	if err != nil {
		return nil, fmt.Errorf("init: %w", err)
	}

	ln, err := listen(lan)
	if err != nil {
		_ = a.Close(context.Background())
		return nil, err
	}

	r := chi.NewRouter()
	a.RegisterMobileRoutes(r)

	ctx, cancel := context.WithCancel(context.Background())
	a.Start(ctx)

	h := &Host{
		app:        a,
		srv:        &http.Server{Handler: r, ReadHeaderTimeout: 10 * time.Second},
		ln:         ln,
		cancel:     cancel,
		instanceID: id.InstanceID,
		lan:        lan,
		done:       make(chan struct{}),
	}
	go func() {
		defer close(h.done)
		if err := h.srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("zolikcore: serve: %v", err)
		}
	}()
	log.Printf("zolikcore: host %s listening on %s", id.InstanceID, ln.Addr())
	current = h
	return h, nil
}

// Current is the running host, or nil.
func Current() *Host {
	mu.Lock()
	defer mu.Unlock()
	return current
}

// Port is the TCP port the host accepts connections on.
func (h *Host) Port() int { return h.ln.Addr().(*net.TCPAddr).Port }

// InstanceID names this install's host. It is stable across restarts, so a
// guest can tell "the same table came back" from "a different phone".
func (h *Host) InstanceID() string { return h.instanceID }

// LAN reports whether other devices can reach this host.
func (h *Host) LAN() bool { return h.lan }

// BaseURL is the address the phone's own client uses to reach the host.
func (h *Host) BaseURL() string { return "http://127.0.0.1:" + strconv.Itoa(h.Port()) }

// Stop shuts the host down and releases its database. Tables are kept on disk
// and resume on the next Start.
func (h *Host) Stop() {
	mu.Lock()
	defer mu.Unlock()
	if current != h {
		return
	}
	current = nil
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h.app.Stop(ctx)
	_ = h.srv.Shutdown(ctx)
	<-h.done
	h.cancel()
	_ = h.app.Close(ctx)
}

func listen(lan bool) (net.Listener, error) {
	if !lan {
		return net.Listen("tcp", "127.0.0.1:0")
	}
	if ln, err := net.Listen("tcp", ":"+strconv.Itoa(LANPort)); err == nil {
		return ln, nil
	}
	return net.Listen("tcp", ":0")
}

// identity is what an install keeps about itself between runs. It is kept
// beside the database, so a player who deletes the app's data gets a new
// host rather than one that cannot read its own tokens.
type identity struct {
	InstanceID    string `json:"instanceId"`
	AccessSecret  string `json:"accessSecret"`
	RefreshSecret string `json:"refreshSecret"`
}

func loadIdentity(dataDir string) (identity, error) {
	path := filepath.Join(dataDir, "host.json")
	var id identity
	if raw, err := os.ReadFile(path); err == nil {
		if json.Unmarshal(raw, &id) == nil && id.InstanceID != "" &&
			len(id.AccessSecret) >= 32 && len(id.RefreshSecret) >= 32 {
			return id, nil
		}
	}
	id = identity{InstanceID: randomHex(8), AccessSecret: randomHex(32), RefreshSecret: randomHex(32)}
	raw, _ := json.Marshal(id)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return identity{}, fmt.Errorf("write host identity: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return identity{}, fmt.Errorf("write host identity: %w", err)
	}
	return id, nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand: " + err.Error())
	}
	return hex.EncodeToString(b)
}
