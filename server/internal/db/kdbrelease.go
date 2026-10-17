package db

import (
	"log/slog"
	"strings"
	"time"

	kdbserver "github.com/limidus/kdb/go/kdb/server"
)

// Letting go of a match's namespace.
//
// A match is a namespace of its own (MatchNS), and a standalone server opened
// one per match and never closed any until it shut down. Each open namespace
// carries a commit graph, document caches and a write gate, which on kdb v0.8.0
// is at least a megabyte whatever the match holds, so a server that had dealt a
// few thousand tables was holding gigabytes for tables nobody was looking at.
// The sync hub already closes idle namespaces (internal/app/sync.go); a
// standalone process had no equivalent.
//
// Closing is cheap to undo: a closed namespace reopens from disk on its next
// use (Namespace), which is why a finished match's replay, stats and rematch
// keep working. What it must never do is close a namespace under a caller, and
// the engine's own idle close is the thing that already knows how not to —
// it takes the namespace out of the set first, waits for writes to drain, and
// leaves a busy one open — so both entry points here go through it rather
// than closing runtimes directly.

const (
	// matchReleaseRetries bounds how often a namespace that was busy is tried
	// again. After that the periodic sweep owns it.
	matchReleaseRetries = 3

	// matchIdleAfter is how long an unfinished or abandoned match may sit
	// untouched before the sweep closes it.
	matchIdleAfter = 10 * time.Minute
	// matchOpenCap bounds how many match namespaces stay open however recently
	// they were used; beyond it the least recently used close first.
	matchOpenCap = 2048
)

// matchReleaseDelay is how long a finished match stays open. The winner's
// screen, the statistics recorder and the rematch offer all read the match in
// the first moments after the last move; closing sooner would only make each of
// them reopen it. A variable only so a test need not wait it out.
var matchReleaseDelay = 30 * time.Second

// isMatchRuntime says whether a namespace-set key names a match's namespace.
// Keys are catalog-qualified ("zolik/m/<id>").
func isMatchRuntime(ns string) bool {
	return strings.HasPrefix(ns, kdbCatalog+"/"+matchPrefix)
}

// closeNamespaceStorage is the Close of every idle policy here: shut the
// namespace's storage and tell this package, which keeps handles of its own
// (ForgetNamespace says why).
func (k *KDB) closeNamespaceStorage(rt *kdbserver.KdbServerRuntime) error {
	ns := rt.Runtime.DefaultNamespace
	err := k.host.CloseNamespace(ns)
	k.ForgetNamespace(ns)
	return err
}

// releaseNow closes ns if it is open and idle, reporting whether it did.
func (k *KDB) releaseNow(ns string) bool {
	closed := k.set.CloseIdle(kdbserver.IdlePolicy{
		// Any namespace that has not been used this very instant, but only
		// this one.
		MinIdle:   time.Nanosecond,
		IdleAfter: time.Nanosecond,
		Pinned:    func(other string) bool { return other != ns },
		Close:     k.closeNamespaceStorage,
	}, time.Now())
	return len(closed) > 0
}

// ReleaseMatch closes a finished match's namespace shortly after its last
// move. It returns at once: the close happens after matchReleaseDelay, and is
// retried a few times if something is using the namespace then. Safe to call
// for a match that is not open, and for an in-memory database (it does
// nothing, since a closed in-memory namespace is gone).
func (k *KDB) ReleaseMatch(matchHex string) {
	if k.host == nil || k.set == nil || matchHex == "" {
		return
	}
	ns := kdbCatalog + "/" + MatchNS(matchHex)
	go func() {
		for attempt := 0; attempt <= matchReleaseRetries; attempt++ {
			select {
			case <-time.After(matchReleaseDelay):
			case <-k.stop:
				return
			}
			if _, open := k.set.Runtimes()[ns]; !open {
				return
			}
			if k.releaseNow(ns) {
				return
			}
		}
		slog.Debug("match namespace still busy; the idle sweep will close it", "namespace", ns)
	}()
}

// StartMatchIdleClose starts the sweep that closes match namespaces nobody has
// used for a while: the ones ReleaseMatch never saw (a table abandoned
// mid-hand, a lobby nobody joined), and a backstop for any it found busy. For
// a process that is not a sync hub; the hub configures the same thing through
// its node. The returned function stops it.
func (k *KDB) StartMatchIdleClose() (stop func()) {
	if k.host == nil || k.set == nil {
		return func() {}
	}
	return k.set.StartIdleClose(k.matchIdlePolicy())
}

func (k *KDB) matchIdlePolicy() kdbserver.IdlePolicy {
	return kdbserver.IdlePolicy{
		MaxOpen:   matchOpenCap,
		IdleAfter: matchIdleAfter,
		MinIdle:   time.Minute,
		Interval:  time.Minute,
		Pinned:    func(ns string) bool { return !isMatchRuntime(ns) },
		Close:     k.closeNamespaceStorage,
	}
}
