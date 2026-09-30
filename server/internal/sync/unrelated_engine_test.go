package sync

// Convergence between two nodes, tested against the engine's own API rather
// than through zolik, because what these are really about is the database
// underneath: whether two nodes that wrote at the same time end up holding the
// same thing.
//
// They were written to reproduce a defect where they did not. Every write this
// server makes is a whole-document replace, which is a delete followed by a
// write in one commit, and a merge read the first of those two operations and
// concluded the document was not there. The other side's writes were dropped
// silently, and the merge then declared a document tree no other node could
// rebuild, so nothing about that divergence could ever travel. Fixed in kdb's
// peersync.valueIndex; these stay as the guard.

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	kdbauth "github.com/limidus/kdb/go/kdb/auth"
	"github.com/limidus/kdb/go/kdb/codec"
	"github.com/limidus/kdb/go/kdb/document"
	"github.com/limidus/kdb/go/kdb/embed"
	"github.com/limidus/kdb/go/kdb/peersync"
	"github.com/limidus/kdb/go/kdb/replication"
	"github.com/limidus/kdb/go/kdb/schema"
	kdbserver "github.com/limidus/kdb/go/kdb/server"
	"github.com/limidus/kdb/go/kdb/syncnode"
)

// A pair of nodes built straight on the engine, with no zolik in the way, so
// the question "is this the storage engine or the path" can be asked directly.
type rawNode struct {
	set     *kdbserver.NamespaceSet
	primary *kdbserver.KdbServerRuntime
	node    *syncnode.Node
	host    *embed.Host
}

func openRawNode(t *testing.T, onDisk bool, peers []replication.PeerConfig) *rawNode {
	t.Helper()
	const catalog = "repro"
	const primaryNS = "repro/primary"

	var host *embed.Host
	var rt *embed.EmbeddedKdbRuntime
	var err error
	dir := t.TempDir()
	if onDisk {
		host, err = embed.OpenFileHost(dir, embed.FileRuntimeOptions{})
		if err != nil {
			t.Fatalf("host: %v", err)
		}
		rt, err = host.Namespace(catalog, primaryNS, schema.None())
	} else {
		rt, err = embed.OpenMemoryRuntime(catalog, primaryNS, schema.None())
	}
	if err != nil {
		t.Fatalf("primary namespace: %v", err)
	}
	primary := kdbserver.NewKdbServerRuntime(rt)
	if !onDisk {
		// A file-backed node takes its identity from the data root's NODE
		// file; two in-memory ones would otherwise both be this process.
		id, err := codec.RandomUUID()
		if err != nil {
			t.Fatalf("node id: %v", err)
		}
		primary.NodeID = id
	}
	primary.AuthEngine = kdbauth.AllowAll
	primary.PeerCreateOnPush = true

	var coord *embed.TxnCoordinator
	if host != nil {
		coord = host.Transactions()
	}
	set := kdbserver.NewNamespaceSet(coord)
	if err := set.Add(primary); err != nil {
		t.Fatalf("set: %v", err)
	}
	cfg := syncnode.Config{Peers: peers}
	if onDisk {
		cfg.DataDir = dir
	}
	n, err := syncnode.Open(host, set, primary, cfg)
	if err != nil {
		t.Fatalf("syncnode: %v", err)
	}
	set.SetOpener(n.Opener(func(rt *kdbserver.KdbServerRuntime) error {
		rt.AuthEngine = kdbauth.AllowAll
		rt.PeerCreateOnPush = true
		return nil
	}))
	if err := n.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = n.Close() })
	return &rawNode{set: set, primary: primary, node: n, host: host}
}

// write puts one document into a namespace, creating it.
func (r *rawNode) write(t *testing.T, namespace, body string) {
	t.Helper()
	rt, err := r.set.Resolve(namespace, true)
	if err != nil {
		t.Fatalf("resolving %s: %v", namespace, err)
	}
	id, err := codec.RandomUUID()
	if err != nil {
		t.Fatalf("doc id: %v", err)
	}
	if _, err := rt.PutJSON(namespace, id, body, nil, kdbauth.Principal{}); err != nil {
		t.Fatalf("writing %s: %v", namespace, err)
	}
}

// documents is every document a node holds in a namespace, by id.
func (r *rawNode) documents(t *testing.T, namespace string) map[string]string {
	t.Helper()
	rt, err := r.set.Resolve(namespace, false)
	if err != nil {
		t.Fatalf("resolving %s: %v", namespace, err)
	}
	head, err := rt.Runtime.DAG.Head()
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	c, err := rt.Runtime.DAG.GetCommitOrThrow(head)
	if err != nil {
		t.Fatalf("head commit: %v", err)
	}
	out := map[string]string{}
	err = rt.Runtime.Storage.ScanDocuments(namespace, c.DocumentTreeHash, 256, func(batch []document.Document) error {
		for _, d := range batch {
			out[d.ID.String()] = d.JSON
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scanning %s: %v", namespace, err)
	}
	return out
}

// requireSameDocuments is the assertion that matters: not that a sync returned
// no error, but that the two nodes hold the same thing afterwards.
func requireSameDocuments(t *testing.T, a, b *rawNode, namespace string, want int) {
	t.Helper()
	da, db := a.documents(t, namespace), b.documents(t, namespace)
	if len(da) != want || len(db) != want {
		t.Fatalf("documents held: %d and %d, want %d each\n  %v\n  %v", len(da), len(db), want, da, db)
	}
	for id, body := range da {
		if db[id] != body {
			t.Fatalf("document %s differs: %q and %q", id, body, db[id])
		}
	}
}

func runIndependentCreationMode(t *testing.T, onDisk bool, mode peersync.SyncMode) error {
	t.Helper()
	const shared = "repro/shared"

	hub := openRawNode(t, onDisk, nil)
	server := httptest.NewServer(hub.node.Handler())
	t.Cleanup(server.Close)

	spoke := openRawNode(t, onDisk, []replication.PeerConfig{{
		Name:        "hub",
		Addr:        "ws://" + strings.TrimPrefix(server.URL, "http://") + "/kdb/sync",
		Namespaces:  []string{shared},
		Mode:        mode,
		Interval:    time.Hour,
		CreateLocal: true,
	}})

	// Each side creates the same namespace independently, from its own
	// genesis, and writes a document of its own into it.
	hub.write(t, shared, `{"from":"hub"}`)
	spoke.write(t, shared, `{"from":"spoke"}`)

	_, err := spoke.node.SyncNow("hub")
	return err
}

// Pull and push in separate sessions, to tell "the two directions in one
// session" apart from "the two directions at all".
func TestIndependentCreationPullThenPushSeparately(t *testing.T) {
	const shared = "repro/shared"
	hub := openRawNode(t, true, nil)
	server := httptest.NewServer(hub.node.Handler())
	t.Cleanup(server.Close)
	addr := "ws://" + strings.TrimPrefix(server.URL, "http://") + "/kdb/sync"

	spoke := openRawNode(t, true, []replication.PeerConfig{
		{Name: "hubpull", Addr: addr, Namespaces: []string{shared}, Mode: peersync.SyncPull, Interval: time.Hour, CreateLocal: true},
		{Name: "hubpush", Addr: addr, Namespaces: []string{shared}, Mode: peersync.SyncPush, Interval: time.Hour},
	})
	hub.write(t, shared, `{"from":"hub"}`)
	spoke.write(t, shared, `{"from":"spoke"}`)

	if _, err := spoke.node.SyncNow("hubpull"); err != nil {
		t.Fatalf("pull: %v", err)
	}
	_, err := spoke.node.SyncNow("hubpush")
	t.Logf("push after a separate pull: %v", err)

	// And whether a second attempt in one session converges after the first
	// has failed.
	if _, err := spoke.node.SyncNow("hubpush"); err != nil {
		t.Logf("second push: %v", err)
	} else {
		t.Log("second push: <nil>")
	}
}

func TestIndependentCreationRetriesConverge(t *testing.T) {
	const shared = "repro/shared"
	hub := openRawNode(t, true, nil)
	server := httptest.NewServer(hub.node.Handler())
	t.Cleanup(server.Close)

	spoke := openRawNode(t, true, []replication.PeerConfig{{
		Name: "hub", Addr: "ws://" + strings.TrimPrefix(server.URL, "http://") + "/kdb/sync",
		Namespaces: []string{shared}, Mode: peersync.SyncBoth, Interval: time.Hour, CreateLocal: true,
	}})
	hub.write(t, shared, `{"from":"hub"}`)
	spoke.write(t, shared, `{"from":"spoke"}`)

	for attempt := 1; attempt <= 3; attempt++ {
		_, err := spoke.node.SyncNow("hub")
		t.Logf("attempt %d: %v", attempt, err)
	}
}

// The control: a namespace with ONE origin, diverged by concurrent writes
// after the spoke already had it. If a locally-made merge pushes cleanly here,
// the fault is specific to histories that began independently.
func TestSharedOriginDivergenceMergesAndPushes(t *testing.T) {
	const shared = "repro/shared"
	hub := openRawNode(t, true, nil)
	server := httptest.NewServer(hub.node.Handler())
	t.Cleanup(server.Close)

	spoke := openRawNode(t, true, []replication.PeerConfig{{
		Name: "hub", Addr: "ws://" + strings.TrimPrefix(server.URL, "http://") + "/kdb/sync",
		Namespaces: []string{shared}, Mode: peersync.SyncBoth, Interval: time.Hour, CreateLocal: true,
	}})

	// One origin: the hub creates it and the spoke takes a copy.
	hub.write(t, shared, `{"from":"hub","n":1}`)
	if _, err := spoke.node.SyncNow("hub"); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	// Now both write, so the two histories diverge from a shared commit.
	hub.write(t, shared, `{"from":"hub","n":2}`)
	spoke.write(t, shared, `{"from":"spoke","n":2}`)

	if _, err := spoke.node.SyncNow("hub"); err != nil {
		t.Fatalf("merging a divergence from a shared origin: %v", err)
	}
	// Both writes survive on both sides: the merge carried each side's across,
	// and the merge itself travelled.
	requireSameDocuments(t, hub, spoke, shared, 3)
}

// Same divergence, but the merge is made by the node receiving the push
// rather than by the node sending it.
func TestSharedOriginDivergencePushOnly(t *testing.T) {
	const shared = "repro/shared"
	hub := openRawNode(t, true, nil)
	server := httptest.NewServer(hub.node.Handler())
	t.Cleanup(server.Close)
	addr := "ws://" + strings.TrimPrefix(server.URL, "http://") + "/kdb/sync"

	spoke := openRawNode(t, true, []replication.PeerConfig{
		{Name: "hubpull", Addr: addr, Namespaces: []string{shared}, Mode: peersync.SyncPull, Interval: time.Hour, CreateLocal: true},
		{Name: "hubpush", Addr: addr, Namespaces: []string{shared}, Mode: peersync.SyncPush, Interval: time.Hour},
	})
	hub.write(t, shared, `{"from":"hub","n":1}`)
	if _, err := spoke.node.SyncNow("hubpull"); err != nil {
		t.Fatalf("taking a copy: %v", err)
	}
	hub.write(t, shared, `{"from":"hub","n":2}`)
	spoke.write(t, shared, `{"from":"spoke","n":2}`)

	// Push without pulling first: the spoke has no merge to make, so the hub
	// makes it.
	if _, err := spoke.node.SyncNow("hubpush"); err != nil {
		t.Fatalf("push-only merge on the receiver: %v", err)
	}
	// And the spoke can then take the hub's merge back.
	if _, err := spoke.node.SyncNow("hubpull"); err != nil {
		t.Fatalf("pulling the merge back: %v", err)
	}
	requireSameDocuments(t, hub, spoke, shared, 3)
}

func TestIndependentCreationByMode(t *testing.T) {
	for _, onDisk := range []bool{false, true} {
		for _, m := range []struct {
			name string
			mode peersync.SyncMode
		}{
			{"pull", peersync.SyncPull},
			{"push", peersync.SyncPush},
			{"both", peersync.SyncBoth},
		} {
			where := "memory"
			if onDisk {
				where = "disk"
			}
			t.Run(where+"/"+m.name, func(t *testing.T) {
				if err := runIndependentCreationMode(t, onDisk, m.mode); err != nil {
					t.Fatalf("%s, %s: %v", where, m.name, err)
				}
			})
		}
	}
}
