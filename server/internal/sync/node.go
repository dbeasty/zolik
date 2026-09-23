package sync

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	kdbauth "github.com/limidus/kdb/go/kdb/auth"
	"github.com/limidus/kdb/go/kdb/peersync"
	"github.com/limidus/kdb/go/kdb/replication"
	kdbserver "github.com/limidus/kdb/go/kdb/server"
	"github.com/limidus/kdb/go/kdb/syncnode"

	"zolik/server/internal/db"
)

// Role is what this process is to the rest of the database.
type Role string

const (
	// RoleOff is a process that replicates with nobody: today's deployment,
	// and the only supported shape on Mongo.
	RoleOff Role = "off"
	// RoleHub is the cloud. It serves peers, owns identity, and never dials
	// out: a spoke may be behind a phone's carrier NAT, and the hub has no
	// business knowing where any of them are.
	RoleHub Role = "hub"
	// RoleSpoke is a phone or a self-hosted server. It dials the hub and holds
	// only what concerns the people using it.
	RoleSpoke Role = "spoke"
)

// Config is how a process is told what kind of node it is. It comes from the
// environment in production (see FromEnv) and is built directly in tests.
type Config struct {
	Role Role
	// DataDir is the database's directory, where replication progress is kept
	// so a restart resumes rather than re-reads everything.
	DataDir string
	// HubURL is the hub's sync endpoint, as a peer address: wss://host/kdb/sync.
	// Spokes only.
	HubURL string
	// Token authenticates this node to the hub. A node credential for a
	// device, a server credential for a self-hosted node or another region.
	Token string
	// Namespaces is what this node asks the hub for. A phone names its own
	// user and its own outbox; a server asks for a pattern.
	Namespaces []string
	// Interval is the anti-entropy tick: a sync at least this often whatever
	// commits did or did not happen. Commit-triggered syncs are what make it
	// feel immediate; this is what makes it correct.
	Interval time.Duration
	// Debounce is how long a burst of local commits is allowed to settle
	// before it is pushed. A hand of cards is a dozen commits in a few
	// seconds, and they should cost one sync.
	Debounce time.Duration
	// Advertise is where other nodes reach this one for the matches it hosts:
	// the address recorded with a home assignment. A phone has no address
	// anybody can dial and leaves it empty, which is fine - it asks the hub
	// for a match rather than the other way round.
	Advertise string
	// IdleClose, when set, closes namespaces nothing has used for a while. The
	// hub holds a namespace per user and per match and cannot keep them all
	// open; a phone holds a handful and has no reason to.
	IdleClose *syncnode.IdleConfig
}

// DefaultInterval and DefaultDebounce are what a node uses when the
// environment does not say. Thirty seconds is the anti-entropy floor the
// engine itself defaults to; two seconds is long enough for a turn's worth of
// commits to arrive together and short enough that the other device sees the
// move while the player is still looking at the screen.
const (
	DefaultInterval = 30 * time.Second
	DefaultDebounce = 2 * time.Second
)

// Node is this process as a member of the distributed database.
type Node struct {
	cfg  Config
	kdb  *db.KDB
	node *syncnode.Node
	// authority settles the conflicts that application rules have to answer.
	authority *authority
	// matches is what the handover policy asks about a match before it lets it
	// change hands.
	matches MatchHome
	// attend is who this node is currently holding data for, on a node that
	// serves whoever signs in rather than one person. See attendance.go.
	attend attendance
}

// Open makes this process a sync node over an already-open database.
//
// It does not start replicating: Start does that, after the caller has
// supplied the things this package cannot know by itself (who a token belongs
// to, who is seated at a match). Opening early is what lets every namespace
// the process opens afterwards be hooked in from the start.
func Open(k *db.KDB, cfg Config, verifier Verifier, seating MatchSeating) (*Node, error) {
	if cfg.Role == RoleOff {
		return nil, nil
	}
	host, set, primary, ok := k.Engine()
	if !ok {
		return nil, fmt.Errorf("sync: replication needs a file-backed KDB database")
	}
	if cfg.Interval <= 0 {
		cfg.Interval = DefaultInterval
	}
	if cfg.Debounce <= 0 {
		cfg.Debounce = DefaultDebounce
	}

	// The auth engine has to be in place before the listener serves anything,
	// and it is shared by every namespace: the set hands each new runtime the
	// primary's engine.
	engine := &authEngine{verifier: verifier, seating: seating, nodeID: primary.NodeID.String()}
	primary.AuthEngine = engine
	// A peer may push into a namespace this node does not hold yet, and has
	// to be able to: a match played on a phone, or a phone's outbox, exists
	// nowhere else until it is pushed. What stops that being an open door is
	// the authorization above, which is asked about the namespace by name
	// before anything is created - a phone may create its own match and its
	// own outbox, and nothing else.
	primary.PeerCreateOnPush = true

	var peers []replication.PeerConfig
	if cfg.Role == RoleSpoke {
		if cfg.HubURL == "" {
			return nil, fmt.Errorf("sync: a spoke needs the hub's address")
		}
		token := cfg.Token
		cfg.Namespaces = qualifyAll(cfg.Namespaces)
		peers = append(peers, replication.PeerConfig{
			Name:       "hub",
			Addr:       cfg.HubURL,
			Namespaces: cfg.Namespaces,
			Mode:       peersync.SyncBoth,
			Interval:   cfg.Interval,
			Token:      &token,
			// A namespace the hub has and this node does not is the normal
			// case for a spoke: it is how a match played on another device,
			// or an account's data after a fresh install, arrives at all.
			CreateLocal: true,
			// A phone that has just signed in fills from a snapshot rather
			// than from the whole history: what happened before it is of no
			// use on a device that has never seen this account, and it is
			// most of the bytes.
			PreferSnapshot: true,
			// Definitions arrive as a view of what this node may sync. The
			// metadata namespace holds every namespace's chain and home,
			// which on the hub means every user's: a phone has no business
			// learning the other names, and no room for them either.
			ScopedMeta: true,
		})
	}

	n := &Node{cfg: cfg, kdb: k, node: nil}
	sn, err := syncnode.Open(host, set, primary, syncnode.Config{
		Peers:                    peers,
		DataDir:                  cfg.DataDir,
		Debounce:                 cfg.Debounce,
		AuthorizePushedDocuments: false,
		Idle:                     cfg.IdleClose,
		// A peer asking to play a match this node is hosting reaches zolik's
		// own rules, not a configuration flag: whether a match may change
		// hands is a question about the table.
		HandoverPolicy: n.handoverPolicy,
	})
	if err != nil {
		return nil, fmt.Errorf("sync: %w", err)
	}
	// Everything this process opens from here on - a namespace per user, per
	// match, per outbox - is hooked into the node as it opens, so a namespace
	// created by a peer's push behaves exactly like one created locally.
	k.PrepareNamespaces(sn.Prepare)

	n.node = sn
	if cfg.Role == RoleHub {
		n.authority = newAuthority(n, 10*time.Second)
	}
	return n, nil
}

// Start applies this node's resolution chains and begins replicating.
func (n *Node) Start() error {
	if n == nil {
		return nil
	}
	if n.cfg.Role == RoleHub {
		if err := n.declareResolution(); err != nil {
			return err
		}
	}
	if err := n.node.Start(); err != nil {
		return fmt.Errorf("sync: %w", err)
	}
	if n.authority != nil {
		n.authority.start()
	}
	return nil
}

// Close stops replicating. The database itself is closed by its owner.
func (n *Node) Close() error {
	if n == nil {
		return nil
	}
	if n.authority != nil {
		n.authority.close()
	}
	return n.node.Close()
}

// Handler is the peer-sync endpoint, mounted by the hub at /kdb/sync. A peer
// dials it as an ordinary WebSocket upgrade carrying a bearer token, so it
// travels the same port, the same certificate and the same load balancer as
// the rest of the API - which is what makes syncing from a phone on a mobile
// network possible at all.
func (n *Node) Handler() http.Handler {
	if n == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "sync is not enabled", http.StatusNotFound)
		})
	}
	return n.node.Handler()
}

// SyncNow asks for an immediate sync with the hub, for the moments where
// waiting for the tick would be visible: the app coming to the foreground, the
// network coming back, a match ending.
func (n *Node) SyncNow() error {
	if n == nil || n.cfg.Role != RoleSpoke {
		return nil
	}
	_, err := n.node.SyncNow("hub")
	return err
}

// Follow adds namespaces to what this node syncs with the hub, and Unfollow
// removes them. A phone's set is not fixed: it grows by a namespace when its
// player joins a match and shrinks when the account signs out.
func (n *Node) Follow(namespaces ...string) error {
	return n.changeNamespaces(namespaces, nil)
}

func (n *Node) Unfollow(namespaces ...string) error {
	return n.changeNamespaces(nil, namespaces)
}

func (n *Node) changeNamespaces(add, remove []string) error {
	if n == nil || n.cfg.Role != RoleSpoke {
		return nil
	}
	r := n.node.Replicator()
	if r == nil {
		return nil
	}
	want := make([]string, 0, len(n.cfg.Namespaces)+len(add))
	drop := map[string]bool{}
	for _, ns := range remove {
		drop[db.Qualified(ns)] = true
	}
	for _, ns := range n.cfg.Namespaces {
		if !drop[ns] {
			want = append(want, ns)
		}
	}
	for _, ns := range add {
		q := db.Qualified(ns)
		if !drop[q] && !contains(want, q) {
			want = append(want, q)
		}
	}
	n.cfg.Namespaces = want
	return r.SetPeerNamespaces("hub", want)
}

// qualifyAll turns the namespace names this server uses into the ids the
// engine knows them by, leaving anything already qualified (a pattern, say)
// alone.
func qualifyAll(names []string) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		if strings.HasPrefix(name, db.Qualified("")) || strings.HasPrefix(name, "_kdb/") {
			out = append(out, name)
			continue
		}
		out = append(out, db.Qualified(name))
	}
	return out
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// NodeID is this node's identity in the database: what a home assignment
// names, what an outbox is named after, and what a peer sees as the author of
// every commit this process makes.
func (n *Node) NodeID() string {
	if n == nil {
		return ""
	}
	_, _, primary, ok := n.kdb.Engine()
	if !ok {
		return ""
	}
	return primary.NodeID.String()
}

// principal is the identity this node writes as when it is acting for itself
// rather than for a request: settling a conflict, taking a match's home.
func (n *Node) principal() kdbauth.Principal {
	return kdbauth.Principal{
		ID:     "node:" + n.NodeID(),
		Claims: map[string]string{"server": "true", claimSelf: "true"},
	}
}

// runtime is the server runtime for a namespace, opening it if necessary.
func (n *Node) runtime(namespace string) (*kdbserver.KdbServerRuntime, error) {
	_, set, _, ok := n.kdb.Engine()
	if !ok {
		return nil, fmt.Errorf("sync: no engine")
	}
	if _, err := n.kdb.Namespace(strings.TrimPrefix(namespace, "")); err != nil {
		return nil, err
	}
	return set.Resolve(db.Qualified(namespace), true)
}

// context for the short internal calls this package makes on its own behalf.
func syncContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}
