package match_test

import (
	"context"
	"testing"

	"zolik/server/internal/module"
)

// The join link is the one thing a player is sure to still have after they
// close the tab, so following it has to put a seated player back at their
// table whatever the table is doing. It used to ask about the status first and
// answer MATCH_ALREADY_STARTED to the very people the game was waiting for.
func TestJoinLetsASeatedPlayerBackInToAStartedTable(t *testing.T) {
	ctx := context.Background()
	m, log := observedManager(t)
	lobby, err := m.Create(ctx, "prsi", module.MatchConfig{}, human("host"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Join(ctx, lobby.ID.Hex(), human("guest")); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start(ctx, lobby.ID.Hex()); err != nil {
		t.Fatal(err)
	}
	closedBefore := log.count(lobby.ID.Hex())

	got, err := m.Join(ctx, lobby.JoinCode, human("guest"))
	if err != nil {
		t.Fatalf("a seated player following the join link: %v", err)
	}
	if got.Status != "active" || len(got.Players) != 2 {
		t.Errorf("rejoining changed the table: status %q, %d seats", got.Status, len(got.Players))
	}
	// Coming back is not the table filling up; the invites were already
	// withdrawn when it was dealt.
	if log.count(lobby.ID.Hex()) != closedBefore {
		t.Error("a player coming back closed the lobby a second time")
	}

	// Somebody new still meets a closed door.
	if _, err := m.Join(ctx, lobby.JoinCode, human("stranger")); module.CodeOf(err) != "MATCH_ALREADY_STARTED" {
		t.Errorf("a stranger joining a dealt table got %v, want MATCH_ALREADY_STARTED", err)
	}
}

// The same for a table the sweeper set aside: the player the others are
// waiting for must be able to get back to the screen that offers the resume.
func TestJoinLetsASeatedPlayerBackInToAnAbandonedTable(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)
	saved := abandonedMatch(t, repo, human("p2"))

	got, err := m.Join(ctx, saved.ID.Hex(), human("p2"))
	if err != nil {
		t.Fatalf("a seated player rejoining an abandoned table: %v", err)
	}
	if got.Status != "abandoned" {
		t.Errorf("rejoining changed the status to %q; only a resume may do that", got.Status)
	}
	if _, err := m.Join(ctx, saved.ID.Hex(), human("stranger")); module.CodeOf(err) != "MATCH_ALREADY_STARTED" {
		t.Errorf("a stranger joining an abandoned table got %v, want MATCH_ALREADY_STARTED", err)
	}
}
