package stats

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/module"
)

// TestRepairPartnershipDraws — records written before the scoreboard knew
// about sides hold a partnership's win as a draw. The repair turns those into
// wins, leaves a genuine tie alone, and rebuilds the lifetime records.
func TestRepairPartnershipDraws(t *testing.T) {
	ctx := context.Background()
	repo := newKDBRepo(t)
	at := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

	// A four-seat Canasta win by p0+p2, recorded as the old scoreboard did:
	// with no sides, so as a draw.
	canasta, standings := testMatch(t, []string{"user:a", "user:b", "user:c", "user:d"},
		[]int{30, 10, 30, 10}, "completed", "p0", "p2")
	canasta.ModuleID = "canasta"
	old := BuildScoreboard(canasta, module.Outcome{Standings: standings})
	if !old.IsDraw {
		t.Fatal("setup: without sides the partnership should record as a draw")
	}
	// A two-seat poker tie: a real draw, and it must stay one.
	poker, standings := testMatch(t, []string{"user:a", "user:b"}, []int{20, 20}, "completed", "p0", "p1")
	poker.ModuleID = "holdem"
	tie := BuildScoreboard(poker, module.Outcome{Standings: standings})

	for i, rec := range []struct {
		sb Scoreboard
		id bson.ObjectID
	}{{old, canasta.ID}, {tie, poker.ID}} {
		when := at.Add(time.Duration(i) * time.Hour)
		if _, err := repo.InsertMatch(ctx, BuildMatchResult(rec.sb, rec.id, when, when, when)); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	claimer := NewClaimer(repo)
	a := Subject{Kind: SubjectUser, ID: "a"}
	if err := claimer.RebuildPlayerStats(ctx, a); err != nil {
		t.Fatal(err)
	}
	if ps, _ := repo.FindPlayerStats(ctx, a.Key()); ps.Overall.Wins != 0 || ps.Overall.Draws != 2 {
		t.Fatalf("setup: a has %d wins, %d draws, want 0 and 2", ps.Overall.Wins, ps.Overall.Draws)
	}

	sidesOf := func(m MatchResult) [][]string {
		if m.ModuleID != "canasta" {
			return nil
		}
		seats := SeatOrder(m)
		return [][]string{{seats[0], seats[2]}, {seats[1], seats[3]}}
	}

	dry, err := RepairPartnershipDraws(ctx, repo, sidesOf, true)
	if err != nil || dry.Draws != 2 || dry.Fixed != 1 || dry.Rebuilt != 0 {
		t.Fatalf("dry run: %+v (%v), want 2 draws, 1 fixed, nothing rebuilt", dry, err)
	}
	if ps, _ := repo.FindPlayerStats(ctx, a.Key()); ps.Overall.Wins != 0 {
		t.Fatal("a dry run wrote a lifetime record")
	}

	report, err := RepairPartnershipDraws(ctx, repo, sidesOf, false)
	if err != nil || report.Fixed != 1 || report.Rebuilt != 4 {
		t.Fatalf("repair: %+v (%v), want 1 fixed and 4 rebuilt", report, err)
	}

	want := map[string][3]int{ // wins, draws, losses
		"a": {1, 1, 0}, // won the Canasta match, tied the poker one
		"b": {0, 1, 1},
		"c": {1, 0, 0},
		"d": {0, 0, 1},
	}
	for id, w := range want {
		ps, err := repo.FindPlayerStats(ctx, Subject{Kind: SubjectUser, ID: id}.Key())
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		got := [3]int{ps.Overall.Wins, ps.Overall.Draws, ps.Overall.Losses}
		if got != w {
			t.Errorf("%s: wins/draws/losses %v, want %v", id, got, w)
		}
	}

	again, err := RepairPartnershipDraws(ctx, repo, sidesOf, false)
	if err != nil || again.Draws != 1 || again.Fixed != 0 {
		t.Fatalf("second run: %+v (%v), want only the poker tie left and nothing fixed", again, err)
	}
}
