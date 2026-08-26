package feedback

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"zolik/server/internal/db"
)

// ErrNotFound is returned when a report lookup finds nothing.
var ErrNotFound = errors.New("feedback report not found")

// Repository is the persistence behind reports. The Mongo-backed
// implementation is the only one today; the interface exists so a future
// backend can be swapped in behind it without touching any consumer.
type Repository interface {
	Insert(ctx context.Context, r Report) (Report, error)
	// List pages reports, newest first.
	List(ctx context.Context, q Query) ([]Report, error)
	// Count counts the reports a query matches, ignoring its paging.
	Count(ctx context.Context, q Query) (int64, error)
	// CountByStatus returns how many reports sit in each triage state, so the
	// console can label its filters without paging the whole collection.
	CountByStatus(ctx context.Context) (map[string]int64, error)
	// CountRecentFrom counts reports from one reporter since a cut-off, which
	// is what the submission throttle is built on.
	CountRecentFrom(ctx context.Context, userID, guestID string, since time.Time) (int64, error)
	Update(ctx context.Context, id bson.ObjectID, set bson.M) error
	Delete(ctx context.Context, id bson.ObjectID) error
}

// Query filters and pages the report list. An empty Status or Kind means "any".
type Query struct {
	Status string
	Kind   string
	Limit  int
	Skip   int
}

func (q Query) filter() bson.M {
	f := bson.M{}
	if q.Status != "" {
		f["status"] = q.Status
	}
	if q.Kind != "" {
		f["kind"] = q.Kind
	}
	return f
}

type mongoRepository struct {
	reports *mongo.Collection
}

func NewRepository(m *db.Mongo) Repository {
	return &mongoRepository{reports: m.Collections().Feedback}
}

var _ Repository = (*mongoRepository)(nil)

func (r *mongoRepository) Insert(ctx context.Context, rep Report) (Report, error) {
	if rep.ID.IsZero() {
		rep.ID = bson.NewObjectID()
	}
	now := time.Now().UTC()
	if rep.CreatedAt.IsZero() {
		rep.CreatedAt = now
	}
	rep.UpdatedAt = now
	if _, err := r.reports.InsertOne(ctx, rep); err != nil {
		return Report{}, err
	}
	return rep, nil
}

func (r *mongoRepository) List(ctx context.Context, q Query) ([]Report, error) {
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

	cur, err := r.reports.Find(ctx, q.filter(), opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	out := []Report{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *mongoRepository) Count(ctx context.Context, q Query) (int64, error) {
	return r.reports.CountDocuments(ctx, q.filter())
}

func (r *mongoRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	cur, err := r.reports.Aggregate(ctx, []bson.M{
		{"$group": bson.M{"_id": "$status", "n": bson.M{"$sum": 1}}},
	})
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ID string `bson:"_id"`
		N  int64  `bson:"n"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	// Every known status is present even at zero, so the console's filter
	// labels do not flicker in and out as a queue empties.
	out := map[string]int64{StatusNew: 0, StatusOpen: 0, StatusResolved: 0}
	for _, row := range rows {
		out[row.ID] = row.N
	}
	return out, nil
}

func (r *mongoRepository) CountRecentFrom(ctx context.Context, userID, guestID string, since time.Time) (int64, error) {
	who := bson.M{}
	switch {
	case userID != "":
		who["userId"] = userID
	case guestID != "":
		who["guestId"] = guestID
	default:
		// Nothing identifies this reporter, so there is no history to count
		// here. Those reports are throttled by address instead — see
		// feedback.anonLimiter, which exists precisely because this cannot
		// see them.
		return 0, nil
	}
	who["createdAt"] = bson.M{"$gte": since}
	return r.reports.CountDocuments(ctx, who)
}

func (r *mongoRepository) Update(ctx context.Context, id bson.ObjectID, set bson.M) error {
	set["updatedAt"] = time.Now().UTC()
	res, err := r.reports.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": set})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *mongoRepository) Delete(ctx context.Context, id bson.ObjectID) error {
	res, err := r.reports.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}
