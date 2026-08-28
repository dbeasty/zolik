package db

import (
	"encoding/json"
	"errors"
	"fmt"

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

// KDBFindUserByUsername scans the users namespace for an account by name,
// returning ErrNotFound when no account holds it.
func KDBFindUserByUsername(k *KDB, username string) (models.User, error) {
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

// KDBUserClash reports ErrDuplicateKey if any account other than self
// already holds the username or (non-empty) email — the stand-in for the
// users collection's unique username index and sparse-unique email index.
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
	return k.Update(NSUsers, func(tx *Tx) error {
		if name, ok := update["username"].(string); ok {
			if err := KDBUserClash(tx, id, name, ""); err != nil {
				return err
			}
		}
		if email, ok := update["email"].(string); ok {
			if err := KDBUserClash(tx, id, "", email); err != nil {
				return err
			}
		}
		cur, err := tx.Get(id.Hex())
		if err != nil {
			if IsNotFound(err) {
				// UpdateOne on a missing document matches nothing and is not
				// an error; keep that shape.
				return nil
			}
			return err
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
		return tx.Put(id.Hex(), next)
	})
}
