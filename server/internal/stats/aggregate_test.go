package stats

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/rules"
)

var testClock = time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)

// recordMatch runs a roster and its scores all the way through the pipeline a
// finished game takes: scoreboard, then permanent record.
func recordMatch(t *testing.T, roster []string, dealScores [][]int) MatchResult {
	t.Helper()
	g := testGame(t, roster, dealScores, string(rules.StatusCompleted))
	sb := BuildScoreboard(g, classic())
	m := BuildMatchResult(sb, g.ID, testClock.Add(-time.Hour), testClock, testClock)
	m.ID = bson.NewObjectID()
	return m
}

// seatFor finds the participant a subject key occupies. Only valid where the
// key holds one seat, which is every case except the two-bots-one-difficulty
// test below.
func seatFor(t *testing.T, m MatchResult, key string) Standing {
	t.Helper()
	for _, p := range m.Participants {
		if p.Subject.Key() == key {
			return p
		}
	}
	t.Fatalf("no participant with subject %s", key)
	return Standing{}
}

func applyOne(t *testing.T, ps PlayerStats, m MatchResult, key string) PlayerStats {
	t.Helper()
	return ApplyMatch(ps, m, seatFor(t, m, key), testClock)
}

// Beating three bots must not read as beating three people, and the two
// splits must both be present for a mixed table.
func TestVsHumansAndVsAISplitsFollowTheOpponents(t *testing.T) {
	me := "user:me"

	vsBots := recordMatch(t, []string{"user:me", "ai:easy", "ai:hard"}, [][]int{{5}, {10}, {20}})
	ps := applyOne(t, ZeroStats(Subject{Kind: SubjectUser, ID: "me"}), vsBots, me)

	if ps.VsAI.Matches != 1 {
		t.Errorf("vsAI matches = %d, want 1", ps.VsAI.Matches)
	}
	if ps.VsHumans.Matches != 0 {
		t.Errorf("vsHumans matches = %d, want 0 — there were no people at that table", ps.VsHumans.Matches)
	}
	if ps.ByAIDifficulty["easy"].Matches != 1 || ps.ByAIDifficulty["hard"].Matches != 1 {
		t.Errorf("byAIDifficulty = %+v, want one match each vs easy and hard", ps.ByAIDifficulty)
	}
	if _, ok := ps.ByAIDifficulty["medium"]; ok {
		t.Error("no medium bot played, so no medium bucket should exist")
	}

	// A mixed table counts in both splits: a person was involved, and so was
	// a bot.
	mixed := recordMatch(t, []string{"user:me", "user:rival", "ai:medium"}, [][]int{{5}, {10}, {20}})
	ps = applyOne(t, ps, mixed, me)

	if ps.VsAI.Matches != 2 {
		t.Errorf("vsAI matches = %d, want 2", ps.VsAI.Matches)
	}
	if ps.VsHumans.Matches != 1 {
		t.Errorf("vsHumans matches = %d, want 1", ps.VsHumans.Matches)
	}
	if ps.Overall.Matches != 2 {
		t.Errorf("overall matches = %d, want 2", ps.Overall.Matches)
	}
}

// A guest has no lifetime record, but is still a person at the table — so
// facing one has to register as playing against a human.
func TestGuestOpponentCountsAsHumanButKeepsNoRecord(t *testing.T) {
	m := recordMatch(t, []string{"user:me", "guest"}, [][]int{{5}, {50}})

	ps := applyOne(t, ZeroStats(Subject{Kind: SubjectUser, ID: "me"}), m, "user:me")
	if ps.VsHumans.Matches != 1 {
		t.Errorf("vsHumans = %d, want 1: a guest is a person", ps.VsHumans.Matches)
	}
	if ps.VsAI.Matches != 0 {
		t.Errorf("vsAI = %d, want 0", ps.VsAI.Matches)
	}
	if len(ps.HeadToHead) != 0 {
		t.Errorf("head-to-head = %+v, want empty: a guest has no durable identity", ps.HeadToHead)
	}

	// And the guest contributes no subject key, so nothing will try to write
	// a lifetime record for them.
	for _, k := range m.SubjectKeys {
		if k == "" {
			t.Fatal("a guest leaked an empty subject key into the record")
		}
	}
	if len(m.SubjectKeys) != 1 {
		t.Errorf("subjectKeys = %v, want just the registered player", m.SubjectKeys)
	}
}

// Bots keep a record on the same footing as people — that is what makes
// "you're 1-3 against hard" answerable from the same data as any rivalry.
func TestBotsAccumulateTheirOwnLifetimeRecord(t *testing.T) {
	m := recordMatch(t, []string{"user:me", "ai:hard"}, [][]int{{40}, {10}})

	bot := applyOne(t, ZeroStats(Subject{Kind: SubjectAI, ID: "hard"}), m, "ai:hard")
	if bot.Overall.Wins != 1 {
		t.Errorf("bot wins = %d, want 1 — it posted the lower total", bot.Overall.Wins)
	}
	if bot.VsHumans.Matches != 1 {
		t.Errorf("bot vsHumans = %d, want 1", bot.VsHumans.Matches)
	}
	if bot.VsAI.Matches != 0 {
		t.Errorf("bot vsAI = %d, want 0 — it was the only bot", bot.VsAI.Matches)
	}
	h2h, ok := bot.HeadToHead["user:me"]
	if !ok {
		t.Fatal("the bot should hold a head-to-head record against the player")
	}
	if h2h.Ahead != 1 {
		t.Errorf("bot ahead = %d, want 1", h2h.Ahead)
	}
}

// Two bots of one difficulty at the same table each played a hand, so each
// counts — and neither records a head-to-head against itself.
func TestOneDifficultyHoldingTwoSeatsCountsBothHands(t *testing.T) {
	m := recordMatch(t, []string{"user:me", "ai:medium", "ai:medium"}, [][]int{{5}, {10}, {20}})

	ps := ZeroStats(Subject{Kind: SubjectAI, ID: "medium"})
	for _, seat := range m.Participants {
		if seat.Subject.Key() == "ai:medium" {
			ps = ApplyMatch(ps, m, seat, testClock)
		}
	}
	if ps.Overall.Matches != 2 {
		t.Errorf("medium matches = %d, want 2 — two seats, two hands", ps.Overall.Matches)
	}
	if _, ok := ps.HeadToHead["ai:medium"]; ok {
		t.Error("a subject must not record a head-to-head against itself")
	}
	if ps.HeadToHead["user:me"].Matches != 2 {
		t.Errorf("h2h vs the player = %d, want 2", ps.HeadToHead["user:me"].Matches)
	}

	// The human, meanwhile, faced "medium" once, not twice.
	human := applyOne(t, ZeroStats(Subject{Kind: SubjectUser, ID: "me"}), m, "user:me")
	if got := human.ByAIDifficulty["medium"].Matches; got != 1 {
		t.Errorf("player's vs-medium matches = %d, want 1: one table, however many bots", got)
	}
}

// In a four-player match the players who came third and fourth still have a
// meaningful record against each other.
func TestHeadToHeadIsPairwiseNotMatchWinner(t *testing.T) {
	m := recordMatch(t,
		[]string{"user:winner", "user:me", "user:rival", "ai:easy"},
		[][]int{{1}, {10}, {20}, {30}})

	ps := applyOne(t, ZeroStats(Subject{Kind: SubjectUser, ID: "me"}), m, "user:me")

	if ps.Overall.Wins != 0 {
		t.Errorf("wins = %d, want 0 — someone else won the match", ps.Overall.Wins)
	}
	if got := ps.HeadToHead["user:rival"].Ahead; got != 1 {
		t.Errorf("ahead of rival = %d, want 1 — losing the match still beats finishing behind", got)
	}
	if got := ps.HeadToHead["user:winner"].Behind; got != 1 {
		t.Errorf("behind winner = %d, want 1", got)
	}
	if got := ps.HeadToHead["user:rival"].PointsAgainst; got != 20 {
		t.Errorf("points against rival = %d, want 20", got)
	}
	if got := ps.HeadToHead["user:rival"].PointsFor; got != 10 {
		t.Errorf("points for = %d, want 10", got)
	}
}

func TestRankAndPenaltiesAccumulate(t *testing.T) {
	subject := Subject{Kind: SubjectUser, ID: "me"}
	ps := ZeroStats(subject)

	// Win with 5, then lose with 40 across two deals.
	ps = applyOne(t, ps, recordMatch(t, []string{"user:me", "ai:easy"}, [][]int{{5}, {9}}), "user:me")
	ps = applyOne(t, ps, recordMatch(t, []string{"user:me", "ai:easy"}, [][]int{{20, 20}, {1, 1}}), "user:me")

	o := ps.Overall
	if o.Matches != 2 || o.Wins != 1 || o.Losses != 1 {
		t.Fatalf("record = %d matches / %dW %dL, want 2 / 1W 1L", o.Matches, o.Wins, o.Losses)
	}
	if o.PenaltyPoints != 45 {
		t.Errorf("penaltyPoints = %d, want 45", o.PenaltyPoints)
	}
	if o.DealsPlayed != 3 {
		t.Errorf("dealsPlayed = %d, want 3", o.DealsPlayed)
	}
	if o.BestMatchTotal != 5 || o.WorstMatchTotal != 40 {
		t.Errorf("best/worst = %d/%d, want 5/40", o.BestMatchTotal, o.WorstMatchTotal)
	}
	if o.RankSum != 3 {
		t.Errorf("rankSum = %d, want 3 (1st then 2nd)", o.RankSum)
	}

	v := o.View()
	if v.WinRate != 0.5 {
		t.Errorf("winRate = %v, want 0.5", v.WinRate)
	}
	if v.AvgRank != 1.5 {
		t.Errorf("avgRank = %v, want 1.5", v.AvgRank)
	}
	if v.AvgPenaltyPerDeal != 15 {
		t.Errorf("avgPenaltyPerDeal = %v, want 15", v.AvgPenaltyPerDeal)
	}
}

// An empty tally must not present a placeholder best score as a real one.
func TestEmptyTallyReportsNoBestOrWorst(t *testing.T) {
	v := Tally{}.View()
	if v.BestMatchTotal != nil || v.WorstMatchTotal != nil {
		t.Error("a player with no matches has no best or worst score")
	}
	if v.WinRate != 0 || v.AvgRank != 0 {
		t.Error("rates over zero matches should be zero, not NaN")
	}
}

func TestStreaks(t *testing.T) {
	subject := Subject{Kind: SubjectUser, ID: "me"}
	ps := ZeroStats(subject)

	win := func() {
		ps = applyOne(t, ps, recordMatch(t, []string{"user:me", "ai:easy"}, [][]int{{1}, {9}}), "user:me")
	}
	loss := func() {
		ps = applyOne(t, ps, recordMatch(t, []string{"user:me", "ai:easy"}, [][]int{{9}, {1}}), "user:me")
	}
	draw := func() {
		ps = applyOne(t, ps, recordMatch(t, []string{"user:me", "ai:easy"}, [][]int{{5}, {5}}), "user:me")
	}

	win()
	win()
	win()
	if ps.CurrentStreak != 3 || ps.LongestWinStreak != 3 {
		t.Fatalf("after three wins: current=%d longest=%d, want 3/3", ps.CurrentStreak, ps.LongestWinStreak)
	}

	loss()
	if ps.CurrentStreak != -1 {
		t.Errorf("a loss should flip the streak negative, got %d", ps.CurrentStreak)
	}
	if ps.LongestWinStreak != 3 {
		t.Errorf("longest win streak should survive a loss, got %d", ps.LongestWinStreak)
	}

	loss()
	if ps.LongestLossStreak != 2 {
		t.Errorf("longestLossStreak = %d, want 2", ps.LongestLossStreak)
	}

	// A draw is neither a win nor a loss, so it ends the run rather than
	// extending it in either direction.
	draw()
	if ps.CurrentStreak != 0 {
		t.Errorf("a draw should reset the streak, got %d", ps.CurrentStreak)
	}
	if ps.Overall.Draws != 1 {
		t.Errorf("draws = %d, want 1", ps.Overall.Draws)
	}
}

func TestRecentMatchesAreNewestFirstAndCapped(t *testing.T) {
	ps := ZeroStats(Subject{Kind: SubjectUser, ID: "me"})
	for i := 0; i < recentMatchesKept+5; i++ {
		m := recordMatch(t, []string{"user:me", "ai:easy"}, [][]int{{i}, {100}})
		ps = applyOne(t, ps, m, "user:me")
	}
	if len(ps.RecentMatches) != recentMatchesKept {
		t.Fatalf("recent matches = %d, want capped at %d", len(ps.RecentMatches), recentMatchesKept)
	}
	// The newest entry is the last one applied, whose total was the highest i.
	if got := ps.RecentMatches[0].Total; got != recentMatchesKept+4 {
		t.Errorf("newest recent match total = %d, want %d", got, recentMatchesKept+4)
	}
	if ps.RecentMatches[0].Outcome != "win" {
		t.Errorf("outcome = %q, want win", ps.RecentMatches[0].Outcome)
	}
	if !ps.RecentMatches[0].AgainstAI || ps.RecentMatches[0].AgainstHumans {
		t.Error("recent match should be flagged as against AI only")
	}
	if ps.Overall.Matches != recentMatchesKept+5 {
		t.Errorf("the cap must only trim the recent list, not the totals: matches = %d", ps.Overall.Matches)
	}
}

func TestPerProfileAndTableSizeBuckets(t *testing.T) {
	m := recordMatch(t, []string{"user:me", "user:b", "ai:easy"}, [][]int{{5}, {10}, {20}})
	ps := applyOne(t, ZeroStats(Subject{Kind: SubjectUser, ID: "me"}), m, "user:me")

	if got := ps.ByProfile["zolik_classic"].Matches; got != 1 {
		t.Errorf("byProfile[zolik_classic] = %d, want 1", got)
	}
	if got := ps.ByPlayerCount["3"].Matches; got != 1 {
		t.Errorf("byPlayerCount[3] = %d, want 1", got)
	}
}

func TestSubjectKeyRoundTrip(t *testing.T) {
	for _, s := range []Subject{
		{Kind: SubjectUser, ID: "65ab00000000000000000001"},
		{Kind: SubjectAI, ID: "hard"},
	} {
		back, err := ParseSubjectKey(s.Key())
		if err != nil {
			t.Fatalf("ParseSubjectKey(%q): %v", s.Key(), err)
		}
		if back.Kind != s.Kind || back.ID != s.ID {
			t.Errorf("round trip of %+v gave %+v", s, back)
		}
	}
	if _, err := ParseSubjectKey(""); err == nil {
		t.Error("an empty key (a guest) must not parse to a subject")
	}
}

// The stored name follows the player, so a rename does not leave stale names
// on the leaderboard.
func TestLatestDisplayNameWins(t *testing.T) {
	first := recordMatch(t, []string{"user:me", "ai:easy"}, [][]int{{1}, {9}})
	ps := applyOne(t, ZeroStats(Subject{Kind: SubjectUser, ID: "me"}), first, "user:me")

	second := recordMatch(t, []string{"user:me", "ai:easy"}, [][]int{{1}, {9}})
	for i := range second.Participants {
		if second.Participants[i].Subject.Key() == "user:me" {
			second.Participants[i].Subject.Name = "Renamed"
		}
	}
	ps = applyOne(t, ps, second, "user:me")

	if ps.Subject.Name != "Renamed" {
		t.Errorf("stored name = %q, want the latest one", ps.Subject.Name)
	}
}
