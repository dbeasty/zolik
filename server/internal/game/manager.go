package game

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/ai"
	"zolik/server/internal/models"
	"zolik/server/internal/rules"
)

type Manager struct {
	repo     *Repository
	hub      *Hub
	registry *ConnRegistry

	aiMu      sync.Mutex
	aiRunning map[string]bool
}

func NewManager(repo *Repository, hub *Hub) *Manager {
	return &Manager{
		repo:      repo,
		hub:       hub,
		registry:  hub.Registry(),
		aiRunning: map[string]bool{},
	}
}

func (m *Manager) HandleAction(ctx context.Context, gameID, playerID string, in WSIncoming) error {
	oid, err := bson.ObjectIDFromHex(gameID)
	if err != nil {
		return fmt.Errorf("invalid game id: %w", err)
	}

	rAction, err := toRulesAction(in)
	if err != nil {
		return err
	}

	game, err := m.repo.FindByID(ctx, oid)
	if err != nil {
		return err
	}

	// Convert models.Game -> rules.GameState (and keep non-rules fields intact in models.Game).
	rState := toRulesState(game)

	outcome, err := rules.ApplyAction(rState, playerID, rAction)
	if err != nil {
		if re, ok := err.(rules.RulesError); ok && re.Code == rules.ErrNoCardsLeft {
			m.suspendNoCardsLeft(ctx, gameID, oid, game)
		}
		return err
	}

	// Apply mutated rules fields back to models.Game.
	nextGame := game
	fromRulesState(&nextGame, outcome.State)

	appendEventsToActionLog(&nextGame, playerID, outcome.Events)

	if nextGame.Status == string(rules.StatusCompleted) {
		now := time.Now().UTC()
		nextGame.CompletedAt = &now
	}

	// Persist with optimistic concurrency.
	expectedVersion := game.Version
	if err := m.repo.UpdateWithVersion(ctx, oid, expectedVersion, nextGame); err != nil {
		return err
	}

	recipients := BroadcastRecipients(nextGame)
	m.broadcastEvents(gameID, playerID, outcome.Events, recipients)
	m.hub.BroadcastGameState(gameID, recipients, func(pid string) interface{} {
		return BuildGameStateMsg(nextGame, pid)
	})

	// If the next actor is an AI, run it.
	m.RunAIIfNeeded(ctx, gameID)
	return nil
}

// SuspendOnDisconnect suspends the game if the disconnected player is currently the active turn.
// This matches the spec's "game immediately suspended" behaviour (simplified for v1).
func (m *Manager) SuspendOnDisconnect(ctx context.Context, gameID, playerID string, reason string) {
	oid, err := bson.ObjectIDFromHex(gameID)
	if err != nil {
		return
	}

	game, err := m.repo.FindByID(ctx, oid)
	if err != nil {
		return
	}
	if game.Status != "active" {
		return
	}
	if game.CurrentTurn != playerID {
		return
	}

	now := time.Now().UTC()
	abandon := now.Add(24 * time.Hour)

	game.PreSuspendPhase = game.Phase
	game.Status = "suspended"
	game.Phase = string(rules.PhaseSuspended)
	game.SuspendedAt = &now
	game.AbandonAt = &abandon

	game.ActionLog = append(game.ActionLog, models.Action{
		Seq:       nextActionSeq(game.ActionLog),
		Timestamp: now,
		Type:      "suspend",
		PlayerID:  playerID,
		Data:      map[string]interface{}{"reason": reason},
	})

	_ = m.repo.UpdateWithVersion(ctx, oid, game.Version, game)

	recipients := BroadcastRecipients(game)
	m.hub.BroadcastGameState(gameID, recipients, func(pid string) interface{} {
		return BuildGameStateMsg(game, pid)
	})
}

// ResumeIfReturning un-suspends a game that was suspended because the
// current-turn player disconnected, once that same player reconnects.
// Games suspended for other reasons (e.g. suspendNoCardsLeft, which has no
// PreSuspendPhase set) are left alone.
func (m *Manager) ResumeIfReturning(ctx context.Context, gameID, playerID string) {
	oid, err := bson.ObjectIDFromHex(gameID)
	if err != nil {
		return
	}

	game, err := m.repo.FindByID(ctx, oid)
	if err != nil {
		return
	}
	if game.Status != "suspended" || game.PreSuspendPhase == "" {
		return
	}
	if game.CurrentTurn != playerID {
		return
	}

	now := time.Now().UTC()
	game.Status = "active"
	game.Phase = game.PreSuspendPhase
	game.PreSuspendPhase = ""
	game.SuspendedAt = nil
	game.AbandonAt = nil

	game.ActionLog = append(game.ActionLog, models.Action{
		Seq:       nextActionSeq(game.ActionLog),
		Timestamp: now,
		Type:      "resume",
		PlayerID:  playerID,
		Data:      map[string]interface{}{"reason": "reconnected"},
	})

	if err := m.repo.UpdateWithVersion(ctx, oid, game.Version, game); err != nil {
		return
	}

	recipients := BroadcastRecipients(game)
	m.hub.BroadcastGameState(gameID, recipients, func(pid string) interface{} {
		return BuildGameStateMsg(game, pid)
	})
}

func (m *Manager) suspendNoCardsLeft(ctx context.Context, gameID string, oid bson.ObjectID, game models.Game) {
	now := time.Now().UTC()
	abandon := now.Add(24 * time.Hour)
	game.Status = "suspended"
	game.Phase = string(rules.PhaseSuspended)
	game.SuspendedAt = &now
	game.AbandonAt = &abandon
	game.ActionLog = append(game.ActionLog, models.Action{
		Seq:       nextActionSeq(game.ActionLog),
		Timestamp: now,
		Type:      "suspend",
		PlayerID:  game.CurrentTurn,
		Data:      map[string]interface{}{"reason": "no_cards_left"},
	})
	_ = m.repo.UpdateWithVersion(ctx, oid, game.Version, game)
	recipients := BroadcastRecipients(game)
	m.broadcastEvents(gameID, game.CurrentTurn, []rules.StateEvent{
		{Type: "game_suspended", Data: map[string]interface{}{
			"suspendedPlayerId": game.CurrentTurn,
			"reason":            "no_cards_left",
		}},
	}, recipients)
}

func nextActionSeq(existing []models.Action) int {
	seq := 0
	for _, a := range existing {
		if a.Seq > seq {
			seq = a.Seq
		}
	}
	return seq + 1
}

func toRulesAction(in WSIncoming) (rules.Action, error) {
	switch in.Type {
	case "draw_card":
		return rules.Action{Type: rules.ActionDrawCard, DrawFrom: rules.DrawFrom(in.From)}, nil
	case "lay_meld":
		return rules.Action{Type: rules.ActionLayMeld, Cards: in.Cards}, nil
	case "lay_off":
		return rules.Action{Type: rules.ActionLayOff, MeldID: in.MeldID, Card: in.Card, Cards: in.Cards, Position: in.Position}, nil
	case "swap_joker":
		return rules.Action{Type: rules.ActionSwapJoker, MeldID: in.MeldID, Card: in.Card}, nil
	case "discard":
		return rules.Action{Type: rules.ActionDiscard, Card: in.Card}, nil
	case "undo_draw_discard":
		return rules.Action{Type: rules.ActionUndoDrawDiscard}, nil
	case "undo_lay_off":
		return rules.Action{Type: rules.ActionUndoLayOff}, nil
	default:
		return rules.Action{}, fmt.Errorf("unknown action type: %s", in.Type)
	}
}

func toRulesLayOffSnapshot(s *models.LayOffSnapshot) *rules.LayOffSnapshot {
	if s == nil {
		return nil
	}
	return &rules.LayOffSnapshot{
		PlayerID:  s.PlayerID,
		MeldID:    s.MeldID,
		PrevCards: s.PrevCards,
		PrevMeta: rules.MeldInfo{
			MeldID:    s.PrevMeta.MeldID,
			Type:      rules.MeldType(s.PrevMeta.Type),
			OwnerID:   s.PrevMeta.OwnerID,
			WildCount: s.PrevMeta.WildCount,
		},
		Cards: s.Cards,
	}
}

func fromRulesLayOffSnapshot(s *rules.LayOffSnapshot) *models.LayOffSnapshot {
	if s == nil {
		return nil
	}
	return &models.LayOffSnapshot{
		PlayerID:  s.PlayerID,
		MeldID:    s.MeldID,
		PrevCards: s.PrevCards,
		PrevMeta: models.MeldInfo{
			MeldID:    s.PrevMeta.MeldID,
			Type:      string(s.PrevMeta.Type),
			OwnerID:   s.PrevMeta.OwnerID,
			WildCount: s.PrevMeta.WildCount,
		},
		Cards: s.Cards,
	}
}

func toRulesState(g models.Game) rules.GameState {
	// MeldMeta needs type conversion.
	rMeldMeta := map[string][]rules.MeldInfo{}
	for owner, infos := range g.MeldMeta {
		for _, mi := range infos {
			rMeldMeta[owner] = append(rMeldMeta[owner], rules.MeldInfo{
				MeldID:    mi.MeldID,
				Type:      rules.MeldType(mi.Type),
				OwnerID:   mi.OwnerID,
				WildCount: mi.WildCount,
			})
		}
	}

	return rules.GameState{
		Status:                      rules.GameStatus(g.Status),
		Rules:                       rules.ResolveProfile(g.RulesProfile),
		GameNumber:                  g.GameNumber,
		Phase:                       rules.Phase(g.Phase),
		Created:                     g.CreatedAt,
		CurrentTurn:                 g.CurrentTurn,
		TurnOrder:                   g.TurnOrder,
		DealStarterID:               g.DealStarterID,
		Round:                       g.Round,
		DrawPile:                    g.DrawPile,
		DiscardPile:                 g.DiscardPile,
		ReshuffleCount:              g.ReshuffleCount,
		DeckSeed:                    g.DeckSeed,
		Hands:                       g.Hands,
		Melds:                       g.Melds,
		MeldMeta:                    rMeldMeta,
		RoundReqMet:                 g.RoundReqMet,
		InitialMeldMinimum:          g.InitialMeldMinimum,
		DiscardDrawMinRound:         g.DiscardDrawMinRound,
		MeldsLaidThisTurn:           g.MeldsLaidThisTurn,
		DiscardDrawnCardPendingMeld: g.DiscardDrawnCardPendingMeld,
		DiscardDrawnCards:           g.DiscardDrawnCards,
		LastLayOff:                  toRulesLayOffSnapshot(g.LastLayOff),
		GameScores:                  g.GameScores,
		TotalScores:                 g.TotalScores,
		WinnerID:                    g.WinnerID,
		IsDraw:                      g.IsDraw,
		NextMeldSeq:                 g.NextMeldSeq,
	}
}

func fromRulesState(g *models.Game, rs rules.GameState) {
	g.Status = string(rs.Status)
	g.GameNumber = rs.GameNumber
	g.Phase = string(rs.Phase)
	g.CurrentTurn = rs.CurrentTurn
	g.TurnOrder = rs.TurnOrder
	g.DealStarterID = rs.DealStarterID
	g.Round = rs.Round
	g.DrawPile = rs.DrawPile
	g.DiscardPile = rs.DiscardPile
	g.ReshuffleCount = rs.ReshuffleCount
	g.DeckSeed = rs.DeckSeed
	g.Hands = rs.Hands
	g.Melds = rs.Melds
	g.RoundReqMet = rs.RoundReqMet
	g.InitialMeldMinimum = rs.InitialMeldMinimum
	g.DiscardDrawMinRound = rs.DiscardDrawMinRound
	g.MeldsLaidThisTurn = rs.MeldsLaidThisTurn
	g.DiscardDrawnCardPendingMeld = rs.DiscardDrawnCardPendingMeld
	g.DiscardDrawnCards = rs.DiscardDrawnCards
	g.LastLayOff = fromRulesLayOffSnapshot(rs.LastLayOff)
	g.GameScores = rs.GameScores
	g.TotalScores = rs.TotalScores

	// MeldMeta
	g.MeldMeta = map[string][]models.MeldInfo{}
	for owner, metas := range rs.MeldMeta {
		for _, mi := range metas {
			g.MeldMeta[owner] = append(g.MeldMeta[owner], models.MeldInfo{
				MeldID:    mi.MeldID,
				Type:      string(mi.Type),
				OwnerID:   mi.OwnerID,
				WildCount: mi.WildCount,
			})
		}
	}

	g.NextMeldSeq = rs.NextMeldSeq
	g.WinnerID = rs.WinnerID
	g.IsDraw = rs.IsDraw
}

// RunAIIfNeeded starts an AI loop for the given game if an AI player is expected to act.
// It will only start one loop per game at a time.
func (m *Manager) RunAIIfNeeded(ctx context.Context, gameID string) {
	m.aiMu.Lock()
	if m.aiRunning[gameID] {
		m.aiMu.Unlock()
		return
	}
	m.aiRunning[gameID] = true
	m.aiMu.Unlock()

	go m.aiLoop(ctx, gameID)
}

func (m *Manager) aiLoop(ctx context.Context, gameID string) {
	defer func() {
		m.aiMu.Lock()
		delete(m.aiRunning, gameID)
		m.aiMu.Unlock()
	}()

	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

	// steps bounds the whole chain of AI sub-actions across however many AI
	// players act in a row before control returns to a human (or the game
	// ends); it exists only as a runaway-loop safety net, not a turn limiter,
	// since a single AI turn can legitimately take several sub-actions
	// (draw, multiple lay_melds, discard) and several AI players can be
	// chained back to back.
	steps := 0
	const maxSteps = 200
	lastActor := ""
	actorStall := 0
	const maxActorStall = 20
	for steps < maxSteps {
		steps++

		oid, err := bson.ObjectIDFromHex(gameID)
		if err != nil {
			return
		}
		game, err := m.repo.FindByID(ctx, oid)
		if err != nil {
			return
		}
		if game.Status != "active" {
			return
		}

		actorID := game.CurrentTurn
		if actorID == "" {
			return
		}

		aiPlayer := findAIPlayer(game.Players, actorID)
		if aiPlayer == nil {
			return
		}

		if actorID == lastActor {
			actorStall++
			if actorStall > maxActorStall {
				log.Printf("ai loop aborting: actor=%s made no progress after %d actions", actorID, actorStall)
				return
			}
		} else {
			lastActor = actorID
			actorStall = 0
		}

		delayMs := 500 + rnd.Intn(1500)
		time.Sleep(time.Duration(delayMs) * time.Millisecond)

		visible := aiVisibleFromGame(game)
		agent := ai.NewHeuristicAgent(aiPlayer.AIDifficulty)
		chosen := agent.ChooseAction(visible, game.Hands[actorID])

		// Translate chosen rules action to WSIncoming and apply it through the usual path.
		in := rulesActionToWSIncoming(chosen)
		if err := m.HandleAction(ctx, gameID, actorID, in); err != nil {
			log.Printf("ai action rejected: actor=%s type=%s err=%v", actorID, in.Type, err)
			// Defense in depth: a rejected lay_meld/lay_off must not be retried
			// verbatim (the agent would pick the same losing move again and
			// burn the whole step budget without ever ending its turn). Fall
			// back to discarding the worst card so the turn always progresses.
			if in.Type == "lay_meld" || in.Type == "lay_off" {
				if hand := game.Hands[actorID]; len(hand) > 0 {
					cfg := rules.ResolveProfile(game.RulesProfile)
					canDiscardJoker := len(hand) == 1 && game.RoundReqMet[actorID]
					fallback := WSIncoming{Type: "discard", Card: ai.PickWorstDiscard(hand, cfg, canDiscardJoker)}
					if err := m.HandleAction(ctx, gameID, actorID, fallback); err != nil {
						log.Printf("ai fallback discard rejected: actor=%s err=%v", actorID, err)
					}
				}
			}
		}
	}
}

func findAIPlayer(players []models.Player, id string) *models.Player {
	for i := range players {
		if players[i].ID == id && players[i].IsAI {
			return &players[i]
		}
	}
	return nil
}

func aiVisibleFromGame(game models.Game) ai.VisibleState {
	rMeldMeta := map[string][]rules.MeldInfo{}
	for owner, infos := range game.MeldMeta {
		for _, mi := range infos {
			rMeldMeta[owner] = append(rMeldMeta[owner], rules.MeldInfo{
				MeldID:    mi.MeldID,
				Type:      rules.MeldType(mi.Type),
				OwnerID:   mi.OwnerID,
				WildCount: mi.WildCount,
			})
		}
	}
	return ai.VisibleState{
		GameNumber:          game.GameNumber,
		Round:               game.Round,
		Phase:               game.Phase,
		CurrentTurn:         game.CurrentTurn,
		DiscardPile:         game.DiscardPile,
		Melds:               game.Melds,
		MeldMeta:            rMeldMeta,
		RoundReqMet:         game.RoundReqMet,
		TotalScores:         game.TotalScores,
		InitialMeldMinimum:  game.InitialMeldMinimum,
		DiscardDrawMinRound: game.DiscardDrawMinRound,
		Rules:               rules.ResolveProfile(game.RulesProfile),
	}
}

func rulesActionToWSIncoming(a rules.Action) WSIncoming {
	switch a.Type {
	case rules.ActionDrawCard:
		return WSIncoming{Type: "draw_card", From: string(a.DrawFrom)}
	case rules.ActionLayMeld:
		return WSIncoming{Type: "lay_meld", Cards: a.Cards}
	case rules.ActionLayOff:
		return WSIncoming{Type: "lay_off", MeldID: a.MeldID, Card: a.Card, Cards: a.Cards}
	case rules.ActionSwapJoker:
		return WSIncoming{Type: "swap_joker", MeldID: a.MeldID, Card: a.Card}
	case rules.ActionDiscard:
		return WSIncoming{Type: "discard", Card: a.Card}
	default:
		return WSIncoming{Type: string(a.Type)}
	}
}
