package metrics_test

import (
	"testing"

	"zolik/server/internal/metrics"
)

// countingSink records counters without a database behind it.
type countingSink struct{ counts map[string]int64 }

func newCountingSink() *countingSink { return &countingSink{counts: map[string]int64{}} }

func (c *countingSink) Add(name string, n int64) { c.counts[name] += n }
func (c *countingSink) SeePlayer(string)         {}

func TestFirstEverBootIsNotACrash(t *testing.T) {
	ctx := t.Context()
	store := newKDBStore(t)
	sink := newCountingSink()

	metrics.NewBootRecorder(store, sink).Start(ctx, "1.0")

	if got := sink.counts[metrics.BootsTotal]; got != 1 {
		t.Errorf("boot = %d, want 1", got)
	}
	// An empty database is not a crash. Getting this wrong would report a
	// crash on every fresh deployment, which is exactly the noise that makes
	// a crash counter stop being read.
	if got := sink.counts[metrics.BootsUnclean]; got != 0 {
		t.Errorf("boot.unclean = %d on a first-ever boot, want 0", got)
	}
}

func TestCleanRestartIsNotACrash(t *testing.T) {
	ctx := t.Context()
	store := newKDBStore(t)

	first := metrics.NewBootRecorder(store, newCountingSink())
	first.Start(ctx, "1.0")
	first.Stop(ctx, metrics.ReasonSignal)

	sink := newCountingSink()
	metrics.NewBootRecorder(store, sink).Start(ctx, "1.1")

	if got := sink.counts[metrics.BootsUnclean]; got != 0 {
		t.Errorf("boot.unclean = %d after a clean shutdown, want 0", got)
	}
}

func TestUncleanExitIsDetectedByTheNextBoot(t *testing.T) {
	ctx := t.Context()
	store := newKDBStore(t)

	// Started and never stopped: the process died without running its
	// shutdown path.
	metrics.NewBootRecorder(store, newCountingSink()).Start(ctx, "1.0")

	sink := newCountingSink()
	metrics.NewBootRecorder(store, sink).Start(ctx, "1.0")

	if got := sink.counts[metrics.BootsUnclean]; got != 1 {
		t.Errorf("boot.unclean = %d after a crash, want 1", got)
	}
}

// The failure this guards is specific: a container that crash-loops restarts
// every few seconds, and each restart sees the same dead predecessor. Counting
// it each time turns one crash into a rising number that describes the
// restart policy rather than the fault.
func TestOneCrashIsCountedOnce(t *testing.T) {
	ctx := t.Context()
	store := newKDBStore(t)

	metrics.NewBootRecorder(store, newCountingSink()).Start(ctx, "1.0")

	total := int64(0)
	for i := 0; i < 5; i++ {
		sink := newCountingSink()
		// Each of these also dies without stopping — so the *first* sees the
		// original crash, and each subsequent one sees its immediate
		// predecessor. Five restarts after one crash is five dead processes,
		// not one, so the honest total is five: what must not happen is the
		// original being counted again and again on top of them.
		metrics.NewBootRecorder(store, sink).Start(ctx, "1.0")
		total += sink.counts[metrics.BootsUnclean]
	}
	if total != 5 {
		t.Errorf("boot.unclean totalled %d over five crash-restarts, want 5 (one per dead process)", total)
	}
}

func TestBootIDsAreDistinct(t *testing.T) {
	ctx := t.Context()
	store := newKDBStore(t)

	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		b := metrics.NewBootRecorder(store, newCountingSink())
		b.Start(ctx, "1.0")
		if seen[b.ID()] {
			t.Fatalf("duplicate boot id %s", b.ID())
		}
		seen[b.ID()] = true
	}
}

// Recording is not load-bearing. A process whose store is broken must still
// start, still serve, and still shut down.
func TestBootRecorderSurvivesABrokenStore(t *testing.T) {
	ctx := t.Context()
	b := metrics.NewBootRecorder(brokenStore{}, newCountingSink())
	b.Start(ctx, "1.0")
	b.Stop(ctx, metrics.ReasonSignal)
}
