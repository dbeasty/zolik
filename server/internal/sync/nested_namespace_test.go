package sync

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/limidus/kdb/go/kdb/peersync"
	"github.com/limidus/kdb/go/kdb/replication"
)

// Two namespaces where one's name is a prefix of the other's - which is the
// layout this server uses, u/<account> for what a person writes and
// u/<account>/ro for what the cloud writes for them.
//
// Bootstrapping the nested one from a snapshot used to fail on a file-backed
// node: the checkpoint file was named by turning the namespace id into a path,
// so the shorter name held as a file the directory the longer one needed.
// Fixed in kdb by flattening that name to one component; this is the guard
// from our side, because the layout that triggers it is the layout this server
// uses for every account.
func TestNestedNamespacesBootstrapFromASnapshot(t *testing.T) {

	runNested(t, true)
}

// The same layout without asking for a snapshot: the nested namespace is
// fetched as history instead, which is what this server now does.
func TestNestedNamespacesSyncWithoutASnapshot(t *testing.T) {
	runNested(t, false)
}

func runNested(t *testing.T, preferSnapshot bool) {
	t.Helper()
	const outer = "repro/account"
	const inner = "repro/account/ro"

	hub := openRawNode(t, true, nil)
	server := httptest.NewServer(hub.node.Handler())
	t.Cleanup(server.Close)

	hub.write(t, outer, `{"from":"hub","which":"outer"}`)
	hub.write(t, inner, `{"from":"hub","which":"inner"}`)

	spoke := openRawNode(t, true, []replication.PeerConfig{{
		Name: "hub", Addr: "ws://" + strings.TrimPrefix(server.URL, "http://") + "/kdb/sync",
		Namespaces: []string{outer, inner}, Mode: peersync.SyncBoth, Interval: time.Hour,
		CreateLocal: true, PreferSnapshot: preferSnapshot,
	}})

	if _, err := spoke.node.SyncNow("hub"); err != nil {
		t.Fatalf("syncing a namespace nested under another: %v", err)
	}
	if got := spoke.documents(t, inner); len(got) != 1 {
		t.Fatalf("the nested namespace holds %v, want the hub's document", got)
	}
}
