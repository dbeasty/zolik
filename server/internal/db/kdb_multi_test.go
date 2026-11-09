package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// UpdateMulti's contract as callers rely on it: reads see the transaction's
// own writes, and a function that fails writes nothing at all.

func TestUpdateMultiReadsItsOwnWrites(t *testing.T) {
	k := openTestKDB(t)
	err := k.UpdateMulti([]string{NSMatches, NSMatchLog}, func(tx *MultiTx) error {
		tx.Put(NSMatchLog, "a", []byte(`{"n":1}`))
		got, err := tx.Get(NSMatchLog, "a")
		if err != nil || string(got) != `{"n":1}` {
			t.Errorf("own write read back as %s, %v", got, err)
		}
		tx.Delete(NSMatchLog, "a")
		if _, err := tx.Get(NSMatchLog, "a"); !IsNotFound(err) {
			t.Errorf("own delete read back as %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpdateMultiWritesNothingWhenItFails(t *testing.T) {
	k := openTestKDB(t)
	boom := errors.New("boom")
	err := k.UpdateMulti([]string{NSMatches, NSMatchLog}, func(tx *MultiTx) error {
		tx.Put(NSMatchLog, "page", []byte(`{"p":0}`))
		tx.Put(NSMatches, "match", []byte(`{"version":2}`))
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("error %v, want the function's own", err)
	}
	for ns, key := range map[string]string{NSMatchLog: "page", NSMatches: "match"} {
		if _, err := k.Get(ns, key); !IsNotFound(err) {
			t.Errorf("%s/%s was written by a failed transaction", ns, key)
		}
	}
}

func TestUpdateMultiAppliesEveryNamespace(t *testing.T) {
	k := openTestKDB(t)
	if err := k.UpdateMulti([]string{NSMatchLog, NSMatches}, func(tx *MultiTx) error {
		tx.Put(NSMatchLog, "page", []byte(`{"p":0}`))
		tx.Put(NSMatches, "match", []byte(`{"version":2}`))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for ns, key := range map[string]string{NSMatchLog: "page", NSMatches: "match"} {
		if _, err := k.Get(ns, key); err != nil {
			t.Errorf("%s/%s: %v", ns, key, err)
		}
	}
}

// The property the match log rests on: a page and the match that records it
// land together or not at all. The decision is failed after both parts are
// on disk — the state a crash between them leaves — and a reopen must drop
// both, while a transaction committed before it survives whole.
func TestUpdateMultiIsAllOrNothingAcrossARestart(t *testing.T) {
	dir := t.TempDir()
	k, err := OpenKDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	write := func(k *KDB, suffix string) error {
		return k.UpdateMulti([]string{NSMatches, NSMatchLog}, func(tx *MultiTx) error {
			tx.Put(NSMatchLog, "page"+suffix, []byte(`{"p":0}`))
			tx.Put(NSMatches, "match"+suffix, []byte(`{"version":2}`))
			return nil
		})
	}
	if err := write(k, "-kept"); err != nil {
		t.Fatalf("committed transaction: %v", err)
	}
	k.host.Transactions().SetDecisionFailureForTest(errors.New("decision log unwritable"))
	if err := write(k, "-lost"); err == nil {
		t.Fatal("a transaction whose decision failed reported success")
	}
	_ = k.Close(context.Background())

	k, err = OpenKDB(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = k.Close(context.Background()) })
	for ns, key := range map[string]string{NSMatchLog: "page-kept", NSMatches: "match-kept"} {
		if _, err := k.Get(ns, key); err != nil {
			t.Errorf("committed %s/%s lost across the restart: %v", ns, key, err)
		}
	}
	for ns, key := range map[string]string{NSMatchLog: "page-lost", NSMatches: "match-lost"} {
		if _, err := k.Get(ns, key); !IsNotFound(err) {
			t.Errorf("%s/%s from an undecided transaction survived the restart", ns, key)
		}
	}
}

// A put replaces the whole document even inside a transaction: a field the
// new body leaves out is gone, not merged over.
func TestUpdateMultiPutReplacesTheWholeDocument(t *testing.T) {
	k := openTestKDB(t)
	put := func(doc string) {
		t.Helper()
		if err := k.UpdateMulti([]string{NSMatches, NSMatchLog}, func(tx *MultiTx) error {
			tx.Put(NSMatches, "m", []byte(doc))
			tx.Put(NSMatchLog, "p", []byte(doc))
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	put(`{"version":1,"abandonAt":"soon"}`)
	put(`{"version":2}`)
	for _, ns := range []string{NSMatches, NSMatchLog} {
		got, err := k.Get(ns, map[string]string{NSMatches: "m", NSMatchLog: "p"}[ns])
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(got), "abandonAt") {
			t.Errorf("%s: a dropped field survived the replace: %s", ns, got)
		}
	}
}

// Each put keeps a version DocumentVersions can list — the history
// BoardAfter searches — even though a replace is a delete and a write.
func TestUpdateMultiKeepsOneVersionPerWrite(t *testing.T) {
	k, err := OpenKDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = k.Close(context.Background()) })
	for i := 1; i <= 3; i++ {
		if err := k.UpdateMulti([]string{NSMatches, NSMatchLog}, func(tx *MultiTx) error {
			tx.Put(NSMatches, "m", []byte(fmt.Sprintf(`{"version":%d}`, i)))
			tx.Put(NSMatchLog, "p", []byte(`{}`))
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	versions, err := k.DocumentVersions(NSMatches, "m")
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 3 {
		t.Fatalf("%d versions listed for 3 writes", len(versions))
	}
	for i, v := range versions {
		doc, err := k.GetAt(NSMatches, "m", v)
		if err != nil || !strings.Contains(string(doc), fmt.Sprintf(`"version":%d`, i+1)) {
			t.Errorf("version %d reads %s, %v", i+1, doc, err)
		}
	}
}

// A namespace the transaction did not name is a programming error, caught at
// the call rather than written outside the locks.
func TestUpdateMultiRefusesANamespaceItDoesNotHold(t *testing.T) {
	k := openTestKDB(t)
	defer func() {
		if recover() == nil {
			t.Error("writing a namespace outside the transaction did not panic")
		}
	}()
	_ = k.UpdateMulti([]string{NSMatches}, func(tx *MultiTx) error {
		tx.Put(NSMatchLog, "x", []byte(`{}`))
		return nil
	})
}

// Close holds k.mu while it waits for each namespace's lock, so nothing may
// wait for k.mu while holding one. UpdateMulti used to look each namespace up
// between taking the locks: a write caught mid-way by a shutdown held the first
// lock and waited for k.mu, and Close waited for that lock, for ever. Seen as a
// test binary hanging in closeRuntimes with a websocket's disconnect in
// UpdateMulti, about one run in fourteen under load.
func TestUpdateMultiNeverWaitsForTheDatabaseLockHoldingANamespace(t *testing.T) {
	k := openTestKDB(t)
	first := k.ns(NSMatchLog) // the first in sorted order

	// Stop UpdateMulti on its first namespace lock.
	first.mu.Lock()
	done := make(chan error, 1)
	go func() {
		done <- k.UpdateMulti([]string{NSMatches, NSMatchLog}, func(*MultiTx) error { return nil })
	}()
	time.Sleep(100 * time.Millisecond)

	// Now be Close: hold the database lock, then want that namespace's.
	k.mu.Lock()
	first.mu.Unlock()
	// Let UpdateMulti take it, or this would simply win the lock back.
	time.Sleep(100 * time.Millisecond)
	deadline := time.Now().Add(3 * time.Second)
	got := first.mu.TryLock()
	for !got && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
		got = first.mu.TryLock()
	}
	if got {
		first.mu.Unlock()
	}
	k.mu.Unlock()
	if !got {
		<-done
		t.Fatal("UpdateMulti holds a namespace lock while waiting for the database lock")
	}
	if err := <-done; err != nil {
		t.Fatalf("UpdateMulti: %v", err)
	}
}
