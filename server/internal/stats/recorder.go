package stats

import (
	"context"
	"errors"
	"log"
	"time"

	"zolik/server/internal/metrics"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// Recorder turns a finished game into a permanent match record plus the
// lifetime updates derived from it.
type Recorder struct {
	repo Repository
	// metrics counts completions for the operator's console. Never nil after
	// NewRecorder; a sink that discards is the default.
	metrics metrics.Sink
}

func NewRecorder(repo Repository) *Recorder {
	return &Recorder{repo: repo, metrics: metrics.Nop()}
}

// SetMetrics attaches the counter sink.
func (r *Recorder) SetMetrics(s metrics.Sink) {
	if s == nil {
		s = metrics.Nop()
	}
	r.metrics = s
}

// RecordMatch writes the record for a completed match and folds it into every
// durable participant's lifetime statistics.
//
// The outcome is the module's own — its final placings and its round history,
// passed in rather than derived here: only the module knows how its game is
// scored, and a second opinion in this package would be a second implementation
// of "who won". That is also what makes this work for every game rather than
// only for rummy.
//
// One value rather than a growing argument list. It was already "the module's
// standings"; it is now also "the module's rounds", and a third fact would
// otherwise be a third parameter and a fourth signature change.
//
// It returns ErrAlreadyRecorded if the match was already recorded, which is a
// normal outcome under retry and not worth logging as a failure.
func (r *Recorder) RecordMatch(ctx context.Context, m0 models.Match, out module.Outcome) (MatchResult, error) {
	if m0.Status != "completed" {
		return MatchResult{}, errors.New("match is not completed")
	}

	completedAt := time.Now().UTC()
	if m0.EndedAt != nil {
		completedAt = *m0.EndedAt
	}
	sb := BuildScoreboard(m0, out)
	m := BuildMatchResult(sb, m0.ID, m0.CreatedAt, completedAt, time.Now().UTC())

	// The record goes in first, and its unique matchId index is what makes the
	// whole operation safe to retry: whoever loses that race stops here and
	// does not touch any aggregate.
	m, err := r.repo.InsertMatch(ctx, m)
	if err != nil {
		return MatchResult{}, err
	}

	// Counted here, at the same moment the permanent record is written and
	// behind the same unique index. That is what makes the daily counter and
	// match_results incapable of disagreeing about how many games were played:
	// a retried completion loses the insert race above and returns before
	// reaching this line, so it is counted exactly once.
	r.countCompletion(m)

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
func (r *Recorder) RecordMatchAsync(m models.Match, out module.Outcome) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if _, err := r.RecordMatch(ctx, m, out); err != nil {
			if errors.Is(err, ErrAlreadyRecorded) {
				return
			}
			log.Printf("match=%s stats: recording failed: %v", m.ID.Hex(), err)
		}
	}()
}

// countCompletion folds one finished match into the daily counters.
//
// Distinct players are people, not seats and not bots. An AI subject holds a
// seat and has a perfectly good subject key, and counting it would make "how
// many people played today" a number that goes up when nobody does — a table
// of one human against three bots would read as four. Guests are counted:
// they are people, they just have no account yet.
func (r *Recorder) countCompletion(m MatchResult) {
	r.metrics.Add(metrics.MatchesCompleted, 1)
	r.metrics.Add(metrics.MatchesCompletedFor(m.ModuleID), 1)
	for _, p := range m.Participants {
		if p.Subject.Kind == SubjectAI {
			continue
		}
		if key := p.Subject.Key(); key != "" {
			r.metrics.SeePlayer(key)
		}
	}
}
