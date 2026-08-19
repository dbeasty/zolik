package game

import (
	"zolik/server/internal/models"
)

type ReplayResponse struct {
	GameState GameStateMsg    `json:"gameState"`
	ActionLog []models.Action `json:"actionLog"`
}

var privateActionDataKeys = []string{
	"card", "cards", "allHands", "meldId",
}

// RedactActionLogForPlayer returns an action log projection safe to send to the given player.
func RedactActionLogForPlayer(actionLog []models.Action, playerID string) []models.Action {
	out := make([]models.Action, 0, len(actionLog))
	for _, a := range actionLog {
		na := a
		na.Data = redactActionData(a, playerID)
		out = append(out, na)
	}
	return out
}

func redactActionData(a models.Action, viewerID string) map[string]interface{} {
	if a.Data == nil {
		return map[string]interface{}{}
	}
	data := map[string]interface{}{}
	for k, v := range a.Data {
		data[k] = v
	}

	switch a.Type {
	case "draw_deck", "player_drew":
		if a.PlayerID != viewerID {
			delete(data, "card")
		}
	case "deal_ended":
		if a.PlayerID != viewerID {
			delete(data, "allHands")
		}
	default:
		if a.PlayerID != viewerID {
			for _, key := range privateActionDataKeys {
				delete(data, key)
			}
		}
	}
	return data
}

func BuildReplayResponse(game models.Game, playerID string) ReplayResponse {
	return ReplayResponse{
		GameState: BuildGameStateMsg(game, playerID),
		ActionLog: RedactActionLogForPlayer(game.ActionLog, playerID),
	}
}
