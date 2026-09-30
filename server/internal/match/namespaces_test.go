package match

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
)

// What these check is the layout itself, not just that a round trip works: a
// match has to be in a namespace of its own, because that is the only unit KDB
// will replicate to one player's devices and not another's. A test that only
// read back through the repository would pass just as happily with everything
// still in one shared namespace.

func newKDBStore(t *testing.T) (*db.KDB, Repository) {
	t.Helper()
	k, err := db.OpenKDB(t.TempDir())
	if err != nil {
		t.Fatalf("opening kdb: %v", err)
	}
	t.Cleanup(func() { _ = k.Close(context.Background()) })
	return k, NewKDBRepository(k)
}

func mustHave(t *testing.T, k *db.KDB, ns, key string) []byte {
	t.Helper()
	raw, err := k.Get(ns, key)
	if err != nil {
		t.Fatalf("%s %q: %v", ns, key, err)
	}
	return raw
}

func TestKDBMatchLivesInItsOwnNamespace(t *testing.T) {
	k, r := newKDBStore(t)
	ctx := context.Background()

	m, err := r.Insert(ctx, models.Match{ModuleID: "prsi", Status: "active", JoinCode: "NSP001"})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	ns := db.MatchNS(m.ID.Hex())

	move := models.MatchAction{Seq: 1, PlayerID: "p1", Action: models.JSONDoc(`{"verb":"draw"}`), At: time.Now()}
	if err := r.AppendMove(ctx, m.ID, move); err != nil {
		t.Fatalf("append: %v", err)
	}
	next := m
	next.Snapshots = []int{2}
	board := models.JSONDoc(`{"drawPile":[1,2,3]}`)
	second := models.MatchAction{Seq: 2, PlayerID: "p1", Action: models.JSONDoc(`{"verb":"pass"}`), At: time.Now()}
	if err := r.CommitSnapshot(ctx, m.ID, m.Version, next, []models.MatchAction{second}, 2, board); err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	// The envelope, both moves and the board, all under the one namespace this
	// match would be replicated by.
	var env models.Match
	if err := db.UnmarshalDoc(mustHave(t, k, ns, keyMatchEnvelope), &env); err != nil {
		t.Fatalf("decoding the envelope: %v", err)
	}
	if env.ID != m.ID || env.JoinCode != "NSP001" {
		t.Fatalf("the namespace holds the wrong match: %+v", env)
	}
	mustHave(t, k, ns, "m/1")
	mustHave(t, k, ns, "m/2")
	mustHave(t, k, ns, "s/2")

	// And nothing in the shared log the old layout used.
	left := 0
	if err := k.Scan(db.NSMatchLog, func([]byte) error { left++; return nil }); err != nil {
		t.Fatalf("scanning the old log: %v", err)
	}
	if left != 0 {
		t.Errorf("%d records went to %s, which no longer holds a match's history", left, db.NSMatchLog)
	}

	// Read back through the repository as well, so the keys above are the ones
	// the reads actually use.
	moves, err := r.Moves(ctx, m.ID, 0, -1)
	if err != nil || len(moves) != 2 {
		t.Fatalf("moves = %d (%v), want 2", len(moves), err)
	}
	got, err := r.Snapshot(ctx, m.ID, 2)
	if err != nil || string(got) != string(board) {
		t.Fatalf("snapshot = %s (%v), want %s", got, err, board)
	}
}

func TestKDBIndexKeepsStepWithTheEnvelope(t *testing.T) {
	k, r := newKDBStore(t)
	ctx := context.Background()

	m, err := r.Insert(ctx, models.Match{ModuleID: "prsi", Status: "lobby", JoinCode: "NSP002"})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	next := m
	next.Status = "active"
	if err := r.UpdateWithVersion(ctx, m.ID, m.Version, next); err != nil {
		t.Fatalf("update: %v", err)
	}

	// The index is what every scan reads, so a change that reached the match
	// and not the index would be invisible to "my games", the join-code lookup
	// and the abandonment sweep alike.
	var indexed, owned models.Match
	if err := db.UnmarshalDoc(mustHave(t, k, db.NSMatches, m.ID.Hex()), &indexed); err != nil {
		t.Fatalf("decoding the index copy: %v", err)
	}
	if err := db.UnmarshalDoc(mustHave(t, k, db.MatchNS(m.ID.Hex()), keyMatchEnvelope), &owned); err != nil {
		t.Fatalf("decoding the envelope: %v", err)
	}
	if indexed.Status != owned.Status || indexed.Version != owned.Version {
		t.Fatalf("index says %s/v%d, the match says %s/v%d",
			indexed.Status, indexed.Version, owned.Status, owned.Version)
	}
	if owned.Status != "active" || owned.Version != 2 {
		t.Fatalf("the update did not land: %s/v%d", owned.Status, owned.Version)
	}

	// And the scan that reads the index agrees with the read that does not.
	byCode, err := r.FindByJoinCode(ctx, "NSP002")
	if err != nil || byCode.Status != "active" {
		t.Fatalf("FindByJoinCode = %+v (%v)", byCode, err)
	}

	// Deleting takes the index copy with it, or the sweeps would keep finding
	// a match nothing holds any more.
	if err := r.DeleteIfUnchanged(ctx, m.ID, 2); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := k.Get(db.NSMatches, m.ID.Hex()); !db.IsNotFound(err) {
		t.Fatalf("the index still lists a deleted match: %v", err)
	}
	if _, err := k.Get(db.MatchNS(m.ID.Hex()), keyMatchEnvelope); !db.IsNotFound(err) {
		t.Fatalf("the match namespace still holds a deleted match: %v", err)
	}
}

// A move is stored once, and the second writer of seq N finds out by being
// refused — the contract the runtime's whole optimistic loop rests on.
func TestKDBDuplicateSeqIsAVersionConflict(t *testing.T) {
	_, r := newKDBStore(t)
	ctx := context.Background()

	m, err := r.Insert(ctx, models.Match{ModuleID: "prsi", Status: "active"})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	move := models.MatchAction{Seq: 1, PlayerID: "p1", Action: models.JSONDoc(`{"verb":"draw"}`), At: time.Now()}
	if err := r.AppendMove(ctx, m.ID, move); err != nil {
		t.Fatalf("first append: %v", err)
	}
	other := move
	other.Action = models.JSONDoc(`{"verb":"pass"}`)
	if err := r.AppendMove(ctx, m.ID, other); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("second move 1: %v, want ErrVersionConflict", err)
	}
	// A snapshot commit carrying the same move is refused the same way.
	next := m
	next.Snapshots = []int{1}
	err = r.CommitSnapshot(ctx, m.ID, m.Version, next, []models.MatchAction{other}, 1, models.JSONDoc(`{}`))
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("snapshot re-committing move 1: %v, want ErrVersionConflict", err)
	}
}

// writeOldLayout stores a match the way the store did before matches had
// namespaces: the envelope in the shared matches namespace, every move and
// snapshot in the shared log. This is what a database that has not been
// through cmd/migrate-namespaces looks like.
func writeOldLayout(t *testing.T, k *db.KDB, m models.Match, moves []models.MatchAction, snapAt int, board models.JSONDoc) {
	t.Helper()
	env, err := db.MarshalDoc(m)
	if err != nil {
		t.Fatalf("marshalling the envelope: %v", err)
	}
	if err := k.Put(db.NSMatches, m.ID.Hex(), env); err != nil {
		t.Fatalf("writing the envelope: %v", err)
	}
	for _, mv := range moves {
		doc, err := db.MarshalDoc(toMoveRecord(m.ID, mv))
		if err != nil {
			t.Fatalf("marshalling move %d: %v", mv.Seq, err)
		}
		if err := k.Put(db.NSMatchLog, logKey(m.ID, kindMove, mv.Seq), doc); err != nil {
			t.Fatalf("writing move %d: %v", mv.Seq, err)
		}
	}
	if board != nil {
		doc, err := db.MarshalDoc(snapshotRecord{Match: m.ID, Kind: kindSnapshot, Seq: snapAt, State: board})
		if err != nil {
			t.Fatalf("marshalling the snapshot: %v", err)
		}
		if err := k.Put(db.NSMatchLog, logKey(m.ID, kindSnapshot, snapAt), doc); err != nil {
			t.Fatalf("writing the snapshot: %v", err)
		}
	}
}

func oldLayoutMatch() (models.Match, []models.MatchAction, models.JSONDoc) {
	m := models.Match{
		ID: bson.NewObjectID(), ModuleID: "prsi", Status: "active", JoinCode: "OLD001",
		Players: []models.Player{{ID: "p1", Name: "Ada"}}, Snapshots: []int{2},
		Version: 7, CreatedAt: time.Now().UTC().Add(-time.Hour),
	}
	moves := []models.MatchAction{
		{Seq: 1, PlayerID: "p1", Action: models.JSONDoc(`{"verb":"draw"}`), At: time.Now().UTC()},
		{Seq: 2, PlayerID: "p1", Action: models.JSONDoc(`{"verb":"pass"}`), At: time.Now().UTC()},
	}
	return m, moves, models.JSONDoc(`{"drawPile":[9,8,7]}`)
}

// A database nobody has migrated keeps working, in full: the envelope, the
// moves and the board all still read.
func TestKDBReadsAMatchInTheOldLayout(t *testing.T) {
	k, r := newKDBStore(t)
	ctx := context.Background()
	m, moves, board := oldLayoutMatch()
	writeOldLayout(t, k, m, moves, 2, board)

	got, err := r.FindByID(ctx, m.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.JoinCode != "OLD001" || got.Version != 7 {
		t.Fatalf("read back %+v, want the stored envelope", got)
	}
	if byCode, err := r.Resolve(ctx, "OLD001"); err != nil || byCode.ID != m.ID {
		t.Fatalf("Resolve by code: %+v (%v)", byCode, err)
	}
	back, err := r.Moves(ctx, m.ID, 0, -1)
	if err != nil || len(back) != 2 || back[1].PlayerID != "p1" {
		t.Fatalf("moves = %+v (%v), want the two logged moves", back, err)
	}
	snap, err := r.Snapshot(ctx, m.ID, 2)
	if err != nil || string(snap) != string(board) {
		t.Fatalf("snapshot = %s (%v), want %s", snap, err, board)
	}

	// Playing on stores the new move in the match's own namespace while the
	// old ones stay in the log, and the read has to span both.
	third := models.MatchAction{Seq: 3, PlayerID: "p1", Action: models.JSONDoc(`{"verb":"draw"}`), At: time.Now()}
	if err := r.AppendMove(ctx, m.ID, third); err != nil {
		t.Fatalf("appending to an unmigrated match: %v", err)
	}
	back, err = r.Moves(ctx, m.ID, 0, -1)
	if err != nil || len(back) != 3 {
		t.Fatalf("moves after playing on = %d (%v), want 3", len(back), err)
	}
	// Re-appending a move the old log already holds is still a conflict, or
	// the same seq would end up in both places with different contents.
	if err := r.AppendMove(ctx, m.ID, moves[1]); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("re-appending a logged move: %v, want ErrVersionConflict", err)
	}

	// An ordinary update migrates the envelope of its own accord: the version
	// check reads the index's copy and the write puts one in the match's
	// namespace.
	next := got
	next.Status = "completed"
	if err := r.UpdateWithVersion(ctx, m.ID, 7, next); err != nil {
		t.Fatalf("update: %v", err)
	}
	var owned models.Match
	if err := db.UnmarshalDoc(mustHave(t, k, db.MatchNS(m.ID.Hex()), keyMatchEnvelope), &owned); err != nil {
		t.Fatalf("decoding the envelope: %v", err)
	}
	if owned.Status != "completed" || owned.Version != 8 {
		t.Fatalf("the match's own namespace has %s/v%d", owned.Status, owned.Version)
	}
}

// Deleting an unmigrated match has to reach into the old log as well, or its
// moves outlive it.
func TestKDBDeletesAMatchInTheOldLayout(t *testing.T) {
	k, r := newKDBStore(t)
	ctx := context.Background()
	m, moves, board := oldLayoutMatch()
	writeOldLayout(t, k, m, moves, 2, board)

	if err := r.DeleteIfUnchanged(ctx, m.ID, 7); err != nil {
		t.Fatalf("delete: %v", err)
	}
	left := 0
	if err := k.Scan(db.NSMatchLog, func([]byte) error { left++; return nil }); err != nil {
		t.Fatalf("scanning the old log: %v", err)
	}
	if left != 0 {
		t.Errorf("%d log records outlived their match", left)
	}
	if _, err := k.Get(db.NSMatches, m.ID.Hex()); !db.IsNotFound(err) {
		t.Fatalf("the index still lists a deleted match: %v", err)
	}
}

func TestMigrateNamespacesMovesAnOldMatch(t *testing.T) {
	k, r := newKDBStore(t)
	ctx := context.Background()
	m, moves, board := oldLayoutMatch()
	writeOldLayout(t, k, m, moves, 2, board)

	dry, err := MigrateNamespacesKDB(ctx, k, true)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if dry.Matches != 1 || dry.Records != 3 {
		t.Fatalf("dry run counted %+v, want one match and its three records", dry)
	}
	// A dry run writes nothing.
	if _, err := k.Get(db.MatchNS(m.ID.Hex()), keyMatchEnvelope); !db.IsNotFound(err) {
		t.Fatalf("the dry run moved the envelope: %v", err)
	}

	report, err := MigrateNamespacesKDB(ctx, k, false)
	if err != nil {
		t.Fatalf("migrating: %v", err)
	}
	if report.Matches != 1 || report.Records != 3 || report.Verified != 1 {
		t.Fatalf("migration reported %+v, want one verified match and three records", report)
	}

	ns := db.MatchNS(m.ID.Hex())
	mustHave(t, k, ns, keyMatchEnvelope)
	mustHave(t, k, ns, "m/1")
	mustHave(t, k, ns, "m/2")
	mustHave(t, k, ns, "s/2")
	// The index copy stays: it is what every scan reads.
	mustHave(t, k, db.NSMatches, m.ID.Hex())
	left := 0
	if err := k.Scan(db.NSMatchLog, func([]byte) error { left++; return nil }); err != nil {
		t.Fatalf("scanning the old log: %v", err)
	}
	if left != 0 {
		t.Errorf("%d records stayed in %s", left, db.NSMatchLog)
	}

	// The match reads the same as it did before it moved.
	got, err := r.FindByID(ctx, m.ID)
	if err != nil || got.JoinCode != "OLD001" || got.Version != 7 {
		t.Fatalf("after migrating: %+v (%v)", got, err)
	}
	back, err := r.Moves(ctx, m.ID, 0, -1)
	if err != nil || len(back) != 2 {
		t.Fatalf("moves after migrating = %d (%v), want 2", len(back), err)
	}
	snap, err := r.Snapshot(ctx, m.ID, 2)
	if err != nil || string(snap) != string(board) {
		t.Fatalf("snapshot after migrating = %s (%v)", snap, err)
	}
}

// Running it again moves nothing and says so, which is what makes it safe to
// run from a deploy script that cannot know whether it has run before.
func TestMigrateNamespacesIsIdempotent(t *testing.T) {
	k, _ := newKDBStore(t)
	ctx := context.Background()
	m, moves, board := oldLayoutMatch()
	writeOldLayout(t, k, m, moves, 2, board)

	if _, err := MigrateNamespacesKDB(ctx, k, false); err != nil {
		t.Fatalf("first run: %v", err)
	}
	again, err := MigrateNamespacesKDB(ctx, k, false)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if again.Matches != 0 || again.Records != 0 || again.Skipped != 1 {
		t.Fatalf("second run reported %+v, want nothing moved and one skipped", again)
	}
	if _, err := k.Get(db.MatchNS(m.ID.Hex()), "m/1"); err != nil {
		t.Fatalf("the second run disturbed a migrated match: %v", err)
	}
}

// A match whose envelope the server already wrote — it was played on after the
// deploy and before the migration — still has its moves in the old log, and
// the migration has to move them rather than take the envelope as proof.
func TestMigrateNamespacesMovesRecordsOfAHalfMovedMatch(t *testing.T) {
	k, r := newKDBStore(t)
	ctx := context.Background()
	m, moves, board := oldLayoutMatch()
	writeOldLayout(t, k, m, moves, 2, board)

	next := m
	next.Status = "suspended"
	if err := r.UpdateWithVersion(ctx, m.ID, 7, next); err != nil {
		t.Fatalf("playing on before the migration: %v", err)
	}
	mustHave(t, k, db.MatchNS(m.ID.Hex()), keyMatchEnvelope)

	report, err := MigrateNamespacesKDB(ctx, k, false)
	if err != nil {
		t.Fatalf("migrating: %v", err)
	}
	if report.Matches != 1 || report.Records != 3 {
		t.Fatalf("migration reported %+v, want the three records moved", report)
	}
	mustHave(t, k, db.MatchNS(m.ID.Hex()), "m/1")
	// The envelope the server wrote is the newer one, and moving the records
	// must not have put the old copy back over it.
	var owned models.Match
	if err := db.UnmarshalDoc(mustHave(t, k, db.MatchNS(m.ID.Hex()), keyMatchEnvelope), &owned); err != nil {
		t.Fatalf("decoding the envelope: %v", err)
	}
	if owned.Status != "suspended" || owned.Version != 8 {
		t.Fatalf("the migration overwrote a newer envelope: %s/v%d", owned.Status, owned.Version)
	}
}
