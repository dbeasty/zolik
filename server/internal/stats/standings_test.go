package stats

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// testMatch assembles a match and its module standings from a roster and a
// score per seat.
//
// The roster spec decides what kind of player each seat holds ("user:alice",
// "guest:bob", "ai:hard"). Scores are the module's own measure, so higher is
// better — which is the direction every module reports, and the opposite of
// what this file assumed when the only game was scored in penalties.
func testMatch(t *testing.T, roster []string, scores []int, status string, winners ...string) (models.Match, []module.Standing) {
	t.Helper()
	if len(roster) != len(scores) {
		t.Fatalf("roster of %d needs %d scores, got %d", len(roster), len(roster), len(scores))
	}

	m := models.Match{
		ID:        bson.NewObjectID(),
		ModuleID:  "zolik",
		Variation: "zolik_classic",
		Status:    status,
		Winners:   winners,
	}
	byID := map[string]int{}
	for i, spec := range roster {
		pid := "p" + string(rune('0'+i))
		p := models.Player{ID: pid, Name: spec}
		switch {
		case len(spec) > 3 && spec[:3] == "ai:":
			p.IsAI, p.AIDifficulty = true, spec[3:]
		case len(spec) > 5 && spec[:5] == "user:":
			p.UserID = spec[5:]
		case len(spec) > 6 && spec[:6] == "guest:":
			// A guest on a device that has a durable guest id — the shape that
			// can later be claimed by an account.
			p.GuestID = spec[6:]
		default: // guest with no device id: not AI, nothing to attribute to
		}
		m.Players = append(m.Players, p)
		m.TurnOrder = append(m.TurnOrder, pid)
		byID[pid] = scores[i]
	}

	order := append([]string(nil), m.TurnOrder...)
	standings := module.RankByScore(order, func(id string) int { return byID[id] }, "zolik.unit.penalty")
	return m, standings
}

// TestScoreboardRecordsTheModuleRanking — this package no longer decides who is
// ahead. The module ranked the players; a second opinion here would be a second
// implementation of "who is winning".
func TestScoreboardRecordsTheModuleRanking(t *testing.T) {
	m, standings := testMatch(t, []string{"user:a", "user:b", "user:c"}, []int{10, 30, 20}, "completed", "p1")
	sb := BuildScoreboard(m, standings)

	if len(sb.Standings) != 3 {
		t.Fatalf("%d standings, want 3", len(sb.Standings))
	}
	// Highest score first, because higher is better everywhere now.
	if sb.Standings[0].PlayerID != "p1" || sb.Standings[0].Score != 30 {
		t.Errorf("leader is %+v, want p1 on 30", sb.Standings[0])
	}
	if sb.Standings[0].Rank != 1 || sb.Standings[2].Rank != 3 {
		t.Errorf("ranks are %d..%d, want 1..3", sb.Standings[0].Rank, sb.Standings[2].Rank)
	}
	if sb.ModuleID != "zolik" || sb.Variation != "zolik_classic" {
		t.Errorf("game identity lost: %q/%q", sb.ModuleID, sb.Variation)
	}
	if !sb.Complete {
		t.Error("a completed match should be marked complete")
	}
}

// TestWinnerComesFromTheEngineNotTheRanking — a match can end on a rule the
// scoreboard does not model, and a record has to agree with the match the
// players actually watched end.
func TestWinnerComesFromTheEngineNotTheRanking(t *testing.T) {
	// The engine says p0 won even though p1 has the higher score. That is not
	// a contrived case: Canasta's partnership and poker's last-player-standing
	// both settle on something other than "most points right now".
	m, standings := testMatch(t, []string{"user:a", "user:b"}, []int{10, 30}, "completed", "p0")
	sb := BuildScoreboard(m, standings)

	won := map[string]bool{}
	for _, s := range sb.Standings {
		won[s.PlayerID] = s.Won
	}
	if !won["p0"] {
		t.Error("the engine's winner should be marked as having won")
	}
	if won["p1"] {
		t.Error("the ranking leader should not be marked a winner against the engine")
	}
}

// TestMoreThanOneWinnerIsADraw — level on chips at the end of a fixed-length
// poker match. A Canasta partnership is two winners too, and is deliberately
// also recorded as a draw here: both members share first place.
func TestMoreThanOneWinnerIsADraw(t *testing.T) {
	m, standings := testMatch(t, []string{"user:a", "user:b"}, []int{20, 20}, "completed", "p0", "p1")
	sb := BuildScoreboard(m, standings)

	if !sb.IsDraw {
		t.Error("two winners should read as a draw")
	}
	for _, s := range sb.Standings {
		if !s.Won || !s.Drew {
			t.Errorf("%s should be marked won and drew: %+v", s.PlayerID, s)
		}
		if s.Rank != 1 {
			t.Errorf("%s has rank %d, want 1", s.PlayerID, s.Rank)
		}
	}
}

// TestInProgressMatchNamesNoWinner — a live table can show who is ahead
// without claiming anyone has won.
func TestInProgressMatchNamesNoWinner(t *testing.T) {
	m, standings := testMatch(t, []string{"user:a", "user:b"}, []int{10, 30}, "active")
	sb := BuildScoreboard(m, standings)

	if sb.Complete {
		t.Error("an active match is not complete")
	}
	if len(sb.Winners) != 0 {
		t.Errorf("winners are %v on a live match", sb.Winners)
	}
	// The provisional leader is still ranked first, so a table can show
	// standings — it just does not say "won".
	if sb.Standings[0].PlayerID != "p1" {
		t.Errorf("leader is %s, want p1", sb.Standings[0].PlayerID)
	}
}

// TestScoreboardClassifiesEverySeat — the composition is what lets a lifetime
// record answer "how do I do against people, versus against bots?".
func TestScoreboardClassifiesEverySeat(t *testing.T) {
	m, standings := testMatch(t,
		[]string{"user:alice", "guest:bob", "ai:hard", "ai:hard"},
		[]int{40, 30, 20, 10}, "completed", "p0")
	sb := BuildScoreboard(m, standings)

	c := sb.Composition
	if c.Players != 4 || c.Users != 1 || c.Guests != 1 || c.AIs != 2 {
		t.Errorf("composition is %+v, want 4 players: 1 user, 1 guest, 2 bots", c)
	}
	// Two bots of one difficulty list it once.
	if len(c.AIDifficulties) != 1 || c.AIDifficulties[0] != "hard" {
		t.Errorf("difficulties are %v, want [hard]", c.AIDifficulties)
	}

	kinds := map[string]SubjectKind{}
	for _, s := range sb.Standings {
		kinds[s.PlayerID] = s.Subject.Kind
	}
	if kinds["p0"] != SubjectUser {
		t.Errorf("p0 is %v, want a user", kinds["p0"])
	}
	if kinds["p2"] != SubjectAI {
		t.Errorf("p2 is %v, want a bot", kinds["p2"])
	}
	// A guest has no durable identity, so it keeps no lifetime record.
	for _, s := range sb.Standings {
		if s.PlayerID == "p1" && s.Subject.Durable() {
			t.Error("a guest should have no durable subject key")
		}
	}
}

// TestScoreUnitTravels — a client needs to know what the number means, and
// only the module can say.
func TestScoreUnitTravels(t *testing.T) {
	m, standings := testMatch(t, []string{"user:a", "user:b"}, []int{1, 2}, "completed", "p1")
	sb := BuildScoreboard(m, standings)
	for _, s := range sb.Standings {
		if s.ScoreLabelKey == "" {
			t.Errorf("%s's score has no unit", s.PlayerID)
		}
	}
}

// TestTiedSeatsKeepAStableOrder — two reads of one state must not disagree
// about the order of tied players.
func TestTiedSeatsKeepAStableOrder(t *testing.T) {
	m, standings := testMatch(t, []string{"user:a", "user:b", "user:c"}, []int{20, 20, 10}, "active")

	first := BuildScoreboard(m, standings).Standings
	second := BuildScoreboard(m, standings).Standings
	for i := range first {
		if first[i].PlayerID != second[i].PlayerID {
			t.Fatalf("order changed between reads: %v then %v", first, second)
		}
	}
	// Tied players share a rank, and the earlier seat is listed first.
	if first[0].Rank != first[1].Rank {
		t.Errorf("tied players have ranks %d and %d", first[0].Rank, first[1].Rank)
	}
	if first[0].Seat > first[1].Seat {
		t.Errorf("tied players are out of seat order: %d then %d", first[0].Seat, first[1].Seat)
	}
}

// TestScoreboardWorksForAGameWithNoDeals is the point of the port: statistics
// used to be rummy arithmetic, so only rummy had any.
func TestScoreboardWorksForAGameWithNoDeals(t *testing.T) {
	m, standings := testMatch(t, []string{"user:a", "user:b"}, []int{1500, 500}, "completed", "p0")
	m.ModuleID, m.Variation = "holdem", "timed"
	for i := range standings {
		standings[i].LabelKey = "holdem.unit.chips"
	}

	sb := BuildScoreboard(m, standings)
	if sb.ModuleID != "holdem" {
		t.Errorf("module is %q", sb.ModuleID)
	}
	if sb.Standings[0].Score != 1500 || sb.Standings[0].ScoreLabelKey != "holdem.unit.chips" {
		t.Errorf("chip leader is %+v", sb.Standings[0])
	}
}
