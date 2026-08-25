package legacy

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"zolik/server/internal/models"
	"zolik/server/internal/rules"
	"zolik/server/internal/zolikmod"
)

// ToRulesState exposes the legacy document's rummy state.
//
// Exported for the migration and for the equivalence test that proves the
// module path reproduces this one. It is the same mapping the live engine
// uses, deliberately — a second copy written for the migration could disagree
// with the first, and a migration that disagrees with the engine is how a
// game silently changes when it moves.

// MatchFromGame converts one legacy document into a match envelope.
func MatchFromGame(g Game) (models.Match, error) {
	state, err := zolikmod.StateFromRules(toRulesState(g), playerRefsOf(g))
	if err != nil {
		return models.Match{}, err
	}

	m := models.Match{
		ModuleID:  "zolik",
		Variation: g.RulesProfile,
		Status:    envelopeStatus(g.Status),
		Players:   g.Players,
		TurnOrder: g.TurnOrder,
		HostID:    g.HostID,
		JoinCode:  g.JoinCode,
		State:     state,
		Seed:      g.DeckSeed,
		WinnerID:  g.WinnerID,
		CreatedAt: g.CreatedAt,
		// MigratedFrom is what makes a re-run idempotent, and what lets a
		// migrated match be traced back to the document it came from if
		// anything looks wrong afterwards.
		MigratedFrom: g.ID,
	}
	if g.WinnerID != "" {
		m.Winners = []string{g.WinnerID}
	}
	return m, nil
}

// envelopeStatus maps a rummy status onto the envelope's vocabulary.
//
// "suspended" survives as itself: the envelope has the same idea, and a match
// paused for a missing player should still be paused after it moves.
func envelopeStatus(s string) string {
	switch s {
	case "completed", "suspended", "lobby":
		return s
	default:
		return "active"
	}
}

func playerRefsOf(g Game) []models.Player { return g.Players }

// MigrateGames copies every `games` document into `matches`.
//
// Returns how many were migrated and how many were skipped as already done.
func MigrateGames(ctx context.Context, db *mongo.Database) (migrated, skipped int, err error) {
	games := db.Collection("games")
	matches := db.Collection("matches")

	cur, err := games.Find(ctx, bson.M{})
	if err != nil {
		return 0, 0, fmt.Errorf("read games: %w", err)
	}
	defer cur.Close(ctx)

	for cur.Next(ctx) {
		var g Game
		if err := cur.Decode(&g); err != nil {
			return migrated, skipped, fmt.Errorf("decode game: %w", err)
		}

		n, err := matches.CountDocuments(ctx, bson.M{"migratedFrom": g.ID})
		if err != nil {
			return migrated, skipped, fmt.Errorf("check %s: %w", g.ID.Hex(), err)
		}
		if n > 0 {
			skipped++
			continue
		}

		m, err := MatchFromGame(g)
		if err != nil {
			// One unconvertible document must not stop the rest: it is almost
			// always an ancient record predating a field, and the operator
			// wants the other ten thousand.
			log.Printf("migrate: skipping game %s: %v", g.ID.Hex(), err)
			skipped++
			continue
		}
		m.CreatedAt = orNow(m.CreatedAt)
		if _, err := matches.InsertOne(ctx, m); err != nil {
			return migrated, skipped, fmt.Errorf("insert match for %s: %w", g.ID.Hex(), err)
		}
		migrated++
	}
	return migrated, skipped, cur.Err()
}

func orNow(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now().UTC()
	}
	return t
}

func toRulesState(g Game) rules.GameState {
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
		Rules:                       GameRules(g),
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
		MeldsLaidThisTurn:           g.MeldsLaidThisTurn,
		DiscardDrawnCardPendingMeld: g.DiscardDrawnCardPendingMeld,
		DiscardTakenCard:            g.DiscardTakenCard,
		DiscardDrawnCards:           g.DiscardDrawnCards,
		LastLayOff:                  toRulesLayOffSnapshot(g.LastLayOff),
		LastMeldLaid:                toRulesMeldLaidSnapshot(g.LastMeldLaid),
		TurnMeldSnapshot:            toRulesTurnMeldSnapshot(g.TurnMeldSnapshot),
		GameScores:                  g.GameScores,
		TotalScores:                 g.TotalScores,
		WinnerID:                    g.WinnerID,
		IsDraw:                      g.IsDraw,
		NextMeldSeq:                 g.NextMeldSeq,
	}
}

func fromRulesState(g *Game, rs rules.GameState) {
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
	// The ruleset is persisted with the game rather than re-derived from the
	// profile name on load, so house-rule overrides survive a reload and an
	// edit to a shipped profile constant can't change a game already running.
	// The two legacy scalar columns are kept mirrored for older readers.
	// RulesProfile is deliberately NOT rewritten here: it is lobby metadata
	// naming the variation the host picked, and the clients key their rule
	// summaries off it. An unrecognised name resolves to the default ruleset
	// without being renamed, so a game labelled "custom house rules" in the
	// lobby doesn't silently relabel itself on the first action.
	cfg := rules.ResolveConfig(rs.Rules)
	g.Rules = fromRulesConfig(cfg)
	g.InitialMeldMinimum = cfg.InitialMeldMinimum
	g.DiscardDrawMinRound = cfg.DiscardDrawMinRound
	g.MeldsLaidThisTurn = rs.MeldsLaidThisTurn
	g.DiscardDrawnCardPendingMeld = rs.DiscardDrawnCardPendingMeld
	g.DiscardTakenCard = rs.DiscardTakenCard
	g.DiscardDrawnCards = rs.DiscardDrawnCards
	g.LastLayOff = fromRulesLayOffSnapshot(rs.LastLayOff)
	g.LastMeldLaid = fromRulesMeldLaidSnapshot(rs.LastMeldLaid)
	g.TurnMeldSnapshot = fromRulesTurnMeldSnapshot(rs.TurnMeldSnapshot)
	g.GameScores = rs.GameScores
	g.TotalScores = rs.TotalScores

	// MeldMeta
	g.MeldMeta = map[string][]MeldInfo{}
	for owner, metas := range rs.MeldMeta {
		for _, mi := range metas {
			g.MeldMeta[owner] = append(g.MeldMeta[owner], MeldInfo{
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
