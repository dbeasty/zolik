package match_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db"
	"zolik/server/internal/match"
	"zolik/server/internal/models"
)

// Reading a board out of the store's own history.
//
// Everything else about replay is a fold: the deal plus the log, run forward.
// This is the one path that does not reconstruct anything — it asks the engine
// what the board actually was — and the property worth pinning is exactly
// that. So the test writes a *lie* into the newest version of the match and
// checks the older ones are untouched by it: a fold could not tell the
// difference, and history must.

func kdbRepo(t *testing.T) match.Repository {
	t.Helper()
	k, err := db.OpenKDB(t.TempDir())
	if err != nil {
		t.Skipf("no embedded kdb available here: %v", err)
	}
	t.Cleanup(func() { _ = k.Close(context.Background()) })
	return match.NewKDBRepository(k)
}

// play writes n successive versions of a match, each one move further on, and
// returns it as it finally stood.
func play(t *testing.T, repo match.Repository, moves int) models.Match {
	t.Helper()
	m, err := repo.Insert(context.Background(), models.Match{
		ModuleID: "prsi",
		Status:   "active",
		Players:  []models.Player{{ID: "p1", Name: "p1"}, {ID: "p2", Name: "p2"}},
		State:    json.RawMessage(`{"move":0}`),
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	for i := 1; i <= moves; i++ {
		expected := m.Version
		m.State = json.RawMessage(fmt.Sprintf(`{"move":%d}`, i))
		m.ActionLog = append(m.ActionLog, models.MatchAction{
			Seq: i, PlayerID: "p1", Action: json.RawMessage(`{"verb":"draw"}`), At: time.Now().UTC(),
		})
		if err := repo.UpdateWithVersion(context.Background(), m.ID, expected, m); err != nil {
			t.Fatalf("update %d: %v", i, err)
		}
		m.Version = expected + 1
	}
	return m
}

func TestKDBBoardAfterReadsTheBoardAsItWas(t *testing.T) {
	repo := kdbRepo(t)
	boards, ok := repo.(match.BoardSource)
	if !ok {
		t.Fatal("the kdb repository should be able to answer for its own history")
	}
	m := play(t, repo, 6)

	for want := 1; want <= 6; want++ {
		at, err := boards.BoardAfter(context.Background(), m.ID, want)
		if err != nil {
			t.Fatalf("BoardAfter(%d): %v", want, err)
		}
		if len(at.ActionLog) != want {
			t.Errorf("BoardAfter(%d) landed on a version with %d moves", want, len(at.ActionLog))
		}
		if got, expect := string(at.State), fmt.Sprintf(`{"move":%d}`, want); got != expect {
			t.Errorf("BoardAfter(%d) = %s, want %s", want, got, expect)
		}
	}
}

// The point of asking the store rather than folding: a board written outside
// the log cannot rewrite the ones already recorded.
func TestKDBHistorySurvivesALaterRewrite(t *testing.T) {
	repo := kdbRepo(t)
	boards := repo.(match.BoardSource)
	m := play(t, repo, 4)

	// Something scribbles over the current board without touching the log —
	// a debug-state seed, a migration, a bug. A fold from the deal would be
	// none the wiser; history is.
	expected := m.Version
	m.State = json.RawMessage(`{"move":999,"scribbled":true}`)
	if err := repo.UpdateWithVersion(context.Background(), m.ID, expected, m); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	at, err := boards.BoardAfter(context.Background(), m.ID, 2)
	if err != nil {
		t.Fatalf("BoardAfter: %v", err)
	}
	if got := string(at.State); got != `{"move":2}` {
		t.Errorf("history reports %s for move 2; the later rewrite leaked backwards", got)
	}
}

// A match this store has never written has no history to give, and says so
// plainly — that error is exactly what makes the fold fall back to a
// checkpoint or to the deal rather than refusing.
func TestKDBBoardAfterOfAMatchItNeverWrote(t *testing.T) {
	repo := kdbRepo(t)
	boards := repo.(match.BoardSource)

	if _, err := boards.BoardAfter(context.Background(), bson.NewObjectID(), 3); err == nil {
		t.Error("a match that was never written reported a board")
	}
}

// Asking past the end lands on the newest version there is, rather than
// failing — a caller asking for frame 900 of a 40-move match wants the end.
func TestKDBBoardAfterPastTheEnd(t *testing.T) {
	repo := kdbRepo(t)
	boards := repo.(match.BoardSource)
	m := play(t, repo, 3)

	at, err := boards.BoardAfter(context.Background(), m.ID, 900)
	if err != nil {
		t.Fatalf("BoardAfter: %v", err)
	}
	if got := string(at.State); got != `{"move":3}` {
		t.Errorf("past the end gave %s, want the last board written", got)
	}
}
