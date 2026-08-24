package stats

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/models"
	"zolik/server/internal/rules"
)

// testGame assembles a game document from per-player deal scores. Players are
// given in seat order; the roster spec decides what kind of player each seat
// holds ("user:alice", "guest:bob", "ai:hard").
func testGame(t *testing.T, roster []string, dealScores [][]int, status string) models.Game {
	t.Helper()
	if len(roster) != len(dealScores) {
		t.Fatalf("roster of %d needs %d score rows, got %d", len(roster), len(roster), len(dealScores))
	}

	g := models.Game{
		ID:           bson.NewObjectID(),
		Status:       status,
		RulesProfile: "zolik_classic",
		GameScores:   map[string][]int{},
		TotalScores:  map[string]int{},
	}
	for i, spec := range roster {
		pid := "p" + string(rune('0'+i))
		p := models.Player{ID: pid, Name: spec}
		switch {
		case len(spec) > 3 && spec[:3] == "ai:":
			p.IsAI = true
			p.AIDifficulty = spec[3:]
		case len(spec) > 5 && spec[:5] == "user:":
			p.UserID = spec[5:]
		case len(spec) > 6 && spec[:6] == "guest:":
			// A guest on a device that has a durable guest id — the shape that
			// can later be claimed by an account.
			p.GuestID = spec[6:]
		default: // guest with no device id: not AI, nothing to attribute to
		}
		g.Players = append(g.Players, p)
		g.TurnOrder = append(g.TurnOrder, pid)
		g.GameScores[pid] = dealScores[i]
		total := 0
		for _, v := range dealScores[i] {
			total += v
		}
		g.TotalScores[pid] = total
	}
	if status == string(rules.StatusCompleted) {
		g.WinnerID, g.IsDraw = rules.DetermineMatchWinner(g.TurnOrder, g.TotalScores, g.GameScores)
	}
	return g
}

func classic() rules.RulesConfig { return rules.ResolveProfile("zolik_classic") }

func findStanding(t *testing.T, sb Scoreboard, playerID string) Standing {
	t.Helper()
	for _, s := range sb.Standings {
		if s.PlayerID == playerID {
			return s
		}
	}
	t.Fatalf("no standing for %s", playerID)
	return Standing{}
}

func TestBuildScoreboardRanksByLowestTotal(t *testing.T) {
	g := testGame(t, []string{"user:u1", "user:u2", "ai:hard"}, [][]int{
		{10, 20}, // 30
		{0, 5},   // 5  <- best
		{40, 40}, // 80
	}, string(rules.StatusCompleted))

	sb := BuildScoreboard(g, classic())

	if got := sb.Standings[0].PlayerID; got != "p1" {
		t.Fatalf("expected p1 ranked first, got %s", got)
	}
	wantRanks := map[string]int{"p1": 1, "p0": 2, "p2": 3}
	for pid, want := range wantRanks {
		if got := findStanding(t, sb, pid).Rank; got != want {
			t.Errorf("%s rank = %d, want %d", pid, got, want)
		}
	}
	if !findStanding(t, sb, "p1").Won {
		t.Error("p1 should be marked as the winner")
	}
	if findStanding(t, sb, "p0").Won {
		t.Error("p0 should not be marked as a winner")
	}
}

// The winner shown at the top of a scoreboard must be the winner the engine
// itself decided. These are two separate code paths — assignRanks orders by
// (total, dealsWon) while the engine picks a winner in
// rules.DetermineMatchWinner — and this is the guard against them drifting.
func TestScoreboardLeaderAgreesWithEngineWinner(t *testing.T) {
	cases := [][][]int{
		{{10}, {20}, {30}},                // clear winner
		{{10, 10}, {5, 5}, {50, 0}},       // p1 wins on total
		{{10}, {10}, {30}},                // p0/p1 tie total and deals won -> draw
		{{0, 30}, {30, 0}, {99, 99}},      // tie on total, tie on deals won -> draw
		{{5, 5}, {4, 7}, {20, 20}},        // p0 10, p1 11
		{{0, 0, 0}, {1, 1, 1}, {2, 2, 2}}, // one player sweeps
	}
	for i, scores := range cases {
		roster := make([]string, len(scores))
		for j := range roster {
			roster[j] = "user:u" + string(rune('0'+j))
		}
		g := testGame(t, roster, scores, string(rules.StatusCompleted))
		sb := BuildScoreboard(g, classic())

		leader := sb.Standings[0]
		if g.IsDraw {
			if !leader.Drew {
				t.Errorf("case %d: engine called a draw but leader is not marked drawn", i)
			}
			if leader.Won {
				t.Errorf("case %d: engine called a draw but leader is marked as a win", i)
			}
			continue
		}
		if leader.PlayerID != g.WinnerID {
			t.Errorf("case %d: scoreboard leader %s but engine winner %s", i, leader.PlayerID, g.WinnerID)
		}
		if !leader.Won {
			t.Errorf("case %d: leader %s not marked as the winner", i, leader.PlayerID)
		}
	}
}

func TestDrawMarksOnlyTiedLeaders(t *testing.T) {
	// p0 and p1 both finish on 10 with the same deals won; p2 is far behind
	// and lost the match, however it ended.
	g := testGame(t, []string{"user:u1", "user:u2", "user:u3"}, [][]int{
		{5, 5},
		{5, 5},
		{50, 50},
	}, string(rules.StatusCompleted))

	if !g.IsDraw {
		t.Fatalf("expected the engine to call this a draw")
	}
	sb := BuildScoreboard(g, classic())

	for _, pid := range []string{"p0", "p1"} {
		if !findStanding(t, sb, pid).Drew {
			t.Errorf("%s tied for the lead and should be marked drawn", pid)
		}
	}
	last := findStanding(t, sb, "p2")
	if last.Drew {
		t.Error("p2 finished last and must not be marked as having drawn")
	}
	if last.Rank != 3 {
		t.Errorf("p2 rank = %d, want 3 (two players share rank 1)", last.Rank)
	}
}

func TestDealsWonRequiresAUniqueLowScore(t *testing.T) {
	// Deal 1: p0 and p1 both score 0 — a tie wins the deal for nobody.
	// Deal 2: p1 alone is lowest.
	g := testGame(t, []string{"user:u1", "user:u2"}, [][]int{
		{0, 20},
		{0, 5},
	}, "active")

	sb := BuildScoreboard(g, classic())
	if got := findStanding(t, sb, "p0").DealsWon; got != 0 {
		t.Errorf("p0 dealsWon = %d, want 0 (deal 1 was tied)", got)
	}
	if got := findStanding(t, sb, "p1").DealsWon; got != 1 {
		t.Errorf("p1 dealsWon = %d, want 1", got)
	}
}

func TestGoOutsComeFromTheActionLog(t *testing.T) {
	g := testGame(t, []string{"user:u1", "ai:medium"}, [][]int{
		{0, 12},
		{30, 0},
	}, "active")
	g.ActionLog = []models.Action{
		{Type: "player_discarded", PlayerID: "p0", Data: map[string]interface{}{}},
		{Type: "deal_ended", PlayerID: "p0", Data: map[string]interface{}{"winnerId": "p0"}},
		{Type: "deal_ended", PlayerID: "p1", Data: map[string]interface{}{"winnerId": "p1"}},
	}

	sb := BuildScoreboard(g, classic())
	for _, pid := range []string{"p0", "p1"} {
		if got := findStanding(t, sb, pid).GoOuts; got != 1 {
			t.Errorf("%s goOuts = %d, want 1", pid, got)
		}
	}
}

func TestScoreboardClassifiesEverySeat(t *testing.T) {
	g := testGame(t, []string{"user:abc", "guest", "ai:easy", "ai:hard"}, [][]int{
		{1}, {2}, {3}, {4},
	}, "active")

	sb := BuildScoreboard(g, classic())

	comp := sb.Composition
	if comp.Players != 4 || comp.Users != 1 || comp.Guests != 1 || comp.AIs != 2 {
		t.Fatalf("composition = %+v, want 4 players / 1 user / 1 guest / 2 ai", comp)
	}
	if len(comp.AIDifficulties) != 2 || comp.AIDifficulties[0] != "easy" || comp.AIDifficulties[1] != "hard" {
		t.Errorf("aiDifficulties = %v, want [easy hard]", comp.AIDifficulties)
	}

	if got := findStanding(t, sb, "p0").Subject.Key(); got != "user:abc" {
		t.Errorf("registered seat key = %q, want user:abc", got)
	}
	if got := findStanding(t, sb, "p1").Subject.Key(); got != "" {
		t.Errorf("guest seat should have no durable key, got %q", got)
	}
	if got := findStanding(t, sb, "p3").Subject.Key(); got != "ai:hard" {
		t.Errorf("bot seat key = %q, want ai:hard", got)
	}
}

// An in-progress match still gets a leader, chosen by the same rule that will
// decide the finished match — that is the point of a live scoreboard.
func TestInProgressMatchHasProvisionalLeaderButNoWinner(t *testing.T) {
	g := testGame(t, []string{"user:u1", "user:u2"}, [][]int{
		{40},
		{10},
	}, "active")

	sb := BuildScoreboard(g, classic())
	if sb.Complete {
		t.Error("an active game must not report as complete")
	}
	if sb.WinnerID != "p1" {
		t.Errorf("provisional leader = %q, want p1", sb.WinnerID)
	}
	if findStanding(t, sb, "p1").Won {
		t.Error("nobody has won an unfinished match")
	}
}

func TestScoreboardCarriesTheMatchEndCondition(t *testing.T) {
	g := testGame(t, []string{"user:u1", "user:u2"}, [][]int{{0}, {0}}, "active")

	// continental is the fixed seven-deal format...
	contSB := BuildScoreboard(g, rules.ResolveProfile("continental"))
	if contSB.MatchEndMode != string(rules.MatchEndAfterDeals) {
		t.Fatalf("continental end mode = %q", contSB.MatchEndMode)
	}
	if contSB.DealCount == 0 {
		t.Error("a fixed-deal profile should report its deal count")
	}
	if contSB.TargetScore != 0 {
		t.Error("a fixed-deal profile has no target score to report")
	}

	// ...while zolik_classic keeps dealing until someone crosses the target.
	classicSB := BuildScoreboard(g, rules.ResolveProfile("zolik_classic"))
	if classicSB.MatchEndMode != string(rules.MatchEndAtScore) {
		t.Fatalf("zolik_classic end mode = %q", classicSB.MatchEndMode)
	}
	if classicSB.TargetScore == 0 {
		t.Error("an at-score profile should report its target")
	}
	if classicSB.DealCount != 0 {
		t.Error("an at-score profile has no fixed deal count to report")
	}
}

// A player on the document but missing from turn order is a malformed state,
// but dropping them from the scoreboard would hide it rather than show it.
func TestPlayerMissingFromTurnOrderStillAppears(t *testing.T) {
	g := testGame(t, []string{"user:u1", "user:u2"}, [][]int{{5}, {9}}, "active")
	g.TurnOrder = []string{"p0"}

	sb := BuildScoreboard(g, classic())
	if len(sb.Standings) != 2 {
		t.Fatalf("got %d standings, want 2", len(sb.Standings))
	}
	findStanding(t, sb, "p1")
}
