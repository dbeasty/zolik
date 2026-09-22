package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/models"
)

// The users namespace has two writers — internal/user's repository and
// internal/auth's store, exactly as both packages share the Mongo users
// collection. The unique-constraint and field-patch logic they must agree on
// lives here because they cannot share it any other way: user imports auth
// for its middleware, so auth importing user back would be a cycle.

// errStopScan aborts a scan that found what it wanted.
var errStopScan = errors.New("stop scan")

// KDBFindUserByUsername returns the account holding a name, or ErrNotFound.
//
// The name's reservation answers this directly: one read of a document id
// derived from the name. The scan behind it is for databases whose accounts
// predate reservations and have not been through cmd/migrate-namespaces yet;
// it goes when that migration has run everywhere.
func KDBFindUserByUsername(k *KDB, username string) (models.User, error) {
	switch owner, err := KDBReservationOwner(k, ReservationUsername, username); {
	case err == nil:
		id, err := bson.ObjectIDFromHex(owner)
		if err != nil {
			return models.User{}, fmt.Errorf("kdb: reservation for %q holds %q, which is not a user id", username, owner)
		}
		doc, err := k.Get(NSUsers, id.Hex())
		if err != nil {
			return models.User{}, err
		}
		var u models.User
		if err := UnmarshalDoc(doc, &u); err != nil {
			return models.User{}, err
		}
		return u, nil
	case !IsNotFound(err):
		return models.User{}, err
	}
	return kdbScanUserByUsername(k, username)
}

func kdbScanUserByUsername(k *KDB, username string) (models.User, error) {
	var out models.User
	found := false
	err := k.Scan(NSUsers, func(doc []byte) error {
		var u models.User
		if err := UnmarshalDoc(doc, &u); err != nil {
			return err
		}
		if u.Username == username {
			out, found = u, true
			return errStopScan
		}
		return nil
	})
	if err != nil && !errors.Is(err, errStopScan) {
		return models.User{}, err
	}
	if !found {
		return models.User{}, ErrNotFound
	}
	return out, nil
}

// KDBReserveUserNames claims a new account's username and email as part of
// tx, which must hold both the users and the reservations namespace.
//
// This is what the unique username index and the sparse-unique email index do
// under Mongo. It replaces a scan of every account: a scan can only see this
// node's copy, so two nodes would each find a name free and each take it,
// whereas both claims land on one derived document id and the second is
// refused.
func KDBReserveUserNames(tx *MultiTx, self bson.ObjectID, username, email string) error {
	if err := KDBReserve(tx, ReservationUsername, username, self.Hex()); err != nil {
		return err
	}
	return KDBReserve(tx, ReservationEmail, email, self.Hex())
}

// KDBRetakeUserName moves a claim from one value to another for the same
// owner: the old value is given up and the new one taken, or nothing happens
// at all, because both are operations in one transaction.
func KDBRetakeUserName(tx *MultiTx, self bson.ObjectID, kind, from, to string) error {
	if strings.EqualFold(from, to) && ReservationKey(kind, from) == ReservationKey(kind, to) {
		return nil
	}
	if err := KDBReserve(tx, kind, to, self.Hex()); err != nil {
		return err
	}
	return KDBRelease(tx, kind, from, self.Hex())
}

// KDBUserClash reports ErrDuplicateKey if any account other than self
// already holds the username or (non-empty) email — the pre-reservation
// stand-in for those indexes, kept for databases that have not been migrated.
func KDBUserClash(tx *Tx, self bson.ObjectID, username, email string) error {
	return tx.Scan(func(doc []byte) error {
		var u models.User
		if err := UnmarshalDoc(doc, &u); err != nil {
			return err
		}
		if u.ID == self {
			return nil
		}
		if username != "" && u.Username == username {
			return fmt.Errorf("username %q: %w", username, ErrDuplicateKey)
		}
		if email != "" && u.Email == email {
			return fmt.Errorf("email %q: %w", email, ErrDuplicateKey)
		}
		return nil
	})
}

// KDBInsertUser writes a new account and claims its username and email in one
// transaction: an account that exists without holding its own name, or a name
// held for an account that was never written, are both states nothing here
// would know how to repair.
func KDBInsertUser(k *KDB, u models.User) error {
	doc, err := MarshalDoc(u)
	if err != nil {
		return err
	}
	return k.UpdateMulti([]string{NSUsers, NSReservations}, func(tx *MultiTx) error {
		if _, err := tx.Get(NSUsers, u.ID.Hex()); err == nil {
			return fmt.Errorf("kdb: user %s: %w", u.ID.Hex(), ErrDuplicateKey)
		} else if !IsNotFound(err) {
			return err
		}
		if err := KDBReserveUserNames(tx, u.ID, u.Username, u.Email); err != nil {
			return err
		}
		tx.Put(NSUsers, u.ID.Hex(), doc)
		return nil
	})
}

// KDBUpdateUserFields applies a flat field map to the stored user document —
// the same shape both Mongo implementations pass to $set. The two call
// sites (profile patch, sign-in touch) only ever use flat top-level keys;
// dotted paths are not part of either repository's contract.
func KDBUpdateUserFields(k *KDB, id bson.ObjectID, update bson.M) error {
	if len(update) == 0 {
		return nil
	}
	patch, err := MarshalDoc(update)
	if err != nil {
		return err
	}
	var patchFields map[string]json.RawMessage
	if err := json.Unmarshal(patch, &patchFields); err != nil {
		return err
	}
	return k.UpdateMulti([]string{NSUsers, NSReservations}, func(tx *MultiTx) error {
		cur, err := tx.Get(NSUsers, id.Hex())
		if err != nil {
			if IsNotFound(err) {
				// UpdateOne on a missing document matches nothing and is not
				// an error; keep that shape.
				return nil
			}
			return err
		}
		var before models.User
		if err := UnmarshalDoc(cur, &before); err != nil {
			return err
		}
		// A name this account gives up has to become free in the same
		// transaction that takes the new one, or a failed rename leaves the
		// old name claimed by nobody.
		if name, ok := update["username"].(string); ok {
			if err := KDBRetakeUserName(tx, id, ReservationUsername, before.Username, name); err != nil {
				return err
			}
		}
		if email, ok := update["email"].(string); ok {
			if err := KDBRetakeUserName(tx, id, ReservationEmail, before.Email, email); err != nil {
				return err
			}
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(cur, &fields); err != nil {
			return err
		}
		for key, v := range patchFields {
			fields[key] = v
		}
		next, err := json.Marshal(fields)
		if err != nil {
			return err
		}
		tx.Put(NSUsers, id.Hex(), next)
		return nil
	})
}
