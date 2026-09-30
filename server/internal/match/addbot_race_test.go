package match_test

import (
	"context"
	"testing"

	"zolik/server/internal/models"
)

// Two add-bots arriving together — a double tap, two tabs — must not both
// decide who sits down from the same snapshot of the table, or both seat the
// same persona. JoinWith hands the seat builder the table as it stands under
// the lock, so a seat taken since the caller last read the table is visible.
func TestJoinWithDecidesTheSeatFromTheLockedTable(t *testing.T) {
	m := seatingManager(t)
	ctx := context.Background()
	table := seatedTable(t, m, 1)

	// Another request seats a bot after this one read the table.
	if _, err := m.Join(ctx, table.ID.Hex(), models.Player{ID: "bot:a", IsAI: true, AIPersona: "hard:ivan"}); err != nil {
		t.Fatalf("Join: %v", err)
	}

	var saw []models.Player
	_, seated, err := m.JoinWith(ctx, table.ID.Hex(), func(cur models.Match) models.Player {
		saw = cur.Players
		return models.Player{ID: "bot:b", IsAI: true, AIPersona: "hard:sona"}
	})
	if err != nil {
		t.Fatalf("JoinWith: %v", err)
	}
	if len(table.Players) != 1 || len(saw) != 2 || saw[1].AIPersona != "hard:ivan" {
		t.Fatalf("the seat was decided from %d seats (snapshot had %d); want the locked table's 2", len(saw), len(table.Players))
	}
	if seated.ID != "bot:b" {
		t.Errorf("JoinWith returned %q, want the seat it built", seated.ID)
	}
}
