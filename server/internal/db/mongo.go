package db

import (
	"context"
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

func (m *Mongo) EnsureIndexes(ctx context.Context) error {
	c := m.Collections()

	// games
	if _, err := c.Games.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "status", Value: 1}}},
		{Keys: bson.D{{Key: "players.userId", Value: 1}}},
		{Keys: bson.D{{Key: "abandonAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
	}); err != nil {
		return err
	}

	// users
	if _, err := c.Users.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "username", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetSparse(true).SetUnique(true)},
	}); err != nil {
		return err
	}

	// match_results — the unique gameId index is load-bearing, not just an
	// optimisation: it is what makes recording a finished match idempotent,
	// so a retried or concurrently-observed completion writes one record
	// instead of double-counting every participant's lifetime statistics.
	if _, err := c.MatchResults.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "gameId", Value: 1}}, Options: options.Index().SetUnique(true)},
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
	}); err != nil {
		return err
	}

	return nil
}

