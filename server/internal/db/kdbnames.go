package db

import (
	"strings"

	"github.com/limidus/kdb/go/kdb/embed"
	kdbserver "github.com/limidus/kdb/go/kdb/server"
)

// The namespaces above this line are fixed: one per collection, declared by
// this binary and opened at startup. The ones below are not, and cannot be,
// because their names are data.
//
// A namespace is what KDB replicates and authorizes, so the layout decides
// what a node can hold. A phone must be able to hold this user's settings and
// the matches this user played, and nothing else, which is only expressible if
// each of those is a namespace of its own. That is the whole reason these
// exist: not tidiness, but the fact that "sync me my rows out of the matches
// collection" is not a thing a peer can ask for, while "sync me namespace
// zolik/m/<id>" is.
const (
	userPrefix       = "u/"
	readOnlySuffix   = "/ro"
	matchPrefix      = "m/"
	nodePrefix       = "node/"
	outboxSuffix     = "/outbox"
	dynamicSeparator = "/"
)

// UserNS is where a user's own mutable data lives: preferences, scoring
// sessions, circle edges, devices. Every node the user signs in on may write
// it, so it is the one part of the layout that genuinely merges.
func UserNS(userHex string) string { return userPrefix + userHex }

// UserReadOnlyNS is what the cloud writes for a user and their nodes only
// read: the profile mirror, computed stats, and the index of matches they have
// played. Kept apart from UserNS so a node can be granted the two differently
// — pull this, push and pull that — which is the difference between a device
// reporting its own stats and a device being told them.
func UserReadOnlyNS(userHex string) string { return userPrefix + userHex + readOnlySuffix }

// MatchNS holds one match: its envelope, its moves and its board snapshots.
// Exactly one node writes it at a time (its home), and every seated player's
// nodes may read it, which is what lets a match be followed from another
// device and picked up on one.
func MatchNS(matchHex string) string { return matchPrefix + matchHex }

// NodeOutboxNS is where a node that is not the cloud puts finished matches for
// the cloud to verify and record. One writer, append only, and pushed rather
// than pulled.
func NodeOutboxNS(nodeID string) string { return nodePrefix + nodeID + outboxSuffix }

// IsDynamicNamespace reports whether a name is one of the layouts above, and
// so may be opened on demand rather than having to have been declared. The
// check is deliberately narrow: an unrecognised name is a bug in this binary,
// and opening it would create a directory under the data root that nothing
// would ever read again.
func IsDynamicNamespace(name string) bool {
	switch {
	case strings.HasPrefix(name, userPrefix):
		rest := strings.TrimPrefix(name, userPrefix)
		rest = strings.TrimSuffix(rest, readOnlySuffix)
		return rest != "" && !strings.Contains(rest, dynamicSeparator)
	case strings.HasPrefix(name, matchPrefix):
		rest := strings.TrimPrefix(name, matchPrefix)
		return rest != "" && !strings.Contains(rest, dynamicSeparator)
	case strings.HasPrefix(name, nodePrefix):
		rest := strings.TrimPrefix(name, nodePrefix)
		if !strings.HasSuffix(rest, outboxSuffix) {
			return false
		}
		rest = strings.TrimSuffix(rest, outboxSuffix)
		return rest != "" && !strings.Contains(rest, dynamicSeparator)
	default:
		return false
	}
}

// UserOfNamespace returns the user a namespace belongs to, and whether it is
// one of theirs at all. Used to decide what a node may sync.
func UserOfNamespace(name string) (userHex string, ok bool) {
	if !strings.HasPrefix(name, userPrefix) {
		return "", false
	}
	rest := strings.TrimSuffix(strings.TrimPrefix(name, userPrefix), readOnlySuffix)
	if rest == "" || strings.Contains(rest, dynamicSeparator) {
		return "", false
	}
	return rest, true
}

// MatchOfNamespace returns the match a namespace holds, and whether it holds
// one.
func MatchOfNamespace(name string) (matchHex string, ok bool) {
	if !strings.HasPrefix(name, matchPrefix) {
		return "", false
	}
	rest := strings.TrimPrefix(name, matchPrefix)
	if rest == "" || strings.Contains(rest, dynamicSeparator) {
		return "", false
	}
	return rest, true
}

// NodeOfOutbox returns the node whose outbox a namespace is.
func NodeOfOutbox(name string) (nodeID string, ok bool) {
	if !strings.HasPrefix(name, nodePrefix) || !strings.HasSuffix(name, outboxSuffix) {
		return "", false
	}
	rest := strings.TrimSuffix(strings.TrimPrefix(name, nodePrefix), outboxSuffix)
	if rest == "" || strings.Contains(rest, dynamicSeparator) {
		return "", false
	}
	return rest, true
}

// Qualified turns a namespace name into the id the engine knows it by, which
// is what every KDB-facing API (peer patterns, homes, resolution chains,
// conflict queues) speaks.
func Qualified(name string) string { return kdbCatalog + "/" + name }

// Unqualified is Qualified's inverse, for turning an engine-side namespace id
// back into the name this package uses.
func Unqualified(namespaceID string) string {
	return strings.TrimPrefix(namespaceID, kdbCatalog+"/")
}

// Engine exposes what a sync node needs of the database: the host it opens
// namespaces under, the set that serves them, and the runtime carrying this
// node's identity. ok is false for an in-memory database, which cannot
// replicate: it has no data root, and a namespace closed there is gone.
//
// internal/sync is the only caller. This is a deliberate seam rather than an
// exported field, so that the engine's own invariants — one runtime per
// namespace, one memory guard, one write gate — stay this package's business.
func (k *KDB) Engine() (host *embed.Host, set *kdbserver.NamespaceSet, primary *kdbserver.KdbServerRuntime, ok bool) {
	if k.host == nil {
		return nil, nil, nil, false
	}
	return k.host, k.set, k.primary, true
}

// PrepareNamespaces registers what every namespace opened from now on must be
// hooked into, and applies it to the ones already open. This is how a sync
// node makes a namespace notify it of local commits and learn the definitions
// that replicated to it.
func (k *KDB) PrepareNamespaces(prepare func(*kdbserver.KdbServerRuntime)) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.prepare = prepare
	if prepare == nil {
		return
	}
	for _, n := range k.nss {
		prepare(n.srv)
	}
}

// OpenNamespaces lists the namespaces this database is holding right now.
func (k *KDB) OpenNamespaces() []string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	out := make([]string, 0, len(k.nss))
	for name := range k.nss {
		out = append(out, name)
	}
	return out
}
