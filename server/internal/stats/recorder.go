package stats

import (
	"context"
	"errors"
	"log"
	"time"

	"zolik/server/internal/models"
	"zolik/server/internal/rules"
)

// Recorder turns a finished game into a permanent match record plus the
// lifetime updates derived from it.
type Recorder struct {
	repo *Repository
}

func NewRecorder(repo *Repository) *Recorder { return &Recorder{repo: repo} }

// RecordMatch writes the record for a completed game and folds it into every
// durable participant's lifetime statistics. cfg is the game's resolved
// ruleset (game.GameRules), passed in so this package never has to duplicate
// that resolution.
//
// It returns ErrAlreadyRecorded if the match was already recorded, which is a
// normal outcome under retry and not worth logging as a failure.
func (r *Recorder) RecordMatch(ctx context.Context, g models.Game, cfg rules.RulesConfig) (MatchResult, error) {
	if g.Status != string(rules.StatusCompleted) {
		return MatchResult{}, errors.New("game is not completed")
	}

	completedAt := time.Now().UTC()
	if g.CompletedAt != nil {
		completedAt = *g.CompletedAt
	}
	sb := BuildScoreboard(g, cfg)
	m := BuildMatchResult(sb, g.ID, g.CreatedAt, completedAt, time.Now().UTC())

	// The record goes in first, and its unique gameId index is what makes the
	// whole operation safe to retry: whoever loses that race stops here and
	// does not touch any aggregate.
	m, err := r.repo.InsertMatch(ctx, m)
	if err != nil {
		return MatchResult{}, err
	}

	// Aggregates are applied per seat, not per subject, because one AI
	// difficulty can hold several seats at the same table and each played its
	// own hand.
	//
	// A failure here leaves the match recorded but an aggregate stale. That is
	// the deliberate trade: the match records are the source of truth, so a
	// stale aggregate is repairable by replaying them, whereas a lost match
	// record is not repairable at all.
	var firstErr error
	for _, seat := range m.Participants {
		key := seat.Subject.Key()
		if key == "" {
			continue // guest: no durable identity to credit
		}
		if err := r.applySeat(ctx, m, seat, key); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return m, firstErr
}

func (r *Recorder) applySeat(ctx context.Context, m MatchResult, seat Standing, key string) error {
	ps, err := r.repo.FindPlayerStats(ctx, key)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	if errors.Is(err, ErrNotFound) {
		ps = ZeroStats(seat.Subject)
	}
	ps = ApplyMatch(ps, m, seat, time.Now().UTC())
	return r.repo.UpsertPlayerStats(ctx, ps)
}

// RecordMatchAsync records in the background, logging rather than returning
// errors. It is the hook the game manager uses: a completed match must not be
// able to fail the action that completed it, and the player who just went out
// should not wait on statistics bookkeeping to see the final screen.
//
// The context is deliberately detached from the caller's — the request or
// socket that carried the winning move is usually gone microseconds later, and
// a cancelled write would lose the record.
func (r *Recorder) RecordMatchAsync(g models.Game, cfg rules.RulesConfig) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if _, err := r.RecordMatch(ctx, g, cfg); err != nil {
			if errors.Is(err, ErrAlreadyRecorded) {
				return
			}
			log.Printf("game=%s stats: recording match failed: %v", g.ID.Hex(), err)
		}
	}()
}
