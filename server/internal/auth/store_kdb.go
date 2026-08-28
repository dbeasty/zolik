package auth

import (
	"context"
	"errors"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
)

// kdbStore is the KDB implementation of Store. Document keys are chosen so
// that every unique index the Mongo schema leans on becomes uniqueness by
// construction: an identity lives at "provider\x00subject", so the
// load-bearing "one external identity, one account" invariant is the
// engine's insert-if-absent rather than a scan; an OAuth flow lives at its
// state. Login codes keep their ObjectID key and are found by scanning,
// which at "codes mailed in the last fifteen minutes" scale is a handful of
// documents.
type kdbStore struct {
	k *db.KDB
}

func NewKDBStore(k *db.KDB) Store {
	return &kdbStore{k: k}
}

var _ Store = (*kdbStore)(nil)

var errStopScan = errors.New("stop scan")

func identityKey(provider, subject string) string {
	// NUL cannot appear in either half, so the compound key cannot be forged
	// by a subject that happens to contain a separator.
	return provider + "\x00" + subject
}

// --- identities ---

func (s *kdbStore) FindIdentity(ctx context.Context, provider, subject string) (models.Identity, error) {
	doc, err := s.k.Get(db.NSIdentities, identityKey(provider, subject))
	if err != nil {
		if db.IsNotFound(err) {
			return models.Identity{}, ErrNotFound
		}
		return models.Identity{}, err
	}
	var id models.Identity
	if err := db.UnmarshalDoc(doc, &id); err != nil {
		return models.Identity{}, err
	}
	return id, nil
}

func (s *kdbStore) InsertIdentity(ctx context.Context, id models.Identity) (models.Identity, error) {
	if id.CreatedAt.IsZero() {
		id.CreatedAt = time.Now().UTC()
	}
	if id.ID.IsZero() {
		id.ID = bson.NewObjectID()
	}
	doc, err := db.MarshalDoc(id)
	if err != nil {
		return models.Identity{}, err
	}
	if err := s.k.Insert(db.NSIdentities, identityKey(id.Provider, id.Subject), doc); err != nil {
		if db.IsDuplicateKey(err) {
			// Same translation the Mongo store makes from its unique-index
			// violation: the loser of a concurrent sign-in re-reads and lands
			// on the winner's account.
			return models.Identity{}, ErrIdentityTaken
		}
		return models.Identity{}, err
	}
	return id, nil
}

func (s *kdbStore) ListIdentities(ctx context.Context, userID string) ([]models.Identity, error) {
	out := []models.Identity{}
	err := s.k.Scan(db.NSIdentities, func(doc []byte) error {
		var id models.Identity
		if err := db.UnmarshalDoc(doc, &id); err != nil {
			return err
		}
		if id.UserID == userID {
			out = append(out, id)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (s *kdbStore) DeleteIdentity(ctx context.Context, userID, provider string) error {
	var key string
	found := false
	err := s.k.Scan(db.NSIdentities, func(doc []byte) error {
		var id models.Identity
		if err := db.UnmarshalDoc(doc, &id); err != nil {
			return err
		}
		if id.UserID == userID && id.Provider == provider {
			key, found = identityKey(id.Provider, id.Subject), true
			return errStopScan
		}
		return nil
	})
	if err != nil && !errors.Is(err, errStopScan) {
		return err
	}
	if !found {
		return ErrNotFound
	}
	deleted, err := s.k.Delete(db.NSIdentities, key)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrNotFound
	}
	return nil
}

func (s *kdbStore) TouchIdentity(ctx context.Context, id bson.ObjectID, email, displayName string) error {
	return s.k.Update(db.NSIdentities, func(tx *db.Tx) error {
		var target models.Identity
		found := false
		err := tx.Scan(func(doc []byte) error {
			var ident models.Identity
			if err := db.UnmarshalDoc(doc, &ident); err != nil {
				return err
			}
			if ident.ID == id {
				target, found = ident, true
				return errStopScan
			}
			return nil
		})
		if err != nil && !errors.Is(err, errStopScan) {
			return err
		}
		if !found {
			return nil // UpdateOne on a missing document matches nothing
		}
		now := time.Now().UTC()
		target.LastLoginAt = &now
		if email != "" {
			target.Email = email
		}
		if displayName != "" {
			target.DisplayName = displayName
		}
		doc, err := db.MarshalDoc(target)
		if err != nil {
			return err
		}
		return tx.Put(identityKey(target.Provider, target.Subject), doc)
	})
}

// --- users ---

func (s *kdbStore) FindUserByID(ctx context.Context, id string) (models.User, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return models.User{}, ErrNotFound
	}
	doc, err := s.k.Get(db.NSUsers, oid.Hex())
	if err != nil {
		if db.IsNotFound(err) {
			return models.User{}, ErrNotFound
		}
		return models.User{}, err
	}
	var u models.User
	if err := db.UnmarshalDoc(doc, &u); err != nil {
		return models.User{}, err
	}
	return u, nil
}

func (s *kdbStore) FindUserByUsername(ctx context.Context, username string) (models.User, error) {
	u, err := db.KDBFindUserByUsername(s.k, username)
	if db.IsNotFound(err) {
		return models.User{}, ErrNotFound
	}
	return u, err
}

func (s *kdbStore) FindUserByVerifiedEmail(ctx context.Context, email string) (models.User, error) {
	var out models.User
	found := false
	err := s.k.Scan(db.NSUsers, func(doc []byte) error {
		var u models.User
		if err := db.UnmarshalDoc(doc, &u); err != nil {
			return err
		}
		// Verified-only, same as the Mongo filter: matching an unverified
		// address would let anyone capture an account by claiming it.
		if u.Email == email && u.EmailVerified {
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

func (s *kdbStore) UsernameTaken(ctx context.Context, username string) (bool, error) {
	_, err := db.KDBFindUserByUsername(s.k, username)
	if db.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *kdbStore) InsertUser(ctx context.Context, u models.User) (models.User, error) {
	now := time.Now().UTC()
	if u.CreatedAt.IsZero() {
		u.CreatedAt = now
	}
	u.LastSeenAt = now
	if u.ID.IsZero() {
		u.ID = bson.NewObjectID()
	}
	doc, err := db.MarshalDoc(u)
	if err != nil {
		return models.User{}, err
	}
	err = s.k.Update(db.NSUsers, func(tx *db.Tx) error {
		if err := db.KDBUserClash(tx, u.ID, u.Username, u.Email); err != nil {
			return err
		}
		return tx.Insert(u.ID.Hex(), doc)
	})
	if err != nil {
		return models.User{}, err
	}
	return u, nil
}

func (s *kdbStore) UpdateUser(ctx context.Context, id bson.ObjectID, set bson.M) error {
	return db.KDBUpdateUserFields(s.k, id, set)
}

// --- one-time login codes ---

func (s *kdbStore) InsertLoginCode(ctx context.Context, c models.LoginCode) error {
	if c.ID.IsZero() {
		c.ID = bson.NewObjectID()
	}
	doc, err := db.MarshalDoc(c)
	if err != nil {
		return err
	}
	return s.k.Insert(db.NSLoginCodes, c.ID.Hex(), doc)
}

func (s *kdbStore) LatestLoginCode(ctx context.Context, email string) (models.LoginCode, error) {
	now := time.Now().UTC()
	var latest models.LoginCode
	found := false
	err := s.k.Scan(db.NSLoginCodes, func(doc []byte) error {
		var c models.LoginCode
		if err := db.UnmarshalDoc(doc, &c); err != nil {
			return err
		}
		if c.Email != email || !c.ExpiresAt.After(now) {
			return nil
		}
		if !found || c.CreatedAt.After(latest.CreatedAt) {
			latest, found = c, true
		}
		return nil
	})
	if err != nil {
		return models.LoginCode{}, err
	}
	if !found {
		return models.LoginCode{}, ErrNotFound
	}
	return latest, nil
}

func (s *kdbStore) CountRecentLoginCodes(ctx context.Context, email string, since time.Time) (int, error) {
	n := 0
	err := s.k.Scan(db.NSLoginCodes, func(doc []byte) error {
		var c models.LoginCode
		if err := db.UnmarshalDoc(doc, &c); err != nil {
			return err
		}
		if c.Email == email && !c.CreatedAt.Before(since) {
			n++
		}
		return nil
	})
	return n, err
}

func (s *kdbStore) ConsumeLoginCode(ctx context.Context, id bson.ObjectID) error {
	return s.k.Update(db.NSLoginCodes, func(tx *db.Tx) error {
		doc, err := tx.Get(id.Hex())
		if err != nil {
			if db.IsNotFound(err) {
				return ErrNotFound
			}
			return err
		}
		var c models.LoginCode
		if err := db.UnmarshalDoc(doc, &c); err != nil {
			return err
		}
		if c.ConsumedAt != nil {
			// The conditional update only matches an unconsumed code — this
			// is what keeps redemption single-use under concurrent requests.
			return ErrNotFound
		}
		now := time.Now().UTC()
		c.ConsumedAt = &now
		next, err := db.MarshalDoc(c)
		if err != nil {
			return err
		}
		return tx.Put(id.Hex(), next)
	})
}

func (s *kdbStore) RecordCodeAttempt(ctx context.Context, id bson.ObjectID) error {
	return s.k.Update(db.NSLoginCodes, func(tx *db.Tx) error {
		doc, err := tx.Get(id.Hex())
		if err != nil {
			if db.IsNotFound(err) {
				return nil // UpdateOne on a missing document matches nothing
			}
			return err
		}
		var c models.LoginCode
		if err := db.UnmarshalDoc(doc, &c); err != nil {
			return err
		}
		c.Attempts++
		next, err := db.MarshalDoc(c)
		if err != nil {
			return err
		}
		return tx.Put(id.Hex(), next)
	})
}

// --- in-flight oauth redirects ---

func (s *kdbStore) InsertFlow(ctx context.Context, f models.OAuthFlow) error {
	if f.ID.IsZero() {
		f.ID = bson.NewObjectID()
	}
	doc, err := db.MarshalDoc(f)
	if err != nil {
		return err
	}
	// Keyed by state: the unique state index made concurrent flows
	// unambiguous in Mongo, and the key makes them so here.
	return s.k.Insert(db.NSOAuthFlows, f.State, doc)
}

func (s *kdbStore) FindFlowByState(ctx context.Context, state string) (models.OAuthFlow, error) {
	doc, err := s.k.Get(db.NSOAuthFlows, state)
	if err != nil {
		if db.IsNotFound(err) {
			return models.OAuthFlow{}, ErrNotFound
		}
		return models.OAuthFlow{}, err
	}
	var f models.OAuthFlow
	if err := db.UnmarshalDoc(doc, &f); err != nil {
		return models.OAuthFlow{}, err
	}
	return f, nil
}

func (s *kdbStore) CompleteFlow(ctx context.Context, id bson.ObjectID, exchangeCode string, result models.OAuthFlowResult) error {
	return s.k.Update(db.NSOAuthFlows, func(tx *db.Tx) error {
		var target models.OAuthFlow
		found := false
		err := tx.Scan(func(doc []byte) error {
			var f models.OAuthFlow
			if err := db.UnmarshalDoc(doc, &f); err != nil {
				return err
			}
			if f.ID == id {
				target, found = f, true
				return errStopScan
			}
			return nil
		})
		if err != nil && !errors.Is(err, errStopScan) {
			return err
		}
		if !found || target.ExchangeCode != "" {
			// Only an incomplete flow may complete — a provider delivering
			// the same callback twice must not mint two sessions.
			return ErrNotFound
		}
		target.ExchangeCode = exchangeCode
		target.Result = &result
		doc, err := db.MarshalDoc(target)
		if err != nil {
			return err
		}
		return tx.Put(target.State, doc)
	})
}

func (s *kdbStore) TakeFlowResult(ctx context.Context, exchangeCode string) (models.OAuthFlow, error) {
	var out models.OAuthFlow
	now := time.Now().UTC()
	err := s.k.Update(db.NSOAuthFlows, func(tx *db.Tx) error {
		var target models.OAuthFlow
		found := false
		err := tx.Scan(func(doc []byte) error {
			var f models.OAuthFlow
			if err := db.UnmarshalDoc(doc, &f); err != nil {
				return err
			}
			if f.ExchangeCode == exchangeCode && f.ExpiresAt.After(now) {
				target, found = f, true
				return errStopScan
			}
			return nil
		})
		if err != nil && !errors.Is(err, errStopScan) {
			return err
		}
		if !found {
			return ErrNotFound
		}
		// Read-and-destroy inside the critical section, so the exchange code
		// works exactly once even against a concurrent replay.
		if _, err := tx.Delete(target.State); err != nil {
			return err
		}
		out = target
		return nil
	})
	if err != nil {
		return models.OAuthFlow{}, err
	}
	return out, nil
}
