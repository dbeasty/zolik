package match

import (
	"context"

	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// Sitting down at a game under way.
//
// Most games are dealt once to the people at the table and cannot take
// another: a rummy hand or a trick has no room for a fifth player halfway
// through. A cash table can — that is what one is — so a module that says so
// (module.LateSeater) is asked to seat the newcomer. It adds the seat to the
// board, joining at the next deal, and the runtime stores that board as the
// match's newest snapshot: a seat is not a move anybody made, and the moves
// after it are played on the board that has it.
func (m *Manager) seatLateLocked(ctx context.Context, e *liveMatch, mod module.GameModule, p models.Player) (models.Match, bool, error) {
	match := e.match
	late, ok := mod.(module.LateSeater)
	if !ok || (match.Status != "active" && match.Status != "suspended") {
		return models.Match{}, false, module.Error{Code: "MATCH_ALREADY_STARTED"}
	}
	if e.stateErr != nil {
		return models.Match{}, false, e.stateErr
	}
	if _, max := mod.Descriptor().SeatRange(match.Variation); len(match.Players) >= max {
		return models.Match{}, false, module.Error{Code: "MATCH_FULL"}
	}
	next, _, err := late.SeatLate(module.State(match.State), module.PlayerRef{ID: p.ID, Name: p.Name, IsAI: p.IsAI})
	if err != nil {
		return models.Match{}, false, err
	}
	match.Players = append(append([]models.Player(nil), match.Players...), p)
	match.TurnOrder = append(append([]string(nil), match.TurnOrder...), p.ID)
	if err := m.snapshotLocked(ctx, e, match, next); err != nil {
		return models.Match{}, false, err
	}
	return e.match, false, nil
}
