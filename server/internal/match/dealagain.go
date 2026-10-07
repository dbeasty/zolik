package match

import (
	"context"

	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// DealAgain deals a finished one-seat game again: the same game, the same
// table settings and the very same cards, at a new table for the player who
// sat at the old one (docs/solitaire-plan.md §4b).
//
// The seed is copied here, on the server, and never leaves it. That is the
// point of doing it this way rather than with a deal number: a deal a player
// could read is a deal a player could choose, and in a game played against
// the deck alone choosing the deal is winning it.
//
// Only a finished game: one still in play could otherwise be dealt again
// beside itself and played out with the knowledge of the other. Only a
// one-seat game, because "the same deal" of a table for several people needs
// the same people, which is what a rematch is for.
func (m *Manager) DealAgain(ctx context.Context, idOrCode, callerID string) (models.Match, error) {
	old, err := m.Current(ctx, idOrCode)
	if err != nil {
		return models.Match{}, err
	}
	if old.Status != "completed" {
		return models.Match{}, module.Error{Code: "MATCH_NOT_OVER"}
	}
	if !m.savedGame(old) {
		return models.Match{}, module.Error{Code: "NOT_A_SOLO_GAME", Message: old.ModuleID}
	}
	caller := playerByID(old.Players, callerID)
	if caller == nil || caller.IsAI {
		return models.Match{}, module.Error{Code: "NOT_AT_THIS_TABLE", Message: callerID}
	}
	host := *caller
	host.ConnectionID = ""
	host.SeatKeyHash = ""

	lobby, err := m.Create(ctx, old.ModuleID, module.MatchConfig{Variation: old.Variation, Options: old.Options}, host)
	if err != nil {
		return models.Match{}, err
	}
	e, err := m.lockMatch(ctx, lobby.ID.Hex())
	if err != nil {
		return models.Match{}, err
	}
	next := e.match
	next.Seed = old.Seed
	next.DealFrom = old.DealFrom
	if next.DealFrom == "" {
		next.DealFrom = old.ID.Hex()
	}
	err = m.saveLocked(ctx, e, next)
	e.mu.Unlock()
	if err != nil {
		return models.Match{}, err
	}
	return m.Start(ctx, lobby.ID.Hex())
}
