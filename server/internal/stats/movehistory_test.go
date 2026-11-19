package stats

import (
	"testing"

	"zolik/server/internal/module"
)

// One person, two accounts: the second one's matches move onto the first,
// whose record then counts them, and the second is left with nothing.
func TestMoveHistoryCarriesASecondAccountsMatchesOver(t *testing.T) {
	ctx := t.Context()
	repo := newKDBRepo(t)
	record := func(seats []string, scores []int, winner string) {
		t.Helper()
		match, standings := testMatch(t, seats, scores, "completed", winner)
		sb := BuildScoreboard(match, module.Outcome{Standings: standings})
		if _, err := repo.InsertMatch(ctx, BuildMatchResult(sb, match.ID, testClock, testClock, testClock)); err != nil {
			t.Fatal(err)
		}
	}
	record([]string{"user:real", "user:rival"}, []int{-5, -50}, "p0")   // the real account wins
	record([]string{"user:second", "user:rival"}, []int{-5, -50}, "p0") // the second account wins
	record([]string{"user:second", "user:rival"}, []int{-50, -5}, "p1") // and loses
	// Both at one table: moving it would seat one person twice.
	record([]string{"user:second", "user:real"}, []int{-5, -50}, "p0")

	real := Subject{Kind: SubjectUser, ID: "real", Name: "real"}
	second := Subject{Kind: SubjectUser, ID: "second"}
	claimer := NewClaimer(repo)
	n, err := claimer.MoveHistory(ctx, second, real)
	if err != nil || n != 2 {
		t.Fatalf("moved %d (%v), want 2", n, err)
	}
	ps, _ := repo.FindPlayerStats(ctx, real.Key())
	if ps.Overall.Matches != 4 || ps.Overall.Wins != 2 {
		t.Errorf("real account: %+v, want 4 played, 2 won: its own win, the moved win and loss, and the shared table it lost (the shared table stays apart)", ps.Overall)
	}
	left, _ := repo.FindPlayerStats(ctx, second.Key())
	if left.Overall.Matches != 1 {
		t.Errorf("second account: %+v, want only the shared table left", left.Overall)
	}

	if again, err := claimer.MoveHistory(ctx, second, real); err != nil || again != 0 {
		t.Errorf("second run moved %d (%v), want nothing", again, err)
	}
	if _, err := claimer.MoveHistory(ctx, real, real); err == nil {
		t.Error("moving an account onto itself was allowed")
	}
}
