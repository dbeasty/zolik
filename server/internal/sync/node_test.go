package sync

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"zolik/server/internal/db"
)

// stubVerifier hands out the identity a test says a token means. Signing is
// internal/auth's business; what this package has to get right is what an
// identity is then allowed to do.
type stubVerifier map[string]Identity

func (v stubVerifier) VerifySyncToken(_ context.Context, token string) (Identity, error) {
	id, ok := v[token]
	if !ok {
		return Identity{}, ErrDenied
	}
	return id, nil
}

// stubSeating answers who is at which match.
type stubSeating map[string][]string

func (s stubSeating) Seated(_ context.Context, matchHex, userHex string) (bool, error) {
	for _, u := range s[matchHex] {
		if u == userHex {
			return true, nil
		}
	}
	return false, nil
}

type testNode struct {
	kdb  *db.KDB
	node *Node
}

func openNode(t *testing.T, cfg Config, v Verifier, s MatchSeating) *testNode {
	t.Helper()
	dir := t.TempDir()
	k, err := db.OpenKDB(dir)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	t.Cleanup(func() { _ = k.Close(context.Background()) })
	cfg.DataDir = dir
	n, err := Open(k, cfg, v, s)
	if err != nil {
		t.Fatalf("opening node: %v", err)
	}
	if err := n.Start(); err != nil {
		t.Fatalf("starting node: %v", err)
	}
	t.Cleanup(func() { _ = n.Close() })
	return &testNode{kdb: k, node: n}
}

// eventually retries until the condition holds. A sync is asked for and
// awaited explicitly in these tests, but the ingest that follows it is another
// node's work, so what is being waited for is arrival, not scheduling.
func eventually(t *testing.T, what string, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestAPhoneSyncsItsOwnUserWithTheHub(t *testing.T) {
	const user = "65f0c0ffeec0ffeec0ffee01"
	verifier := stubVerifier{
		"phone-token": {NodeID: "phone", OwnerHex: user},
	}
	seating := stubSeating{}

	hub := openNode(t, Config{Role: RoleHub}, verifier, seating)
	server := httptest.NewServer(hub.node.Handler())
	t.Cleanup(server.Close)

	phone := openNode(t, Config{
		Role:       RoleSpoke,
		HubURL:     "ws://" + strings.TrimPrefix(server.URL, "http://") + "/kdb/sync",
		Token:      "phone-token",
		Namespaces: []string{db.UserNS(user), db.UserReadOnlyNS(user)},
	}, verifier, seating)

	// What the cloud writes for this person reaches their phone.
	if err := hub.kdb.Put(db.UserReadOnlyNS(user), "stats", []byte(`{"_kind":"stats","played":7}`)); err != nil {
		t.Fatalf("hub write: %v", err)
	}
	if err := phone.node.SyncNow(); err != nil {
		t.Fatalf("sync: %v", err)
	}
	eventually(t, "the hub's write to reach the phone", func() bool {
		doc, err := phone.kdb.Get(db.UserReadOnlyNS(user), "stats")
		return err == nil && strings.Contains(string(doc), "7")
	})

	// And what the person changes on their phone reaches the cloud.
	if err := phone.kdb.Put(db.UserNS(user), "prefs", []byte(`{"_kind":"prefs","cardStyle":"vector"}`)); err != nil {
		t.Fatalf("phone write: %v", err)
	}
	if err := phone.node.SyncNow(); err != nil {
		t.Fatalf("sync back: %v", err)
	}
	eventually(t, "the phone's write to reach the hub", func() bool {
		doc, err := hub.kdb.Get(db.UserNS(user), "prefs")
		return err == nil && strings.Contains(string(doc), "vector")
	})
}

func TestAPhoneCannotSyncSomebodyElsesNamespaces(t *testing.T) {
	const mine = "65f0c0ffeec0ffeec0ffee01"
	const theirs = "65f0c0ffeec0ffeec0ffee02"
	verifier := stubVerifier{"phone-token": {NodeID: "phone", OwnerHex: mine}}
	seating := stubSeating{}

	hub := openNode(t, Config{Role: RoleHub}, verifier, seating)
	server := httptest.NewServer(hub.node.Handler())
	t.Cleanup(server.Close)

	if err := hub.kdb.Put(db.UserNS(theirs), "prefs", []byte(`{"_kind":"prefs","secret":true}`)); err != nil {
		t.Fatalf("hub write: %v", err)
	}

	phone := openNode(t, Config{
		Role:       RoleSpoke,
		HubURL:     "ws://" + strings.TrimPrefix(server.URL, "http://") + "/kdb/sync",
		Token:      "phone-token",
		Namespaces: []string{db.UserNS(theirs)},
	}, verifier, seating)

	// The sync itself may or may not report an error depending on how far it
	// got; what must be true is that the data did not arrive.
	_ = phone.node.SyncNow()
	time.Sleep(500 * time.Millisecond)
	if _, err := phone.kdb.Get(db.UserNS(theirs), "prefs"); err == nil {
		t.Fatal("another person's namespace was synced to this phone")
	}
}

func TestAPhoneCannotPushWhatTheCloudWrites(t *testing.T) {
	const user = "65f0c0ffeec0ffeec0ffee01"
	verifier := stubVerifier{"phone-token": {NodeID: "phone", OwnerHex: user}}
	seating := stubSeating{}

	hub := openNode(t, Config{Role: RoleHub}, verifier, seating)
	server := httptest.NewServer(hub.node.Handler())
	t.Cleanup(server.Close)

	phone := openNode(t, Config{
		Role:       RoleSpoke,
		HubURL:     "ws://" + strings.TrimPrefix(server.URL, "http://") + "/kdb/sync",
		Token:      "phone-token",
		Namespaces: []string{db.UserReadOnlyNS(user)},
	}, verifier, seating)

	// A modified client awarding itself a rating.
	if err := phone.kdb.Put(db.UserReadOnlyNS(user), "stats", []byte(`{"_kind":"stats","rating":9999}`)); err != nil {
		t.Fatalf("phone write: %v", err)
	}
	_ = phone.node.SyncNow()
	time.Sleep(500 * time.Millisecond)
	if doc, err := hub.kdb.Get(db.UserReadOnlyNS(user), "stats"); err == nil {
		t.Fatalf("the phone pushed into the cloud's own namespace: %s", doc)
	}
}

func TestAPhoneFollowsAMatchItIsSeatedAt(t *testing.T) {
	const user = "65f0c0ffeec0ffeec0ffee01"
	const match = "65f0dead65f0dead65f0dead"
	verifier := stubVerifier{"phone-token": {NodeID: "phone", OwnerHex: user}}
	seating := stubSeating{match: {user}}

	hub := openNode(t, Config{Role: RoleHub}, verifier, seating)
	server := httptest.NewServer(hub.node.Handler())
	t.Cleanup(server.Close)

	phone := openNode(t, Config{
		Role:       RoleSpoke,
		HubURL:     "ws://" + strings.TrimPrefix(server.URL, "http://") + "/kdb/sync",
		Token:      "phone-token",
		Namespaces: []string{db.UserNS(user)},
	}, verifier, seating)

	if err := hub.kdb.Put(db.MatchNS(match), "m/1", []byte(`{"seq":1,"action":"draw"}`)); err != nil {
		t.Fatalf("hub write: %v", err)
	}
	// The match is not in the phone's set until it joins one.
	if err := phone.node.Follow(db.MatchNS(match)); err != nil {
		t.Fatalf("follow: %v", err)
	}
	if err := phone.node.SyncNow(); err != nil {
		t.Fatalf("sync: %v", err)
	}
	eventually(t, "the match to reach the phone", func() bool {
		doc, err := phone.kdb.Get(db.MatchNS(match), "m/1")
		return err == nil && strings.Contains(string(doc), "draw")
	})

	// Leaving stops it following, so signing out or finishing with a match
	// does not leave a phone syncing it forever.
	if err := phone.node.Unfollow(db.MatchNS(match)); err != nil {
		t.Fatalf("unfollow: %v", err)
	}
	if err := hub.kdb.Put(db.MatchNS(match), "m/2", []byte(`{"seq":2,"action":"discard"}`)); err != nil {
		t.Fatalf("hub write: %v", err)
	}
	_ = phone.node.SyncNow()
	time.Sleep(500 * time.Millisecond)
	if _, err := phone.kdb.Get(db.MatchNS(match), "m/2"); err == nil {
		t.Fatal("the phone kept syncing a match it stopped following")
	}
}

func TestASecureHubIsDialledWithTLS(t *testing.T) {
	// The engine refuses a wss:// peer that was given no TLS settings rather
	// than falling back to plaintext, which is right and is easy to be caught
	// by: every deployment is wss and every test is ws.
	if tlsFor("wss://jokerless.com/kdb/sync") == nil {
		t.Fatal("a secure hub was going to be dialled with no TLS settings")
	}
	if got := tlsFor("wss://jokerless.com/kdb/sync"); got.InsecureSkipVerify {
		t.Fatal("verification was turned off")
	}
	if tlsFor("ws://127.0.0.1:8099/kdb/sync") != nil {
		t.Fatal("a plaintext hub was given TLS settings")
	}
}
