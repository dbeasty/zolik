package match_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"zolik/server/internal/match"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// finishedMatch writes a played-out prsi table seating players in that order.
func finishedMatch(t *testing.T, repo match.Repository, players ...models.Player) models.Match {
	t.Helper()
	ended := time.Now().UTC().Add(-time.Minute)
	order := make([]string, 0, len(players))
	for _, p := range players {
		order = append(order, p.ID)
	}
	m, err := repo.Insert(context.Background(), models.Match{
		ModuleID:  "prsi",
		Status:    "completed",
		Players:   players,
		TurnOrder: order,
		HostID:    players[0].ID,
		JoinCode:  "OLDTBL",
		EndedAt:   &ended,
		CreatedAt: ended,
	})
	if err != nil {
		t.Fatalf("inserting a finished match: %v", err)
	}
	return m
}

func persona(id, name, key string) models.Player {
	return models.Player{ID: id, Name: name, IsAI: true, AIDifficulty: "hard", AIPersona: key}
}

// The case this exists for: two people and a bot finish, one asks to play
// again, and the other is sent to the same table rather than a second one —
// sitting where they sat, across from the same opponent.
func TestRematchHoldsASeatForTheOtherPlayerAndKeepsTheTable(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)
	old := finishedMatch(t, repo, human("ann"), human("bob"), persona("bot:x", "Miroslav", "hard:miroslav"))

	next, err := m.Rematch(ctx, old.ID.Hex(), "bob")
	if err != nil {
		t.Fatalf("rematch: %v", err)
	}
	if next.Status != "lobby" || next.HostID != "bob" || next.RematchOf != old.ID.Hex() {
		t.Fatalf("rematch = status %q host %q of %q; want a lobby hosted by bob from the old table",
			next.Status, next.HostID, next.RematchOf)
	}
	if got := ids(next.Players); fmt.Sprint(got) != "[bob bot:x]" {
		t.Errorf("seated %v, want bob and the same bot", got)
	}
	if bot := next.Players[1]; bot.AIPersona != "hard:miroslav" || bot.AIDifficulty != "hard" {
		t.Errorf("the bot came back as %+v, want the same persona and skill", bot)
	}
	if len(next.Reserved) != 1 || next.Reserved[0].PlayerID != "ann" {
		t.Errorf("reserved %+v, want a seat held for ann", next.Reserved)
	}

	reloaded, err := repo.FindByID(ctx, old.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Rematch == nil || reloaded.Rematch.MatchID != next.ID.Hex() || reloaded.Rematch.HostID != "bob" {
		t.Fatalf("the finished table points at %+v, want bob's rematch", reloaded.Rematch)
	}

	// Ann asks too, and sits down at Bob's — back in her own seat.
	joined, err := m.Rematch(ctx, old.ID.Hex(), "ann")
	if err != nil {
		t.Fatalf("second rematch press: %v", err)
	}
	if joined.ID != next.ID {
		t.Fatalf("ann was sent to %s, want bob's table %s", joined.ID.Hex(), next.ID.Hex())
	}
	if got := ids(joined.Players); fmt.Sprint(got) != "[ann bob bot:x]" {
		t.Errorf("seated %v, want the old order [ann bob bot:x]", got)
	}
	if fmt.Sprint(joined.TurnOrder) != "[ann bob bot:x]" {
		t.Errorf("turn order %v, want the old order", joined.TurnOrder)
	}
	if len(joined.Reserved) != 0 {
		t.Errorf("still holding %+v after ann sat down", joined.Reserved)
	}
}

// Two presses at once still make one table.
func TestConcurrentRematchPressesOpenOneTable(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)
	old := finishedMatch(t, repo, human("ann"), human("bob"))

	var wg sync.WaitGroup
	got := make([]models.Match, 2)
	errs := make([]error, 2)
	for i, who := range []string{"ann", "bob"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got[i], errs[i] = m.Rematch(ctx, old.ID.Hex(), who)
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("press %d: %v", i, err)
		}
	}
	if got[0].ID != got[1].ID {
		t.Fatalf("two presses opened two tables: %s and %s", got[0].ID.Hex(), got[1].ID.Hex())
	}
}

// A held seat is not an open one: somebody holding only the join code is
// turned away until the person it is held for says no.
func TestAHeldSeatIsClosedToStrangersUntilDeclined(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)
	_, max := m.Registry().Get("prsi").Descriptor().SeatRange("")
	seats := []models.Player{human("ann"), human("bob")}
	for i := 2; i < max; i++ {
		seats = append(seats, persona(fmt.Sprintf("bot:%d", i), "Bot", ""))
	}
	old := finishedMatch(t, repo, seats...)

	next, err := m.Rematch(ctx, old.ID.Hex(), "ann")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Join(ctx, next.JoinCode, human("stranger")); module.CodeOf(err) != "MATCH_FULL" {
		t.Fatalf("a stranger took bob's held seat: %v, want MATCH_FULL", err)
	}

	if _, err := m.DeclineRematch(ctx, next.ID.Hex(), "bob"); err != nil {
		t.Fatalf("declining: %v", err)
	}
	// Twice is not an error.
	if _, err := m.DeclineRematch(ctx, next.ID.Hex(), "bob"); err != nil {
		t.Fatalf("declining again: %v", err)
	}
	if _, err := m.Join(ctx, next.JoinCode, human("stranger")); err != nil {
		t.Fatalf("the seat bob gave up is still closed: %v", err)
	}
}

// With nobody to wait for there is no lobby: the same bots, dealt at once.
func TestABotsOnlyRematchIsDealtStraightAway(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)
	old := finishedMatch(t, repo, human("ann"), persona("bot:x", "Miroslav", "hard:miroslav"))

	next, err := m.Rematch(ctx, old.ID.Hex(), "ann")
	if err != nil {
		t.Fatalf("rematch: %v", err)
	}
	if next.Status != "active" {
		t.Errorf("status %q, want a table already dealt", next.Status)
	}
	if got := ids(next.Players); fmt.Sprint(got) != "[ann bot:x]" {
		t.Errorf("seated %v, want ann and the same bot", got)
	}
}

// A dealt table starting over mid-game would be a way to throw one away; and
// only somebody who played at a table can ask for it again.
func TestRematchRefusals(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)

	live, err := m.Create(ctx, "prsi", module.MatchConfig{}, human("ann"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Rematch(ctx, live.ID.Hex(), "ann"); module.CodeOf(err) != "MATCH_NOT_OVER" {
		t.Errorf("rematch of a lobby: %v, want MATCH_NOT_OVER", err)
	}

	old := finishedMatch(t, repo, human("ann"), persona("bot:x", "Miroslav", ""))
	for _, who := range []string{"stranger", "bot:x"} {
		if _, err := m.Rematch(ctx, old.ID.Hex(), who); module.CodeOf(err) != "NOT_AT_THIS_TABLE" {
			t.Errorf("rematch by %s: %v, want NOT_AT_THIS_TABLE", who, err)
		}
	}
}

func ids(players []models.Player) []string {
	out := make([]string, len(players))
	for i, p := range players {
		out[i] = p.ID
	}
	return out
}
