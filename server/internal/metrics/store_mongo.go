package metrics

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"zolik/server/internal/db"
)

type mongoStore struct {
	days  *mongo.Collection
	boots *mongo.Collection
}

// NewMongoStore builds the Mongo-backed store.
func NewMongoStore(m *db.Mongo) Store {
	c := m.Collections()
	return &mongoStore{days: c.DailyMetrics, boots: c.Boots}
}

var _ Store = (*mongoStore)(nil)

func (s *mongoStore) Merge(ctx context.Context, day string, counters map[string]int64, players []string, truncated bool) error {
	update := bson.M{}

	if len(counters) > 0 {
		inc := bson.M{}
		for name, n := range counters {
			inc["counters."+encodeCounterName(name)] = n
		}
		update["$inc"] = inc
	}

	if len(players) > 0 {
		// $addToSet is the whole reason this is one round trip: the set
		// semantics live in the database, so two processes flushing the same
		// player concurrently still produce one entry.
		update["$addToSet"] = bson.M{"players": bson.M{"$each": players}}
	}

	set := bson.M{}
	if truncated {
		set["playersTruncated"] = true
	}
	if len(set) > 0 {
		update["$set"] = set
	}
	if len(update) == 0 {
		return nil
	}
	update["$setOnInsert"] = bson.M{"_id": day}

	_, err := s.days.UpdateOne(ctx, bson.M{"_id": day}, update, options.UpdateOne().SetUpsert(true))
	return err
}

func (s *mongoStore) Days(ctx context.Context, from, to string) ([]Day, error) {
	cur, err := s.days.Find(ctx,
		bson.M{"_id": bson.M{"$gte": from, "$lte": to}},
		options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}),
	)
	if err != nil {
		return nil, err
	}
	// Decoded through a raw shape rather than straight into Day, because the
	// stored field names are encoded — see encodeCounterName.
	var rows []struct {
		Date             string   `bson:"_id"`
		Counters         bson.M   `bson:"counters"`
		Players          []string `bson:"players"`
		PlayersTruncated bool     `bson:"playersTruncated"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	out := make([]Day, 0, len(rows))
	for _, r := range rows {
		out = append(out, Day{
			Date:             r.Date,
			Counters:         readCounters(r.Counters),
			Players:          r.Players,
			PlayersTruncated: r.PlayersTruncated,
		})
	}
	return out, nil
}

// counterSeparator is what a dot in a counter name becomes on its way into
// Mongo.
//
// The dots have to go, and not merely for tidiness. Mongo reads a dot in an
// update key as a path, so `$inc: {"counters.matches.completed": 1}` does not
// increment a field called "matches.completed" — it walks two levels into a
// subdocument. That works right up until the day two counters in the same
// family are both live: `matches.completed` becomes the *number* at
// counters.matches.completed, and `matches.completed.zolik` then needs that
// same number to be a *document* so it can hold a "zolik" field. Mongo
// refuses, the whole flush errors, and every counter in it is held back — a
// storage detail taking out the unrelated numbers next to it.
//
// A colon cannot appear in a counter name: the constants in names.go have
// none, and sanitise maps everything outside [a-z0-9_-] to an underscore. So
// the encoding is total and reversible, and every counter is one flat field.
const counterSeparator = ":"

func encodeCounterName(name string) string {
	return strings.ReplaceAll(name, ".", counterSeparator)
}

func decodeCounterName(field string) string {
	return strings.ReplaceAll(field, counterSeparator, ".")
}

// readCounters turns the stored counters subdocument back into the flat map
// the rest of the package uses.
func readCounters(m bson.M) map[string]int64 {
	out := make(map[string]int64, len(m))
	for field, v := range m {
		n, ok := asInt64(v)
		if !ok {
			continue
		}
		out[decodeCounterName(field)] = n
	}
	return out
}

// asInt64 accepts every numeric shape the driver may hand back. A counter
// written as an int32 and read after an $inc that promoted it is the same
// number, and the reporting layer should not have to care which.
func asInt64(v any) (int64, bool) {
	switch t := v.(type) {
	case int32:
		return int64(t), true
	case int64:
		return t, true
	case float64:
		return int64(t), true
	default:
		return 0, false
	}
}

func (s *mongoStore) InsertBoot(ctx context.Context, b Boot) error {
	_, err := s.boots.InsertOne(ctx, b)
	return err
}

func (s *mongoStore) LatestOpenBoot(ctx context.Context, exceptID string) (Boot, bool, error) {
	var b Boot
	err := s.boots.FindOne(ctx,
		bson.M{"stoppedAt": nil, "counted": bson.M{"$ne": true}, "_id": bson.M{"$ne": exceptID}},
		options.FindOne().SetSort(bson.D{{Key: "startedAt", Value: -1}}),
	).Decode(&b)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Boot{}, false, nil
	}
	if err != nil {
		return Boot{}, false, err
	}
	return b, true, nil
}

func (s *mongoStore) CloseBoot(ctx context.Context, id string, stoppedAt time.Time, reason Reason, counted bool) error {
	set := bson.M{"stoppedAt": stoppedAt}
	if reason != "" {
		set["reason"] = reason
	}
	if counted {
		set["counted"] = true
	}
	_, err := s.boots.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": set})
	return err
}

func (s *mongoStore) Boots(ctx context.Context, from, to time.Time) ([]Boot, error) {
	cur, err := s.boots.Find(ctx,
		bson.M{"startedAt": bson.M{"$gte": from, "$lte": to}},
		options.Find().SetSort(bson.D{{Key: "startedAt", Value: 1}}),
	)
	if err != nil {
		return nil, err
	}
	var out []Boot
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}
