package db

import (
	"fmt"
	"strings"
	"time"
)

// Reservations are how this engine holds a unique constraint: one document
// per claimed value, keyed by the value itself, so "is this username taken?"
// is a direct read of a document id rather than a scan of every account.
//
// The scan they replace was correct only because one process held the whole
// database: it read every row inside a critical section no other writer in
// this process could enter. A replicating peer enters no critical section of
// ours at all, and two nodes scanning their own copy of the users namespace
// would both find a name free and both take it. A reservation makes that a
// conflict on one document — the same document on both nodes, because the id
// is derived from the value — which the engine can see and refuse, and which
// a resolver can settle when it does slip through on a namespace that is not
// single-homed.
//
// The kinds are deliberately short: they are part of a document id that will
// be derived on other nodes and must never drift.
const (
	// ReservationUsername claims an account name. Case-folded: names differing
	// only in case are the same claim.
	ReservationUsername = "uname"
	// ReservationEmail claims a verified address, also case-folded.
	ReservationEmail = "email"
	// ReservationFriendCode claims a notify friend code.
	ReservationFriendCode = "friend"
)

// Reservation is the stored document. Owner is what holds the value: a user's
// ObjectID hex for a name or an address, a subject key for a friend code.
type Reservation struct {
	Key       string    `bson:"_key"`
	Kind      string    `bson:"kind"`
	Value     string    `bson:"value"`
	Owner     string    `bson:"owner"`
	CreatedAt time.Time `bson:"createdAt"`
}

// ReservationKey is the document key for a claim. Folding case here rather
// than at the call sites is what makes "Ada" and "ada" one reservation on
// every node that derives it.
func ReservationKey(kind, value string) string {
	if kind == ReservationFriendCode {
		// Friend codes are generated in one case and compared literally.
		return kind + ":" + value
	}
	return kind + ":" + strings.ToLower(value)
}

// NewReservation is the document to store for a claim.
func NewReservation(kind, value, owner string) Reservation {
	return Reservation{
		Key:       ReservationKey(kind, value),
		Kind:      kind,
		Value:     value,
		Owner:     owner,
		CreatedAt: time.Now().UTC(),
	}
}

// KDBReservationOwner returns what holds value, or ErrNotFound when it is
// free.
func KDBReservationOwner(k *KDB, kind, value string) (string, error) {
	if value == "" {
		return "", ErrNotFound
	}
	doc, err := k.Get(NSReservations, ReservationKey(kind, value))
	if err != nil {
		return "", err
	}
	var r Reservation
	if err := UnmarshalDoc(doc, &r); err != nil {
		return "", err
	}
	return r.Owner, nil
}

// KDBReserve claims value for owner as part of tx, which must hold the
// reservations namespace. Re-claiming a value the same owner already holds
// succeeds and writes nothing new, so a repeated sign-up attempt or a replayed
// migration is not a failure; another owner holding it is ErrDuplicateKey.
func KDBReserve(tx *MultiTx, kind, value, owner string) error {
	if value == "" {
		return nil
	}
	key := ReservationKey(kind, value)
	switch cur, err := tx.Get(NSReservations, key); {
	case err == nil:
		var r Reservation
		if err := UnmarshalDoc(cur, &r); err != nil {
			return err
		}
		if r.Owner == owner {
			return nil
		}
		return fmt.Errorf("%s %q: %w", kind, value, ErrDuplicateKey)
	case IsNotFound(err):
	default:
		return err
	}
	doc, err := MarshalDoc(NewReservation(kind, value, owner))
	if err != nil {
		return err
	}
	tx.Put(NSReservations, key, doc)
	return nil
}

// KDBRelease gives up owner's claim on value. A claim held by somebody else is
// left alone: releasing what you do not hold is not yours to do, and silently
// dropping it would hand the value to whoever asked last.
func KDBRelease(tx *MultiTx, kind, value, owner string) error {
	if value == "" {
		return nil
	}
	key := ReservationKey(kind, value)
	cur, err := tx.Get(NSReservations, key)
	if IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var r Reservation
	if err := UnmarshalDoc(cur, &r); err != nil {
		return err
	}
	if r.Owner != owner {
		return nil
	}
	tx.Delete(NSReservations, key)
	return nil
}
