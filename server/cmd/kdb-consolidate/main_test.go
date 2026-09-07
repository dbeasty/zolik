package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/limidus/kdb/go/kdb/embed"
	"github.com/limidus/kdb/go/kdb/schema"

	"zolik/server/internal/db"
)

// seedOldNamespace writes n documents into namespace name at the old,
// single-tenant data root (root/name) — exactly the shape the server wrote
// before the multi-namespace Host existed.
func seedOldNamespace(t *testing.T, root, catalog, name string, n int) {
	t.Helper()
	nsRoot := filepath.Join(root, name)
	nsID := catalog + "/" + name
	rt, err := embed.OpenFileRuntimeWithOptions(nsRoot, catalog, nsID, schema.None(), embed.FileRuntimeOptionsFromEnv())
	if err != nil {
		t.Fatalf("seeding %s: opening: %v", name, err)
	}
	defer rt.Close()
	for i := 0; i < n; i++ {
		json := fmt.Sprintf(`{"seed":%d}`, i)
		if _, err := embed.PutJSONDocument(rt, nsID, json); err != nil {
			t.Fatalf("seeding %s: put %d: %v", name, i, err)
		}
	}
}

func TestConsolidateMovesAndVerifies(t *testing.T) {
	root := t.TempDir()
	catalog := db.KDBCatalog

	seedOldNamespace(t, root, catalog, db.NSMatches, 3)
	seedOldNamespace(t, root, catalog, db.NSUsers, 2)
	// Every other namespace is left absent, the common case for a store that
	// has never seen (say) an OAuth sign-in.

	if err := run(root, true); err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if !exists(filepath.Join(root, db.NSMatches)) {
		t.Fatal("dry run must not touch the old directories")
	}
	if exists(nsMetaPath(root, catalog, db.NSMatches)) {
		t.Fatal("dry run must not create the new layout")
	}

	if err := run(root, false); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if exists(filepath.Join(root, db.NSMatches)) {
		t.Error("old matches directory should be gone after a verified migration")
	}
	if exists(filepath.Join(root, db.NSUsers)) {
		t.Error("old users directory should be gone after a verified migration")
	}
	if !exists(nsMetaPath(root, catalog, db.NSMatches)) {
		t.Error("matches should now live at the new consolidated path")
	}

	k, err := db.OpenKDB(root)
	if err != nil {
		t.Fatalf("opening consolidated store: %v", err)
	}
	defer k.Close(context.Background())

	for name, want := range map[string]int{db.NSMatches: 3, db.NSUsers: 2} {
		got := 0
		if err := k.Scan(name, func([]byte) error { got++; return nil }); err != nil {
			t.Fatalf("scanning %s: %v", name, err)
		}
		if got != want {
			t.Errorf("%s: got %d documents, want %d", name, got, want)
		}
	}

	// Idempotent: everything migrated already, nothing left to do or break.
	if err := k.Close(context.Background()); err != nil {
		t.Fatalf("closing before re-run: %v", err)
	}
	if err := run(root, false); err != nil {
		t.Fatalf("second run should be a no-op, got: %v", err)
	}
}

func TestConsolidateRefusesAmbiguousNamespace(t *testing.T) {
	root := t.TempDir()
	catalog := db.KDBCatalog

	seedOldNamespace(t, root, catalog, db.NSMatches, 1)

	// Fabricate a namespace that already exists at the new path too, as if
	// a previous run had partially succeeded some other way.
	newDir := filepath.Join(root, "ns", catalog, db.NSMatches)
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newDir, "meta.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := run(root, false); err == nil {
		t.Fatal("expected an error when a namespace exists at both the old and new path")
	}
	if !exists(filepath.Join(root, db.NSMatches)) {
		t.Error("refusing to migrate must leave the old directory untouched")
	}
}

// TestConsolidateLeavesTheNewLayoutOwnedLikeTheDataRoot guards the failure
// that took the dev stack down after the first real migration: the tool ran
// as root in a maintenance container, so the lock files and directories the
// first open created were root-owned, and the server — an unprivileged uid
// in a distroless image — could not open them. The engine reports that as
// "data directory is locked for maintenance", which reads as a stuck lock
// rather than as the permissions problem it is, so this is worth pinning.
func TestConsolidateLeavesTheNewLayoutOwnedLikeTheDataRoot(t *testing.T) {
	root := t.TempDir()
	catalog := db.KDBCatalog
	seedOldNamespace(t, root, catalog, db.NSMatches, 1)

	if err := run(root, false); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	want, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	wantStat, ok := want.Sys().(*syscall.Stat_t)
	if !ok {
		t.Skip("no unix ownership on this platform")
	}
	for _, name := range []string{".kdb.lock", ".kdb.write.lock", "ns", "snap"} {
		p := filepath.Join(root, name)
		if !exists(p) {
			continue
		}
		err := filepath.WalkDir(p, func(path string, _ os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			info, err := os.Stat(path)
			if err != nil {
				return err
			}
			got, ok := info.Sys().(*syscall.Stat_t)
			if !ok {
				return nil
			}
			if got.Uid != wantStat.Uid || got.Gid != wantStat.Gid {
				t.Errorf("%s is owned by %d:%d, want %d:%d (the data root's owner)",
					path, got.Uid, got.Gid, wantStat.Uid, wantStat.Gid)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestConsolidateNoopOnFreshStore(t *testing.T) {
	root := t.TempDir()
	if err := run(root, false); err != nil {
		t.Fatalf("a store with nothing to migrate should be a clean no-op: %v", err)
	}
}
