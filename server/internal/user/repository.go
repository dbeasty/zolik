package user

import (
	"context"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
)

// Repository is the persistence behind accounts. The Mongo-backed
// implementation is the only one today; the interface exists so a future
// backend can be swapped in behind it without touching any consumer.
type Repository interface {
	CreateUser(ctx context.Context, u models.User) (models.User, error)
	FindByUsername(ctx context.Context, username string) (models.User, error)
	FindByID(ctx context.Context, id bson.ObjectID) (models.User, error)
	UpdateByID(ctx context.Context, id bson.ObjectID, update bson.M) error
	// DeleteByID removes an account outright. Nothing else is cascaded from
	// here — identities, sessions and match records each belong to another
	// package, and the caller is what knows which of them to follow.
	DeleteByID(ctx context.Context, id bson.ObjectID) error
	// ListUsers pages the roster, newest account first.
	ListUsers(ctx context.Context, q Query) ([]models.User, error)
	// CountUsers counts the accounts a query matches, ignoring its paging —
	// it is the total the pager is drawn from.
	CountUsers(ctx context.Context, q Query) (int64, error)
	// CountUsersSeenSince counts accounts active since a cut-off. LastSeenAt
	// is the only activity signal an account document carries.
	CountUsersSeenSince(ctx context.Context, since time.Time) (int64, error)
}

// Query filters and pages the account roster.
type Query struct {
	// Search matches username or email as a case-insensitive substring.
	Search string
	Limit  int
	Skip   int
}

// filter builds the Mongo predicate for a query.
//
// regexp.QuoteMeta is load-bearing rather than tidiness: the search term is
// caller input that lands inside a $regex, so left as-is a "." would widen the
// match and a nested quantifier could pin a core backtracking. Escaping makes
// the term match itself literally, which is also the only behaviour someone
// searching for "o.brien" would expect.
func (q Query) filter() bson.M {
	term := strings.TrimSpace(q.Search)
	if term == "" {
		return bson.M{}
	}
	pattern := regexp.QuoteMeta(term)
	return bson.M{"$or": []bson.M{
		{"username": bson.M{"$regex": pattern, "$options": "i"}},
		{"email": bson.M{"$regex": pattern, "$options": "i"}},
	}}
}

type mongoRepository struct {
	users *mongo.Collection
}

func NewRepository(m *db.Mongo) Repository {
	c := m.Collections()
	return &mongoRepository{users: c.Users}
}

var _ Repository = (*mongoRepository)(nil)

func (r *mongoRepository) CreateUser(ctx context.Context, u models.User) (models.User, error) {
	now := time.Now().UTC()
	if u.CreatedAt.IsZero() {
		u.CreatedAt = now
	}
	u.LastSeenAt = now
	res, err := r.users.InsertOne(ctx, u)
	if err != nil {
		return models.User{}, err
	}
	u.ID = res.InsertedID.(bson.ObjectID)
	return u, nil
}

func (r *mongoRepository) FindByUsername(ctx context.Context, username string) (models.User, error) {
	var u models.User
	if err := r.users.FindOne(ctx, bson.M{"username": username}).Decode(&u); err != nil {
		return models.User{}, err
	}
	return u, nil
}

func (r *mongoRepository) FindByID(ctx context.Context, id bson.ObjectID) (models.User, error) {
	var u models.User
	if err := r.users.FindOne(ctx, bson.M{"_id": id}).Decode(&u); err != nil {
		return models.User{}, err
	}
	return u, nil
}

func (r *mongoRepository) UpdateByID(ctx context.Context, id bson.ObjectID, update bson.M) error {
	_, err := r.users.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	return err
}

func (r *mongoRepository) DeleteByID(ctx context.Context, id bson.ObjectID) error {
	res, err := r.users.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (r *mongoRepository) ListUsers(ctx context.Context, q Query) ([]models.User, error) {
	limit := q.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	skip := q.Skip
	if skip < 0 {
		skip = 0
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}, {Key: "_id", Value: -1}}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cur, err := r.users.Find(ctx, q.filter(), opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	out := []models.User{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *mongoRepository) CountUsers(ctx context.Context, q Query) (int64, error) {
	return r.users.CountDocuments(ctx, q.filter())
}

func (r *mongoRepository) CountUsersSeenSince(ctx context.Context, since time.Time) (int64, error) {
	return r.users.CountDocuments(ctx, bson.M{"lastSeenAt": bson.M{"$gte": since}})
}
