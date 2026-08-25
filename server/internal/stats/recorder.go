package stats

import (
	"context"
	"errors"
	"log"
	"time"

	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// Recorder turns a finished game into a permanent match record plus the
// lifetime updates derived from it.
type Recorder struct {
	repo *Repository
}

func NewRecorder(repo *Repository) *Recorder { return &Recorder{repo: repo} }

// RecordMatch writes the record for a completed match and folds it into every
// durable participant's lifetime statistics.
//
// standings are the module's own final placings, passed in rather than derived
// here: only the module knows how its game is scored, and a second opinion in
// this package would be a second implementation of "who won". That is also
// what makes this work for every game rather than only for rummy.
//
// It returns ErrAlreadyRecorded if the match was already recorded, which is a
// normal outcome under retry and not worth logging as a failure.
func (r *Recorder) RecordMatch(ctx context.Context, m0 models.Match, standings []module.Standing) (MatchResult, error) {
	if m0.Status != "completed" {
		return MatchResult{}, errors.New("match is not completed")
	}

	completedAt := time.Now().UTC()
	if m0.EndedAt != nil {
		completedAt = *m0.EndedAt
	}
	sb := BuildScoreboard(m0, standings)
	m := BuildMatchResult(sb, m0.ID, m0.CreatedAt, completedAt, time.Now().UTC())

	// The record goes in first, and its unique matchId index is what makes the
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
		if !seat.Subject.Durable() {
			// Guest: the match record carries their key so the game can be
			// found (and claimed on sign-up), but no lifetime record is
			// created for a per-device identity.
			continue
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
func (r *Recorder) RecordMatchAsync(m models.Match, standings []module.Standing) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if _, err := r.RecordMatch(ctx, m, standings); err != nil {
			if errors.Is(err, ErrAlreadyRecorded) {
				return
			}
			log.Printf("match=%s stats: recording failed: %v", m.ID.Hex(), err)
		}
	}()
}
