package match

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
)

// kdbRepository gives every match a namespace of its own — db.MatchNS(hex) —
// holding that match and nothing else: its envelope under "match", each move
// under "m/<seq>", each board snapshot under "s/<seq>".
//
// A namespace is what KDB replicates and what it authorizes, so this is not a
// tidier filing scheme; it is the only shape in which "sync this match to the
// phones of the four people playing it, and nothing else of mine" is a thing a
// peer can be asked for. The match's namespace is therefore the authority:
// every read answers from it, and no write has happened until it has landed
// there.
//
// db.NSMatches stays, as a node-local index — one copy of each envelope, for
// the questions that cannot be put to a single match's namespace. Find the
// table with this join code, list the games this player is sitting at, sweep
// the ones that have been suspended too long: those are scans, and a scan
// needs somewhere to scan. The index is this node's own listing of the matches
// it holds; it is never replicated and no peer ever sees it, while the match
// namespaces are and do. Every write that changes an envelope writes the
// match's copy and the index's copy in one db.UpdateMulti — a cross-namespace
// transaction the engine commits atomically — so the index cannot drift out of
// step with the match it indexes, or outlive it.
//
// The version check UpdateWithVersion promises is enforced inside the
// namespaces' critical section — the engine itself has no conditional replace,
// and does not need one when every writer in the (single) process goes through
// the same locks, and a peer's merge landing mid-section is caught by the read
// preconditions db.UpdateMulti carries.
type kdbRepository struct {
	k *db.KDB
}

func NewKDBRepository(k *db.KDB) Repository {
	return &kdbRepository{k: k}
}

var _ Repository = (*kdbRepository)(nil)

// errStopScan aborts a scan that found what it wanted.
var errStopScan = errors.New("stop scan")

// Reading a store the migration has not finished with.
//
// Before matches had namespaces of their own, a match's envelope lived only in
// db.NSMatches and its moves and snapshots only in db.NSMatchLog, keyed
// "<hex>/m/<seq>". cmd/migrate-namespaces moves both, offline; until it has
// run — or while it is part way through a store — every read here tries the
// match's own namespace first and falls back to the old place, one key at a
// time. Per key, rather than per match, because a match can genuinely be half
// in each: the server keeps playing a match it has not migrated, and the moves
// it writes from now on go to the namespace while the older ones are still in
// the log.
//
// This is the whole of the fallback, and all of it goes together — with
// logKey, and with db.NSMatchLog itself — once no store has a match_log record
// left.

// record reads one stored move or snapshot: from the match's own namespace, or
// from the old shared log for a match the migration has not reached.
func (r *kdbRepository) record(id bson.ObjectID, kind string, seq int) ([]byte, error) {
	raw, err := r.k.Get(db.MatchNS(id.Hex()), recordKey(kind, seq))
	if !db.IsNotFound(err) {
		return raw, err
	}
	return r.k.Get(db.NSMatchLog, logKey(id, kind, seq))
}

// recordLocked is record inside a transaction, so that what it read is what
// the write that follows requires to still be true.
func recordLocked(tx *db.MultiTx, id bson.ObjectID, kind string, seq int) ([]byte, error) {
	raw, err := tx.Get(db.MatchNS(id.Hex()), recordKey(kind, seq))
	if !db.IsNotFound(err) {
		return raw, err
	}
	return tx.Get(db.NSMatchLog, logKey(id, kind, seq))
}

// deleteRecord removes a move or a snapshot from wherever it is stored.
// Deleting a key that is not there is a no-op, so both places are named rather
// than asked about first.
func deleteRecord(tx *db.MultiTx, id bson.ObjectID, kind string, seq int) {
	tx.Delete(db.MatchNS(id.Hex()), recordKey(kind, seq))
	tx.Delete(db.NSMatchLog, logKey(id, kind, seq))
}

// putEnvelope writes the match's envelope and the index's copy of it. Always
// both, always in the one transaction: an index that can name a version of a
// match the match itself does not have is worse than no index at all, because
// every scan in this file would believe it.
func putEnvelope(tx *db.MultiTx, id bson.ObjectID, doc []byte) {
	tx.Put(db.MatchNS(id.Hex()), keyMatchEnvelope, doc)
	tx.Put(db.NSMatches, id.Hex(), doc)
}

// envelopeNamespaces is what a write to a match's envelope holds: the match's
// own namespace and the index.
func envelopeNamespaces(id bson.ObjectID) []string {
	return []string{db.MatchNS(id.Hex()), db.NSMatches}
}

// logNamespaces is envelopeNamespaces plus the old shared log, for the writes
// that also read or delete moves — which, until the migration has run, may
// still be in it.
func logNamespaces(id bson.ObjectID) []string {
	return append(envelopeNamespaces(id), db.NSMatchLog)
}

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
	hex := m.ID.Hex()
	err = r.k.UpdateMulti(envelopeNamespaces(m.ID), func(tx *db.MultiTx) error {
		// The index is what says whether this node holds the match at all, so
		// it is where a duplicate id is caught. Reading it here is also what
		// makes the write below assert it was still absent, which is the only
		// thing that can refuse an id a peer created between the two.
		if _, err := tx.Get(db.NSMatches, hex); err == nil {
			return fmt.Errorf("kdb: %s %q: %w", db.NSMatches, hex, db.ErrDuplicateKey)
		} else if !db.IsNotFound(err) {
			return err
		}
		putEnvelope(tx, m.ID, doc)
		return nil
	})
	if err != nil {
		return models.Match{}, err
	}
	return m, nil
}

func (r *kdbRepository) FindByID(ctx context.Context, id bson.ObjectID) (models.Match, error) {
	hex := id.Hex()
	// The index is consulted first, and not because it is the authority — the
	// match's own namespace is — but because namespaces open on demand, and
	// opening one creates it. The id in this call came off a URL, so an id that
	// names no match at all has to be refused without KDB ever being asked for
	// its namespace; otherwise a mistyped link would leave an empty namespace,
	// and a runtime holding it, behind for good. Every write puts the envelope
	// in both places in one transaction, so a match this node holds is listed
	// here without exception.
	indexed, err := r.k.Get(db.NSMatches, hex)
	if err != nil {
		return models.Match{}, err
	}
	raw, err := r.k.Get(db.MatchNS(hex), keyMatchEnvelope)
	if db.IsNotFound(err) {
		raw = indexed
	} else if err != nil {
		return models.Match{}, err
	}
	var m models.Match
	if err := db.UnmarshalDoc(raw, &m); err != nil {
		return models.Match{}, err
	}
	return m, nil
}

// FindByJoinCode scans the index: a join code is a question about every match
// this node holds, which is precisely what no single match's namespace can
// answer.
func (r *kdbRepository) FindByJoinCode(ctx context.Context, code string) (models.Match, error) {
	var out models.Match
	found := false
	err := r.k.Scan(db.NSMatches, func(doc []byte) error {
		var m models.Match
		if err := db.UnmarshalDoc(doc, &m); err != nil {
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

func (r *kdbRepository) UpdateWithVersion(ctx context.Context, id bson.ObjectID, expected int64, next models.Match) error {
	next.Version = expected + 1
	next.ID = id
	next.UpdatedAt = time.Now().UTC()
	doc, err := db.MarshalDoc(next)
	if err != nil {
		return err
	}
	return r.k.UpdateMulti(envelopeNamespaces(id), func(tx *db.MultiTx) error {
		if _, err := envelopeLocked(tx, id, expected); err != nil {
			return err
		}
		putEnvelope(tx, id, doc)
		return nil
	})
}

// FindRetired scans the index for resolved matches past their retention
// window.
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
		var m models.Match
		if err := db.UnmarshalDoc(raw, &m); err != nil {
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

// DeleteIfUnchanged removes a match inside the namespaces' critical section,
// which is where every other conditional write in this repository is enforced —
// the engine has no conditional delete, and does not need one while one node
// writes a given match.
func (r *kdbRepository) DeleteIfUnchanged(ctx context.Context, id bson.ObjectID, expected int64) error {
	return r.k.UpdateMulti(logNamespaces(id), func(tx *db.MultiTx) error {
		cur, err := envelopeLocked(tx, id, expected)
		if err != nil {
			return err
		}
		// Every move, found by counting on from the newest snapshot, every
		// snapshot, the envelope and the index's copy of it — one transaction,
		// so a match is never left half deleted. What is not deleted is the
		// match's namespace itself: an emptied namespace is what a node that
		// has stopped following a match is left holding either way.
		from, _ := latestSnapshot(cur)
		head := from
		for {
			if _, err := recordLocked(tx, id, kindMove, head+1); err != nil {
				if db.IsNotFound(err) {
					break
				}
				return err
			}
			head++
		}
		for seq := 1; seq <= head; seq++ {
			deleteRecord(tx, id, kindMove, seq)
		}
		for _, seq := range cur.Snapshots {
			deleteRecord(tx, id, kindSnapshot, seq)
		}
		tx.Delete(db.MatchNS(id.Hex()), keyMatchEnvelope)
		tx.Delete(db.NSMatches, id.Hex())
		return nil
	})
}

// envelopeLocked reads the stored match inside a transaction and checks it is
// still at version expected. A missing match is a conflict too: the same
// "someone else won" Mongo's filtered writes answer.
func envelopeLocked(tx *db.MultiTx, id bson.ObjectID, expected int64) (models.Match, error) {
	raw, err := tx.Get(db.MatchNS(id.Hex()), keyMatchEnvelope)
	if db.IsNotFound(err) {
		// Not migrated yet: the only envelope is the index's. Reading it here
		// is what lets the write that follows put a copy in the match's own
		// namespace, so an old match migrates itself the first time it is
		// played.
		raw, err = tx.Get(db.NSMatches, id.Hex())
	}
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

// AppendMove is one insert into the match's own namespace: the key names the
// seq, so a second move N finds the first one there and is refused.
func (r *kdbRepository) AppendMove(ctx context.Context, id bson.ObjectID, move models.MatchAction) error {
	doc, err := db.MarshalDoc(toMoveRecord(id, move))
	if err != nil {
		return err
	}
	// One lookup in the old shared log first, because "already stored" has two
	// places to be true in while a store is part way migrated. A match whose
	// history is still in the log would otherwise take a second move N into its
	// namespace, where it would shadow — not replace — the one already logged,
	// and the match would have two different moves at the same seq with only
	// the newer one readable. The lookup costs a miss on an empty namespace
	// once cmd/migrate-namespaces has run, and goes when the fallback does.
	if _, err := r.k.Get(db.NSMatchLog, logKey(id, kindMove, move.Seq)); err == nil {
		return ErrVersionConflict
	} else if !db.IsNotFound(err) {
		return err
	}
	if err := r.k.Insert(db.MatchNS(id.Hex()), recordKey(kindMove, move.Seq), doc); err != nil {
		if errors.Is(err, db.ErrDuplicateKey) {
			return ErrVersionConflict
		}
		return err
	}
	return nil
}

// CommitSnapshot is one transaction across the match's namespace and the
// index: the moves, the snapshot, the envelope that lists it and the index's
// copy of that envelope land together or not at all.
func (r *kdbRepository) CommitSnapshot(ctx context.Context, id bson.ObjectID, expected int64, next models.Match,
	moves []models.MatchAction, seq int, state models.JSONDoc) error {
	return r.k.UpdateMulti(logNamespaces(id), func(tx *db.MultiTx) error {
		if _, err := envelopeLocked(tx, id, expected); err != nil {
			return err
		}
		for _, mv := range moves {
			if _, err := recordLocked(tx, id, kindMove, mv.Seq); err == nil {
				return ErrVersionConflict
			} else if !db.IsNotFound(err) {
				return err
			}
			doc, err := db.MarshalDoc(toMoveRecord(id, mv))
			if err != nil {
				return err
			}
			tx.Put(db.MatchNS(id.Hex()), recordKey(kindMove, mv.Seq), doc)
		}
		doc, err := db.MarshalDoc(snapshotRecord{Match: id, Kind: kindSnapshot, Seq: seq, State: state})
		if err != nil {
			return err
		}
		tx.Put(db.MatchNS(id.Hex()), recordKey(kindSnapshot, seq), doc)

		next.ID, next.Version, next.UpdatedAt = id, expected+1, time.Now().UTC()
		env, err := db.MarshalDoc(next)
		if err != nil {
			return err
		}
		putEnvelope(tx, id, env)
		return nil
	})
}

// Moves reads by key, counting up from from+1: the store has no range scan,
// and does not need one when the keys are the seqs.
func (r *kdbRepository) Moves(ctx context.Context, id bson.ObjectID, from, to int) ([]models.MatchAction, error) {
	var out []models.MatchAction
	for seq := from + 1; to < 0 || seq <= to; seq++ {
		raw, err := r.record(id, kindMove, seq)
		if db.IsNotFound(err) {
			break
		}
		if err != nil {
			return nil, err
		}
		var rec moveRecord
		if err := db.UnmarshalDoc(raw, &rec); err != nil {
			return nil, err
		}
		out = append(out, rec.action())
	}
	return out, nil
}

func (r *kdbRepository) Snapshot(ctx context.Context, id bson.ObjectID, seq int) (models.JSONDoc, error) {
	raw, err := r.record(id, kindSnapshot, seq)
	if err != nil {
		return nil, err
	}
	var snap snapshotRecord
	if err := db.UnmarshalDoc(raw, &snap); err != nil {
		return nil, err
	}
	return snap.State, nil
}

// FindAbandonable scans the index for suspended matches past their deadline.
//
// A scan, like every other cross-match query this backend answers: KDB has no
// secondary index, and the alternative — keeping a separate namespace of
// deadlines — would be a second thing to keep in step with the match document
// for a sweep that runs twice a minute over a collection holding live games
// only.
func (r *kdbRepository) FindAbandonable(ctx context.Context, now time.Time, limit int) ([]models.Match, error) {
	var out []models.Match
	err := r.k.Scan(db.NSMatches, func(raw []byte) error {
		var m models.Match
		if err := db.UnmarshalDoc(raw, &m); err != nil {
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

// FindStranded scans the index for active matches untouched since idleBefore.
func (r *kdbRepository) FindStranded(ctx context.Context, idleBefore time.Time, limit int) ([]models.Match, error) {
	var out []models.Match
	err := r.k.Scan(db.NSMatches, func(raw []byte) error {
		var m models.Match
		if err := db.UnmarshalDoc(raw, &m); err != nil {
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

// FindForPlayer scans the index for matches a seat id sits at.
//
// A scan, like every other cross-match read on this backend: KDB has no
// secondary index, so "every match with this player" costs a full pass over
// the index regardless of how the caller phrases it — a cheap pass, now that a
// match document holds the envelope and not the board or the moves, and one
// that the per-match namespaces could not answer at all without opening every
// one of them.
func (r *kdbRepository) FindForPlayer(ctx context.Context, playerID string, f PlayerMatchFilter) ([]models.Match, error) {
	statuses, limit := resolvePlayerMatchFilter(f)
	want := make(map[string]bool, len(statuses))
	for _, s := range statuses {
		want[s] = true
	}
	var out []models.Match
	err := r.k.Scan(db.NSMatches, func(raw []byte) error {
		var m models.Match
		if err := db.UnmarshalDoc(raw, &m); err != nil {
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
