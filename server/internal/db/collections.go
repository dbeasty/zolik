package db

import "go.mongodb.org/mongo-driver/v2/mongo"

type Collections struct {
	Games      *mongo.Collection
	Users      *mongo.Collection
	Statistics *mongo.Collection
	Sessions   *mongo.Collection
	Scoring    *mongo.Collection
}

func (m *Mongo) Collections() Collections {
	return Collections{
		Games:      m.DB.Collection("games"),
		Users:      m.DB.Collection("users"),
		Statistics: m.DB.Collection("statistics"),
		Sessions:   m.DB.Collection("sessions"),
		Scoring:    m.DB.Collection("scoring_sessions"),
	}
}
