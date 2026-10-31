package match

import (
	"context"
	"errors"
	"fmt"
	"log"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// ForgetAccount takes a deleted account's name off every table it sat at.
//
// The tables themselves stay: the other players were there too, and a
// finished game is in their history and on their replays. The account's seat
// becomes an anonymous guest named for its place at the table ("Player 2"),
// exactly as stats.Claimer.ForgetUser renames it in the match record, so a
// replay and a history row agree. A seat held for the account at a rematch is
// let go, since nobody is coming to take it.
//
// The seat keeps its player id. The board and every stored move refer to the
// seat by it, so changing it would break the game; and once the account is
// gone the id is a number nothing else leads to.
//
// It is safe to run again: a second pass finds no seat the account holds.
func (m *Manager) ForgetAccount(ctx context.Context, userID string) error {
	if userID == "" {
		return errors.New("user id is required")
	}
	ids, err := m.repo.MatchIDsForUser(ctx, userID)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := m.forgetSeats(ctx, id, userID); err != nil {
			return fmt.Errorf("match %s: %w", id.Hex(), err)
		}
	}
	return nil
}

// forgetSeats anonymises one match through its live entry, so a table in play
// keeps its board and everyone watching it is told.
func (m *Manager) forgetSeats(ctx context.Context, id bson.ObjectID, userID string) error {
	e, err := m.lockMatch(ctx, id.Hex())
	var refusal module.Error
	switch {
	case errors.As(err, &refusal) && refusal.Code == "MATCH_NOT_FOUND":
		return nil // swept up since it was listed
	case err != nil:
		// A board that will not load — an old game whose moves no longer
		// apply under today's rules — must not keep its players' names for
		// ever. The envelope is what names them, and it can be rewritten on
		// its own: nothing holds the match live.
		log.Printf("match %s: forgetting an account without its board: %v", id.Hex(), err)
		return m.forgetSeatsInStore(ctx, id, userID)
	}
	next, changed := withoutAccount(e.match, userID)
	if !changed {
		e.mu.Unlock()
		return nil
	}
	if err := m.saveLocked(ctx, e, next); err != nil {
		e.mu.Unlock()
		return err
	}
	saved := e.match
	e.mu.Unlock()
	m.Broadcast(saved)
	return nil
}

func (m *Manager) forgetSeatsInStore(ctx context.Context, id bson.ObjectID, userID string) error {
	cur, err := m.repo.FindByID(ctx, id)
	if db.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	next, changed := withoutAccount(cur, userID)
	if !changed {
		return nil
	}
	return m.repo.UpdateWithVersion(ctx, id, cur.Version, next)
}

// withoutAccount is match with every seat the account holds made anonymous
// and every seat reserved for it let go. It reports whether anything changed.
func withoutAccount(match models.Match, userID string) (models.Match, bool) {
	changed := false
	players := append([]models.Player(nil), match.Players...)
	for i, p := range players {
		if p.UserID != userID {
			continue
		}
		players[i].UserID = ""
		players[i].GuestID = models.AnonymousGuestID(match.ID.Hex(), p.ID)
		players[i].Name = models.AnonymousSeatName(i)
		players[i].Avatar = ""
		changed = true
	}
	var reserved []models.Reservation
	for _, r := range match.Reserved {
		if r.UserID == userID {
			changed = true
			continue
		}
		reserved = append(reserved, r)
	}
	if !changed {
		return match, false
	}
	match.Players = players
	match.Reserved = reserved
	return match, true
}
