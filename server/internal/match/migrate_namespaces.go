package match

import (
	"context"
	"fmt"
	"sort"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
)

// Moving matches into namespaces of their own.
//
// The store used to keep every match in one namespace (db.NSMatches, keyed by
// hex) and every move and snapshot in another (db.NSMatchLog, keyed
// "<hex>/m/<seq>"). A namespace is what KDB replicates and authorizes, so that
// layout can only be synced all-or-nothing: there is no way to hand a phone
// the four matches its owner is playing without handing it everyone else's.
// This migration gives each match the namespace it needs — its envelope under
// "match", its moves under "m/<seq>", its snapshots under "s/<seq>" — and
// leaves a copy of the envelope in db.NSMatches, which stops being the matches
// and becomes a node-local index of them (see repository_kdb.go).
//
// Each match moves in one cross-namespace transaction, so it is safe to
// interrupt and safe to run again: a match is either wholly moved or wholly
// where it was. It is also safe to run against a store the server has already
// half moved by playing on it — writing an envelope puts a copy in the match's
// namespace whether or not its moves have been migrated — which is why the
// envelope and the records are considered separately below rather than the
// first being taken as proof of the second.
//
// Nothing here deletes a match. The only deletes are of records that have just
// been written to their new home in the same transaction.

// NamespaceMigrationReport counts what the move did.
type NamespaceMigrationReport struct {
	// Matches is how many matches had something to move.
	Matches int
	// Records is how many move and snapshot records moved with them.
	Records int
	// Skipped is how many matches were already wholly in their own namespace.
	Skipped int
	// Orphans is how many records in the old log name a match this store does
	// not have. They are counted and left alone: a match that was deleted took
	// its records with it, so these are either debris from a much older bug or
	// something worth looking at before it is thrown away.
	Orphans int
	// Verified is how many matches were read back out of their own namespace,
	// record by record, and found whole.
	Verified int
}

func (r NamespaceMigrationReport) String() string {
	return fmt.Sprintf("%d matches moved into their own namespace (%d records), "+
		"%d already there, %d verified, %d orphaned records left in the old log",
		r.Matches, r.Records, r.Skipped, r.Verified, r.Orphans)
}

// recordRef names one stored move or snapshot in the old shared log.
type recordRef struct {
	kind string
	seq  int
}

// logProbe reads the three fields of a stored record that say what it is: the
// match it belongs to, whether it is a move or a snapshot, and its seq. The
// bodies are left in the store and re-read one match at a time, so a store
// with a million moves in it does not have to fit in this process's memory.
type logProbe struct {
	Match bson.ObjectID `bson:"m"`
	Kind  string        `bson:"k"`
	Seq   int           `bson:"s"`
}

// matchIDProbe reads a match envelope's id and nothing else.
type matchIDProbe struct {
	ID bson.ObjectID `bson:"_id"`
}

// namespacePlan is what one match has left to move.
type namespacePlan struct {
	id bson.ObjectID
	// envelope is the index's copy, to be written into the match's own
	// namespace; nil when it is already there.
	envelope []byte
	records  []recordRef
}

// MigrateNamespacesKDB moves every match in an embedded store into a namespace
// of its own. The server must be stopped: the store's directory lock refuses a
// second opener, so opening it at all is the check.
//
// dry counts what would move and writes nothing.
func MigrateNamespacesKDB(ctx context.Context, k *db.KDB, dry bool) (NamespaceMigrationReport, error) {
	envelopes, err := scanEnvelopes(k)
	if err != nil {
		return NamespaceMigrationReport{}, err
	}
	records, orphans, err := scanOldLog(k, envelopes)
	if err != nil {
		return NamespaceMigrationReport{}, err
	}

	report := NamespaceMigrationReport{Orphans: orphans}
	plans := make([]namespacePlan, 0, len(envelopes))
	for _, hex := range sortedHex(envelopes) {
		id, err := bson.ObjectIDFromHex(hex)
		if err != nil {
			return report, fmt.Errorf("match %q: %w", hex, err)
		}
		plan := namespacePlan{id: id, records: records[hex]}
		if _, err := k.Get(db.MatchNS(hex), keyMatchEnvelope); db.IsNotFound(err) {
			plan.envelope = envelopes[hex]
		} else if err != nil {
			return report, fmt.Errorf("match %s: %w", hex, err)
		}
		if plan.envelope == nil && len(plan.records) == 0 {
			report.Skipped++
			continue
		}
		plans = append(plans, plan)
	}

	if dry {
		for _, plan := range plans {
			report.Matches++
			report.Records += len(plan.records)
		}
		return report, nil
	}

	for _, plan := range plans {
		if err := moveMatchNamespace(k, plan); err != nil {
			return report, fmt.Errorf("match %s: %w", plan.id.Hex(), err)
		}
		report.Matches++
		report.Records += len(plan.records)
		moved, err := verifyMatchNamespace(k, plan)
		if err != nil {
			return report, fmt.Errorf("match %s: %w", plan.id.Hex(), err)
		}
		if moved != len(plan.records) {
			return report, fmt.Errorf("match %s: %d records planned, %d in its namespace",
				plan.id.Hex(), len(plan.records), moved)
		}
		report.Verified++
	}
	return report, nil
}

// scanEnvelopes reads every match envelope out of the index, keyed by hex.
// The bodies are kept because they are what gets written into each match's own
// namespace, and they are small — an envelope has never held a board or a move.
func scanEnvelopes(k *db.KDB) (map[string][]byte, error) {
	out := map[string][]byte{}
	err := k.Scan(db.NSMatches, func(raw []byte) error {
		var probe matchIDProbe
		if err := db.UnmarshalDoc(raw, &probe); err != nil {
			return err
		}
		if probe.ID.IsZero() {
			return fmt.Errorf("a document in %s has no _id", db.NSMatches)
		}
		out[probe.ID.Hex()] = append([]byte(nil), raw...)
		return nil
	})
	return out, err
}

// scanOldLog lists what is still in the shared move log, per match, and counts
// the records whose match the index does not have.
func scanOldLog(k *db.KDB, envelopes map[string][]byte) (map[string][]recordRef, int, error) {
	out := map[string][]recordRef{}
	orphans := 0
	err := k.Scan(db.NSMatchLog, func(raw []byte) error {
		var probe logProbe
		if err := db.UnmarshalDoc(raw, &probe); err != nil {
			return err
		}
		if probe.Kind != kindMove && probe.Kind != kindSnapshot {
			return fmt.Errorf("a record in %s has kind %q", db.NSMatchLog, probe.Kind)
		}
		hex := probe.Match.Hex()
		if _, ok := envelopes[hex]; !ok {
			orphans++
			return nil
		}
		out[hex] = append(out[hex], recordRef{kind: probe.Kind, seq: probe.Seq})
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	// Oldest first, so that an interrupted run and a re-run write a match's
	// records in the same order and its namespace's history reads the same way.
	for hex := range out {
		refs := out[hex]
		sort.Slice(refs, func(i, j int) bool {
			if refs[i].kind != refs[j].kind {
				return refs[i].kind < refs[j].kind
			}
			return refs[i].seq < refs[j].seq
		})
	}
	return out, orphans, nil
}

// readEnvelope reads one stored match envelope.
func readEnvelope(k *db.KDB, ns, key string) (models.Match, error) {
	raw, err := k.Get(ns, key)
	if err != nil {
		return models.Match{}, err
	}
	var m models.Match
	if err := db.UnmarshalDoc(raw, &m); err != nil {
		return models.Match{}, err
	}
	return m, nil
}

func sortedHex(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for hex := range m {
		out = append(out, hex)
	}
	sort.Strings(out)
	return out
}

// moveMatchNamespace writes one match's envelope and records into its own
// namespace and removes the records from the old log, in one transaction, so
// there is no moment at which a record is in neither place.
func moveMatchNamespace(k *db.KDB, plan namespacePlan) error {
	ns := db.MatchNS(plan.id.Hex())
	return k.UpdateMulti([]string{ns, db.NSMatchLog}, func(tx *db.MultiTx) error {
		if plan.envelope != nil {
			tx.Put(ns, keyMatchEnvelope, plan.envelope)
		}
		for _, ref := range plan.records {
			old := logKey(plan.id, ref.kind, ref.seq)
			raw, err := tx.Get(db.NSMatchLog, old)
			if err != nil {
				if db.IsNotFound(err) {
					// Moved by an earlier run that stopped after committing
					// this match. Nothing to do, and nothing to undo.
					continue
				}
				return err
			}
			tx.Put(ns, recordKey(ref.kind, ref.seq), raw)
			tx.Delete(db.NSMatchLog, old)
		}
		return nil
	})
}

// verifyMatchNamespace reads back what moveMatchNamespace claims to have
// written — the envelope, every record — and checks the old log no longer has
// any of it. Returns how many records it found in the match's namespace.
func verifyMatchNamespace(k *db.KDB, plan namespacePlan) (int, error) {
	ns := db.MatchNS(plan.id.Hex())
	owned, err := readEnvelope(k, ns, keyMatchEnvelope)
	if err != nil {
		return 0, fmt.Errorf("envelope: %w", err)
	}
	// The index's copy has to still be there — it is what every scan reads —
	// and it has to be the same match. Two envelopes that disagree is the one
	// failure this layout makes possible that the old one did not.
	indexed, err := readEnvelope(k, db.NSMatches, plan.id.Hex())
	if err != nil {
		return 0, fmt.Errorf("index copy: %w", err)
	}
	if owned.Version != indexed.Version {
		return 0, fmt.Errorf("the index says version %d and the match's own namespace says %d",
			indexed.Version, owned.Version)
	}
	moved := 0
	for _, ref := range plan.records {
		if _, err := k.Get(ns, recordKey(ref.kind, ref.seq)); err != nil {
			return moved, fmt.Errorf("%s/%d: %w", ref.kind, ref.seq, err)
		}
		if _, err := k.Get(db.NSMatchLog, logKey(plan.id, ref.kind, ref.seq)); !db.IsNotFound(err) {
			return moved, fmt.Errorf("%s/%d is still in %s (%v)", ref.kind, ref.seq, db.NSMatchLog, err)
		}
		moved++
	}
	return moved, nil
}
