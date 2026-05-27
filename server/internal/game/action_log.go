package game

import (
	"time"

	"zolik/server/internal/models"
	"zolik/server/internal/rules"
)

func appendEventsToActionLog(game *models.Game, playerID string, events []rules.StateEvent) {
	for _, ev := range events {
		data := map[string]interface{}{}
		for k, v := range ev.Data {
			data[k] = v
		}
		game.ActionLog = append(game.ActionLog, models.Action{
			Seq:       nextActionSeq(game.ActionLog),
			Timestamp: time.Now().UTC(),
			Type:      ev.Type,
			PlayerID:  playerID,
			Data:      data,
		})
	}
}
