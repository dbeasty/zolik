package game

import (
	"time"

	"zolik/server/internal/models"
	"zolik/server/internal/rules"
)

// appendEventsToActionLog appends the derived display/audit events for one
// player action. turnSeq is the RawAction.Seq that produced these events
// (see appendRawAction) — stamped onto each so a takeback can drop every
// event belonging to a truncated turn.
func appendEventsToActionLog(game *models.Game, playerID string, turnSeq int, events []rules.StateEvent) {
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
			TurnSeq:   turnSeq,
		})
	}
}

func nextRawActionSeq(existing []models.RawAction) int {
	seq := 0
	for _, a := range existing {
		if a.Seq > seq {
			seq = a.Seq
		}
	}
	return seq + 1
}

// appendRawAction records the raw player input behind one ApplyAction call
// — the unit rules.ReplayActions / takeback operate on — and returns its
// Seq (the "turn" number within RawActionLog) for stamping onto the
// resulting events.
func appendRawAction(game *models.Game, playerID string, action rules.Action) int {
	seq := nextRawActionSeq(game.RawActionLog)
	game.RawActionLog = append(game.RawActionLog, models.RawAction{
		Seq:       seq,
		Timestamp: time.Now().UTC(),
		PlayerID:  playerID,
		Input:     toModelsActionInput(action),
	})
	return seq
}

func toModelsActionInput(a rules.Action) models.ActionInput {
	return models.ActionInput{
		Type:      string(a.Type),
		DrawFrom:  string(a.DrawFrom),
		Cards:     a.Cards,
		MeldID:    a.MeldID,
		Card:      a.Card,
		CardIndex: a.CardIndex,
		Position:  a.Position,
	}
}

func fromModelsActionInput(mi models.ActionInput) rules.Action {
	return rules.Action{
		Type:      rules.ActionType(mi.Type),
		DrawFrom:  rules.DrawFrom(mi.DrawFrom),
		Cards:     mi.Cards,
		MeldID:    mi.MeldID,
		Card:      mi.Card,
		CardIndex: mi.CardIndex,
		Position:  mi.Position,
	}
}

// captureDealSnapshot takes the rules-relevant fields of g (assumed to be
// the state immediately after a deal, before any player action in that
// deal) as the replay anchor for the current GameNumber. sinceSeq is the
// RawActionLog Seq in effect at that moment (0 for game creation).
func captureDealSnapshot(g models.Game, sinceSeq int) *models.DealSnapshot {
	return &models.DealSnapshot{
		GameNumber:          g.GameNumber,
		SinceSeq:            sinceSeq,
		Phase:               g.Phase,
		CurrentTurn:         g.CurrentTurn,
		TurnOrder:           append([]string(nil), g.TurnOrder...),
		DealStarterID:       g.DealStarterID,
		Round:               g.Round,
		DrawPile:            append([]string(nil), g.DrawPile...),
		DiscardPile:         append([]string(nil), g.DiscardPile...),
		ReshuffleCount:      g.ReshuffleCount,
		DeckSeed:            g.DeckSeed,
		Hands:               copyHandMap(g.Hands),
		Melds:               copyMeldMap(g.Melds),
		MeldMeta:            g.MeldMeta,
		RoundReqMet:         copyBoolMap(g.RoundReqMet),
		InitialMeldMinimum:  g.InitialMeldMinimum,
		DiscardDrawMinRound: g.DiscardDrawMinRound,
		GameScores:          copyIntSliceMap(g.GameScores),
		TotalScores:         copyIntMap(g.TotalScores),
		NextMeldSeq:         g.NextMeldSeq,
	}
}

func copyHandMap(m map[string][]string) map[string][]string {
	out := make(map[string][]string, len(m))
	for k, v := range m {
		out[k] = append([]string(nil), v...)
	}
	return out
}

func copyMeldMap(m map[string][][]string) map[string][][]string {
	out := make(map[string][][]string, len(m))
	for k, v := range m {
		melds := make([][]string, len(v))
		for i, meld := range v {
			melds[i] = append([]string(nil), meld...)
		}
		out[k] = melds
	}
	return out
}

func copyBoolMap(m map[string]bool) map[string]bool {
	out := make(map[string]bool, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func copyIntMap(m map[string]int) map[string]int {
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func copyIntSliceMap(m map[string][]int) map[string][]int {
	out := make(map[string][]int, len(m))
	for k, v := range m {
		out[k] = append([]int(nil), v...)
	}
	return out
}

// dealSnapshotToRulesState rebuilds a full rules.GameState anchored at a
// deal's start, filling in fields captureDealSnapshot didn't need to store
// because they're always zero right after a deal (no turn has happened
// yet): scratch/undo state (LastLayOff, LastMeldLaid, TurnMeldSnapshot,
// DiscardDrawnCard*, MeldsLaidThisTurn) and Status/WinnerID/IsDraw, which
// only change on match completion and never occur mid-replay here.
// cfg is the game's own resolved ruleset (GameRules), not a profile name:
// re-deriving the config from the profile would quietly drop any tunable the
// game was actually started with, replaying the deal under profile defaults
// instead. Only InitialMeldMinimum and DiscardDrawMinRound are configurable
// today — so the two agree in practice — but the moment a profile grows
// another knob, a name-based rebuild starts lying and takeback silently
// replays under the wrong rules.
func dealSnapshotToRulesState(s models.DealSnapshot, cfg rules.RulesConfig) rules.GameState {
	rMeldMeta := map[string][]rules.MeldInfo{}
	for owner, infos := range s.MeldMeta {
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
		Status: rules.StatusActive,
		// ResolveConfig guards the zero value, the same way every other
		// entry point into the engine does.
		Rules:          rules.ResolveConfig(cfg),
		GameNumber:     s.GameNumber,
		Phase:          rules.Phase(s.Phase),
		CurrentTurn:    s.CurrentTurn,
		TurnOrder:      s.TurnOrder,
		DealStarterID:  s.DealStarterID,
		Round:          s.Round,
		DrawPile:       s.DrawPile,
		DiscardPile:    s.DiscardPile,
		ReshuffleCount: s.ReshuffleCount,
		DeckSeed:       s.DeckSeed,
		Hands:          s.Hands,
		Melds:          s.Melds,
		MeldMeta:       rMeldMeta,
		RoundReqMet:    s.RoundReqMet,
		GameScores:     s.GameScores,
		TotalScores:    s.TotalScores,
		NextMeldSeq:    s.NextMeldSeq,
	}
}
