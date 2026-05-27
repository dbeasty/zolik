package game

import (
	"zolik/server/internal/models"
)

type ReplayResponse struct {
	GameState GameStateMsg   `json:"gameState"`
	ActionLog []models.Action `json:"actionLog"`
}

// RedactActionLogForPlayer returns an action log projection safe to send to the given player.
// v1 rule: if the action is not by the player, remove any card-related data.
func RedactActionLogForPlayer(actionLog []models.Action, playerID string) []models.Action {
	out := make([]models.Action, 0, len(actionLog))
	for _, a := range actionLog {
		na := a
		if a.PlayerID != playerID {
			// redact all action data; v1 action log uses minimal data but keep safe anyway.
			na.Data = map[string]interface{}{}
		}
		out = append(out, na)
	}
	return out
}

func BuildReplayResponse(game models.Game, playerID string) ReplayResponse {
	return ReplayResponse{
		GameState: BuildGameStateMsg(game, playerID),
		ActionLog: RedactActionLogForPlayer(game.ActionLog, playerID),
	}
}

