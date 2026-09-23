package sync

import (
	"testing"
	"time"

	"github.com/limidus/kdb/go/kdb/syncnode"

	"zolik/server/internal/db"
)

// A hub holds a namespace per person and per match, which is only affordable
// because the engine closes the ones nobody is using. This is the regression
// for what that did to a running server: the namespace was closed, this
// process went on holding the runtime it closed, and every write to it came
// back as "server is shutting down" - which is true of a drained runtime and
// says nothing about what happened. It showed up as an account being unable
// to sign in after the server had been quiet for a quarter of an hour.
func TestANamespaceClosedForIdlenessIsUsableAgain(t *testing.T) {
	const user = "65f0c0ffeec0ffeec0ffee01"
	hub := openNode(t, Config{
		Role:      RoleHub,
		Advertise: "hub",
		IdleClose: &syncnode.IdleConfig{
			MaxOpen: 1, IdleAfter: 0, MinIdle: 0, Interval: time.Hour,
		},
	}, stubVerifier{}, stubSeating{})

	for _, ns := range []string{db.NSUsers, db.UserNS(user)} {
		if err := hub.kdb.Put(ns, "doc", []byte(`{"v":1}`)); err != nil {
			t.Fatalf("first write to %s: %v", ns, err)
		}
	}

	closed := hub.node.CloseIdle(time.Now().Add(time.Hour))
	if len(closed) == 0 {
		t.Fatal("nothing was closed, so this proves nothing")
	}

	for _, ns := range []string{db.NSUsers, db.UserNS(user)} {
		if err := hub.kdb.Put(ns, "doc", []byte(`{"v":2}`)); err != nil {
			t.Fatalf("writing to %s after the engine closed it: %v", ns, err)
		}
		doc, err := hub.kdb.Get(ns, "doc")
		if err != nil {
			t.Fatalf("reading %s back: %v", ns, err)
		}
		var got struct {
			V int `bson:"v"`
		}
		if err := db.UnmarshalDoc(doc, &got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got.V != 2 {
			t.Fatalf("%s holds v=%d, want the write made after the close", ns, got.V)
		}
	}
}
