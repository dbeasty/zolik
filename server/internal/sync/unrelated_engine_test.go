package sync

// Reproductions for an engine defect this package found and does not work
// around: a merge commit cannot be transferred to another node. Whichever node
// makes the merge, the node that receives it rebuilds a different document
// tree than the merge declares, and refuses it as an integrity failure:
//
//	peer sync: commit <hash> declares tree <a> but its history builds <b>
//
// It never converges - the same commit is refused on every retry - so two
// nodes whose histories have genuinely diverged can never exchange the
// resolution of that divergence. Everything that only ever fast-forwards is
// unaffected, which is why the rest of this package's tests pass.
//
// These are written against the engine's own API rather than through zolik, so
// that they are a reproduction somebody can hand to the database rather than a
// report about a card game. Remove the t.Skip lines to run them.

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	kdbauth "github.com/limidus/kdb/go/kdb/auth"
	"github.com/limidus/kdb/go/kdb/codec"
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
	t.Skip("kdb: a merge commit is refused by the node it is transferred to; here the sender makes it")
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
}

// Same divergence, but the merge is made by the node receiving the push
// rather than by the node sending it.
func TestSharedOriginDivergencePushOnly(t *testing.T) {
	t.Skip("kdb: a merge commit is refused by the node it is transferred to; here the receiver makes it and the sender pulls it back")
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
				err := runIndependentCreationMode(t, onDisk, m.mode)
				t.Logf("result: %v", err)
			})
		}
	}
}
