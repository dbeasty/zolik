package metrics_test

import (
	"testing"
	"time"

	"zolik/server/internal/metrics"
)

func day(s string) time.Time {
	t, err := time.Parse(metrics.DayFormat, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestReportGapFills(t *testing.T) {
	ctx := t.Context()
	store := newKDBStore(t)
	_ = store.Merge(ctx, "2026-09-01", map[string]int64{metrics.MatchesCompleted: 4}, nil, false)
	_ = store.Merge(ctx, "2026-09-04", map[string]int64{metrics.MatchesCompleted: 2}, nil, false)

	rep, err := metrics.NewReporter(store).Build(ctx, day("2026-09-01"), day("2026-09-05"), metrics.BucketDay)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	// Five days asked for, five days returned. A chart drawn from three rows
	// would close the gap and imply play on the 2nd and 3rd that never
	// happened.
	if len(rep.Buckets) != 5 {
		t.Fatalf("got %d buckets, want 5", len(rep.Buckets))
	}
	want := []int64{4, 0, 0, 2, 0}
	for i, w := range want {
		if got := rep.Buckets[i].Matches.Completed; got != w {
			t.Errorf("%s completed = %d, want %d", rep.Buckets[i].Label, got, w)
		}
	}
	if rep.Timezone != "UTC" {
		t.Errorf("timezone = %q, want UTC", rep.Timezone)
	}
}

// The three meanings of "unfinished" have to stay apart, and the completion
// rate must not be computed over a lobby that never became a game.
func TestUnfinishedSplitsThreeWays(t *testing.T) {
	ctx := t.Context()
	store := newKDBStore(t)
	_ = store.Merge(ctx, "2026-09-08", map[string]int64{
		metrics.MatchesCreated:   10,
		metrics.MatchesStarted:   6,
		metrics.MatchesCompleted: 4,
		metrics.MatchesAbandoned: 1,
	}, nil, false)

	rep, _ := metrics.NewReporter(store).Build(ctx, day("2026-09-08"), day("2026-09-08"), metrics.BucketDay)
	m := rep.Buckets[0].Matches

	if m.NeverStarted != 4 {
		t.Errorf("neverStarted = %d, want 4 (10 created, 6 started)", m.NeverStarted)
	}
	// 4 of 5 resolved games finished. Over `created` it would read 40%, which
	// would blame the players for four lobbies nobody joined.
	if got := m.CompletionRate; got < 0.79 || got > 0.81 {
		t.Errorf("completionRate = %v, want 0.8", got)
	}
}

// A bucket in which nothing resolved reports zero, not an absence. Every other
// figure on the row is a zero in that case, and a reader scanning the column
// should not have to work out what a blank means.
func TestCompletionRateIsZeroWhenNothingResolved(t *testing.T) {
	ctx := t.Context()
	store := newKDBStore(t)
	_ = store.Merge(ctx, "2026-09-08", map[string]int64{metrics.MatchesCreated: 3}, nil, false)

	rep, _ := metrics.NewReporter(store).Build(ctx, day("2026-09-08"), day("2026-09-08"), metrics.BucketDay)
	if got := rep.Buckets[0].Matches.CompletionRate; got != 0 {
		t.Errorf("completionRate = %v, want 0", got)
	}
	// And an entirely empty day is a full row of zeroes, not a gap.
	empty, _ := metrics.NewReporter(store).Build(ctx, day("2026-09-07"), day("2026-09-07"), metrics.BucketDay)
	b := empty.Buckets[0]
	if b.Matches.Completed != 0 || b.Matches.CompletionRate != 0 || b.Players.Distinct != 0 {
		t.Errorf("an empty day did not report zeroes: %+v", b)
	}
}

// The property a counter could never have: one person playing on two days is
// one player for the week, not two.
func TestDistinctPlayersAreUnionedNotSummed(t *testing.T) {
	ctx := t.Context()
	store := newKDBStore(t)
	_ = store.Merge(ctx, "2026-09-07", nil, []string{"user:a", "user:b"}, false)
	_ = store.Merge(ctx, "2026-09-08", nil, []string{"user:b", "user:c"}, false)

	rep, _ := metrics.NewReporter(store).Build(ctx, day("2026-09-07"), day("2026-09-08"), metrics.BucketWeek)

	if len(rep.Buckets) != 1 {
		t.Fatalf("got %d weekly buckets, want 1 (both days are the same ISO week)", len(rep.Buckets))
	}
	if got := rep.Buckets[0].Players.Distinct; got != 3 {
		t.Errorf("weekly distinct players = %d, want 3", got)
	}
	if got := rep.Totals.Players.Distinct; got != 3 {
		t.Errorf("total distinct players = %d, want 3", got)
	}
}

func TestTruncationSurfacesAsAFloor(t *testing.T) {
	ctx := t.Context()
	store := newKDBStore(t)
	_ = store.Merge(ctx, "2026-09-08", nil, []string{"user:a"}, true)

	rep, _ := metrics.NewReporter(store).Build(ctx, day("2026-09-08"), day("2026-09-08"), metrics.BucketDay)
	if !rep.Buckets[0].Players.IsFloor {
		t.Error("a saturated day does not report its player count as a floor")
	}
	if !rep.Totals.Players.IsFloor {
		t.Error("the range total does not carry the floor forward")
	}
}

// ISO weeks start on Monday. Go's Weekday puts Sunday at zero, so the naive
// arithmetic makes Sunday the start of the following week and moves every
// Sunday's play into the wrong bucket.
func TestWeeksStartOnMonday(t *testing.T) {
	ctx := t.Context()
	store := newKDBStore(t)
	// 2026-09-06 is a Sunday; 2026-09-07 the Monday after it.
	_ = store.Merge(ctx, "2026-09-06", map[string]int64{metrics.MatchesCompleted: 1}, nil, false)
	_ = store.Merge(ctx, "2026-09-07", map[string]int64{metrics.MatchesCompleted: 1}, nil, false)

	if got := day("2026-09-06").Weekday(); got != time.Sunday {
		t.Fatalf("test fixture wrong: 2026-09-06 is a %v", got)
	}

	rep, _ := metrics.NewReporter(store).Build(ctx, day("2026-09-06"), day("2026-09-07"), metrics.BucketWeek)
	if len(rep.Buckets) != 2 {
		t.Fatalf("got %d buckets, want 2 — the Sunday and the Monday are different ISO weeks", len(rep.Buckets))
	}
}

// A range that only partly covers a week or a month keeps the partial bucket
// and says so. Dropping it loses real play; drawing it unmarked next to full
// buckets invites the reader to see a decline that is an artefact of the range.
func TestPartialBucketsAreMarked(t *testing.T) {
	ctx := t.Context()
	store := newKDBStore(t)
	_ = store.Merge(ctx, "2026-09-10", map[string]int64{metrics.MatchesCompleted: 1}, nil, false)

	rep, _ := metrics.NewReporter(store).Build(ctx, day("2026-09-10"), day("2026-09-20"), metrics.BucketMonth)
	if len(rep.Buckets) != 1 {
		t.Fatalf("got %d monthly buckets, want 1", len(rep.Buckets))
	}
	b := rep.Buckets[0]
	if !b.Partial {
		t.Error("a ten-day slice of September is not marked partial")
	}
	// Clipped to the range, not expanded to the whole month: a bucket must
	// not claim to cover days the caller did not ask about.
	if b.Start != "2026-09-10" || b.End != "2026-09-20" {
		t.Errorf("bucket spans %s..%s, want the requested range", b.Start, b.End)
	}
	if b.Label != "2026-09" {
		t.Errorf("label = %q, want 2026-09", b.Label)
	}
}

func TestPerModuleAndPerReasonSplits(t *testing.T) {
	ctx := t.Context()
	store := newKDBStore(t)
	_ = store.Merge(ctx, "2026-09-08", map[string]int64{
		metrics.MatchesCompleted:                       6,
		metrics.MatchesCompletedFor("zolik"):           4,
		metrics.MatchesCompletedFor("prsi"):            2,
		metrics.AdmissionRefusedFor("at_capacity"):     7,
		metrics.AdmissionRefusedFor("memory_pressure"): 3,
		metrics.AdmissionRefusedMatchStart:             5,
	}, nil, false)

	rep, _ := metrics.NewReporter(store).Build(ctx, day("2026-09-08"), day("2026-09-08"), metrics.BucketDay)
	b := rep.Buckets[0]

	if len(b.Matches.ByModule) != 2 || b.Matches.ByModule[0].ModuleID != "zolik" {
		t.Errorf("byModule = %+v, want zolik first", b.Matches.ByModule)
	}
	if b.Admission.Total != 10 {
		t.Errorf("refusals total = %d, want 10", b.Admission.Total)
	}
	// Counted on its own line and deliberately not in Total: a player told
	// "not now" before a socket opened was refused once, and adding the two
	// would report them twice.
	if b.Admission.MatchStartDenied != 5 {
		t.Errorf("matchStartDenied = %d, want 5", b.Admission.MatchStartDenied)
	}
	if len(b.Admission.ByReason) != 2 || b.Admission.ByReason[0].Reason != "at_capacity" {
		t.Errorf("byReason = %+v, want at_capacity first", b.Admission.ByReason)
	}
}

func TestRangeIsClampedAndOrdered(t *testing.T) {
	ctx := t.Context()
	store := newKDBStore(t)
	r := metrics.NewReporter(store)

	// Backwards: normalised rather than returning nothing.
	rep, err := r.Build(ctx, day("2026-09-08"), day("2026-09-01"), metrics.BucketDay)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if rep.From != "2026-09-01" || rep.To != "2026-09-08" {
		t.Errorf("range %s..%s was not reordered", rep.From, rep.To)
	}

	// Absurdly long: clamped, so one request cannot become an unbounded
	// number of lookups.
	rep, err = r.Build(ctx, day("2000-01-01"), day("2026-09-08"), metrics.BucketDay)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(rep.Buckets) > metrics.MaxRangeDays {
		t.Errorf("got %d buckets, want at most %d", len(rep.Buckets), metrics.MaxRangeDays)
	}
}

func TestOpsCountsComeFromBootRows(t *testing.T) {
	ctx := t.Context()
	store := newKDBStore(t)

	// One process that died, and the successor that noticed.
	metrics.NewBootRecorder(store, newCountingSink()).Start(ctx, "1.0")
	metrics.NewBootRecorder(store, newCountingSink()).Start(ctx, "1.0")

	today := time.Now().UTC()
	rep, err := metrics.NewReporter(store).Build(ctx, today, today, metrics.BucketDay)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if rep.Totals.Ops.Boots != 2 {
		t.Errorf("boots = %d, want 2", rep.Totals.Ops.Boots)
	}
	if rep.Totals.Ops.UncleanBoots != 1 {
		t.Errorf("uncleanBoots = %d, want 1", rep.Totals.Ops.UncleanBoots)
	}
}

func TestParseBucket(t *testing.T) {
	for in, want := range map[string]metrics.Bucket{
		"day": metrics.BucketDay, "week": metrics.BucketWeek, "MONTH": metrics.BucketMonth,
		"": metrics.BucketDay, "fortnight": metrics.BucketDay,
	} {
		if got := metrics.ParseBucket(in); got != want {
			t.Errorf("ParseBucket(%q) = %q, want %q", in, got, want)
		}
	}
}
