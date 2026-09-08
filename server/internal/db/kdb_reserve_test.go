package db

import (
	"testing"

	kdbserver "github.com/limidus/kdb/go/kdb/server"
)

// The rescue reserve is a real, page-touched allocation, and this process
// opens one storage runtime per namespace. Taking the engine's default in each
// of them held nine copies of it — 432 MiB, live and uncollectable, before a
// single player connected — which on a 1 GiB container left the server idling
// close enough to the admission watermark that a handful of arrivals were
// refused.
//
// The number of namespaces is a list that grows, so the thing worth pinning is
// not the share but the total. A tenth namespace should cost a tenth of a
// reserve, not another whole one.
func TestRescueReserveIsSharedAcrossNamespaces(t *testing.T) {
	share := kdbRescueReserveBytes()
	if share <= 0 {
		t.Fatalf("rescue reserve share is %d; every namespace needs something to drop", share)
	}

	total := share * int64(len(kdbNamespaceNames))
	if total > kdbserver.DefaultRescueReserveBytes {
		t.Errorf(
			"the %d namespaces hold %d MiB of rescue reserve between them, more than the one "+
				"reserve a single-runtime deployment holds (%d MiB). One process has one memory "+
				"ceiling and needs one reserve's worth of headroom to abort within.",
			len(kdbNamespaceNames),
			total>>20,
			kdbserver.DefaultRescueReserveBytes>>20,
		)
	}
}

// The engine clamps a reserve to a quarter of the budget, so a share that is
// somehow larger than the whole default would be silently cut rather than
// reported. Nothing should be able to produce one.
func TestRescueReserveShareIsNotLargerThanTheWholeReserve(t *testing.T) {
	if share := kdbRescueReserveBytes(); share > kdbserver.DefaultRescueReserveBytes {
		t.Errorf("one namespace's share is %d bytes, larger than the whole default reserve (%d)",
			share, kdbserver.DefaultRescueReserveBytes)
	}
}
