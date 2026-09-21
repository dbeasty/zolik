package match

import (
	"context"
	"fmt"
	"testing"
	"time"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
)

// The repository contract for where a match's log lives. Each test runs
// against the embedded store, in whichever layout the case names.

type logStore struct {
	k    *db.KDB
	repo Repository
}

func openLogStore(t *testing.T) logStore {
	t.Helper()
	k, err := db.OpenKDB(t.TempDir())
	if err != nil {
		t.Skipf("no embedded kdb available here: %v", err)
	}
	t.Cleanup(func() { _ = k.Close(context.Background()) })
	// Each test starts in the original layout with migration off, and says so
	// when it wants otherwise, so the defaults a release ships with cannot
	// quietly change what these cases exercise.
	withSwitches(t, "", false)
	return logStore{k: k, repo: NewKDBRepository(k)}
}

func withSwitches(t *testing.T, format string, migrate bool) {
	t.Helper()
	f, m := newMatchLogFormat, migrateLegacyOnAppend
	newMatchLogFormat, migrateLegacyOnAppend = format, migrate
	t.Cleanup(func() { newMatchLogFormat, migrateLegacyOnAppend = f, m })
}

func (s logStore) insert(t *testing.T, format string) models.Match {
	t.Helper()
	m, err := s.repo.Insert(context.Background(), models.Match{
		ModuleID:  "prsi",
		Status:    "active",
		Players:   []models.Player{{ID: "p1", Name: "p1"}, {ID: "p2", Name: "p2"}},
		State:     models.JSONDoc(`{"move":0}`),
		LogFormat: format,
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	return m
}

// appendN plays n moves through AppendAction, the way HandleAction does,
// closing a round with a stored board every boardEvery moves (0 for never).
func (s logStore) appendN(t *testing.T, m models.Match, n, boardEvery int) models.Match {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < n; i++ {
		seq := m.ActionCount + 1
		m.State = models.JSONDoc(fmt.Sprintf(`{"move":%d}`, seq))
		var cp *models.MatchCheckpoint
		var board models.JSONDoc
		if boardEvery > 0 && seq%boardEvery == 0 {
			cp = &models.MatchCheckpoint{Seq: seq, Round: len(m.Checkpoints) + 1, At: time.Now().UTC()}
			board = m.State
		}
		if err := s.repo.AppendAction(ctx, m.ID, m.Version, m, action(seq), cp, board); err != nil {
			t.Fatalf("append %d: %v", seq, err)
		}
		var err error
		if m, err = s.repo.FindByID(ctx, m.ID); err != nil {
			t.Fatalf("reload after %d: %v", seq, err)
		}
	}
	return m
}

func assertLog(t *testing.T, repo Repository, m models.Match, from, to int) {
	t.Helper()
	got, err := repo.ActionLog(context.Background(), m.ID, from, to)
	if err != nil {
		t.Fatalf("ActionLog(%d,%d): %v", from, to, err)
	}
	if len(got) != to-from {
		t.Fatalf("ActionLog(%d,%d) returned %d entries", from, to, len(got))
	}
	for i, a := range got {
		want := action(from + i + 1)
		if a.Seq != want.Seq || string(a.Action) != string(want.Action) || a.PlayerID != want.PlayerID {
			t.Fatalf("entry %d = %+v, want %+v", i, a, want)
		}
	}
}

func TestLogAppendsAcrossPageBoundaries(t *testing.T) {
	for _, format := range []string{"", models.LogFormatPages} {
		t.Run("format="+format, func(t *testing.T) {
			s := openLogStore(t)
			m := s.appendN(t, s.insert(t, format), 70, 0)
			if m.ActionCount != 70 {
				t.Fatalf("count %d after 70 appends", m.ActionCount)
			}
			assertLog(t, s.repo, m, 0, 70)
			assertLog(t, s.repo, m, 31, 33)
			assertLog(t, s.repo, m, 64, 70)
		})
	}
}

// A stale version, and a seq that is not the next one, both refuse without
// touching the log.
func TestLogRefusesAStaleOrOutOfOrderAppend(t *testing.T) {
	for _, format := range []string{"", models.LogFormatPages} {
		t.Run("format="+format, func(t *testing.T) {
			ctx := context.Background()
			s := openLogStore(t)
			m := s.appendN(t, s.insert(t, format), 5, 0)

			if err := s.repo.AppendAction(ctx, m.ID, m.Version-1, m, action(6), nil, nil); err != ErrVersionConflict {
				t.Errorf("stale version: %v, want a conflict", err)
			}
			if err := s.repo.AppendAction(ctx, m.ID, m.Version, m, action(8), nil, nil); err != ErrVersionConflict {
				t.Errorf("skipped seq: %v, want a conflict", err)
			}
			after, _ := s.repo.FindByID(ctx, m.ID)
			if after.ActionCount != 5 || after.Version != m.Version {
				t.Errorf("a refused append moved the match: count %d version %d", after.ActionCount, after.Version)
			}
			assertLog(t, s.repo, after, 0, 5)
		})
	}
}

// The case the review found: a writer fed by one of the scans hands back a
// match whose log it never loaded. The stored log and count must survive it.
func TestAnEnvelopeWriteKeepsTheLog(t *testing.T) {
	for _, format := range []string{"", models.LogFormatPages} {
		t.Run("format="+format, func(t *testing.T) {
			ctx := context.Background()
			s := openLogStore(t)
			m := s.appendN(t, s.insert(t, format), 40, 20)

			scanned, err := s.repo.FindStranded(ctx, time.Now().Add(time.Hour), 10)
			if err != nil || len(scanned) != 1 {
				t.Fatalf("FindStranded: %v, %d rows", err, len(scanned))
			}
			row := scanned[0]
			row.ActionCount, row.Checkpoints = 0, nil // as a careless writer would
			row.Status = "abandoned"
			if err := s.repo.UpdateWithVersion(ctx, row.ID, row.Version, row); err != nil {
				t.Fatalf("envelope write: %v", err)
			}

			after, _ := s.repo.FindByID(ctx, m.ID)
			if after.ActionCount != 40 || len(after.Checkpoints) != 2 {
				t.Errorf("after an envelope write: count %d, %d checkpoints", after.ActionCount, len(after.Checkpoints))
			}
			assertLog(t, s.repo, after, 0, 40)
			if b, err := s.repo.CheckpointBoard(ctx, m.ID, 40); err != nil || string(b) != `{"move":40}` {
				t.Errorf("board at 40: %s, %v", b, err)
			}
		})
	}
}

// Moving an original-layout match to pages happens on its next move, and
// leaves every action and board readable exactly as before.
func TestAMoveMovesALegacyMatchToPages(t *testing.T) {
	ctx := context.Background()
	s := openLogStore(t)
	m := s.appendN(t, s.insert(t, ""), 45, 20)

	withSwitches(t, "", true)
	m = s.appendN(t, m, 1, 0)

	raw, err := s.k.Get(db.NSMatches, m.ID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	var stored models.Match
	if err := db.UnmarshalDoc(raw, &stored); err != nil {
		t.Fatal(err)
	}
	if stored.LogFormat != models.LogFormatPages || len(stored.LegacyActionLog) != 0 || stored.ActionCount != 46 {
		t.Fatalf("not moved: format %q, %d inline, count %d", stored.LogFormat, len(stored.LegacyActionLog), stored.ActionCount)
	}
	assertLog(t, s.repo, m, 0, 46)
	for _, seq := range []int{20, 40} {
		if b, err := s.repo.CheckpointBoard(ctx, m.ID, seq); err != nil || string(b) != fmt.Sprintf(`{"move":%d}`, seq) {
			t.Errorf("board at %d after the move: %s, %v", seq, b, err)
		}
	}
}

// BoardAfter searches versions on how many actions each recorded; the version
// that moved the match to pages must not look like a step backwards.
func TestBoardAfterAcrossTheLayoutSwitch(t *testing.T) {
	ctx := context.Background()
	s := openLogStore(t)
	m := s.appendN(t, s.insert(t, ""), 10, 0)
	withSwitches(t, "", true)
	m = s.appendN(t, m, 10, 0)

	boards := s.repo.(BoardSource)
	for _, want := range []int{3, 10, 11, 20} {
		at, err := boards.BoardAfter(ctx, m.ID, want)
		if err != nil {
			t.Fatalf("BoardAfter(%d): %v", want, err)
		}
		if at.ActionCount != want || string(at.State) != fmt.Sprintf(`{"move":%d}`, want) {
			t.Errorf("BoardAfter(%d) = %d actions, %s", want, at.ActionCount, at.State)
		}
	}
}

// A page gone missing — which the atomic transaction should make impossible —
// shortens the answer rather than failing it, so a replay stops at the gap.
func TestAMissingPageShortensTheLog(t *testing.T) {
	s := openLogStore(t)
	m := s.appendN(t, s.insert(t, models.LogFormatPages), 70, 0)
	if _, err := s.k.Delete(db.NSMatchLog, pageKey(m.ID.Hex(), 1)); err != nil {
		t.Fatal(err)
	}
	got, err := s.repo.ActionLog(context.Background(), m.ID, 0, 70)
	if err != nil {
		t.Fatalf("ActionLog: %v", err)
	}
	if len(got) != 32 {
		t.Errorf("with page 1 missing got %d entries, want the 32 before it", len(got))
	}
}

func TestDeleteTakesTheLogWithIt(t *testing.T) {
	ctx := context.Background()
	s := openLogStore(t)
	m := s.appendN(t, s.insert(t, models.LogFormatPages), 40, 20)

	if err := s.repo.DeleteIfUnchanged(ctx, m.ID, m.Version); err != nil {
		t.Fatalf("delete: %v", err)
	}
	var left int
	if err := s.k.Scan(db.NSMatchLog, func([]byte) error { left++; return nil }); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Errorf("%d log records outlived their match", left)
	}
}

// A build that writes the original layout and binary must read, play and
// delete what a build writing pages and text left behind — the rollback.
func TestTheOlderWriterHandlesTheNewerLayout(t *testing.T) {
	ctx := context.Background()
	s := openLogStore(t)
	was := models.SetJSONDocWritesText(true)
	t.Cleanup(func() { models.SetJSONDocWritesText(was) })
	m := s.appendN(t, s.insert(t, models.LogFormatPages), 35, 20)

	models.SetJSONDocWritesText(false)
	withSwitches(t, "", false)
	m = s.appendN(t, m, 5, 0)
	assertLog(t, s.repo, m, 0, 40)
	if m.LogFormat != models.LogFormatPages {
		t.Errorf("the older writer changed the layout to %q", m.LogFormat)
	}
	if err := s.repo.DeleteIfUnchanged(ctx, m.ID, m.Version); err != nil {
		t.Errorf("delete: %v", err)
	}
}
