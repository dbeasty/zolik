package auth

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
)

// ErrNotFound is returned when a lookup finds nothing. Handlers turn it into
// the appropriate status; it is never surfaced to a client verbatim, because
// "no such identity" and "wrong code" must look the same from outside.
var ErrNotFound = errors.New("not found")

// ErrIdentityTaken is returned when an identity already belongs to a different
// account. It is the signal behind "this Google account is already linked to
// another player" — a refusal, not a failure.
var ErrIdentityTaken = errors.New("identity already linked to another account")

// Store is the persistence behind accounts and their identities.
type Store struct {
	users      *mongo.Collection
	identities *mongo.Collection
	codes      *mongo.Collection
	flows      *mongo.Collection
}

func NewStore(m *db.Mongo) *Store {
	c := m.Collections()
	return &Store{users: c.Users, identities: c.Identities, codes: c.LoginCodes, flows: c.OAuthFlows}
}

// --- identities ---

// FindIdentity looks up one external identity by its (provider, subject) key.
func (s *Store) FindIdentity(ctx context.Context, provider, subject string) (models.Identity, error) {
	var id models.Identity
	err := s.identities.FindOne(ctx, bson.M{"provider": provider, "subject": subject}).Decode(&id)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return models.Identity{}, ErrNotFound
	}
	return id, err
}

// InsertIdentity creates an identity, translating the unique-index violation
// into ErrIdentityTaken.
//
// That translation is the whole safety story of concurrent sign-in: two
// requests for a brand-new Google account both see no identity, both try to
// create one, and exactly one succeeds. The loser is told the identity is
// taken and re-reads it, which lands them on the account the winner just made
// instead of a second account holding half their history.
func (s *Store) InsertIdentity(ctx context.Context, id models.Identity) (models.Identity, error) {
	if id.CreatedAt.IsZero() {
		id.CreatedAt = time.Now().UTC()
	}
	res, err := s.identities.InsertOne(ctx, id)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return models.Identity{}, ErrIdentityTaken
		}
		return models.Identity{}, err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		id.ID = oid
	}
	return id, nil
}

// ListIdentities returns every identity attached to an account, oldest first.
func (s *Store) ListIdentities(ctx context.Context, userID string) ([]models.Identity, error) {
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}})
	cur, err := s.identities.Find(ctx, bson.M{"userId": userID}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	out := []models.Identity{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteIdentity removes one identity from an account.
func (s *Store) DeleteIdentity(ctx context.Context, userID, provider string) error {
	res, err := s.identities.DeleteOne(ctx, bson.M{"userId": userID, "provider": provider})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// TouchIdentity records a successful sign-in and refreshes the provider's
// snapshot of the person's address and name, which are display-only.
func (s *Store) TouchIdentity(ctx context.Context, id bson.ObjectID, email, displayName string) error {
	now := time.Now().UTC()
	set := bson.M{"lastLoginAt": now}
	if email != "" {
		set["email"] = email
	}
	if displayName != "" {
		set["displayName"] = displayName
	}
	_, err := s.identities.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": set})
	return err
}

// --- users ---

func (s *Store) FindUserByID(ctx context.Context, id string) (models.User, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return models.User{}, ErrNotFound
	}
	var u models.User
	err = s.users.FindOne(ctx, bson.M{"_id": oid}).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return models.User{}, ErrNotFound
	}
	return u, err
}

func (s *Store) FindUserByUsername(ctx context.Context, username string) (models.User, error) {
	var u models.User
	err := s.users.FindOne(ctx, bson.M{"username": username}).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return models.User{}, ErrNotFound
	}
	return u, err
}

// FindUserByVerifiedEmail finds an account whose address has been proven.
//
// The verified-only restriction is load-bearing rather than fussy: this lookup
// is what lets a second provider attach to an existing account automatically,
// so matching on an *unverified* address would let anyone take over an account
// by signing up elsewhere claiming its owner's address.
func (s *Store) FindUserByVerifiedEmail(ctx context.Context, email string) (models.User, error) {
	var u models.User
	err := s.users.FindOne(ctx, bson.M{"email": email, "emailVerified": true}).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return models.User{}, ErrNotFound
	}
	return u, err
}

// UsernameTaken reports whether a name is already in use.
func (s *Store) UsernameTaken(ctx context.Context, username string) (bool, error) {
	n, err := s.users.CountDocuments(ctx, bson.M{"username": username}, options.Count().SetLimit(1))
	return n > 0, err
}

func (s *Store) InsertUser(ctx context.Context, u models.User) (models.User, error) {
	now := time.Now().UTC()
	if u.CreatedAt.IsZero() {
		u.CreatedAt = now
	}
	u.LastSeenAt = now
	res, err := s.users.InsertOne(ctx, u)
	if err != nil {
		return models.User{}, err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		u.ID = oid
	}
	return u, nil
}

func (s *Store) UpdateUser(ctx context.Context, id bson.ObjectID, set bson.M) error {
	if len(set) == 0 {
		return nil
	}
	_, err := s.users.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": set})
	return err
}

// --- one-time login codes ---

func (s *Store) InsertLoginCode(ctx context.Context, c models.LoginCode) error {
	_, err := s.codes.InsertOne(ctx, c)
	return err
}

// LatestLoginCode returns the most recent unexpired code for an address.
//
// Only the newest is ever considered: requesting a second code must invalidate
// the first, or a code mailed to an address the person no longer controls
// stays usable for its full lifetime.
func (s *Store) LatestLoginCode(ctx context.Context, email string) (models.LoginCode, error) {
	opts := options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	var c models.LoginCode
	err := s.codes.FindOne(ctx, bson.M{
		"email":     email,
		"expiresAt": bson.M{"$gt": time.Now().UTC()},
	}, opts).Decode(&c)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return models.LoginCode{}, ErrNotFound
	}
	return c, err
}

// CountRecentLoginCodes counts codes issued to an address since a cut-off,
// which is what the per-address request throttle is built on.
func (s *Store) CountRecentLoginCodes(ctx context.Context, email string, since time.Time) (int, error) {
	n, err := s.codes.CountDocuments(ctx, bson.M{"email": email, "createdAt": bson.M{"$gte": since}})
	return int(n), err
}

// ConsumeLoginCode marks a code used, and does so conditionally: the update
// only matches a code that has not been consumed yet.
//
// That condition is what makes redemption single-use even if two requests
// arrive at once — the second update matches nothing and the caller is told
// the code is spent, rather than both being allowed through.
func (s *Store) ConsumeLoginCode(ctx context.Context, id bson.ObjectID) error {
	now := time.Now().UTC()
	res, err := s.codes.UpdateOne(ctx,
		bson.M{"_id": id, "consumedAt": bson.M{"$exists": false}},
		bson.M{"$set": bson.M{"consumedAt": now}},
	)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// RecordCodeAttempt counts a wrong guess so a code can be burned by brute
// force as well as by use.
func (s *Store) RecordCodeAttempt(ctx context.Context, id bson.ObjectID) error {
	_, err := s.codes.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$inc": bson.M{"attempts": 1}})
	return err
}

// --- in-flight oauth redirects ---

func (s *Store) InsertFlow(ctx context.Context, f models.OAuthFlow) error {
	_, err := s.flows.InsertOne(ctx, f)
	return err
}

func (s *Store) FindFlowByState(ctx context.Context, state string) (models.OAuthFlow, error) {
	var f models.OAuthFlow
	err := s.flows.FindOne(ctx, bson.M{"state": state}).Decode(&f)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return models.OAuthFlow{}, ErrNotFound
	}
	return f, err
}

// CompleteFlow attaches the result and the one-time exchange code to a flow,
// but only if it has not been completed already — a provider that delivers the
// same callback twice must not mint two sessions.
func (s *Store) CompleteFlow(ctx context.Context, id bson.ObjectID, exchangeCode string, result models.OAuthFlowResult) error {
	res, err := s.flows.UpdateOne(ctx,
		bson.M{"_id": id, "exchangeCode": bson.M{"$exists": false}},
		bson.M{"$set": bson.M{"exchangeCode": exchangeCode, "result": result}},
	)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// TakeFlowResult atomically reads and destroys a completed flow, so the
// exchange code works exactly once. Deleting rather than flagging means a
// replayed exchange finds nothing at all.
func (s *Store) TakeFlowResult(ctx context.Context, exchangeCode string) (models.OAuthFlow, error) {
	var f models.OAuthFlow
	err := s.flows.FindOneAndDelete(ctx, bson.M{
		"exchangeCode": exchangeCode,
		"expiresAt":    bson.M{"$gt": time.Now().UTC()},
	}).Decode(&f)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return models.OAuthFlow{}, ErrNotFound
	}
	return f, err
}
