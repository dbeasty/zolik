package db

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// openTestKDB opens an on-disk engine in the test's temp dir — the same code
// path production runs, minus nothing.
func openTestKDB(t *testing.T) *KDB {
	t.Helper()
	k, err := OpenKDB(t.TempDir())
	if err != nil {
		t.Fatalf("opening kdb: %v", err)
	}
	t.Cleanup(func() { _ = k.Close(context.Background()) })
	return k
}

func TestKDBPutGetRoundTrip(t *testing.T) {
	k := openTestKDB(t)

	if err := k.Put(NSUsers, "u1", []byte(`{"username":"ada","n":1}`)); err != nil {
		t.Fatalf("put: %v", err)
	}
	doc, err := k.Get(NSUsers, "u1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(doc) == "" {
		t.Fatal("empty document back")
	}

	// Replace must be whole-document, not a merge: a key dropped by the
	// rewrite must not survive it.
	if err := k.Put(NSUsers, "u1", []byte(`{"username":"ada"}`)); err != nil {
		t.Fatalf("replace: %v", err)
	}
	doc, err = k.Get(NSUsers, "u1")
	if err != nil {
		t.Fatalf("get after replace: %v", err)
	}
	var probe struct {
		N *int `bson:"n"`
	}
	if err := UnmarshalDoc(doc, &probe); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if probe.N != nil {
		t.Fatalf("replace merged instead of swapping: n=%d survived", *probe.N)
	}
}

func TestKDBGetMissingIsNotFound(t *testing.T) {
	k := openTestKDB(t)
	_, err := k.Get(NSUsers, "nope")
	if !IsNotFound(err) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestKDBInsertRefusesDuplicates(t *testing.T) {
	k := openTestKDB(t)
	if err := k.Insert(NSSessions, "tok", []byte(`{"token":"tok"}`)); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	err := k.Insert(NSSessions, "tok", []byte(`{"token":"tok"}`))
	if !IsDuplicateKey(err) {
		t.Fatalf("got %v, want ErrDuplicateKey", err)
	}
}

func TestKDBDeleteReportsExistence(t *testing.T) {
	k := openTestKDB(t)
	if err := k.Put(NSMatches, "m1", []byte(`{"a":1}`)); err != nil {
		t.Fatalf("put: %v", err)
	}
	found, err := k.Delete(NSMatches, "m1")
	if err != nil || !found {
		t.Fatalf("delete existing: found=%v err=%v", found, err)
	}
	if _, err := k.Get(NSMatches, "m1"); !IsNotFound(err) {
		t.Fatalf("still readable after delete: %v", err)
	}
	found, err = k.Delete(NSMatches, "m1")
	if err != nil || found {
		t.Fatalf("delete missing: found=%v err=%v", found, err)
	}
}

func TestKDBScanSeesEveryDocument(t *testing.T) {
	k := openTestKDB(t)
	for _, key := range []string{"a", "b", "c"} {
		if err := k.Put(NSPlayerStats, key, []byte(`{"subjectKey":"`+key+`"}`)); err != nil {
			t.Fatalf("put %s: %v", key, err)
		}
	}
	seen := map[string]bool{}
	err := k.Scan(NSPlayerStats, func(doc []byte) error {
		var p struct {
			SubjectKey string `bson:"subjectKey"`
		}
		if err := UnmarshalDoc(doc, &p); err != nil {
			return err
		}
		seen[p.SubjectKey] = true
		return nil
	})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(seen) != 3 || !seen["a"] || !seen["b"] || !seen["c"] {
		t.Fatalf("scan saw %v, want a,b,c", seen)
	}
}

func TestKDBUpdateIsACriticalSection(t *testing.T) {
	k := openTestKDB(t)
	// A conditional write built from get-check-put inside Update must not
	// interleave with another writer: run two incrementers and expect an
	// exact total.
	if err := k.Put(NSScoring, "s", []byte(`{"n":0}`)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	const perWorker = 25
	errs := make(chan error, 2)
	for w := 0; w < 2; w++ {
		go func() {
			for i := 0; i < perWorker; i++ {
				err := k.Update(NSScoring, func(tx *Tx) error {
					doc, err := tx.Get("s")
					if err != nil {
						return err
					}
					var p struct {
						N int `bson:"n"`
					}
					if err := UnmarshalDoc(doc, &p); err != nil {
						return err
					}
					next, err := MarshalDoc(struct {
						N int `bson:"n"`
					}{N: p.N + 1})
					if err != nil {
						return err
					}
					return tx.Put("s", next)
				})
				if err != nil {
					errs <- err
					return
				}
			}
			errs <- nil
		}()
	}
	for w := 0; w < 2; w++ {
		if err := <-errs; err != nil {
			t.Fatalf("worker: %v", err)
		}
	}
	doc, err := k.Get(NSScoring, "s")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	var p struct {
		N int `bson:"n"`
	}
	if err := UnmarshalDoc(doc, &p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if p.N != 2*perWorker {
		t.Fatalf("n = %d, want %d — lost updates", p.N, 2*perWorker)
	}
}

func TestKDBSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	k, err := OpenKDB(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := k.Put(NSUsers, "u", []byte(`{"username":"ada"}`)); err != nil {
		t.Fatalf("put: %v", err)
	}
	if err := k.Close(context.Background()); err != nil {
		t.Fatalf("close: %v", err)
	}

	k2, err := OpenKDB(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = k2.Close(context.Background()) }()
	doc, err := k2.Get(NSUsers, "u")
	if err != nil {
		t.Fatalf("get after reopen: %v", err)
	}
	var u struct {
		Username string `bson:"username"`
	}
	if err := UnmarshalDoc(doc, &u); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if u.Username != "ada" {
		t.Fatalf("username = %q after reopen, want ada", u.Username)
	}
}

// TestKDBAsyncSurvivesClose is TestKDBSurvivesReopen under async
// durability: an acked write may not be flushed yet, but Close drains the
// commit log, so close-then-reopen must still see it.
func TestKDBAsyncSurvivesClose(t *testing.T) {
	dir := t.TempDir()
	k, err := OpenKDBWithStorage(dir, KDBStorage{Durability: "async", AsyncSyncIntervalMillis: 100})
	if err != nil {
		t.Fatalf("open async: %v", err)
	}
	if err := k.Put(NSUsers, "u", []byte(`{"username":"ada"}`)); err != nil {
		t.Fatalf("put: %v", err)
	}
	if err := k.Close(context.Background()); err != nil {
		t.Fatalf("close: %v", err)
	}

	k2, err := OpenKDB(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = k2.Close(context.Background()) }()
	if _, err := k2.Get(NSUsers, "u"); err != nil {
		t.Fatalf("get after async close+reopen: %v", err)
	}
}

func TestKDBStorageFromEnv(t *testing.T) {
	t.Setenv("KDB_DURABILITY", "async")
	t.Setenv("KDB_SYNC_MODE", "full")
	t.Setenv("KDB_ASYNC_SYNC_INTERVAL_MS", "100")
	sc, err := KDBStorageFromEnv()
	if err != nil {
		t.Fatalf("from env: %v", err)
	}
	if sc.Durability != "async" || sc.SyncMode != "full" || sc.AsyncSyncIntervalMillis != 100 {
		t.Fatalf("parsed %+v, want async/full/100", sc)
	}

	// An unrecognised value must refuse to open, not silently change what an
	// acknowledged write means.
	t.Setenv("KDB_DURABILITY", "yolo")
	if _, err := KDBStorageFromEnv(); err == nil {
		t.Fatal("KDB_DURABILITY=yolo: want error")
	}
	t.Setenv("KDB_DURABILITY", "")
	t.Setenv("KDB_SYNC_MODE", "fastest")
	if _, err := KDBStorageFromEnv(); err == nil {
		t.Fatal("KDB_SYNC_MODE=fastest: want error")
	}
	t.Setenv("KDB_SYNC_MODE", "")
	t.Setenv("KDB_ASYNC_SYNC_INTERVAL_MS", "soon")
	if _, err := KDBStorageFromEnv(); err == nil {
		t.Fatal("KDB_ASYNC_SYNC_INTERVAL_MS=soon: want error")
	}
}

func TestKDBMemoryModeWorks(t *testing.T) {
	k, err := OpenKDB("")
	if err != nil {
		t.Fatalf("open in-memory: %v", err)
	}
	defer func() { _ = k.Close(context.Background()) }()
	if err := k.Put(NSUsers, "u", []byte(`{"username":"mem"}`)); err != nil {
		t.Fatalf("put: %v", err)
	}
	if _, err := k.Get(NSUsers, "u"); err != nil {
		t.Fatalf("get: %v", err)
	}
}

func TestKDBExpirySweep(t *testing.T) {
	k := openTestKDB(t)
	past, err := MarshalDoc(struct {
		Token     string    `bson:"token"`
		ExpiresAt time.Time `bson:"expiresAt"`
	}{Token: "dead", ExpiresAt: time.Now().UTC().Add(-time.Hour)})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := k.Put(NSSessions, "dead", past); err != nil {
		t.Fatalf("put: %v", err)
	}
	k.sweepNamespace(k.nss[NSSessions])
	if _, err := k.Get(NSSessions, "dead"); !IsNotFound(err) {
		t.Fatalf("expired session survived the sweep: %v", err)
	}
}

func TestDocCodecKeepsHiddenFields(t *testing.T) {
	// The models hide server-only fields from JSON (`json:"-"`) but persist
	// them via bson tags. The KDB codec must behave like the Mongo driver
	// here, not like encoding/json.
	type m struct {
		Secret string `bson:"secret" json:"-"`
	}
	doc, err := MarshalDoc(m{Secret: "s3"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back m
	if err := UnmarshalDoc(doc, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Secret != "s3" {
		t.Fatalf("secret = %q, want s3 — codec dropped a json:\"-\" field", back.Secret)
	}
}

// TestKDBConcurrentPutInsertDeleteAllLand is a regression guard for
// kdbNamespace.mu: put() and deleteByUUID() go through two engine
// serialization domains that do not order against each other (see mu's
// comment in kdb.go), so mu is the only thing keeping a concurrent Put and
// Delete on one namespace mutually exclusive. If a future write path ever
// forgot to take it, this test would start losing writes the same way an
// unserialized embed.PutJSONDocument does.
func TestKDBConcurrentPutInsertDeleteAllLand(t *testing.T) {
	k := openTestKDB(t)

	const workers, perWorker = 8, 40
	var wg sync.WaitGroup
	errs := make(chan error, workers*perWorker)
	keep := func(i int) bool { return i%3 != 2 } // the third op per worker is put-then-delete
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				key := fmt.Sprintf("w%02d-%03d", w, i)
				doc := []byte(fmt.Sprintf(`{"subjectKey":%q}`, key))
				switch i % 3 {
				case 0:
					if err := k.Put(NSScoring, key, doc); err != nil {
						errs <- fmt.Errorf("put %s: %w", key, err)
					}
				case 1:
					if err := k.Insert(NSScoring, key, doc); err != nil {
						errs <- fmt.Errorf("insert %s: %w", key, err)
					}
				default:
					if err := k.Put(NSScoring, key, doc); err != nil {
						errs <- fmt.Errorf("put (pre-delete) %s: %w", key, err)
						continue
					}
					if _, err := k.Delete(NSScoring, key); err != nil {
						errs <- fmt.Errorf("delete %s: %w", key, err)
					}
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("write error: %v", err)
	}

	seen := map[string]bool{}
	err := k.Scan(NSScoring, func(doc []byte) error {
		var p struct {
			SubjectKey string `bson:"subjectKey"`
		}
		if err := UnmarshalDoc(doc, &p); err != nil {
			return err
		}
		seen[p.SubjectKey] = true
		return nil
	})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	wantCount := 0
	for w := 0; w < workers; w++ {
		for i := 0; i < perWorker; i++ {
			key := fmt.Sprintf("w%02d-%03d", w, i)
			want := keep(i)
			if want {
				wantCount++
			}
			if seen[key] != want {
				t.Errorf("key %s: visible=%v, want %v — a write was lost or a delete didn't land", key, seen[key], want)
			}
		}
	}
	if len(seen) != wantCount {
		t.Fatalf("scan saw %d documents, want %d", len(seen), wantCount)
	}
}

// TestKDBReadDuringCloseIsSafe pins down behavior this package relies on but
// never asserted: reads take no lock (only writes do), so a read can overlap
// Close tearing down the runtime — including the background sync goroutines
// under async durability. There is no outcome to assert for the in-flight
// reads beyond "no panic" (a Close mid-scan may legitimately error); the
// point is running this under -race with nothing to report.
func TestKDBReadDuringCloseIsSafe(t *testing.T) {
	k, err := OpenKDB(t.TempDir())
	if err != nil {
		t.Fatalf("opening kdb: %v", err)
	}
	for i := 0; i < 20; i++ {
		key := fmt.Sprintf("k%02d", i)
		if err := k.Put(NSMatches, key, []byte(fmt.Sprintf(`{"i":%d}`, i))); err != nil {
			t.Fatalf("seed %s: %v", key, err)
		}
	}

	var wg sync.WaitGroup
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { _ = recover() }()
			for j := 0; j < 100; j++ {
				_ = k.Scan(NSMatches, func(doc []byte) error { return nil })
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = k.Close(context.Background())
	}()
	wg.Wait()
}

var _ = errors.Is // keep errors imported if assertions above change shape
