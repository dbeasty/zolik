package db

import "go.mongodb.org/mongo-driver/v2/mongo"

type Collections struct {
	Games *mongo.Collection
	// Matches holds module-hosted games (see models.Match). Separate from
	// Games rather than a migration of it: the Žolíky documents already in
	// there have a rummy-shaped schema, and moving them is a one-shot script
	// worth running only once the module path is the live one.
	Matches    *mongo.Collection
	Users      *mongo.Collection
	Statistics *mongo.Collection
	Sessions   *mongo.Collection
	Scoring    *mongo.Collection
}

func (m *Mongo) Collections() Collections {
	return Collections{
		Games:      m.DB.Collection("games"),
		Matches:    m.DB.Collection("matches"),
		Users:      m.DB.Collection("users"),
		Statistics: m.DB.Collection("statistics"),
		Sessions:   m.DB.Collection("sessions"),
		Scoring:    m.DB.Collection("scoring_sessions"),
	}
}
