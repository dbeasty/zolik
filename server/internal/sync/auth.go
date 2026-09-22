// Package sync makes this process a node of the distributed database: it
// replicates namespaces with the cloud and with other nodes, decides who may
// sync what, and settles the conflicts that application rules rather than the
// engine have to answer.
//
// The shape of the system is hub and spoke. The cloud is the hub and the
// identity authority; a phone or a self-hosted server is a spoke holding only
// what concerns the people using it. Nothing here makes anything eventually
// consistent that has to be ordered: a match is written by one node at a time
// and moves between them by handover, accounts are written by the cloud alone,
// and sessions and one-time codes never leave the node that issued them. What
// does replicate is either append-only (a match's moves, a finished result) or
// genuinely mergeable (a person's settings and circle).
package sync

import (
	"context"
	"errors"
	"fmt"
	"strings"

	kdbauth "github.com/limidus/kdb/go/kdb/auth"

	"zolik/server/internal/db"
)

// Identity is what a bearer token turned out to be. Exactly one of UserHex and
// NodeID is set: a person's own device syncing their data, or an enrolled node
// syncing on its own behalf.
type Identity struct {
	// UserHex is the account, for a token a person's client presented.
	UserHex string
	// NodeID is the KDB node id, for a node credential.
	NodeID string
	// OwnerHex is the account a node was enrolled by. A node syncs its
	// owner's namespaces, which is what makes a phone's replica partial.
	OwnerHex string
	// Server is true for a peer that is another server of ours - a
	// self-hosted node or another region - rather than somebody's device.
	// Such a peer may hold any user it is serving, so its namespaces are not
	// bounded by one account.
	Server bool
}

// Verifier turns a bearer token into an identity. internal/auth implements it;
// this package deliberately knows nothing about how a token is signed.
type Verifier interface {
	VerifySyncToken(ctx context.Context, token string) (Identity, error)
}

// MatchSeating answers whether a user sits at a match, and which node is
// writing it. internal/match implements it. Both questions are about the
// namespace layout rather than about game rules: who may read a match's
// namespace, and who may push to it.
type MatchSeating interface {
	// Seated reports whether the user is one of the match's players.
	Seated(ctx context.Context, matchHex, userHex string) (bool, error)
}

// ErrDenied is every refusal this package makes. The reason is in the message
// for the log; a peer is told only that it was refused.
var ErrDenied = errors.New("sync: not allowed")

// authEngine is the KDB auth engine a sync listener runs behind. It answers
// two questions the engine asks per connection and per namespace: who is this,
// and may they pull or push this namespace.
type authEngine struct {
	verifier Verifier
	seating  MatchSeating
	// nodeID is this node's own id, so a peer pushing into our outbox - which
	// nobody but us may write - is refused by name.
	nodeID string
}

func (e *authEngine) Authenticator() kdbauth.Authenticator { return e }
func (e *authEngine) Authorizer() kdbauth.Authorizer       { return e }

// Authenticate resolves the bearer token the peer presented. A connection with
// no token is refused outright rather than becoming an anonymous principal:
// there is nothing in this database that everyone may read.
func (e *authEngine) Authenticate(ctx context.Context, creds kdbauth.Credentials) (kdbauth.Principal, error) {
	if creds.Token == nil || *creds.Token == "" {
		return kdbauth.Principal{}, fmt.Errorf("%w: no token", ErrDenied)
	}
	id, err := e.verifier.VerifySyncToken(ctx, *creds.Token)
	if err != nil {
		return kdbauth.Principal{}, fmt.Errorf("%w: %v", ErrDenied, err)
	}
	claims := map[string]string{}
	if id.UserHex != "" {
		claims["user"] = id.UserHex
	}
	if id.NodeID != "" {
		claims["node"] = id.NodeID
	}
	if id.OwnerHex != "" {
		claims["owner"] = id.OwnerHex
	}
	if id.Server {
		claims["server"] = "true"
	}
	principalID := id.UserHex
	if principalID == "" {
		principalID = "node:" + id.NodeID
	}
	return kdbauth.Principal{ID: principalID, Claims: claims}, nil
}

// identityOf reads back what Authenticate put in the principal's claims.
func identityOf(p kdbauth.Principal) Identity {
	return Identity{
		UserHex:  p.Claims["user"],
		NodeID:   p.Claims["node"],
		OwnerHex: p.Claims["owner"],
		Server:   p.Claims["server"] == "true",
	}
}

// Authorize decides one action. Pull and push are decided separately, which is
// the difference between a device being told its stats and a device claiming
// them: a phone pulls u/<uid>/ro and may never push it.
func (e *authEngine) Authorize(ctx context.Context, p kdbauth.Principal, action kdbauth.Action) error {
	id := identityOf(p)
	// This engine is the database's engine, not only the sync listener's, so
	// it is also asked about every write this server makes for itself. Those
	// arrive with no principal at all, because they never came from a
	// connection: the process holding the directory lock is already as
	// authorized as anything can be, and what a request is allowed to make it
	// do was decided by the HTTP layer long before. A peer, by contrast, is
	// never principal-less: Authenticate refuses a connection without a token.
	if p.ID == "" {
		switch action.(type) {
		case kdbauth.PeerPullAction, kdbauth.PeerPushAction, kdbauth.PeerSyncAction:
			return fmt.Errorf("%w: peer sync without a principal", ErrDenied)
		default:
			return nil
		}
	}
	switch a := action.(type) {
	case kdbauth.PeerPullAction:
		return e.allow(ctx, id, db.Unqualified(a.Namespace), false)
	case kdbauth.PeerPushAction:
		return e.allow(ctx, id, db.Unqualified(a.Namespace), true)
	case kdbauth.PeerSyncAction:
		// Both directions at once. Granting it would give a puller the right
		// to push, so it is refused and the engine asks again, one direction
		// at a time.
		return fmt.Errorf("%w: %s in both directions", ErrDenied, a.Namespace)
	default:
		// A peer that authenticated and then asked for something else - SQL, a
		// session, a direct commit - is not doing what a sync peer connects
		// for.
		return fmt.Errorf("%w: %T", ErrDenied, action)
	}
}

// allow is the whole access-control matrix of the distributed database, and it
// is deliberately small enough to read in one sitting.
func (e *authEngine) allow(ctx context.Context, id Identity, namespace string, push bool) error {
	// Definitions - resolution chains, homes, schemas - are pulled by every
	// peer and written by nobody but their own node.
	if strings.HasPrefix(namespace, "_kdb/") {
		if push {
			return fmt.Errorf("%w: %s is not pushable", ErrDenied, namespace)
		}
		return nil
	}

	subject := id.UserHex
	if subject == "" {
		subject = id.OwnerHex
	}

	if user, ok := db.UserOfNamespace(namespace); ok {
		if id.Server {
			return nil
		}
		if subject == "" || user != subject {
			return fmt.Errorf("%w: %s belongs to somebody else", ErrDenied, namespace)
		}
		// The cloud writes a person's profile mirror, their computed stats and
		// the index of their matches; their devices read it. A device that
		// could push it could award itself a rating.
		if push && strings.HasSuffix(namespace, "/ro") {
			return fmt.Errorf("%w: %s is written by the cloud", ErrDenied, namespace)
		}
		return nil
	}

	if matchHex, ok := db.MatchOfNamespace(namespace); ok {
		if id.Server {
			return nil
		}
		if subject == "" {
			return fmt.Errorf("%w: %s needs an account", ErrDenied, namespace)
		}
		seated, err := e.seating.Seated(ctx, matchHex, subject)
		if err != nil {
			return err
		}
		if !seated {
			return fmt.Errorf("%w: %s is not your match", ErrDenied, namespace)
		}
		// A seated player's node may push, because a match's home moves to
		// whichever device is playing it. Pushing without being the home is
		// refused by the home fence in the engine, not here: this layer knows
		// who is at the table, not whose turn it is to write.
		return nil
	}

	if node, ok := db.NodeOfOutbox(namespace); ok {
		if node == e.nodeID {
			// Our own outbox: the cloud pulls it, nobody pushes to it.
			if push {
				return fmt.Errorf("%w: %s is ours to write", ErrDenied, namespace)
			}
			return nil
		}
		// Somebody else's outbox. Only the node itself may push it, and only
		// a server pulls one.
		if push {
			if id.NodeID == node {
				return nil
			}
			return fmt.Errorf("%w: %s belongs to node %s", ErrDenied, namespace, node)
		}
		if id.Server || id.NodeID == node {
			return nil
		}
		return fmt.Errorf("%w: %s belongs to node %s", ErrDenied, namespace, node)
	}

	// Everything else - accounts, identities, reservations, results, metrics -
	// is the cloud's, and is replicated only between servers.
	if id.Server && !push {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrDenied, namespace)
}
