package metrics_test

import (
	"context"
	"testing"
	"time"

	"zolik/server/internal/db"
	"zolik/server/internal/metrics"
)

func newKDBStore(t *testing.T) metrics.Store {
	t.Helper()
	k, err := db.OpenKDB(t.TempDir())
	if err != nil {
		t.Fatalf("open kdb: %v", err)
	}
	t.Cleanup(func() { _ = k.Close(context.Background()) })
	return metrics.NewKDBStore(k)
}

func TestMergeAccumulates(t *testing.T) {
	ctx := t.Context()
	s := newKDBStore(t)

	if err := s.Merge(ctx, "2026-09-08", map[string]int64{metrics.MatchesCompleted: 3}, nil, false); err != nil {
		t.Fatalf("first merge: %v", err)
	}
	if err := s.Merge(ctx, "2026-09-08", map[string]int64{metrics.MatchesCompleted: 4, metrics.BootsTotal: 1}, nil, false); err != nil {
		t.Fatalf("second merge: %v", err)
	}

	days, err := s.Days(ctx, "2026-09-08", "2026-09-08")
	if err != nil {
		t.Fatalf("days: %v", err)
	}
	if len(days) != 1 {
		t.Fatalf("got %d days, want 1", len(days))
	}
	// Adding, not replacing: the second flush must not lose the first.
	if got := days[0].Counter(metrics.MatchesCompleted); got != 7 {
		t.Errorf("matches.completed = %d, want 7", got)
	}
	if got := days[0].Counter(metrics.BootsTotal); got != 1 {
		t.Errorf("boot = %d, want 1", got)
	}
	if got := days[0].Counter("never.incremented"); got != 0 {
		t.Errorf("absent counter = %d, want 0", got)
	}
}

// A per-module counter and its parent are both live at once. Under Mongo this
// is the case that would collide if the dots were written as a path, so it is
// worth asserting on both engines that the two coexist and stay distinct.
func TestMergeKeepsFamilyMembersDistinct(t *testing.T) {
	ctx := t.Context()
	s := newKDBStore(t)

	err := s.Merge(ctx, "2026-09-08", map[string]int64{
		metrics.MatchesCompleted:                   5,
		metrics.MatchesCompletedFor("zolik"):       3,
		metrics.MatchesCompletedFor("prsi"):        2,
		metrics.AdmissionRefusedFor("at_capacity"): 9,
	}, nil, false)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}

	days, _ := s.Days(ctx, "2026-09-08", "2026-09-08")
	if len(days) != 1 {
		t.Fatalf("got %d days, want 1", len(days))
	}
	d := days[0]
	for name, want := range map[string]int64{
		"matches.completed":             5,
		"matches.completed.zolik":       3,
		"matches.completed.prsi":        2,
		"admission.refused.at_capacity": 9,
	} {
		if got := d.Counter(name); got != want {
			t.Errorf("%s = %d, want %d", name, got, want)
		}
	}
}

func TestMergePlayersAreASet(t *testing.T) {
	ctx := t.Context()
	s := newKDBStore(t)

	_ = s.Merge(ctx, "2026-09-08", nil, []string{"user:a", "user:b"}, false)
	_ = s.Merge(ctx, "2026-09-08", nil, []string{"user:b", "user:c"}, false)

	days, _ := s.Days(ctx, "2026-09-08", "2026-09-08")
	if len(days) != 1 {
		t.Fatalf("got %d days, want 1", len(days))
	}
	// b appeared in both flushes and must be one player, not two — the whole
	// reason this is a set and not a counter.
	if got := len(days[0].Players); got != 3 {
		t.Errorf("distinct players = %d (%v), want 3", got, days[0].Players)
	}
}

func TestMergeTruncationIsSticky(t *testing.T) {
	ctx := t.Context()
	s := newKDBStore(t)

	_ = s.Merge(ctx, "2026-09-08", nil, []string{"user:a"}, true)
	// A later flush that did not itself saturate must not clear the flag: the
	// day's number is still a floor, and un-flagging it would turn a stated
	// approximation back into a number that reads as exact.
	_ = s.Merge(ctx, "2026-09-08", nil, []string{"user:b"}, false)

	days, _ := s.Days(ctx, "2026-09-08", "2026-09-08")
	if !days[0].PlayersTruncated {
		t.Error("truncation flag was cleared by a later merge")
	}
}

func TestDaysIsARangeAndSkipsEmptyDays(t *testing.T) {
	ctx := t.Context()
	s := newKDBStore(t)

	_ = s.Merge(ctx, "2026-09-01", map[string]int64{metrics.MatchesCompleted: 1}, nil, false)
	_ = s.Merge(ctx, "2026-09-03", map[string]int64{metrics.MatchesCompleted: 1}, nil, false)
	// Outside the range asked for below.
	_ = s.Merge(ctx, "2026-09-09", map[string]int64{metrics.MatchesCompleted: 1}, nil, false)

	days, err := s.Days(ctx, "2026-09-01", "2026-09-05")
	if err != nil {
		t.Fatalf("days: %v", err)
	}
	if len(days) != 2 {
		t.Fatalf("got %d days, want 2 (the 2nd was never written)", len(days))
	}
	if days[0].Date != "2026-09-01" || days[1].Date != "2026-09-03" {
		t.Errorf("days out of order or wrong: %s, %s", days[0].Date, days[1].Date)
	}
}

func TestBootLifecycle(t *testing.T) {
	ctx := t.Context()
	s := newKDBStore(t)

	start := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
	if err := s.InsertBoot(ctx, metrics.Boot{ID: "boot-1", Version: "1.0", StartedAt: start}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// A process asking about its predecessor must never find itself: the
	// row it just wrote is open too.
	if _, ok, _ := s.LatestOpenBoot(ctx, "boot-1"); ok {
		t.Error("LatestOpenBoot returned the excluded row")
	}

	prev, ok, err := s.LatestOpenBoot(ctx, "boot-2")
	if err != nil || !ok {
		t.Fatalf("LatestOpenBoot: ok=%v err=%v", ok, err)
	}
	if prev.ID != "boot-1" {
		t.Errorf("got %s, want boot-1", prev.ID)
	}
	if prev.Clean() {
		t.Error("an open row reported itself as a clean exit")
	}

	stop := start.Add(2 * time.Hour)
	if err := s.CloseBoot(ctx, "boot-1", stop, "signal", false); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, ok, _ := s.LatestOpenBoot(ctx, "boot-2"); ok {
		t.Error("a closed row still reads as open")
	}

	boots, err := s.Boots(ctx, start.Add(-time.Hour), start.Add(time.Hour))
	if err != nil || len(boots) != 1 {
		t.Fatalf("boots: %d, err %v", len(boots), err)
	}
	if got := boots[0].Uptime(); got != 2*time.Hour {
		t.Errorf("uptime = %v, want 2h", got)
	}
	if !boots[0].Clean() {
		t.Error("a closed row does not read as a clean exit")
	}
}

// A crash-looping container restarts over and over. Each restart sees the same
// dead predecessor, and without the counted flag it would report a fresh crash
// every time — a rising crash count from one crash.
func TestCountedBootIsNotOfferedAgain(t *testing.T) {
	ctx := t.Context()
	s := newKDBStore(t)

	start := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
	_ = s.InsertBoot(ctx, metrics.Boot{ID: "dead", StartedAt: start})

	_, ok, _ := s.LatestOpenBoot(ctx, "next")
	if !ok {
		t.Fatal("the dead row was not offered even once")
	}
	if err := s.CloseBoot(ctx, "dead", start, "", true); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, ok, _ := s.LatestOpenBoot(ctx, "next"); ok {
		t.Error("the same dead process would be counted twice")
	}
}
