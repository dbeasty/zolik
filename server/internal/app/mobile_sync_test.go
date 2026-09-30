package app

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"slices"
	"testing"

	"zolik/server/internal/auth"
	"zolik/server/internal/db"
)

// A phone that plays a match with no internet writes it to its outbox and
// waits for a connection. What carries it up is replication, and replication
// only moves namespaces the peer asked for — so an outbox left out of that
// list is a match that is written, looks recorded, and never leaves the
// device. This is that list.
func TestAPhoneReplicatesItsOwnOutbox(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	t.Cleanup(auth.SnapshotKeysForTest())
	// NewMobile installs this install's keys process-wide, and a test that
	// leaves the embedded secret behind makes the next one believe a
	// production deployment has one.
	t.Cleanup(func() { auth.SetAccessSecret("") })

	const user = "6ab5a0b3d71e872332ed588e"
	a, _, err := NewMobile(t.TempDir(), MobileIdentity{
		AccessSecret:   "a-development-secret-of-at-least-32-bytes",
		NodeKeySeed:    hex.EncodeToString(seed),
		NodeCredential: "credential-from-enrolment",
		UserHex:        user,
		CloudBaseURL:   "http://127.0.0.1:1",
	})
	if err != nil {
		t.Fatalf("NewMobile: %v", err)
	}
	t.Cleanup(func() { _ = a.Close(context.Background()) })

	want := db.NodeOutboxNS(auth.NodeID())
	if !slices.Contains(a.cfg.Sync.Namespaces, want) {
		t.Fatalf("this phone syncs %v, which does not include its outbox %s", a.cfg.Sync.Namespaces, want)
	}
	// The account's own two namespaces are the rest of what it asks for.
	for _, ns := range []string{db.UserNS(user), db.UserReadOnlyNS(user)} {
		if !slices.Contains(a.cfg.Sync.Namespaces, ns) {
			t.Errorf("this phone syncs %v, which does not include %s", a.cfg.Sync.Namespaces, ns)
		}
	}
}
