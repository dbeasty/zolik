package game

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

// Migrating `games` documents into `matches`.
//
// This is the first of the two things `extensibility-plan.md` §3.x listed as
// left over, and the enabling step for the second. A `games` document is a
// Žolíky-shaped Mongo record with `drawPile`, `melds`, `meldMeta` and
// `roundReqMet` as first-class columns; a `matches` document is a generic
// envelope with the module's own state as opaque bytes.
//
// The conversion is mechanical because the two hold the same thing. `models.
// Game` is, field for field, `rules.GameState` plus an envelope — that is
// exactly the 30-field hand-mapping the module split exists to remove — so
// migrating is: read the rummy state out with the mapping that already exists,
// hand it to the module, and keep the envelope.
//
// Deliberately additive. It writes new `matches` documents and does not delete
// or alter the `games` they came from, so it can be run, checked, and run
// again. `MigrateGames` is idempotent on the same source document: a game
// already migrated is skipped rather than duplicated.

// ToRulesState exposes the legacy document's rummy state.
//
// Exported for the migration and for the equivalence test that proves the
// module path reproduces this one. It is the same mapping the live engine
// uses, deliberately — a second copy written for the migration could disagree
// with the first, and a migration that disagrees with the engine is how a
// game silently changes when it moves.
func ToRulesState(g models.Game) rules.GameState { return toRulesState(g) }

// MatchFromGame converts one legacy document into a match envelope.
func MatchFromGame(g models.Game) (models.Match, error) {
	state, err := zolikmod.StateFromRules(ToRulesState(g), playerRefsOf(g))
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

func playerRefsOf(g models.Game) []models.Player { return g.Players }

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
		var g models.Game
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
