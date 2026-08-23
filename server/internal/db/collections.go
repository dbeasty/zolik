package db

import "go.mongodb.org/mongo-driver/v2/mongo"

type Collections struct {
	Games    *mongo.Collection
	Users    *mongo.Collection
	Sessions *mongo.Collection
	Scoring  *mongo.Collection
	// MatchResults holds one immutable record per finished match, and
	// PlayerStats the lifetime aggregates derived from them. The aggregates
	// are a cache of the records, never the other way round.
	MatchResults *mongo.Collection
	PlayerStats  *mongo.Collection
}

func (m *Mongo) Collections() Collections {
	return Collections{
		Games:        m.DB.Collection("games"),
		Users:        m.DB.Collection("users"),
		Sessions:     m.DB.Collection("sessions"),
		Scoring:      m.DB.Collection("scoring_sessions"),
		MatchResults: m.DB.Collection("match_results"),
		PlayerStats:  m.DB.Collection("player_stats"),
	}
}
