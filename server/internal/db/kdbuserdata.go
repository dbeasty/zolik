package db

import (
	"encoding/json"
	"strings"
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
