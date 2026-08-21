package game

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/models"
	"zolik/server/internal/rules"
)

// RequestTakeback starts a takeback proposal: revert the current deal back
// to right after RawAction turn toSeq, pending the other active human
// players' approval (AI players auto-approve — see maybeApplyTakeback).
func (m *Manager) RequestTakeback(ctx context.Context, gameID, requesterID string, toSeq int) error {
	oid, err := bson.ObjectIDFromHex(gameID)
	if err != nil {
		return fmt.Errorf("invalid game id: %w", err)
	}
	game, err := m.repo.FindByID(ctx, oid)
	if err != nil {
		return err
	}
	if game.Status != "active" {
		return fmt.Errorf("game not active")
	}
	if game.PendingTakeback != nil {
		return fmt.Errorf("a takeback request is already pending")
	}
	if game.DealInitialState == nil || toSeq < game.DealInitialState.SinceSeq {
		return fmt.Errorf("toSeq is outside the current deal")
	}
	maxSeq := 0
	for _, ra := range game.RawActionLog {
		if ra.Seq > maxSeq {
			maxSeq = ra.Seq
		}
	}
	if toSeq >= maxSeq {
		return fmt.Errorf("nothing to take back")
	}

	game.PendingTakeback = &models.TakebackRequest{
		RequesterID: requesterID,
		ToSeq:       toSeq,
		Approvals:   map[string]bool{requesterID: true},
		CreatedAt:   time.Now().UTC(),
	}

	if err := m.repo.UpdateWithVersion(ctx, oid, game.Version, game); err != nil {
		return err
	}

	recipients := BroadcastRecipients(game)
	m.broadcastToAll(gameID, recipients, map[string]interface{}{
		"type":        "takeback_requested",
		"requesterId": requesterID,
		"toSeq":       toSeq,
	})
	return m.maybeApplyTakeback(ctx, gameID)
}

// RespondTakeback records one player's answer to the pending takeback
// request. A single "no" cancels it; once every other active human player
// has said "yes" it's applied immediately.
func (m *Manager) RespondTakeback(ctx context.Context, gameID, playerID string, approve bool) error {
	oid, err := bson.ObjectIDFromHex(gameID)
	if err != nil {
		return fmt.Errorf("invalid game id: %w", err)
	}
	game, err := m.repo.FindByID(ctx, oid)
	if err != nil {
		return err
	}
	if game.PendingTakeback == nil {
		return fmt.Errorf("no pending takeback request")
	}

	if !approve {
		game.PendingTakeback = nil
		if err := m.repo.UpdateWithVersion(ctx, oid, game.Version, game); err != nil {
			return err
		}
		recipients := BroadcastRecipients(game)
		m.broadcastToAll(gameID, recipients, map[string]interface{}{
			"type":       "takeback_rejected",
			"rejectedBy": playerID,
		})
		return nil
	}

	game.PendingTakeback.Approvals[playerID] = true
	if err := m.repo.UpdateWithVersion(ctx, oid, game.Version, game); err != nil {
		return err
	}
	return m.maybeApplyTakeback(ctx, gameID)
}

// maybeApplyTakeback applies the pending takeback once every active human
// player (everyone in Players who isn't an AI) has approved it. AI players
// don't get a vote — they always go along with it.
func (m *Manager) maybeApplyTakeback(ctx context.Context, gameID string) error {
	oid, err := bson.ObjectIDFromHex(gameID)
	if err != nil {
		return fmt.Errorf("invalid game id: %w", err)
	}
	game, err := m.repo.FindByID(ctx, oid)
	if err != nil {
		return err
	}
	pending := game.PendingTakeback
	if pending == nil {
		return nil
	}
	for _, p := range game.Players {
		if p.IsAI {
			continue
		}
		if !pending.Approvals[p.ID] {
			return nil // still waiting on someone
		}
	}

	if game.DealInitialState == nil {
		game.PendingTakeback = nil
		return m.repo.UpdateWithVersion(ctx, oid, game.Version, game)
	}

	initialState := dealSnapshotToRulesState(*game.DealInitialState, game.RulesProfile)
	var toReplay []rules.LoggedAction
	for _, ra := range game.RawActionLog {
		if ra.Seq <= game.DealInitialState.SinceSeq || ra.Seq > pending.ToSeq {
			continue
		}
		toReplay = append(toReplay, rules.LoggedAction{PlayerID: ra.PlayerID, Action: fromModelsActionInput(ra.Input)})
	}

	newState, err := rules.ReplayActions(initialState, toReplay)
	if err != nil {
		// The stored log itself is inconsistent with the rules engine —
		// refuse the takeback rather than leave the game in a state
		// nobody actually agreed to.
		game.PendingTakeback = nil
		_ = m.repo.UpdateWithVersion(ctx, oid, game.Version, game)
		return fmt.Errorf("takeback replay failed: %w", err)
	}

	nextGame := game
	fromRulesState(&nextGame, newState)

	truncatedRaw := make([]models.RawAction, 0, len(nextGame.RawActionLog))
	for _, ra := range nextGame.RawActionLog {
		if ra.Seq <= pending.ToSeq {
			truncatedRaw = append(truncatedRaw, ra)
		}
	}
	nextGame.RawActionLog = truncatedRaw

	truncatedEvents := make([]models.Action, 0, len(nextGame.ActionLog))
	for _, a := range nextGame.ActionLog {
		if a.TurnSeq == 0 || a.TurnSeq <= pending.ToSeq {
			truncatedEvents = append(truncatedEvents, a)
		}
	}
	nextGame.ActionLog = truncatedEvents

	nextGame.PendingTakeback = nil

	if err := m.repo.UpdateWithVersion(ctx, oid, game.Version, nextGame); err != nil {
		return err
	}

	recipients := BroadcastRecipients(nextGame)
	m.broadcastToAll(gameID, recipients, map[string]interface{}{
		"type":  "takeback_applied",
		"toSeq": pending.ToSeq,
	})
	m.hub.BroadcastGameState(gameID, recipients, func(pid string) interface{} {
		return BuildGameStateMsg(nextGame, pid)
	})
	m.RunAIIfNeeded(ctx, gameID)
	return nil
}

func (m *Manager) broadcastToAll(gameID string, recipients []string, payload map[string]interface{}) {
	if len(recipients) == 0 {
		return
	}
	msgs := make([]PlayerMessage, 0, len(recipients))
	for _, pid := range recipients {
		msgs = append(msgs, PlayerMessage{PlayerID: pid, Payload: payload})
	}
	m.hub.Publish(gameID, msgs)
}
