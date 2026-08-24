package ui

import (
	"fmt"
	"sort"
	"strings"
)

// formatLifetimeStats renders the /users/me/stats body for the menu screen.
//
// It reads the response as a loose map rather than a typed struct because the
// TUI shares no types with the server, and a stats screen must not be the
// thing that breaks when the server adds a field. Anything missing simply
// renders as absent.
func formatLifetimeStats(data map[string]any) string {
	played := numField(data, "gamesPlayed")
	if played == 0 {
		return "  No finished matches yet — your record starts with your first one."
	}

	var b strings.Builder
	fmt.Fprintf(&b, "  Lifetime: %s\n", recordLine(data["overall"]))

	// The split is the point of the record: beating bots and beating people
	// are different achievements, so they are never shown merged.
	if line := splitLine("  vs people:", data["vsHumans"]); line != "" {
		b.WriteString(line + "\n")
	}
	if line := splitLine("  vs AI:    ", data["vsAI"]); line != "" {
		b.WriteString(line + "\n")
	}
	for _, l := range difficultyLines(data["byAIDifficulty"]) {
		b.WriteString(l + "\n")
	}
	if s := streakLine(data); s != "" {
		b.WriteString(s + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// recordLine renders one tally as "3W 1L 1D (60%), avg 42 pts/deal".
func recordLine(v any) string {
	t, ok := v.(map[string]any)
	if !ok {
		return "—"
	}
	matches := numField(t, "matches")
	if matches == 0 {
		return "no matches"
	}
	out := fmt.Sprintf("%d played · %dW %dL %dD (%.0f%%)",
		matches,
		numField(t, "wins"), numField(t, "losses"), numField(t, "draws"),
		floatField(t, "winRate")*100)
	// The module's own measure, whatever it counts. A deal-level average used
	// to go here, and described rummy: a game with no deals had nothing to put
	// in it.
	if d := floatField(t, "avgScore"); d != 0 {
		out += fmt.Sprintf(" · %.0f avg", d)
	}
	return out
}

// splitLine renders a labelled tally, or nothing at all when the player has
// never played that way — an empty "vs AI: 0 played" line is noise.
func splitLine(label string, v any) string {
	t, ok := v.(map[string]any)
	if !ok || numField(t, "matches") == 0 {
		return ""
	}
	return label + " " + recordLine(t)
}

// difficultyLines breaks the AI record down per difficulty, easiest first, so
// "I can beat medium but not hard" is legible at a glance.
func difficultyLines(v any) []string {
	m, ok := v.(map[string]any)
	if !ok || len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		oi, oj := difficultyRank(keys[i]), difficultyRank(keys[j])
		if oi != oj {
			return oi < oj
		}
		return keys[i] < keys[j]
	})

	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, fmt.Sprintf("    %-7s %s", k, recordLine(m[k])))
	}
	return out
}

func difficultyRank(d string) int {
	switch d {
	case "easy":
		return 0
	case "medium":
		return 1
	case "hard":
		return 2
	default:
		return 3
	}
}

// streakLine reports the current run. The value is signed — negative is a
// losing run — because a streak that only ever counts up would quietly hide
// the direction that matters most.
func streakLine(data map[string]any) string {
	cur := numField(data, "currentStreak")
	best := numField(data, "longestWinStreak")
	switch {
	case cur > 0:
		return fmt.Sprintf("  Streak: %d wins in a row (best %d)", cur, best)
	case cur < 0:
		return fmt.Sprintf("  Streak: %d losses in a row (best run %d wins)", -cur, best)
	case best > 0:
		return fmt.Sprintf("  Best win streak: %d", best)
	default:
		return ""
	}
}

// numField reads an integer out of decoded JSON, where every number arrives as
// a float64.
func numField(m map[string]any, key string) int {
	return int(floatField(m, key))
}

func floatField(m map[string]any, key string) float64 {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	default:
		return 0
	}
}
