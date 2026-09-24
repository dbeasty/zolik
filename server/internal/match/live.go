package match

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// The state of every match in play, held in memory.
//
// Nothing stores a match's state on every move any more: a move stores itself
// (Repository.AppendMove) and the state is snapshotted now and then. So the
// state lives here, and a match not held here is rebuilt on first use from
// its newest snapshot and the moves after it. Apply is deterministic, which
// is what makes the rebuild exact.
//
// Every read and every write of a match goes through its entry's lock. That is
// what keeps a move and an envelope write — a suspend, a reap, a seat — from
// interleaving: the envelope no longer carries the state, so the version check
// alone would not stop a move being accepted into a match that was just
// abandoned.

// liveMatch is one match as the runtime holds it.
type liveMatch struct {
	mu sync.Mutex
	// loaded is false until the first lock fills the entry, and again when a
	// write found the store ahead of it.
	loaded bool
	// match is the envelope as stored, with State the board after seq moves.
	match models.Match
	seq   int
	// rounds is how many rounds the board has closed, so a move can tell
	// whether it closed another.
	rounds int
	// touched is when the stored envelope was last written, to keep UpdatedAt
	// within a minute of the last move without a write per move.
	touched time.Time
	// used is when the entry was last locked, for eviction.
	used time.Time
	// stateErr is why the board could not be rebuilt, if it could not. The
	// envelope is still held, so a match whose board cannot be rebuilt can
	// still be deleted, swept or listed; only a move needs the board, and the
	// next lock tries again.
	stateErr error
}

// liveMatches is the Manager's table of entries.
type liveMatches struct {
	mu   sync.Mutex
	byID map[string]*liveMatch
}

func (t *liveMatches) entry(id string) *liveMatch {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.byID == nil {
		t.byID = map[string]*liveMatch{}
	}
	e, ok := t.byID[id]
	if !ok {
		e = &liveMatch{}
		t.byID[id] = e
	}
	return e
}

// peek returns the entry for a match if this process is holding one, without
// creating it. Used to ask whether anything is going on with a match here,
// which entry would otherwise answer by making it true.
func (t *liveMatches) peek(id string) *liveMatch {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.byID[id]
}

func (t *liveMatches) holds(id string, e *liveMatch) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.byID[id] == e
}

// forget drops an entry, so the next use reloads it — after a delete, or a
// write this process did not make.
func (t *liveMatches) forget(id string) {
	t.mu.Lock()
	delete(t.byID, id)
	t.mu.Unlock()
}

// liveIdle is how long an entry nobody has used stays in memory. Reloading is
// a snapshot and at most snapshotEvery moves, so this only has to cover the
// pauses of a game in progress.
const liveIdle = 10 * time.Minute

// evictIdle drops entries unused for liveIdle, and finished ones sooner.
func (t *liveMatches) evictIdle(now time.Time) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := 0
	for id, e := range t.byID {
		if !e.mu.TryLock() {
			continue // in use
		}
		idle := now.Sub(e.used)
		over := e.match.Status != "active" && e.match.Status != "suspended" && e.match.Status != "lobby"
		if idle > liveIdle || (over && idle > time.Minute) {
			delete(t.byID, id)
			n++
		}
		e.mu.Unlock()
	}
	return n
}

// matchID turns an id or a join code into the match's id.
func (m *Manager) matchID(ctx context.Context, idOrCode string) (bson.ObjectID, error) {
	if oid, err := bson.ObjectIDFromHex(idOrCode); err == nil {
		return oid, nil
	}
	env, err := m.repo.FindByJoinCode(ctx, idOrCode)
	if err != nil {
		return bson.ObjectID{}, err
	}
	return env.ID, nil
}

// lockMatch returns the match, loaded if it was not held, with its entry
// locked. The caller unlocks it.
func (m *Manager) lockMatch(ctx context.Context, idOrCode string) (*liveMatch, error) {
	id, err := m.matchID(ctx, idOrCode)
	if err != nil {
		return nil, err
	}
	var e *liveMatch
	for {
		e = m.live.entry(id.Hex())
		e.mu.Lock()
		// Evicted between the lookup and the lock: take the entry the table
		// holds now, so a match never has two.
		if m.live.holds(id.Hex(), e) {
			break
		}
		e.mu.Unlock()
	}
	e.used = time.Now()
	if e.loaded && m.hub != nil && m.hub.RedisEnabled() {
		// Another instance may have written this match. Its moves do not
		// change the envelope, so the check is the next move's key.
		if more, err := m.repo.Moves(ctx, id, e.seq, e.seq+1); err != nil || len(more) > 0 {
			e.loaded = false
		} else if env, err := m.repo.FindByID(ctx, id); err != nil || env.Version != e.match.Version {
			e.loaded = false
		}
	}
	if !e.loaded || e.stateErr != nil {
		if err := m.load(ctx, e, id); err != nil {
			e.mu.Unlock()
			if db.IsNotFound(err) {
				m.live.forget(id.Hex())
			}
			return nil, err
		}
	}
	return e, nil
}

// current is the match as held now, state and all, for readers.
func (m *Manager) current(ctx context.Context, idOrCode string) (models.Match, error) {
	e, err := m.lockMatch(ctx, idOrCode)
	if err != nil {
		return models.Match{}, err
	}
	defer e.mu.Unlock()
	return e.match, nil
}

// Current is the match with its state, for handlers that show a board.
func (m *Manager) Current(ctx context.Context, idOrCode string) (models.Match, error) {
	return m.current(ctx, idOrCode)
}

// load fills e from the store: the envelope, then the newest snapshot it lists
// and every move after it.
func (m *Manager) load(ctx context.Context, e *liveMatch, id bson.ObjectID) error {
	env, err := m.repo.FindByID(ctx, id)
	if err != nil {
		e.loaded = false
		return err
	}
	e.match, e.seq, e.rounds, e.stateErr = env, 0, 0, nil
	e.touched = env.UpdatedAt
	e.loaded = true
	if env.Status == "lobby" {
		return nil
	}
	state, seq, err := m.rebuild(ctx, env)
	if err != nil {
		e.stateErr = err
		return nil
	}
	e.match.State = models.JSONDoc(state)
	e.seq = seq
	if r := module.RoundsFor(m.registry.Get(env.ModuleID), state); r != nil {
		e.rounds = len(r.Rounds)
	}
	return nil
}

// rebuild is the board of a stored match: its newest snapshot with every move
// after it applied, and how many moves that is.
func (m *Manager) rebuild(ctx context.Context, env models.Match) (module.State, int, error) {
	mod := m.registry.Get(env.ModuleID)
	if mod == nil {
		return nil, 0, module.Error{Code: "UNKNOWN_MODULE", Message: env.ModuleID}
	}
	from, ok := latestSnapshot(env)
	var state module.State
	if ok {
		snap, err := m.repo.Snapshot(ctx, env.ID, from)
		if err != nil {
			return nil, 0, fmt.Errorf("match %s: snapshot %d: %w", env.ID.Hex(), from, err)
		}
		state = module.State(snap)
	} else {
		// A started match always has its deal as snapshot 0; this is a match
		// from before snapshots existed that the migration did not reach.
		// Its deal is reproducible — Join and Seat refuse once it leaves the
		// lobby, and Seed is written once.
		var err error
		state, err = mod.NewMatch(module.MatchConfig{Variation: env.Variation, Options: env.Options},
			playerRefs(env.Players), env.Seed)
		if err != nil {
			return nil, 0, err
		}
	}
	moves, err := m.repo.Moves(ctx, env.ID, from, -1)
	if err != nil {
		return nil, 0, err
	}
	for _, mv := range moves {
		var a module.Action
		if err := json.Unmarshal(mv.Action, &a); err != nil {
			return nil, 0, fmt.Errorf("match %s: move %d: %w", env.ID.Hex(), mv.Seq, err)
		}
		next, _, err := mod.Apply(state, mv.PlayerID, a)
		if err != nil {
			return nil, 0, fmt.Errorf("match %s: move %d no longer applies: %w", env.ID.Hex(), mv.Seq, err)
		}
		state = next
	}
	return state, from + len(moves), nil
}

// saveLocked writes next as e's envelope under the version check and adopts
// it, keeping the board e holds. e must be locked.
func (m *Manager) saveLocked(ctx context.Context, e *liveMatch, next models.Match) error {
	expected := e.match.Version
	if err := m.repo.UpdateWithVersion(ctx, e.match.ID, expected, next); err != nil {
		if errors.Is(err, ErrVersionConflict) {
			e.loaded = false
		}
		return err
	}
	now := time.Now().UTC()
	next.ID, next.Version, next.UpdatedAt = e.match.ID, expected+1, now
	next.State = e.match.State
	e.match, e.touched = next, now
	return nil
}

// snapshotLocked stores the board e holds as a snapshot at its seq, with next
// as the envelope, and adopts it. e must be locked.
func (m *Manager) snapshotLocked(ctx context.Context, e *liveMatch, next models.Match, state module.State) error {
	expected := e.match.Version
	next.Snapshots = withSnapshot(next.Snapshots, e.seq)
	if err := m.repo.CommitSnapshot(ctx, e.match.ID, expected, next, nil, e.seq, models.JSONDoc(state)); err != nil {
		if errors.Is(err, ErrVersionConflict) {
			e.loaded = false
		}
		return err
	}
	now := time.Now().UTC()
	next.ID, next.Version, next.UpdatedAt = e.match.ID, expected+1, now
	next.State = models.JSONDoc(state)
	e.match, e.touched = next, now
	return nil
}

// withSnapshot lists seq as the newest snapshot, once.
func withSnapshot(snaps []int, seq int) []int {
	if n := len(snaps); n > 0 && snaps[n-1] == seq {
		return snaps
	}
	return append(append([]int(nil), snaps...), seq)
}

// SeedState replaces a match's board outright, for the dev-only debug-state
// endpoint. The board is stored as a snapshot at the current move, so every
// rebuild starts from it — a board written any other way would be lost on the
// next restart, because the moves before it do not lead there.
func (m *Manager) SeedState(ctx context.Context, idOrCode string, state module.State, status string) (models.Match, error) {
	e, err := m.lockMatch(ctx, idOrCode)
	if err != nil {
		return models.Match{}, err
	}
	defer e.mu.Unlock()
	next := e.match
	if status != "" {
		next.Status = status
	}
	if err := m.snapshotLocked(ctx, e, next, state); err != nil {
		return models.Match{}, err
	}
	e.rounds = 0
	if r := module.RoundsFor(m.registry.Get(next.ModuleID), state); r != nil {
		e.rounds = len(r.Rounds)
	}
	return e.match, nil
}
