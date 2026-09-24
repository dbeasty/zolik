package sync

import (
	"errors"
	"net/http/httptest"
	"testing"

	kdbserver "github.com/limidus/kdb/go/kdb/server"

	"zolik/server/internal/db"
)

// isNotHome reports whether a write was refused because it reached a node
// that is not where the namespace is written.
func isNotHome(err error) bool {
	var notHome *kdbserver.NotHomeError
	return errors.As(err, &notHome)
}

// Which server is the root one, as a fact in the database rather than a flag
// each server reads about itself.
//
// An environment variable is no use to anybody else: a second server brought
// up with the same one would believe the same thing, and nothing would
// contradict either of them. A home assignment replicates, and the engine
// enforces it, which is what makes "the accounts are written over there"
// something another node can act on rather than something an operator has to
// remember.
func TestTheRootServerClaimsWhatOnlyItWrites(t *testing.T) {
	hub := openNode(t, Config{Role: RoleHub, Advertise: "hub"}, stubVerifier{}, stubSeating{})

	for _, name := range CanonicalNamespaces() {
		rt, err := hub.node.runtime(name)
		if err != nil {
			t.Fatalf("resolving %s: %v", name, err)
		}
		home, ok := rt.HomeOf()
		if !ok {
			t.Fatalf("%s has no home: nothing says which server writes it", name)
		}
		if home.Node != hub.node.NodeID() {
			t.Fatalf("%s is homed on %s, want this node", name, home.Node)
		}
		if home.Addr != "hub" {
			t.Fatalf("%s names %q as where to reach its home, want the advertised address", name, home.Addr)
		}
	}

	// The accounts are the point of the exercise: a name has to mean one
	// person, and that only holds while one server decides.
	if !containsName(CanonicalNamespaces(), db.NSUsers) {
		t.Fatal("the users namespace is not among the ones the root server claims")
	}
}

// TestAnotherServerCannotWriteWhatTheRootOwns is the half that matters: the
// assignment is not a label, it is enforced.
func TestAnotherServerCannotWriteWhatTheRootOwns(t *testing.T) {
	verifier := stubVerifier{"region": {NodeID: "region", Server: true}}
	hub := openNode(t, Config{Role: RoleHub, Advertise: "hub"}, verifier, stubSeating{})
	server := httptest.NewServer(hub.node.Handler())
	t.Cleanup(server.Close)

	// A second server of ours, syncing the canonical namespaces.
	region := openNode(t, Config{
		Role: RoleSpoke, HubURL: wsURL(server), Token: "region", Advertise: "region",
		Namespaces: []string{db.NSUsers},
	}, verifier, stubSeating{})

	if err := hub.kdb.Put(db.NSUsers, "u1", []byte(`{"username":"ada"}`)); err != nil {
		t.Fatalf("the root server writing an account: %v", err)
	}
	if err := region.node.SyncNow(); err != nil {
		t.Fatalf("the region pulling accounts: %v", err)
	}
	eventually(t, "the account to reach the region", func() bool {
		_, err := region.kdb.Get(db.NSUsers, "u1")
		return err == nil
	})

	// And now the thing that must not happen: the region minting an account
	// of its own. The write is refused where it is made, by the fence, rather
	// than being made and merged into a second answer to "who is ada".
	err := region.kdb.Put(db.NSUsers, "u2", []byte(`{"username":"ada"}`))
	if err == nil {
		t.Fatal("a second server wrote an account the root server owns")
	}
	if !isNotHome(err) {
		t.Fatalf("the refusal was %v, want one naming where the home is", err)
	}
}

func containsName(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
