package ui

import (
	"encoding/json"
	"strings"
	"testing"
)

// decode mirrors how the API client hands the body over: a loose map with
// every number already widened to float64.
func decode(t *testing.T, body string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("bad test fixture: %v", err)
	}
	return m
}

func TestFormatLifetimeStats_NoMatchesYet(t *testing.T) {
	got := formatLifetimeStats(decode(t, `{"gamesPlayed":0,"overall":{"matches":0}}`))
	if !strings.Contains(got, "No finished matches yet") {
		t.Fatalf("a new player should be told their record is empty, got %q", got)
	}
}

// Beating bots and beating people must never be presented as one number —
// that is the whole reason the split is recorded.
func TestFormatLifetimeStats_SeparatesHumanAndAIRecords(t *testing.T) {
	got := formatLifetimeStats(decode(t, `{
		"gamesPlayed": 10,
		"overall":  {"matches":10,"wins":6,"losses":3,"draws":1,"winRate":0.6,"avgScore":42.4},
		"vsHumans": {"matches":4,"wins":1,"losses":3,"draws":0,"winRate":0.25},
		"vsAI":     {"matches":6,"wins":5,"losses":0,"draws":1,"winRate":0.8333},
		"byAIDifficulty": {
			"hard":   {"matches":2,"wins":0,"losses":2,"draws":0,"winRate":0},
			"easy":   {"matches":3,"wins":3,"losses":0,"draws":0,"winRate":1},
			"medium": {"matches":1,"wins":1,"losses":0,"draws":0,"winRate":1}
		},
		"currentStreak": 2,
		"longestWinStreak": 4
	}`))

	for _, want := range []string{"vs people:", "vs AI:", "10 played", "6W 3L 1D"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "2 wins in a row") || !strings.Contains(got, "best 4") {
		t.Errorf("streak not rendered in:\n%s", got)
	}

	// Difficulties read easiest-first, so the progression is legible.
	iEasy := strings.Index(got, "easy")
	iMedium := strings.Index(got, "medium")
	iHard := strings.Index(got, "hard")
	if iEasy < 0 || iMedium < 0 || iHard < 0 {
		t.Fatalf("all three difficulties should appear in:\n%s", got)
	}
	if !(iEasy < iMedium && iMedium < iHard) {
		t.Errorf("difficulties out of order (easy=%d medium=%d hard=%d) in:\n%s", iEasy, iMedium, iHard, got)
	}
}

// A losing run has to show as a losing run rather than as no streak at all.
func TestFormatLifetimeStats_ReportsLosingStreaks(t *testing.T) {
	got := formatLifetimeStats(decode(t, `{
		"gamesPlayed": 3,
		"overall": {"matches":3,"wins":0,"losses":3,"draws":0,"winRate":0},
		"currentStreak": -3,
		"longestWinStreak": 0
	}`))
	if !strings.Contains(got, "3 losses in a row") {
		t.Fatalf("expected a losing streak, got %q", got)
	}
}

// A split the player has never experienced is left out rather than shown as a
// row of zeroes.
func TestFormatLifetimeStats_OmitsEmptySplits(t *testing.T) {
	got := formatLifetimeStats(decode(t, `{
		"gamesPlayed": 2,
		"overall":  {"matches":2,"wins":2,"losses":0,"draws":0,"winRate":1},
		"vsHumans": {"matches":0,"wins":0,"losses":0,"draws":0,"winRate":0},
		"vsAI":     {"matches":2,"wins":2,"losses":0,"draws":0,"winRate":1}
	}`))
	if strings.Contains(got, "vs people:") {
		t.Errorf("a player who has never faced a person should get no vs-people line:\n%s", got)
	}
	if !strings.Contains(got, "vs AI:") {
		t.Errorf("the vs-AI line should be present:\n%s", got)
	}
}

// The server owning the response shape means the TUI must survive fields it
// does not recognise, and fields it expected going missing.
func TestFormatLifetimeStats_ToleratesUnexpectedShapes(t *testing.T) {
	got := formatLifetimeStats(decode(t, `{"gamesPlayed":1,"overall":"not an object","somethingNew":42}`))
	if got == "" {
		t.Fatal("an unfamiliar body should still render something")
	}
}
