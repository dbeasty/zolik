package db

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/limidus/kdb/go/kdb/codec"
	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/models"
)

// A person's own data - their settings, their circle, their devices, their
// scorepads - is written twice on a server that replicates: once into the
// namespace its feature scans (notify_circle, scoring_sessions and the rest),
// and once into the account's own namespace, u/<hex>.
//
// The two are not redundant. The per-feature namespaces answer the questions
// that cross accounts: who has this person in their circle, which scorepads
// are open, which devices to push to. The account's namespace answers the only
// question a phone can ask, which is "give me everything that is mine": a peer
// syncs namespaces, not rows, so a person's data has to live somewhere named
// after them before any of it can reach their devices.
//
// The per-feature namespaces are node-local and are never replicated. The
// account namespaces are. Where both exist the writes go into one transaction,
// so the copy a device holds and the copy the server scans cannot disagree.

// Subject key prefixes, as internal/stats and internal/notify spell them.
const (
	subjectUserPrefix  = "user:"
	subjectGuestPrefix = "guest:"
)

// Document kinds inside an account's namespace. Every document carries one so
// that a merge rule can be chosen for it: a document id is a hash of its key
// and says nothing about what is inside. See internal/sync, which holds the
// rule for each of these.
const (
	KindPrefs   = "prefs"
	KindScoring = "scoring"
	KindCircle  = "circle"
	KindDevice  = "device"
	KindNotify  = "notify"
	KindProfile = "profile"
	KindStats   = "stats"
	KindMatch   = "match"
)

// DocKindField is where the kind is stored.
const DocKindField = "_kind"

// UserNSForSubject returns the namespace a subject's own data belongs in, and
// whether the subject has one at all.
//
// A guest does not. A guest exists only on the node that minted them, has no
// account to sync to, and stops existing as a separate person the moment they
// sign in and their history is claimed. Their data stays node-local until then.
func UserNSForSubject(subjectKey string) (string, bool) {
	if hex, ok := strings.CutPrefix(subjectKey, subjectUserPrefix); ok && hex != "" {
		return UserNS(hex), true
	}
	return "", false
}

// IsGuestSubject reports whether a subject key names a guest.
func IsGuestSubject(subjectKey string) bool {
	return strings.HasPrefix(subjectKey, subjectGuestPrefix)
}

// WithKind stamps a document with the kind a merge rule is chosen by.
func WithKind(doc []byte, kind string) ([]byte, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(doc, &fields); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(kind)
	if err != nil {
		return nil, err
	}
	fields[DocKindField] = raw
	return json.Marshal(fields)
}

// MirrorUserDoc writes a document into the account's namespace as part of tx,
// which must hold that namespace. It is a no-op for a subject with no account,
// so callers do not have to ask twice.
func MirrorUserDoc(tx *MultiTx, subjectKey, key, kind string, doc []byte) error {
	ns, ok := UserNSForSubject(subjectKey)
	if !ok {
		return nil
	}
	stamped, err := WithKind(doc, kind)
	if err != nil {
		return err
	}
	tx.Put(ns, key, stamped)
	return nil
}

// DropUserDoc removes a document from the account's namespace, for the same
// subjects MirrorUserDoc writes one for.
func DropUserDoc(tx *MultiTx, subjectKey, key string) {
	if ns, ok := UserNSForSubject(subjectKey); ok {
		tx.Delete(ns, key)
	}
}

// UserNamespacesOf returns the namespaces to include in a transaction that
// touches a subject's data: the account's, when there is one.
func UserNamespacesOf(subjectKeys ...string) []string {
	var out []string
	seen := map[string]bool{}
	for _, key := range subjectKeys {
		ns, ok := UserNSForSubject(key)
		if !ok || seen[ns] {
			continue
		}
		seen[ns] = true
		out = append(out, ns)
	}
	return out
}

// KDBClearUserData deletes every document in an account's two namespaces —
// its preferences, profile, circle, devices and scoring sessions, and the
// stats and match index the cloud wrote for it — for an account that is
// deleted. A namespace that was never opened is left unopened: opening one
// creates it, and there is nothing in it to delete.
//
// The namespaces themselves stay. A node that follows them sees every
// document go, which is how a phone holding the account's data learns that
// it is gone.
func KDBClearUserData(k *KDB, userHex string) error {
	for _, ns := range []string{UserNS(userHex), UserReadOnlyNS(userHex)} {
		if !k.HoldsNamespace(ns) {
			continue
		}
		n := k.ns(ns)
		n.mu.Lock()
		var ids []codec.UUID
		err := n.scan(func(id codec.UUID, _ []byte) error {
			ids = append(ids, id)
			return nil
		})
		for _, id := range ids {
			if err != nil {
				break
			}
			_, err = n.deleteByUUID(id)
		}
		n.mu.Unlock()
		if err != nil {
			return fmt.Errorf("kdb: clearing %s: %w", ns, err)
		}
	}
	return nil
}

// KDBDeleteUser removes an account and gives up the username and email it
// held, in one transaction, so the name is free again the moment the account
// is gone and never names a missing account.
func KDBDeleteUser(k *KDB, id bson.ObjectID) error {
	namespaces := []string{NSUsers, NSReservations}
	return k.UpdateMulti(namespaces, func(tx *MultiTx) error {
		cur, err := tx.Get(NSUsers, id.Hex())
		if IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		var u models.User
		if err := UnmarshalDoc(cur, &u); err != nil {
			return err
		}
		if err := KDBRelease(tx, ReservationUsername, u.Username, id.Hex()); err != nil {
			return err
		}
		if err := KDBRelease(tx, ReservationEmail, u.Email, id.Hex()); err != nil {
			return err
		}
		tx.Delete(NSUsers, id.Hex())
		return nil
	})
}
