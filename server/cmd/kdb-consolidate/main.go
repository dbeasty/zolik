// Command kdb-consolidate moves an existing KDB_PATH from the old
// one-data-root-per-namespace layout onto the new single-host layout that
// internal/db/kdb.go's OpenKDBWithStorage now expects (see
// github.com/limidus/kdb PR #37, "one runtime over many spaces").
//
// Old layout: KDB_PATH/<namespace>/ns/zolik/<namespace>/..., one directory
// lock per namespace. New layout: KDB_PATH/ns/zolik/<namespace>/..., one
// directory lock for the whole store. The two subtrees a namespace owns —
// ns/zolik/<name> and, if present, snap/kdb_checkpoint_zolik/<name> — are
// byte-for-byte what the new layout expects at KDB_PATH directly, because
// both layouts open a namespace through the exact same code path
// (Host.openNamespace); only the root they sit under differs. So this is a
// pure filesystem move, not a format conversion.
//
// This must run with the server (and anything else that might open
// KDB_PATH) stopped: opening each old namespace takes its writer lock
// exclusively, so a running server makes the tool fail loudly rather than
// racing it.
//
// It is safe to run more than once. A namespace already migrated (its new
// path exists, its old one does not) is skipped; a namespace never created
// (neither path exists) is skipped. Finding both an old and a new copy of
// the same namespace is refused rather than guessed at.
//
// Nothing is deleted until every namespace has moved AND the consolidated
// store has been reopened through the same db.OpenKDB path the server uses,
// with every migrated namespace's document count checked against what was
// counted before the move. A mismatch aborts before any cleanup, leaving
// both the moved data and the old per-namespace directories in place for
// inspection.
//
//	go run ./cmd/kdb-consolidate -path /path/to/kdb-data
//
// Pass -dry to count what would move without writing anything.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/limidus/kdb/go/kdb/document"
	"github.com/limidus/kdb/go/kdb/embed"
	"github.com/limidus/kdb/go/kdb/schema"

	"zolik/server/internal/db"
)

func main() {
	path := flag.String("path", "", "KDB_PATH to consolidate (required)")
	dry := flag.Bool("dry", false, "count what would migrate, write nothing")
	flag.Parse()

	if *path == "" {
		log.Fatal("-path is required")
	}

	if err := run(*path, *dry); err != nil {
		log.Fatal(err)
	}
}

// nsPaths returns the two paths a namespace occupies under root: the old
// shape (its own data root, root/name) and the marker that says it exists
// there, and the marker for the new shape (root itself). meta.json is the
// same marker embed.ListNamespaces uses to decide a namespace is present.
func nsMetaPath(root, catalog, name string) string {
	return filepath.Join(root, "ns", catalog, name, "meta.json")
}

func snapDir(root, catalog, name string) string {
	return filepath.Join(root, "snap", "kdb_checkpoint_"+catalog, name)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// fixOwnership makes every path this tool created under root match root's
// own ownership. Needed because a migration commonly runs as a different
// user than the server does — an ad-hoc maintenance container, say — and
// the marker files/directories embed.OpenFileHost creates on first open
// (.kdb.lock, .kdb.write.lock, ns/, snap/) default to whoever ran this
// process, not whoever the server runs as. root itself predates the
// migration (the server created it), so it names the uid/gid this data
// must stay readable and writable by; leaving a mismatch would trade "the
// server can't find its data" for "the server can't open its data", which
// surfaces identically as a boot failure but is much less obviously a
// migration problem when it happens on a live deploy.
func fixOwnership(root string) error {
	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		// Non-Unix: nothing to do, and nothing that would have gone wrong —
		// this whole scheme (embed's dir_lock_unix.go) is unix-only already.
		return nil
	}
	uid, gid := int(stat.Uid), int(stat.Gid)

	for _, name := range []string{".kdb.lock", ".kdb.write.lock", "ns", "snap"} {
		p := filepath.Join(root, name)
		if !exists(p) {
			continue
		}
		err := filepath.WalkDir(p, func(path string, _ os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			return os.Chown(path, uid, gid)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// countDocs counts every live document in a namespace as of its current
// head commit — the same walk internal/db/kdb.go's kdbNamespace.scan does,
// reimplemented here against the public embed/dag/storage API since this
// tool has no access to that unexported type.
func countDocs(rt *embed.EmbeddedKdbRuntime, nsID string) (int, error) {
	head, err := rt.DAG.Head()
	if err != nil {
		return 0, fmt.Errorf("head: %w", err)
	}
	commit, err := rt.DAG.GetCommitOrThrow(head)
	if err != nil {
		return 0, fmt.Errorf("commit %s: %w", head, err)
	}
	n := 0
	err = rt.Storage.ScanDocuments(nsID, commit.DocumentTreeHash, 256, func(batch []document.Document) error {
		n += len(batch)
		return nil
	})
	return n, err
}

// countOldNamespace opens namespace name at its old, single-tenant data
// root (root/name) just long enough to count its documents, then closes it
// — releasing the writer lock before the move touches anything.
func countOldNamespace(root, catalog, name string) (int, error) {
	nsRoot := filepath.Join(root, name)
	nsID := catalog + "/" + name
	rt, err := embed.OpenFileRuntimeWithOptions(nsRoot, catalog, nsID, schema.None(), embed.FileRuntimeOptionsFromEnv())
	if err != nil {
		return 0, fmt.Errorf("opening old namespace %s at %s: %w", nsID, nsRoot, err)
	}
	defer rt.Close()
	return countDocs(rt, nsID)
}

// moveNamespace relocates namespace name's two owned subtrees from its old
// per-namespace root (root/name) onto the new shared root (root). Refuses
// rather than overwrites if the destination is somehow already there.
func moveNamespace(root, catalog, name string) error {
	oldRoot := filepath.Join(root, name)

	src := filepath.Join(oldRoot, "ns", catalog, name)
	dst := filepath.Join(root, "ns", catalog, name)
	if exists(dst) {
		return fmt.Errorf("destination %s already exists, refusing to overwrite", dst)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("moving %s -> %s: %w", src, dst, err)
	}

	if srcSnap := snapDir(oldRoot, catalog, name); exists(srcSnap) {
		dstSnap := snapDir(root, catalog, name)
		if exists(dstSnap) {
			return fmt.Errorf("destination %s already exists, refusing to overwrite", dstSnap)
		}
		if err := os.MkdirAll(filepath.Dir(dstSnap), 0o755); err != nil {
			return err
		}
		if err := os.Rename(srcSnap, dstSnap); err != nil {
			return fmt.Errorf("moving %s -> %s: %w", srcSnap, dstSnap, err)
		}
	}
	return nil
}

func run(root string, dry bool) error {
	catalog := db.KDBCatalog
	names := db.KDBNamespaceNames()

	type plan struct {
		name     string
		oldCount int
	}
	var toMigrate []plan

	for _, name := range names {
		oldMarker := nsMetaPath(filepath.Join(root, name), catalog, name)
		newMarker := nsMetaPath(root, catalog, name)
		switch {
		case exists(oldMarker) && exists(newMarker):
			return fmt.Errorf("namespace %s exists at both the old path (%s) and the new path (%s) — "+
				"resolve manually before running this tool", name, oldMarker, newMarker)
		case exists(newMarker):
			log.Printf("%s: already migrated, skipping", name)
		case !exists(oldMarker):
			log.Printf("%s: not present, skipping", name)
		default:
			n, err := countOldNamespace(root, catalog, name)
			if err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			log.Printf("%s: %d document(s) to migrate", name, n)
			toMigrate = append(toMigrate, plan{name: name, oldCount: n})
		}
	}

	if dry {
		log.Printf("dry run: %d namespace(s) would migrate", len(toMigrate))
		return nil
	}
	if len(toMigrate) == 0 {
		log.Printf("nothing to migrate")
		return nil
	}

	for _, p := range toMigrate {
		if err := moveNamespace(root, catalog, p.name); err != nil {
			return fmt.Errorf("%s: %w", p.name, err)
		}
		log.Printf("%s: moved", p.name)
	}

	k, err := db.OpenKDB(root)
	if err != nil {
		return fmt.Errorf("opening consolidated store at %s for verification: %w", root, err)
	}

	var mismatches []string
	for _, p := range toMigrate {
		n := 0
		if err := k.Scan(p.name, func([]byte) error { n++; return nil }); err != nil {
			return fmt.Errorf("verifying %s: %w", p.name, err)
		}
		if n != p.oldCount {
			mismatches = append(mismatches, fmt.Sprintf("%s: had %d, now %d", p.name, p.oldCount, n))
			continue
		}
		log.Printf("%s: verified, %d document(s)", p.name, n)
	}
	if len(mismatches) > 0 {
		_ = k.Close(context.Background())
		return fmt.Errorf("verification failed, old directories left in place for inspection: %s",
			strings.Join(mismatches, "; "))
	}

	if err := k.Close(context.Background()); err != nil {
		return fmt.Errorf("closing consolidated store: %w", err)
	}

	if err := fixOwnership(root); err != nil {
		return fmt.Errorf("matching ownership of the new layout to %s: %w — the server's own uid "+
			"may not be able to open its data; run this tool as that uid (or as root) and retry", root, err)
	}

	for _, p := range toMigrate {
		oldRoot := filepath.Join(root, p.name)
		if err := os.RemoveAll(oldRoot); err != nil {
			return fmt.Errorf("removing migrated directory %s: %w", oldRoot, err)
		}
	}
	log.Printf("done: %d namespace(s) migrated and verified", len(toMigrate))
	return nil
}
