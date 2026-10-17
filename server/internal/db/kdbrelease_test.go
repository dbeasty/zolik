package db

import (
	"strings"
	"testing"
	"time"
)

func withReleaseDelay(t *testing.T, d time.Duration) {
	t.Helper()
	old := matchReleaseDelay
	matchReleaseDelay = d
	t.Cleanup(func() { matchReleaseDelay = old })
}

// matchIsOpen is true while the match's namespace is in the set or this
// package still holds a handle to it. A release removes the first and then the
// second, so "closed" means both are gone.
func matchIsOpen(k *KDB, hex string) bool {
	_, open := k.set.Runtimes()[kdbCatalog+"/"+MatchNS(hex)]
	k.mu.RLock()
	_, held := k.nss[MatchNS(hex)]
	k.mu.RUnlock()
	return open || held
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// A finished match's namespace is closed, and its data is still there: the next
// read opens it again from disk.
func TestReleaseMatchClosesTheNamespaceAndItReopens(t *testing.T) {
	withReleaseDelay(t, 20*time.Millisecond)
	k := openTestKDB(t)
	ns := MatchNS("65f0aaaa")
	if err := k.Put(ns, "envelope", []byte(`{"status":"completed"}`)); err != nil {
		t.Fatal(err)
	}
	if !matchIsOpen(k, "65f0aaaa") {
		t.Fatal("the match namespace should be open after a write")
	}

	k.ReleaseMatch("65f0aaaa")
	waitFor(t, "the namespace to close", func() bool { return !matchIsOpen(k, "65f0aaaa") })

	got, err := k.Get(ns, "envelope")
	if err != nil || !strings.Contains(string(got), `"status":"completed"`) {
		t.Fatalf("read after release = %s, %v", got, err)
	}
	if !matchIsOpen(k, "65f0aaaa") {
		t.Fatal("a read should have reopened the namespace")
	}
	// ...and it takes writes again, which a stale handle would refuse.
	if err := k.Put(ns, "envelope", []byte(`{"status":"completed","n":2}`)); err != nil {
		t.Fatalf("write after reopen: %v", err)
	}
}

// Only the named match goes: other matches and the fixed collections stay open.
func TestReleaseMatchLeavesEverythingElseOpen(t *testing.T) {
	withReleaseDelay(t, 20*time.Millisecond)
	k := openTestKDB(t)
	for _, hex := range []string{"65f0bbbb", "65f0cccc"} {
		if err := k.Put(MatchNS(hex), "envelope", []byte(`{}`)); err != nil {
			t.Fatal(err)
		}
	}
	k.ReleaseMatch("65f0bbbb")
	waitFor(t, "the first match to close", func() bool { return !matchIsOpen(k, "65f0bbbb") })
	if !matchIsOpen(k, "65f0cccc") {
		t.Fatal("releasing one match closed another")
	}
	if _, open := k.set.Runtimes()[kdbCatalog+"/"+NSUsers]; !open {
		t.Fatal("releasing a match closed the users collection")
	}
	if err := k.Put(NSUsers, "u1", []byte(`{"username":"ada"}`)); err != nil {
		t.Fatalf("users collection unusable after a release: %v", err)
	}
}

// Releasing a match that was never opened, or twice, or on an in-memory
// database, is harmless.
func TestReleaseMatchIsHarmlessWhenThereIsNothingToClose(t *testing.T) {
	withReleaseDelay(t, time.Millisecond)
	k := openTestKDB(t)
	k.ReleaseMatch("65f0dddd")
	k.ReleaseMatch("")
	mem, err := OpenKDB("")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mem.Close(t.Context()) })
	mem.ReleaseMatch("65f0eeee")
	time.Sleep(50 * time.Millisecond)
}

// The sweep closes match namespaces and nothing else.
func TestMatchIdleSweepOnlyTouchesMatches(t *testing.T) {
	k := openTestKDB(t)
	if err := k.Put(MatchNS("65f0ffff"), "envelope", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	// One sweep as if an hour had passed.
	closed := k.set.CloseIdle(k.matchIdlePolicy(), time.Now().Add(time.Hour))
	if len(closed) != 1 || closed[0] != kdbCatalog+"/"+MatchNS("65f0ffff") {
		t.Fatalf("sweep closed %v, want only the match namespace", closed)
	}
}
