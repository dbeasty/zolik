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

// ProtocolVersion is what a guest and a host must agree on before a guest
// sits down. It rises when the wire between client and server changes in a
// way an older app cannot follow. The advertisement and /nearby/info both
// carry it, and a guest with a different number is told to update rather
// than dropped into a table it cannot read.
const ProtocolVersion = 1

// Host is one running embedded server. A process has at most one: see Start.
type Host struct {
	app        *app.App
	handler    http.Handler
	srv        *http.Server
	ln         net.Listener
	cancel     context.CancelFunc
	instanceID string
	done       chan struct{}

	// lanSrv serves the same handler on every interface while the host is
	// inviting players from the room. It is separate from the loopback
	// listener, so opening and closing it never drops the host's own socket.
	lanMu  sync.Mutex
	lanSrv *http.Server
	lanLn  *trackingListener
}

var (
	mu      sync.Mutex
	current *Host
)

// Start brings the host up on dataDir, which must be a directory the app
// owns and keeps (Application Support on iOS, filesDir on Android). It
// listens on loopback only, which is all a player alone against bots needs.
// OpenLAN lets the rest of the room in.
//
// Calling Start again while a host is running returns that host. JS reloads,
// a second screen and a quick re-tap all land here, and none of them should
// get a second database on the same directory.
func Start(dataDir string) (*Host, error) {
	mu.Lock()
	defer mu.Unlock()
	if current != nil {
		return current, nil
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

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = a.Close(context.Background())
		return nil, err
	}

	r := chi.NewRouter()
	a.RegisterMobileRoutes(r)
	r.Get("/nearby/info", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"protocol":   ProtocolVersion,
			"instanceId": id.InstanceID,
		})
	})

	ctx, cancel := context.WithCancel(context.Background())
	a.Start(ctx)

	h := &Host{
		app:        a,
		handler:    r,
		srv:        newServer(r),
		ln:         ln,
		cancel:     cancel,
		instanceID: id.InstanceID,
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

func newServer(h http.Handler) *http.Server {
	return &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second}
}

// OpenLAN starts accepting connections from other devices, on LANPort if it
// is free and on any port otherwise, and returns that port. It is idempotent:
// a host that is already open answers with the port it already has.
func (h *Host) OpenLAN() (int, error) {
	h.lanMu.Lock()
	defer h.lanMu.Unlock()
	if h.lanLn != nil {
		return h.lanLn.Addr().(*net.TCPAddr).Port, nil
	}
	raw, err := net.Listen("tcp", ":"+strconv.Itoa(LANPort))
	if err != nil {
		if raw, err = net.Listen("tcp", ":0"); err != nil {
			return 0, fmt.Errorf("lan listen: %w", err)
		}
	}
	ln := newTrackingListener(raw)
	srv := newServer(h.handler)
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("zolikcore: lan serve: %v", err)
		}
	}()
	h.lanSrv, h.lanLn = srv, ln
	log.Printf("zolikcore: host %s open to the room on %s", h.instanceID, ln.Addr())
	return ln.Addr().(*net.TCPAddr).Port, nil
}

// CloseLAN stops accepting other devices and drops the ones connected. The
// host's own loopback socket, and every table, are untouched.
func (h *Host) CloseLAN() {
	h.lanMu.Lock()
	defer h.lanMu.Unlock()
	if h.lanSrv == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = h.lanSrv.Shutdown(ctx)
	// Shutdown leaves hijacked connections alone, and every guest at a table
	// is on one: see trackingListener.
	h.lanLn.closeAll()
	h.lanSrv, h.lanLn = nil, nil
}

// LANPort is the port other devices reach the host on, or 0 when it is not
// open to the room.
func (h *Host) LANPort() int {
	h.lanMu.Lock()
	defer h.lanMu.Unlock()
	if h.lanLn == nil {
		return 0
	}
	return h.lanLn.Addr().(*net.TCPAddr).Port
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
	h.CloseLAN()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h.app.Stop(ctx)
	_ = h.srv.Shutdown(ctx)
	<-h.done
	h.cancel()
	_ = h.app.Close(ctx)
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
