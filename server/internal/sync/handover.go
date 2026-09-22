package sync

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	kdbserver "github.com/limidus/kdb/go/kdb/server"

	"zolik/server/internal/db"
)

// A match is written by one node at a time. That is not a limitation of the
// database but a property of the game: turn order is exactly the kind of thing
// eventual consistency cannot give you, and a match whose moves merged would
// be a match where two people both went fourth.
//
// So a match's namespace is single-homed, and playing it from another device
// means moving that home rather than writing from two places. The device that
// wants to play asks the current home; the current home's own rules decide -
// is the match idle, and is the asker actually seated at it - and on yes the
// home moves, with the fence rising so that anything the old home writes
// afterwards is refused wherever it arrives.
//
// The awkward case is a home that cannot be asked, which on a phone is the
// ordinary case: the device is in somebody's pocket with no signal. Then the
// cloud, which the chain names as the authority, may take the home by force.
// The cost is real and has to be owned: moves the old device made and never
// pushed become unadoptable, and the player is told so rather than having
// them silently vanish.

// MatchHome is what this package needs to know about a match before it will
// move it. internal/match implements it.
type MatchHome interface {
	// Seated reports whether a user is one of the match's players.
	Seated(ctx context.Context, matchHex, userHex string) (bool, error)
	// Idle reports whether the match can be handed over right now: no move is
	// being applied, and nothing is mid-write.
	Idle(ctx context.Context, matchHex string) (bool, error)
	// Release gives up a match this node was playing: it stops the bots,
	// evicts the live table and flushes anything outstanding, so the node that
	// takes over is not racing this one.
	Release(ctx context.Context, matchHex string) error
	// Adopt loads a match this node has just become the home of.
	Adopt(ctx context.Context, matchHex string) error
}

// SetMatchHome supplies the match rules the handover policy asks. It is set
// after Open because the match manager needs the database this node was built
// from.
func (n *Node) SetMatchHome(h MatchHome) {
	if n == nil {
		return
	}
	n.matches = h
}

// handoverPolicy is what a peer's request to take a match reaches. A refusal
// is the requester's answer, so its reason is written to be read by somebody
// looking at a phone rather than by a log.
func (n *Node) handoverPolicy(req kdbserver.HandoverRequest) error {
	name := db.Unqualified(req.Namespace)
	matchHex, ok := db.MatchOfNamespace(name)
	if !ok {
		// Nothing else in this database moves home on request. Accounts,
		// reservations and results are the cloud's, and a node asking for them
		// is a node asking to be the cloud.
		return fmt.Errorf("only a match may change hands")
	}
	if n.matches == nil {
		return fmt.Errorf("this node cannot hand a match over")
	}
	id := identityOf(req.Principal)
	subject := id.UserHex
	if subject == "" {
		subject = id.OwnerHex
	}
	if subject == "" && !id.Server {
		return fmt.Errorf("only a seated player may take a match over")
	}

	ctx, cancel := syncContext()
	defer cancel()

	if subject != "" {
		seated, err := n.matches.Seated(ctx, matchHex, subject)
		if err != nil {
			return fmt.Errorf("checking the table: %w", err)
		}
		if !seated {
			return fmt.Errorf("you are not sitting at this match")
		}
	}
	idle, err := n.matches.Idle(ctx, matchHex)
	if err != nil {
		return fmt.Errorf("checking the match: %w", err)
	}
	if !idle {
		// Handing over mid-move would mean the move lands on one node and the
		// next one on another, with the fence refusing whichever was slower.
		return fmt.Errorf("a move is being played right now")
	}
	// The old home stops playing before the new one starts, not after.
	if err := n.matches.Release(ctx, matchHex); err != nil {
		return fmt.Errorf("putting the match down: %w", err)
	}
	return nil
}

// TakeMatch makes this node the writer of a match: what a device calls when
// its player opens a match that is being hosted somewhere else.
//
// It returns only once this node holds both the assignment and everything the
// previous home had written, so the caller may play the next move immediately.
func (n *Node) TakeMatch(ctx context.Context, matchHex, reason string) error {
	if n == nil {
		return errors.New("sync: this process is not part of the distributed database")
	}
	namespace := db.Qualified(db.MatchNS(matchHex))
	if _, err := n.node.RequestHome("hub", namespace, n.cfg.Advertise, reason, false); err != nil {
		var refused *kdbserver.HandoverRefusedError
		if errors.As(err, &refused) {
			return fmt.Errorf("sync: %w", err)
		}
		return fmt.Errorf("sync: asking to take match %s: %w", matchHex, err)
	}
	if n.matches != nil {
		if err := n.matches.Adopt(ctx, matchHex); err != nil {
			return fmt.Errorf("sync: loading match %s after taking it: %w", matchHex, err)
		}
	}
	return nil
}

// GiveMatch hands a match to another node: the cloud taking a table back when
// a player closes the app, or a device passing it on.
func (n *Node) GiveMatch(ctx context.Context, matchHex, toNode, addr string) error {
	if n == nil {
		return nil
	}
	if n.matches != nil {
		if err := n.matches.Release(ctx, matchHex); err != nil {
			return err
		}
	}
	_, err := n.node.Handover(db.Qualified(db.MatchNS(matchHex)), toNode, addr)
	return err
}

// ReclaimMatch is the cloud taking a match back from a node that cannot be
// asked, because it is offline. Only the authority may, which is the hub.
//
// The idle window is what makes this safe to do at all: a device that has
// been silent for longer than a game could plausibly be paused is either gone
// or has stopped playing, and a player waiting at another device should not be
// held hostage to it. Anything the old home wrote and never pushed is refused
// by the fence when it comes back, and the player is told how much of it was
// lost rather than being left to notice.
func (n *Node) ReclaimMatch(ctx context.Context, matchHex string, idleFor time.Duration) error {
	if n == nil || n.cfg.Role != RoleHub {
		return errors.New("sync: only the hub may take a match back")
	}
	if n.matches != nil {
		idle, err := n.matches.Idle(ctx, matchHex)
		if err != nil {
			return err
		}
		if !idle {
			return fmt.Errorf("sync: match %s is being played here", matchHex)
		}
	}
	namespace := db.Qualified(db.MatchNS(matchHex))
	if _, err := n.node.ForceHandover(namespace, n.NodeID(), n.cfg.Advertise); err != nil {
		return fmt.Errorf("sync: taking match %s back: %w", matchHex, err)
	}
	log.Printf("sync: took match %s back after %s of silence from its host", matchHex, idleFor)
	if n.matches != nil {
		return n.matches.Adopt(ctx, matchHex)
	}
	return nil
}

// HomeOf reports which node writes a match, and whether anybody does. A match
// with no home has never left the node that created it.
func (n *Node) HomeOf(matchHex string) (node string, addr string, ok bool) {
	if n == nil {
		return "", "", false
	}
	rt, err := n.runtime(db.MatchNS(matchHex))
	if err != nil {
		return "", "", false
	}
	h, ok := rt.HomeOf()
	if !ok {
		return "", "", false
	}
	return h.Node, h.Addr, true
}

// WritesMatches reports whether this node is the one writing a match.
func (n *Node) WritesMatches(matchHex string) bool {
	node, _, ok := n.HomeOf(matchHex)
	if !ok {
		// No assignment at all: whoever holds it writes it, which is this node
		// for a match it created and never handed on.
		return true
	}
	return node == n.NodeID()
}

// ClaimMatch makes this node the home of a match it has just created, so that
// every other node knows where to ask for it.
func (n *Node) ClaimMatch(matchHex string) error {
	if n == nil {
		return nil
	}
	meta := n.node.Meta()
	if meta == nil {
		return nil
	}
	_, err := meta.AssignHome(db.Qualified(db.MatchNS(matchHex)), n.NodeID(), n.cfg.Advertise)
	return err
}
