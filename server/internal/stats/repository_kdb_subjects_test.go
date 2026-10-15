package stats

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db"
)

func openSubjectTestRepo(t *testing.T) (*db.KDB, *kdbRepository) {
	t.Helper()
	k, err := db.OpenKDB(t.TempDir())
	if err != nil {
		t.Fatalf("opening kdb: %v", err)
	}
	t.Cleanup(func() { _ = k.Close(context.Background()) })
	return k, NewKDBRepository(k).(*kdbRepository)
}

func subjectRecord(keys []string, at time.Time) MatchResult {
	rec := kdbRecord(bson.NewObjectID(), keys[0], at)
	rec.ID = bson.NewObjectID()
	rec.SubjectKeys = keys
	return rec
}

// putLegacy writes a result exactly as a volume from before the subject index
// would hold it: a match_results row, and nothing else.
func putLegacy(t *testing.T, k *db.KDB, m MatchResult, key string) {
	t.Helper()
	doc, err := db.MarshalDoc(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := k.Put(db.NSMatchResults, key, doc); err != nil {
		t.Fatal(err)
	}
}

func TestSubjectIndexIsBuiltFromLegacyResultsOnFirstQuery(t *testing.T) {
	k, r := openSubjectTestRepo(t)
	ctx := context.Background()
	base := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)

	// One row keyed by its match, one by its own id (older still), neither
	// indexed.
	a := subjectRecord([]string{"guest:g1", "ai:hard"}, base)
	putLegacy(t, k, a, resultKey(a.MatchID))
	b := subjectRecord([]string{"guest:g1", "user:u1"}, base.Add(time.Hour))
	putLegacy(t, k, b, b.ID.Hex())
	c := subjectRecord([]string{"user:u1"}, base.Add(2*time.Hour))
	putLegacy(t, k, c, resultKey(c.MatchID))

	if n, err := r.CountMatchesForSubject(ctx, "guest:g1"); err != nil || n != 2 {
		t.Fatalf("guest count = %d (%v), want 2", n, err)
	}
	if n, err := r.CountMatchesForSubject(ctx, "guest:nobody"); err != nil || n != 0 {
		t.Fatalf("unknown guest count = %d (%v), want 0", n, err)
	}
	// Bots are not indexed and still answer, by scan.
	if n, err := r.CountMatchesForSubject(ctx, "ai:hard"); err != nil || n != 1 {
		t.Fatalf("ai count = %d (%v), want 1", n, err)
	}
	list, err := r.ListMatchesForSubject(ctx, "user:u1", time.Time{}, 10)
	if err != nil || len(list) != 2 || list[0].MatchID != c.MatchID || list[1].MatchID != b.MatchID {
		t.Fatalf("list = %v (%v), want newest first [c b]", list, err)
	}
	if _, err := k.Get(db.NSMatchSubjects, subjectIndexBuiltKey); err != nil {
		t.Fatalf("marker not written: %v", err)
	}
}

func TestSubjectIndexFollowsRecordingAndReattribution(t *testing.T) {
	_, r := openSubjectTestRepo(t)
	ctx := context.Background()
	base := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)

	// Query first, so the index is built and empty before anything is recorded.
	if n, _ := r.CountMatchesForSubject(ctx, "guest:g1"); n != 0 {
		t.Fatalf("count before = %d", n)
	}
	rec := subjectRecord([]string{"guest:g1", "ai:easy"}, base)
	if _, err := r.InsertMatch(ctx, rec); err != nil {
		t.Fatal(err)
	}
	if _, err := r.InsertMatch(ctx, rec); err != ErrAlreadyRecorded {
		t.Fatalf("second insert = %v", err)
	}
	if n, _ := r.CountMatchesForSubject(ctx, "guest:g1"); n != 1 {
		t.Fatalf("guest count after insert = %d, want 1", n)
	}

	// The guest signs in: the same record now names the account.
	moved := rec
	moved.SubjectKeys = []string{"user:u1", "ai:easy"}
	if err := r.ReplaceMatchAttribution(ctx, moved); err != nil {
		t.Fatal(err)
	}
	if n, _ := r.CountMatchesForSubject(ctx, "guest:g1"); n != 0 {
		t.Fatalf("guest count after claim = %d, want 0", n)
	}
	var walked int
	if err := r.EachMatchForSubject(ctx, "user:u1", func(m MatchResult) error {
		walked++
		if m.SubjectKeys[0] != "user:u1" {
			t.Errorf("record not re-attributed: %v", m.SubjectKeys)
		}
		return nil
	}); err != nil || walked != 1 {
		t.Fatalf("walk = %d (%v), want 1", walked, err)
	}
}

func TestSubjectIndexRebuildsAfterItIsInvalidated(t *testing.T) {
	k, r := openSubjectTestRepo(t)
	ctx := context.Background()
	base := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)

	if _, err := r.InsertMatch(ctx, subjectRecord([]string{"guest:g1"}, base)); err != nil {
		t.Fatal(err)
	}
	if n, _ := r.CountMatchesForSubject(ctx, "guest:g1"); n != 1 {
		t.Fatalf("count = %d", n)
	}
	// A result that arrives some way the index never heard of, then the
	// marker is cleared as a failed update would clear it.
	late := subjectRecord([]string{"guest:g1"}, base.Add(time.Hour))
	putLegacy(t, k, late, resultKey(late.MatchID))
	r.invalidateSubjectIndex()
	if n, _ := r.CountMatchesForSubject(ctx, "guest:g1"); n != 2 {
		t.Fatalf("count after rebuild = %d, want 2", n)
	}
}

func TestSubjectIndexPagingMatchesTheScan(t *testing.T) {
	_, r := openSubjectTestRepo(t)
	ctx := context.Background()
	base := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 7; i++ {
		if _, err := r.InsertMatch(ctx, subjectRecord([]string{"user:pager"}, base.Add(time.Duration(i%4)*time.Hour))); err != nil {
			t.Fatal(err)
		}
	}
	want, err := r.scanMatchesForSubject("user:pager")
	if err != nil || len(want) != 7 {
		t.Fatalf("scan = %d (%v)", len(want), err)
	}
	got, err := r.ListMatchesForSubject(ctx, "user:pager", time.Time{}, 100)
	if err != nil || len(got) != 7 {
		t.Fatalf("indexed list = %d (%v)", len(got), err)
	}
	for i := 1; i < len(got); i++ {
		if got[i].CompletedAt.After(got[i-1].CompletedAt) {
			t.Fatalf("not newest-first at %d", i)
		}
	}
	page, _ := r.ListMatchesForSubject(ctx, "user:pager", time.Time{}, 3)
	rest, _ := r.ListMatchesForSubject(ctx, "user:pager", page[2].CompletedAt, 100)
	for _, m := range rest {
		if !m.CompletedAt.Before(page[2].CompletedAt) {
			t.Fatalf("keyset paging returned %v", m.CompletedAt)
		}
	}
}
