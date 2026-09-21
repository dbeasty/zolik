package match

import (
	"context"
	"errors"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
)

// kdbRepository stores each match as one KDB document keyed by its ObjectID
// hex. The version check UpdateWithVersion promises is enforced inside the
// namespace's critical section — the engine itself has no conditional
// replace, and does not need one when every writer in the (single) process
// goes through the same lock.
type kdbRepository struct {
	k *db.KDB
}

func NewKDBRepository(k *db.KDB) Repository {
	return &kdbRepository{k: k}
}

var _ Repository = (*kdbRepository)(nil)

// errStopScan aborts a scan that found what it wanted.
var errStopScan = errors.New("stop scan")

func (r *kdbRepository) Insert(ctx context.Context, m models.Match) (models.Match, error) {
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}
	m.Version = 1
	if m.ID.IsZero() {
		m.ID = bson.NewObjectID()
	}
	doc, err := db.MarshalDoc(m)
	if err != nil {
		return models.Match{}, err
	}
	if err := r.k.Insert(db.NSMatches, m.ID.Hex(), doc); err != nil {
		return models.Match{}, err
	}
	return m, nil
}

func (r *kdbRepository) FindByID(ctx context.Context, id bson.ObjectID) (models.Match, error) {
	doc, err := r.k.Get(db.NSMatches, id.Hex())
	if err != nil {
		return models.Match{}, err
	}
	return decodeMatch(doc)
}

func (r *kdbRepository) FindByJoinCode(ctx context.Context, code string) (models.Match, error) {
	var out models.Match
	found := false
	err := r.k.Scan(db.NSMatches, func(doc []byte) error {
		m, err := decodeMatch(doc)
		if err != nil {
			return err
		}
		if m.JoinCode == code {
			out, found = m, true
			return errStopScan
		}
		return nil
	})
	if err != nil && !errors.Is(err, errStopScan) {
		return models.Match{}, err
	}
	if !found {
		return models.Match{}, db.ErrNotFound
	}
	return out, nil
}

// Resolve accepts either an object id or a join code, so a URL can carry
// whichever the player has.
func (r *kdbRepository) Resolve(ctx context.Context, idOrCode string) (models.Match, error) {
	if oid, err := bson.ObjectIDFromHex(idOrCode); err == nil {
		return r.FindByID(ctx, oid)
	}
	return r.FindByJoinCode(ctx, idOrCode)
}

// decodeMatch reads a stored match into the shape callers get. See settle.
func decodeMatch(doc []byte) (models.Match, error) {
	var m models.Match
	if err := db.UnmarshalDoc(doc, &m); err != nil {
		return models.Match{}, err
	}
	return settle(m), nil
}

// storedLocked reads the match as stored, original-layout log and all, and
// checks it is still at version expected. A missing match is a conflict too:
// the same "someone else won" Mongo's filtered writes answer.
func storedLocked(tx *db.MultiTx, id bson.ObjectID, expected int64) (models.Match, error) {
	raw, err := tx.Get(db.NSMatches, id.Hex())
	if err != nil {
		if db.IsNotFound(err) {
			return models.Match{}, ErrVersionConflict
		}
		return models.Match{}, err
	}
	var cur models.Match
	if err := db.UnmarshalDoc(raw, &cur); err != nil {
		return models.Match{}, err
	}
	if cur.Version != expected {
		return models.Match{}, ErrVersionConflict
	}
	return cur, nil
}

// carryLog copies what the repository owns from the stored match onto next.
func carryLog(next *models.Match, cur models.Match) {
	next.ActionCount = cur.ActionCount
	next.LogFormat = cur.LogFormat
	next.LegacyActionLog = cur.LegacyActionLog
	next.Checkpoints = cur.Checkpoints
}

func (r *kdbRepository) put(tx *db.MultiTx, id bson.ObjectID, expected int64, next models.Match) error {
	next.Version = expected + 1
	next.ID = id
	next.UpdatedAt = time.Now().UTC()
	doc, err := db.MarshalDoc(next)
	if err != nil {
		return err
	}
	tx.Put(db.NSMatches, id.Hex(), doc)
	return nil
}

func (r *kdbRepository) UpdateWithVersion(ctx context.Context, id bson.ObjectID, expected int64, next models.Match) error {
	return r.k.UpdateMulti([]string{db.NSMatches}, func(tx *db.MultiTx) error {
		cur, err := storedLocked(tx, id, expected)
		if err != nil {
			return err
		}
		carryLog(&next, cur)
		return r.put(tx, id, expected, next)
	})
}

// AppendAction writes the action's page (and board) and then the match, in
// one transaction over both namespaces. The order matters only until that
// transaction is atomic: a page that lands without its match holds an entry
// past ActionCount, which readers never reach and the next append overwrites.
func (r *kdbRepository) AppendAction(ctx context.Context, id bson.ObjectID, expected int64, next models.Match,
	entry models.MatchAction, cp *models.MatchCheckpoint, board models.JSONDoc) error {
	return r.k.UpdateMulti([]string{db.NSMatches, db.NSMatchLog}, func(tx *db.MultiTx) error {
		cur, err := storedLocked(tx, id, expected)
		if err != nil {
			return err
		}
		if entry.Seq != actionsIn(cur)+1 {
			return ErrVersionConflict
		}
		carryLog(&next, cur)
		if cur.LogFormat != models.LogFormatPages && migrateLegacyOnAppend {
			if err := moveToPages(tx, id.Hex(), &next); err != nil {
				return err
			}
		}

		var checkpoint *models.MatchCheckpoint
		if cp != nil {
			c := *cp
			checkpoint = &c
		}
		if next.LogFormat == models.LogFormatPages {
			if err := appendPage(tx, id.Hex(), entry); err != nil {
				return err
			}
			next.ActionCount = entry.Seq
			if checkpoint != nil && len(board) > 0 {
				if err := putBoard(tx, id.Hex(), checkpoint.Seq, board); err != nil {
					return err
				}
				checkpoint.Board = true
			}
		} else {
			next.LegacyActionLog = append(append([]models.MatchAction(nil), next.LegacyActionLog...), entry)
			if checkpoint != nil && len(board) > 0 {
				checkpoint.LegacyState = board
			}
		}
		if checkpoint != nil {
			next.Checkpoints = append(append([]models.MatchCheckpoint(nil), next.Checkpoints...), *checkpoint)
		}
		return r.put(tx, id, expected, next)
	})
}

func appendPage(tx *db.MultiTx, matchHex string, entry models.MatchAction) error {
	n := pageOf(entry.Seq)
	p := logPage{Match: matchHex, Page: n}
	raw, err := tx.Get(db.NSMatchLog, pageKey(matchHex, n))
	switch {
	case err == nil:
		if err := db.UnmarshalDoc(raw, &p); err != nil {
			return err
		}
	case !db.IsNotFound(err):
		return err
	}
	return putPage(tx, appendToPage(p, entry))
}

func putPage(tx *db.MultiTx, p logPage) error {
	doc, err := db.MarshalDoc(p)
	if err != nil {
		return err
	}
	tx.Put(db.NSMatchLog, pageKey(p.Match, p.Page), doc)
	return nil
}

func putBoard(tx *db.MultiTx, matchHex string, seq int, state models.JSONDoc) error {
	doc, err := db.MarshalDoc(boardRecord{Match: matchHex, Seq: seq, State: state})
	if err != nil {
		return err
	}
	tx.Put(db.NSMatchLog, boardKey(matchHex, seq), doc)
	return nil
}

// moveToPages rewrites an original-layout match into pages: its inline log
// becomes pages and its inline boards board records, in the same transaction
// as the move that triggered it, so the match is never seen half-moved.
func moveToPages(tx *db.MultiTx, matchHex string, next *models.Match) error {
	for _, p := range pagesFor(matchHex, next.LegacyActionLog) {
		if err := putPage(tx, p); err != nil {
			return err
		}
	}
	cps := make([]models.MatchCheckpoint, len(next.Checkpoints))
	for i, c := range next.Checkpoints {
		if len(c.LegacyState) > 0 {
			if err := putBoard(tx, matchHex, c.Seq, c.LegacyState); err != nil {
				return err
			}
			c.Board = true
			c.LegacyState = nil
		}
		cps[i] = c
	}
	next.Checkpoints = cps
	next.ActionCount = len(next.LegacyActionLog)
	next.LegacyActionLog = nil
	next.LogFormat = models.LogFormatPages
	return nil
}

// stored reads a match as stored, without settling it.
func (r *kdbRepository) stored(id bson.ObjectID) (models.Match, error) {
	raw, err := r.k.Get(db.NSMatches, id.Hex())
	if err != nil {
		return models.Match{}, err
	}
	var m models.Match
	return m, db.UnmarshalDoc(raw, &m)
}

func (r *kdbRepository) ActionLog(ctx context.Context, id bson.ObjectID, from, to int) ([]models.MatchAction, error) {
	m, err := r.stored(id)
	if err != nil {
		return nil, err
	}
	if count := actionsIn(m); to > count {
		to = count
	}
	if from < 0 {
		from = 0
	}
	if m.LogFormat != models.LogFormatPages {
		return sliceLog(m.LegacyActionLog, from, to), nil
	}
	var out []models.MatchAction
	for seq := from + 1; seq <= to; {
		raw, err := r.k.Get(db.NSMatchLog, pageKey(id.Hex(), pageOf(seq)))
		if db.IsNotFound(err) {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		var p logPage
		if err := db.UnmarshalDoc(raw, &p); err != nil {
			return nil, err
		}
		for _, e := range p.Entries {
			if e.Seq < seq {
				continue
			}
			if e.Seq != seq || seq > to {
				break
			}
			out = append(out, fromPageEntry(e))
			seq++
		}
		if seq <= to && pageOf(seq) == p.Page {
			// The page ends before the log does.
			return out, nil
		}
	}
	return out, nil
}

func (r *kdbRepository) CheckpointBoard(ctx context.Context, id bson.ObjectID, seq int) (models.JSONDoc, error) {
	m, err := r.stored(id)
	if err != nil {
		return nil, err
	}
	for _, c := range m.Checkpoints {
		if c.Seq != seq {
			continue
		}
		if len(c.LegacyState) > 0 {
			return c.LegacyState, nil
		}
		if !c.Board {
			break
		}
		raw, err := r.k.Get(db.NSMatchLog, boardKey(id.Hex(), seq))
		if err != nil {
			return nil, err
		}
		var b boardRecord
		if err := db.UnmarshalDoc(raw, &b); err != nil {
			return nil, err
		}
		return b.State, nil
	}
	return nil, db.ErrNotFound
}

// FindRetired scans for resolved matches past their retention window.
//
// A scan rather than a query because the engine has no secondary indexes; the
// predicate is the same one the sweeper re-applies before deleting, and lives
// in retention.go so the two can never disagree about what "retired" means.
func (r *kdbRepository) FindRetired(ctx context.Context, now time.Time, w RetentionWindows, limit int) ([]models.Match, error) {
	if !w.Any() {
		return nil, nil
	}
	var out []models.Match
	err := r.k.Scan(db.NSMatches, func(raw []byte) error {
		m, err := decodeMatch(raw)
		if err != nil {
			return err
		}
		if !retired(m, now, w) {
			return nil
		}
		out = append(out, m)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// DeleteIfUnchanged removes a match inside the namespace's critical section,
// which is where every other conditional write in this repository is enforced —
// the engine has no conditional delete, and does not need one while kdb mode is
// single-process.
//
// The match's log goes with it, in the same transaction and before it: until
// that transaction is atomic, a delete cut short leaves the match behind, and
// the next sweep or host delete finds it and finishes the job.
func (r *kdbRepository) DeleteIfUnchanged(ctx context.Context, id bson.ObjectID, expected int64) error {
	return r.k.UpdateMulti([]string{db.NSMatches, db.NSMatchLog}, func(tx *db.MultiTx) error {
		cur, err := storedLocked(tx, id, expected)
		if err != nil {
			return err
		}
		if cur.LogFormat == models.LogFormatPages {
			// One page past the count: an append whose match write did not
			// land can have opened it.
			for p := 0; p <= pageOf(cur.ActionCount+1); p++ {
				tx.Delete(db.NSMatchLog, pageKey(id.Hex(), p))
			}
			for _, c := range cur.Checkpoints {
				if c.Board {
					tx.Delete(db.NSMatchLog, boardKey(id.Hex(), c.Seq))
				}
			}
		}
		tx.Delete(db.NSMatches, id.Hex())
		return nil
	})
}

// FindAbandonable scans for suspended matches past their deadline.
//
// A scan, like every other cross-document query this backend answers: KDB has
// no secondary index, and the alternative — keeping a separate namespace of
// deadlines — would be a second thing to keep in step with the match document
// for a sweep that runs twice a minute over a collection holding live games
// only.
func (r *kdbRepository) FindAbandonable(ctx context.Context, now time.Time, limit int) ([]models.Match, error) {
	var out []models.Match
	err := r.k.Scan(db.NSMatches, func(raw []byte) error {
		m, err := decodeMatch(raw)
		if err != nil {
			return err
		}
		if m.Status != "suspended" || m.AbandonAt == nil || m.AbandonAt.After(now) {
			return nil
		}
		out = append(out, m)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AbandonAt.Before(*out[j].AbandonAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// FindStranded scans for active matches untouched since idleBefore.
func (r *kdbRepository) FindStranded(ctx context.Context, idleBefore time.Time, limit int) ([]models.Match, error) {
	var out []models.Match
	err := r.k.Scan(db.NSMatches, func(raw []byte) error {
		m, err := decodeMatch(raw)
		if err != nil {
			return err
		}
		if m.Status != "active" || lastActivity(m).After(idleBefore) {
			return nil
		}
		out = append(out, m)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return lastActivity(out[i]).Before(lastActivity(out[j])) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// FindForPlayer scans for matches a seat id sits at.
//
// A scan, like every other cross-document read on this backend: KDB has no
// secondary index, so "every match with this player" costs a full pass over
// the namespace regardless of how the caller phrases it. State and ActionLog
// are dropped after decoding rather than never read — the engine has no
// server-side projection to skip them with — so this saves nothing over the
// wire that a real server doesn't have, but it does keep the returned rows
// the same shape Mongo hands back, which is what callers rely on.
func (r *kdbRepository) FindForPlayer(ctx context.Context, playerID string, f PlayerMatchFilter) ([]models.Match, error) {
	statuses, limit := resolvePlayerMatchFilter(f)
	want := make(map[string]bool, len(statuses))
	for _, s := range statuses {
		want[s] = true
	}
	var out []models.Match
	err := r.k.Scan(db.NSMatches, func(raw []byte) error {
		m, err := decodeMatch(raw)
		if err != nil {
			return err
		}
		if !want[m.Status] {
			return nil
		}
		seated := false
		for _, p := range m.Players {
			if p.ID == playerID {
				seated = true
				break
			}
		}
		if !seated {
			return nil
		}
		m.State = nil
		// A list row needs none of the checkpoints.
		m.Checkpoints = nil
		out = append(out, m)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		ui, uj := out[i].UpdatedAt, out[j].UpdatedAt
		if !ui.Equal(uj) {
			return ui.After(uj)
		}
		return out[i].ID.Hex() > out[j].ID.Hex()
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// RetainsHistory reports whether this store still keeps the versions
// BoardAfter walks, and refuses with the engine's own remedy when it does not.
//
// Asked rather than assumed because history is a per-namespace *mode*, not a
// property of the engine: a KDB deployment migrated to history=none still
// answers DocumentVersions, and still would right up to the moment its
// retention window swept the commits out from under a replay somebody was
// halfway through. Offering the feature in that shape is worse than not
// offering it.
func (r *kdbRepository) RetainsHistory(ctx context.Context) error {
	return r.k.RetainsHistory(db.NSMatches, "stepping back through a game that has stopped")
}

// BoardAfter returns the match's board as it stood after a given number of
// accepted actions, read out of the store's own history.
//
// This is the one thing a fold cannot offer: the board as it *was*, rather
// than the board a module would produce from the same log today. It does not
// depend on Apply still being pure, it does not care that a module's rules
// have moved since, and it cannot be defeated by a state written outside the
// log. Where it answers, a replay frame is evidence rather than reconstruction.
//
// KDB only, and deliberately so — see the BoardSource capability in replay.go
// for how the runtime asks without knowing which engine it has.
//
// The search is over document *versions*, not actions, because they are not
// the same thing: suspending, resuming, seating and joining all write the
// match without touching the log. So the versions are binary-searched on how
// many actions each one records (actionsIn), which is monotonic by
// construction — the log is append-only, and the version that moved a match
// to pages recorded exactly as many as the one before it.
func (r *kdbRepository) BoardAfter(ctx context.Context, id bson.ObjectID, actions int) (models.Match, error) {
	commits, err := r.k.DocumentVersions(db.NSMatches, id.Hex())
	if err != nil {
		return models.Match{}, err
	}
	if len(commits) == 0 {
		return models.Match{}, db.ErrNotFound
	}

	at := func(i int) (models.Match, error) {
		doc, err := r.k.GetAt(db.NSMatches, id.Hex(), commits[i])
		if err != nil {
			return models.Match{}, err
		}
		var m models.Match
		return m, db.UnmarshalDoc(doc, &m)
	}

	// The newest version whose log is no longer than asked for. Everything
	// after it happened later than the frame being looked at.
	lo, hi, best := 0, len(commits)-1, -1
	for lo <= hi {
		mid := (lo + hi) / 2
		m, err := at(mid)
		if err != nil {
			return models.Match{}, err
		}
		if actionsIn(m) <= actions {
			best = mid
			lo = mid + 1
			continue
		}
		hi = mid - 1
	}
	if best < 0 {
		return models.Match{}, db.ErrNotFound
	}

	m, err := at(best)
	if err != nil {
		return models.Match{}, err
	}
	if len(m.State) == 0 {
		// A version from before the match was dealt. There is a board later,
		// but not one this far back.
		return models.Match{}, db.ErrNotFound
	}
	return settle(m), nil
}
