package stats

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/module"
)

// A match a bot finished for somebody is in their history and in none of
// their totals — or dropping on purpose would let a Hard bot win for them.
func TestAStoodInSeatKeepsTheMatchButNotTheResult(t *testing.T) {
	match, standings := testMatch(t, []string{"user:me", "user:rival"}, []int{-5, -50}, "completed", "p0")
	match.StoodIn = []string{"p0"}
	sb := BuildScoreboard(match, module.Outcome{Standings: standings})
	m := BuildMatchResult(sb, match.ID, testClock.Add(-time.Hour), testClock, testClock)
	m.ID = bson.NewObjectID()

	mine := seatFor(t, m, "user:me")
	if !mine.StandIn {
		t.Fatal("the stood-in seat is not marked on the record")
	}
	if seatFor(t, m, "user:rival").StandIn {
		t.Error("the seat nobody stood in for is marked as stood in for")
	}

	ps := ApplyMatch(ZeroStats(Subject{Kind: SubjectUser, ID: "me"}), m, mine, testClock)
	if ps.Overall.Matches != 0 || ps.Overall.Wins != 0 || ps.VsHumans.Matches != 0 {
		t.Errorf("a stood-in win counted: overall %+v, vsHumans %+v", ps.Overall, ps.VsHumans)
	}
	if len(ps.ByModule) != 0 || len(ps.HeadToHead) != 0 {
		t.Errorf("a stood-in match reached the splits: byModule %v, headToHead %v", ps.ByModule, ps.HeadToHead)
	}
	if ps.CurrentStreak != 0 {
		t.Errorf("streak = %d, want 0", ps.CurrentStreak)
	}
	if len(ps.RecentMatches) != 1 || !ps.RecentMatches[0].StandIn {
		t.Fatalf("recent = %+v, want the match, marked as stood in for", ps.RecentMatches)
	}
	if !ps.LastMatchAt.Equal(testClock) {
		t.Errorf("last match = %v, want %v", ps.LastMatchAt, testClock)
	}

	// The rival played the whole match against what was partly a bot, and
	// keeps the result: they did nothing to bring the bot in.
	rival := ApplyMatch(ZeroStats(Subject{Kind: SubjectUser, ID: "rival"}), m, seatFor(t, m, "user:rival"), testClock)
	if rival.Overall.Matches != 1 || rival.Overall.Losses != 1 {
		t.Errorf("rival overall = %+v, want one loss", rival.Overall)
	}
}
