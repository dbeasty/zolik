package db

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Config struct {
	URI string
	DB  string
}

type Mongo struct {
	Client *mongo.Client
	DB     *mongo.Database
}

func Connect(ctx context.Context, cfg Config) (*Mongo, error) {
	if cfg.URI == "" {
		return nil, fmt.Errorf("mongo uri is empty")
	}
	if cfg.DB == "" {
		return nil, fmt.Errorf("mongo db is empty")
	}

	client, err := mongo.Connect(options.Client().ApplyURI(cfg.URI))
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(ctx)
		return nil, err
	}

	return &Mongo{
		Client: client,
		DB:     client.Database(cfg.DB),
	}, nil
}

func (m *Mongo) Close(ctx context.Context) error {
	return m.Client.Disconnect(ctx)
}

// isMissingIndex reports whether an index drop failed only because there was
// nothing there — a fresh database, or one already migrated.
func isMissingIndex(err error) bool {
	var ce mongo.CommandError
	if errors.As(err, &ce) {
		// 26 NamespaceNotFound, 27 IndexNotFound.
		return ce.Code == 26 || ce.Code == 27
	}
	return false
}

func (m *Mongo) EnsureIndexes(ctx context.Context) error {
	c := m.Collections()

	// games
	//
	// abandonAt deliberately carries no TTL index. It used to, and the index
	// destroyed games: abandonAt is not "delete me at", it is "decide about me
	// at", and match.StartReaper is what decides — it marks the table
	// abandoned, clears abandonAt, and leaves the row. A TTL index on the same
	// field made that a race between Mongo's expiry monitor and the reaper,
	// and when Mongo won the whole match was gone: no abandoned row for the
	// statistics, and a player following their own link got a table that had
	// never existed rather than one that had ended.
	if _, err := c.Games.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "status", Value: 1}}},
		{Keys: bson.D{{Key: "players.userId", Value: 1}}},
	}); err != nil {
		return err
	}
	// Dropped rather than merely not created: an index lives in the database,
	// not in this file, so a deployment that ever ran the old build keeps
	// expiring matches until something removes it. Named by its field, which
	// is how CreateMany named it. A NamespaceNotFound or IndexNotFound simply
	// means there is nothing to undo.
	if err := c.Games.Indexes().DropOne(ctx, "abandonAt_1"); err != nil && !isMissingIndex(err) {
		return err
	}

	// users
	if _, err := c.Users.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "username", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetSparse(true).SetUnique(true)},
	}); err != nil {
		return err
	}

	// match_results — the unique matchId index is load-bearing, not just an
	// optimisation: it is what makes recording a finished match idempotent,
	// so a retried or concurrently-observed completion writes one record
	// instead of double-counting every participant's lifetime statistics.
	if _, err := c.MatchResults.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "matchId", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "subjectKeys", Value: 1}, {Key: "completedAt", Value: -1}}},
		{Keys: bson.D{{Key: "completedAt", Value: -1}}},
	}); err != nil {
		return err
	}

	// player_stats
	if _, err := c.PlayerStats.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "subjectKey", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "subject.kind", Value: 1}, {Key: "overall.wins", Value: -1}}},
	}); err != nil {
		return err
	}

	// sessions
	if _, err := c.Sessions.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "token", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
		// Guest sessions are looked up by guest id when a signed-in account
		// claims a device's play history.
		{Keys: bson.D{{Key: "guestId", Value: 1}}, Options: options.Index().SetSparse(true)},
	}); err != nil {
		return err
	}

	// identities — the unique (provider, subject) index is the load-bearing
	// one. It is not an optimisation: it is the only thing that makes "this
	// Google account belongs to exactly one user here" true under concurrent
	// first sign-ins, where two requests can both find no existing identity
	// and both try to create one. The loser gets a duplicate-key error and
	// re-reads, instead of quietly minting a second account for the same
	// person — which would strand half their statistics.
	if _, err := c.Identities.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "provider", Value: 1}, {Key: "subject", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{Keys: bson.D{{Key: "userId", Value: 1}}},
	}); err != nil {
		return err
	}

	// login_codes — TTL on expiresAt is what retires spent codes, so nothing
	// has to sweep them and a forgotten code cannot be redeemed a week later.
	if _, err := c.LoginCodes.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "email", Value: 1}, {Key: "createdAt", Value: -1}}},
		{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
	}); err != nil {
		return err
	}

	// oauth_flows
	if _, err := c.OAuthFlows.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "state", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "exchangeCode", Value: 1}}, Options: options.Index().SetSparse(true)},
		{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
	}); err != nil {
		return err
	}

	return nil
}
