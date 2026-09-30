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
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
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
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"zolik/server/internal/app"
	"zolik/server/internal/auth"
	"zolik/server/internal/db"
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
	cloudKeys  *auth.JWKSCache
	bleKey     *ecdh.PrivateKey
	srv        *http.Server
	ln         net.Listener
	cancel     context.CancelFunc
	instanceID string
	// userHex is the account this host replicates for, empty when it is only
	// hosting a table.
	userHex string
	done    chan struct{}

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
	return StartNode(dataDir, "", "", "")
}

// StartNode is Start for a phone that has been enrolled with the cloud and has
// somebody signed in: the same host, which additionally replicates that
// account's data and serves it back from this device.
//
// nodeCredential is what the cloud handed this install at enrolment, and
// userHex is the account signed in now. With either missing the host comes up
// exactly as Start's does: a table for the room, and nothing synced anywhere.
//
// cloudBaseURL is the server the app itself is talking to. It is passed in
// rather than assumed, because a development build points at a server on the
// same machine, and a host that synced with production while the app read
// from localhost would be two different databases wearing one account.
//
// A host already running for a different account is not silently re-pointed.
// The database on disk is that account's, and swapping who it belongs to
// underneath a live table is not something a sign-in should do quietly; the
// caller stops the host first.
func StartNode(dataDir, nodeCredential, userHex, cloudBaseURL string) (*Host, error) {
	mu.Lock()
	defer mu.Unlock()
	want := strings.TrimSpace(userHex)
	if current != nil {
		switch {
		case current.userHex == want:
			return current, nil
		case want == "":
			// Naming nobody is not naming somebody else. Hosting a table for
			// the room is a mode of the same host, and the app asks for it
			// that way - with no account - while the person may well be
			// signed in and this host may well be replicating their data.
			// Refusing would mean a signed-in player could not start an
			// offline table at all.
			return current, nil
		case current.userHex == "":
			// A host that was serving nobody, and somebody has just signed
			// in. Whether a host replicates is fixed when it opens, so this
			// is a restart rather than a change of mind: the database on disk
			// is the same one either way, and what is lost is a live
			// connection rather than a table.
			current.stopLocked()
		default:
			return nil, fmt.Errorf("a host is already running for %s", describeAccount(current.userHex))
		}
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

	a, cloudKeys, err := app.NewMobile(dataDir, app.MobileIdentity{
		AccessSecret:   id.AccessSecret,
		NodeKeySeed:    id.NodeKey,
		NodeCredential: strings.TrimSpace(nodeCredential),
		UserHex:        strings.TrimSpace(userHex),
		CloudBaseURL:   strings.TrimSpace(cloudBaseURL),
	})
	if err != nil {
		return nil, fmt.Errorf("init: %w", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = a.Close(context.Background())
		return nil, err
	}

	rawKey, _ := hex.DecodeString(id.BLEKey)
	bleKey, err := ecdh.X25519().NewPrivateKey(rawKey)
	if err != nil {
		_ = a.Close(context.Background())
		_ = ln.Close()
		return nil, fmt.Errorf("ble key: %w", err)
	}
	publicKey := base64.StdEncoding.EncodeToString(bleKey.PublicKey().Bytes())

	r := chi.NewRouter()
	if strings.TrimSpace(userHex) != "" {
		// The signed-in player's own history, settings and circle, served
		// from the copy this device holds, so the app can show them with no
		// connection at all.
		a.RegisterNodeRoutes(r)
	} else {
		a.RegisterMobileRoutes(r)
	}
	// publicKey is the Bluetooth tunnel's static key. A guest that met this
	// table over Wi-Fi first pins it here, so a later Bluetooth join is
	// authenticated rather than trusted on first use.
	r.Get("/nearby/info", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"protocol":   ProtocolVersion,
			"instanceId": id.InstanceID,
			"publicKey":  publicKey,
		})
	})

	ctx, cancel := context.WithCancel(context.Background())
	a.Start(ctx)

	h := &Host{
		app:        a,
		handler:    r,
		cloudKeys:  cloudKeys,
		bleKey:     bleKey,
		srv:        newServer(r),
		ln:         ln,
		cancel:     cancel,
		instanceID: id.InstanceID,
		userHex:    strings.TrimSpace(userHex),
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

// describeAccount names an account for a refusal, without putting an id the
// player never chose in front of them.
func describeAccount(userHex string) string {
	if userHex == "" {
		return "an offline table"
	}
	return "another account"
}

// SyncNow replicates with the cloud immediately, for the moments where waiting
// for the next tick would be visible: the app coming to the foreground, the
// network coming back, a match ending. It is a no-op on a host that is not
// enrolled or has nobody signed in.
func (h *Host) SyncNow() error {
	if h == nil || h.app.Sync() == nil {
		return nil
	}
	return h.app.Sync().SyncNow()
}

// ReplicaReady reports whether this device is holding the signed-in account's
// data yet, so the app can show a history screen rather than an empty one that
// looks like a person who has never played.
func (h *Host) ReplicaReady() bool {
	if h == nil {
		return false
	}
	return h.app.Replica().Ready()
}

// FollowMatch adds a match to what this device syncs, for a match the player
// opened that was started somewhere else.
func (h *Host) FollowMatch(matchHex string) error {
	if h == nil || h.app.Sync() == nil {
		return nil
	}
	return h.app.Sync().Follow(db.MatchNS(matchHex))
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

// PublicKey is the Bluetooth tunnel's static public key, base64.
func (h *Host) PublicKey() string {
	return base64.StdEncoding.EncodeToString(h.bleKey.PublicKey().Bytes())
}

// NodeID names this install as a replica, derived from its node key. It is
// what a seat receipt issued at this table says, and what the cloud records
// when the install enrols.
func (h *Host) NodeID() string { return auth.NodeID() }

// NodePublicKey is the key half a node hands the cloud when it enrols,
// base64url. The private half never leaves the phone.
func (h *Host) NodePublicKey() string {
	pub, ok := auth.NodePublicKey()
	if !ok {
		return ""
	}
	return auth.EncodeNodePublicKey(pub)
}

// RefreshCloudKeys fetches the cloud's signing keys and writes them beside
// the database, so that this host can verify an offline pass later with no
// connection at all. The app calls it whenever it has one: the point of the
// cached copy is that it was fetched at a better moment than the one where it
// is needed.
func (h *Host) RefreshCloudKeys() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return h.cloudKeys.Refresh(ctx)
}

// BLEInfo is what the GATT info characteristic answers with: everything a
// guest needs to decide whether to join before it spends a handshake.
// An advertisement has no room for it: iOS strips all but the service UUID.
func (h *Host) BLEInfo(name string) []byte {
	raw, _ := json.Marshal(map[string]any{
		"v":  ProtocolVersion,
		"id": h.instanceID,
		"n":  name,
		"pk": h.PublicKey(),
	})
	return raw
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
	h.stopLocked()
}

// stopLocked is Stop with the package lock already held, for the callers that
// are already in it - StartNode, which has to put a host down before it can
// bring the same database up replicating for somebody who has just signed in.
func (h *Host) stopLocked() {
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
	InstanceID   string `json:"instanceId"`
	AccessSecret string `json:"accessSecret"`
	// NodeKey is this install's Ed25519 seed, hex: the key it signs as a node
	// with, as opposed to the access secret it signs tokens with for itself.
	// It is what a seat taken at this table can be checked against afterwards,
	// by the cloud or by another phone, so like the BLE key it has to last as
	// long as the install does.
	NodeKey string `json:"nodeKey"`
	// BLEKey is the host's static X25519 private key for the Bluetooth
	// tunnel, hex. Guests pin its public half per instance id, so it has to
	// last as long as the install does.
	BLEKey string `json:"bleKey"`
}

// loadIdentity reads host.json, fills in anything missing (a file from an
// older build has no BLE key yet) and writes it back if it changed. The
// parts that were already there never change: they are what guests and
// their tokens recognise this host by.
func loadIdentity(dataDir string) (identity, error) {
	path := filepath.Join(dataDir, "host.json")
	var id identity
	if raw, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(raw, &id)
	}
	changed := false
	if id.InstanceID == "" || len(id.AccessSecret) < 32 {
		id.InstanceID, id.AccessSecret = randomHex(8), randomHex(32)
		changed = true
	}
	if k, err := hex.DecodeString(id.BLEKey); err != nil || len(k) != 32 {
		id.BLEKey = randomHex(32)
		changed = true
	}
	// An install from before nodes existed gains one here, exactly the way it
	// gained a BLE key. The parts already in the file are untouched, so the
	// host keeps the identity its guests and their tokens know it by.
	if k, err := hex.DecodeString(id.NodeKey); err != nil || len(k) != 32 {
		id.NodeKey = randomHex(32)
		changed = true
	}
	if !changed {
		return id, nil
	}
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
