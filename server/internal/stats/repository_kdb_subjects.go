package stats

import (
	"log"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db"
)

// The subject index answers "which matches did this account or guest play?"
// without reading the matches. Mongo answers it with the compound
// {subjectKeys, completedAt} index; the KDB backend has no secondary indexes,
// so it keeps the equivalent itself: one document per subject in
// match_subjects, keyed by the subject key, listing that subject's results.
//
// It exists because the question is asked on the hot path. Every guest
// sign-in asks how many matches the device has ("N games on this device —
// sign in to keep them"), and answering by scanning match_results decodes
// every row ever recorded: a profile at 800 concurrent bot tables put a
// quarter of all CPU there, and the cost grows with every game anyone has
// finished.
//
// Only accounts and guests are indexed. A bot persona sits in a large share of
// all matches, so its document would grow without bound and be rewritten by
// every finished game; nothing asks for a bot's history on a request path, so
// those keys keep the scan.
//
// The index is derived data and never the authority. A volume written before
// it existed has match_results rows and no index, so it is built lazily, once,
// by the first query that needs it — no offline migration. "Built" is the
// marker document below; anything that makes the index untrustworthy (a failed
// update) removes the marker, and the next query rebuilds from the results.

const subjectIndexBuiltKey = "meta:built"

type subjectIndexEntry struct {
	MatchID     bson.ObjectID `bson:"m"`
	RecordID    bson.ObjectID `bson:"i"`
	CompletedAt time.Time     `bson:"t"`
}

type subjectIndexDoc struct {
	Subject string              `bson:"subject"`
	Matches []subjectIndexEntry `bson:"matches"`
}

type subjectIndexMarker struct {
	BuiltAt time.Time `bson:"builtAt"`
}

func indexedSubject(key string) bool {
	if rest, ok := strings.CutPrefix(key, "user:"); ok {
		return rest != ""
	}
	rest, ok := strings.CutPrefix(key, "guest:")
	return ok && rest != ""
}

func entryOf(m MatchResult) subjectIndexEntry {
	return subjectIndexEntry{MatchID: m.MatchID, RecordID: m.ID, CompletedAt: m.CompletedAt}
}

// indexMatchForSubjects makes the index agree with a record: every indexed
// subject in m.SubjectKeys lists it, and every subject in previous (the keys
// the record carried before a re-attribution) that no longer does has it
// removed. Failure is logged rather than returned, because the record is the
// one document that must not be lost; it invalidates the index instead, so the
// next query rebuilds it rather than trusting a list known to be short.
func (r *kdbRepository) indexMatchForSubjects(m MatchResult, previous []string) {
	r.idxMu.RLock()
	defer r.idxMu.RUnlock()

	current := map[string]bool{}
	for _, key := range m.SubjectKeys {
		current[key] = true
	}
	var drop []string
	for _, key := range previous {
		if !current[key] && indexedSubject(key) {
			drop = append(drop, key)
		}
	}
	err := r.k.Update(db.NSMatchSubjects, func(tx *db.Tx) error {
		for _, key := range drop {
			if err := editSubjectDoc(tx, key, func(d *subjectIndexDoc) {
				kept := d.Matches[:0]
				for _, e := range d.Matches {
					if e.MatchID != m.MatchID {
						kept = append(kept, e)
					}
				}
				d.Matches = kept
			}); err != nil {
				return err
			}
		}
		for key := range current {
			if !indexedSubject(key) {
				continue
			}
			if err := editSubjectDoc(tx, key, func(d *subjectIndexDoc) {
				for _, e := range d.Matches {
					if e.MatchID == m.MatchID {
						return
					}
				}
				d.Matches = append(d.Matches, entryOf(m))
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("stats: updating the subject index for match %s: %v", m.MatchID.Hex(), err)
		r.invalidateSubjectIndex()
	}
}

// editSubjectDoc reads a subject's document (an absent one is empty), applies
// edit and writes it back, deleting it when nothing is left.
func editSubjectDoc(tx *db.Tx, key string, edit func(*subjectIndexDoc)) error {
	d := subjectIndexDoc{Subject: key}
	raw, err := tx.Get(key)
	switch {
	case err == nil:
		if err := db.UnmarshalDoc(raw, &d); err != nil {
			return err
		}
	case !db.IsNotFound(err):
		return err
	}
	edit(&d)
	if len(d.Matches) == 0 {
		_, err := tx.Delete(key)
		return err
	}
	raw, err = db.MarshalDoc(d)
	if err != nil {
		return err
	}
	return tx.Put(key, raw)
}

func (r *kdbRepository) invalidateSubjectIndex() {
	r.idxBuilt.Store(false)
	if _, err := r.k.Delete(db.NSMatchSubjects, subjectIndexBuiltKey); err != nil {
		log.Printf("stats: clearing the subject index marker: %v", err)
	}
}

// ensureSubjectIndex builds the index if this volume has never had one.
//
// It holds the index lock exclusively while it works, which holds off
// recording and re-attribution for the length of one scan, once. That is what
// makes the build safe without any bookkeeping: a mutation either finished
// before the scan (and is in it) or runs after the write (and applies on top).
func (r *kdbRepository) ensureSubjectIndex() error {
	if r.idxBuilt.Load() {
		return nil
	}
	r.idxMu.Lock()
	defer r.idxMu.Unlock()
	if r.idxBuilt.Load() {
		return nil
	}
	if _, err := r.k.Get(db.NSMatchSubjects, subjectIndexBuiltKey); err == nil {
		r.idxBuilt.Store(true)
		return nil
	} else if !db.IsNotFound(err) {
		return err
	}

	bySubject := map[string][]subjectIndexEntry{}
	err := r.k.Scan(db.NSMatchResults, func(doc []byte) error {
		var m MatchResult
		if err := db.UnmarshalDoc(doc, &m); err != nil {
			return err
		}
		seen := map[string]bool{}
		for _, key := range m.SubjectKeys {
			if indexedSubject(key) && !seen[key] {
				seen[key] = true
				bySubject[key] = append(bySubject[key], entryOf(m))
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Documents left by an earlier index that no result now supports.
	var stale []string
	err = r.k.Scan(db.NSMatchSubjects, func(doc []byte) error {
		var d subjectIndexDoc
		if err := db.UnmarshalDoc(doc, &d); err != nil {
			return err
		}
		if d.Subject != "" {
			if _, ok := bySubject[d.Subject]; !ok {
				stale = append(stale, d.Subject)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	const batch = 200
	keys := make([]string, 0, len(bySubject))
	for key := range bySubject {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for start := 0; start < len(keys); start += batch {
		end := min(start+batch, len(keys))
		err := r.k.Update(db.NSMatchSubjects, func(tx *db.Tx) error {
			for _, key := range keys[start:end] {
				doc, err := db.MarshalDoc(subjectIndexDoc{Subject: key, Matches: bySubject[key]})
				if err != nil {
					return err
				}
				if err := tx.Put(key, doc); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	for _, key := range stale {
		if _, err := r.k.Delete(db.NSMatchSubjects, key); err != nil {
			return err
		}
	}
	marker, err := db.MarshalDoc(subjectIndexMarker{BuiltAt: time.Now().UTC()})
	if err != nil {
		return err
	}
	if err := r.k.Put(db.NSMatchSubjects, subjectIndexBuiltKey, marker); err != nil {
		return err
	}
	r.idxBuilt.Store(true)
	log.Printf("stats: built the subject index (%d subjects)", len(keys))
	return nil
}

// subjectEntries returns the index entries for a subject, and whether the
// index covers that subject at all. A subject with no document has played
// nothing: that is an answer, not a miss, because the index is complete once
// built.
func (r *kdbRepository) subjectEntries(key string) ([]subjectIndexEntry, bool, error) {
	if !indexedSubject(key) {
		return nil, false, nil
	}
	if err := r.ensureSubjectIndex(); err != nil {
		return nil, false, err
	}
	raw, err := r.k.Get(db.NSMatchSubjects, key)
	if db.IsNotFound(err) {
		return nil, true, nil
	}
	if err != nil {
		return nil, false, err
	}
	var d subjectIndexDoc
	if err := db.UnmarshalDoc(raw, &d); err != nil {
		return nil, false, err
	}
	return d.Matches, true, nil
}

// loadRecord fetches the result an index entry names. Results are keyed by
// their match; older ones still sit under their own id. A result that is gone
// is skipped rather than failing the whole list.
func (r *kdbRepository) loadRecord(e subjectIndexEntry) (MatchResult, bool, error) {
	raw, err := r.k.Get(db.NSMatchResults, resultKey(e.MatchID))
	if db.IsNotFound(err) {
		raw, err = r.k.Get(db.NSMatchResults, e.RecordID.Hex())
	}
	if db.IsNotFound(err) {
		return MatchResult{}, false, nil
	}
	if err != nil {
		return MatchResult{}, false, err
	}
	var m MatchResult
	if err := db.UnmarshalDoc(raw, &m); err != nil {
		return MatchResult{}, false, err
	}
	return m, true, nil
}

func (r *kdbRepository) loadSubjectRecords(entries []subjectIndexEntry, want int) ([]MatchResult, error) {
	out := make([]MatchResult, 0, min(want, len(entries)))
	for _, e := range entries {
		if len(out) >= want {
			break
		}
		m, ok, err := r.loadRecord(e)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, m)
		}
	}
	return out, nil
}

// listIndexed pages a subject's history newest first from the index, reading
// only the records of the page itself.
func (r *kdbRepository) listIndexed(entries []subjectIndexEntry, before time.Time, limit int) ([]MatchResult, error) {
	filtered := make([]subjectIndexEntry, 0, len(entries))
	for _, e := range entries {
		if before.IsZero() || e.CompletedAt.Before(before) {
			filtered = append(filtered, e)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if !filtered[i].CompletedAt.Equal(filtered[j].CompletedAt) {
			return filtered[i].CompletedAt.After(filtered[j].CompletedAt)
		}
		return filtered[i].RecordID.Hex() > filtered[j].RecordID.Hex()
	})
	return r.loadSubjectRecords(filtered, limit)
}
