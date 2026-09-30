package auth

import (
	"context"
	"sort"

	"zolik/server/internal/db"
)

// kdbNodeRepository keeps enrolled nodes in the database rather than in this
// process's memory, which is what makes an enrolment survive a restart.
//
// It is the cloud's record of which installs exist and what each one signs
// with, so it is also the thing that decides whether a match handed up by some
// device is believed at all. Losing it on a restart meant every phone silently
// stopped being able to hand anything up until its owner enrolled it again.
type kdbNodeRepository struct {
	k *db.KDB
}

// NewKDBNodeRepository returns the database-backed node repository.
func NewKDBNodeRepository(k *db.KDB) NodeRepository { return &kdbNodeRepository{k: k} }

var _ NodeRepository = (*kdbNodeRepository)(nil)

func (r *kdbNodeRepository) SaveNode(_ context.Context, n Node) error {
	doc, err := db.MarshalDoc(n)
	if err != nil {
		return err
	}
	return r.k.Put(db.NSNodes, n.ID, doc)
}

func (r *kdbNodeRepository) FindNode(_ context.Context, id string) (Node, error) {
	doc, err := r.k.Get(db.NSNodes, id)
	if err != nil {
		if db.IsNotFound(err) {
			return Node{}, ErrNotFound
		}
		return Node{}, err
	}
	var n Node
	if err := db.UnmarshalDoc(doc, &n); err != nil {
		return Node{}, err
	}
	return n, nil
}

// NodesForOwner scans, as every secondary read on this engine does. An account
// has a handful of devices, and this is read when somebody opens the list of
// them rather than on any hot path.
func (r *kdbNodeRepository) NodesForOwner(_ context.Context, ownerID string) ([]Node, error) {
	var out []Node
	err := r.k.Scan(db.NSNodes, func(doc []byte) error {
		var n Node
		if err := db.UnmarshalDoc(doc, &n); err != nil {
			return err
		}
		if n.OwnerID == ownerID {
			out = append(out, n)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].EnrolledAt.Before(out[j].EnrolledAt) })
	return out, nil
}

// GuestClaim records that a guest id turned out to be an account's.
//
// It is kept forever, and deliberately. A match played as a guest may be
// handed up by a phone weeks after that person signed in - the phone was in a
// drawer - and if the claim had expired by then the match would be credited to
// a guest nobody can sign in as. The record is small and there is one per
// guest id a person ever claimed.
type GuestClaim struct {
	GuestID   string `bson:"guestId"`
	UserID    string `bson:"userId"`
	ClaimedAt string `bson:"claimedAt"`
	// NodeID is the node whose seat receipt proved the claim, kept so a
	// support conversation can start somewhere.
	NodeID string `bson:"nodeId,omitempty"`
}

// KDBGuestClaims is the store of those records.
type KDBGuestClaims struct {
	k *db.KDB
}

// NewKDBGuestClaims returns the database-backed guest-claim store.
func NewKDBGuestClaims(k *db.KDB) *KDBGuestClaims { return &KDBGuestClaims{k: k} }

// Claim records a guest id as belonging to an account. Re-claiming by the same
// account is not an error: a phone that hands the same receipt up twice is
// doing what a phone with no acknowledgement does.
func (c *KDBGuestClaims) Claim(_ context.Context, guestID, userID, nodeID, when string) error {
	doc, err := db.MarshalDoc(GuestClaim{GuestID: guestID, UserID: userID, NodeID: nodeID, ClaimedAt: when})
	if err != nil {
		return err
	}
	return c.k.Update(db.NSGuestClaims, func(tx *db.Tx) error {
		switch cur, err := tx.Get(guestID); {
		case err == nil:
			var existing GuestClaim
			if err := db.UnmarshalDoc(cur, &existing); err != nil {
				return err
			}
			if existing.UserID == userID {
				return nil
			}
			// A guest id belongs to whoever proved it first. Letting a second
			// account take it would move somebody's history to a stranger who
			// had got hold of an old receipt.
			return db.ErrDuplicateKey
		case !db.IsNotFound(err):
			return err
		}
		return tx.Put(guestID, doc)
	})
}

// AccountForGuest answers which account a guest became, if any.
func (c *KDBGuestClaims) AccountForGuest(_ context.Context, guestID string) (string, bool, error) {
	doc, err := c.k.Get(db.NSGuestClaims, guestID)
	if db.IsNotFound(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	var claim GuestClaim
	if err := db.UnmarshalDoc(doc, &claim); err != nil {
		return "", false, err
	}
	return claim.UserID, claim.UserID != "", nil
}
