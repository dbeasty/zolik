package db

import "go.mongodb.org/mongo-driver/v2/mongo"

type Collections struct {
	Games *mongo.Collection
	// Matches holds module-hosted games (see models.Match). Separate from
	// Games rather than a migration of it: the Žolíky documents already in
	// there have a rummy-shaped schema, and moving them is a one-shot script
	// worth running only once the module path is the live one.
	Matches  *mongo.Collection
	Users    *mongo.Collection
	Sessions *mongo.Collection
	Scoring  *mongo.Collection
	// MatchResults holds one immutable record per finished match, and
	// PlayerStats the lifetime aggregates derived from them. The aggregates
	// are a cache of the records, never the other way round. These replaced
	// the old per-user Statistics collection.
	MatchResults *mongo.Collection
	PlayerStats  *mongo.Collection
	// Identities maps an external identity (Google subject, verified email,
	// claimed guest id) onto an account. Separate from Users so the
	// (provider, subject) unique index can do the work of guaranteeing that
	// one identity never attaches to two accounts — see models.Identity.
	Identities *mongo.Collection
	// LoginCodes holds the one-time codes mailed for passwordless email
	// sign-in, and OAuthFlows the in-flight browser redirects. Both are
	// short-lived and TTL-swept.
	LoginCodes *mongo.Collection
	OAuthFlows *mongo.Collection
	// Feedback holds bug reports and suggestions sent from the clients. Kept
	// as its own collection rather than hung off a user, because most of it
	// arrives from guests — who have no account document to hang it on.
	Feedback *mongo.Collection
}

func (m *Mongo) Collections() Collections {
	return Collections{
		Games:        m.DB.Collection("games"),
		Matches:      m.DB.Collection("matches"),
		Users:        m.DB.Collection("users"),
		Sessions:     m.DB.Collection("sessions"),
		Scoring:      m.DB.Collection("scoring_sessions"),
		MatchResults: m.DB.Collection("match_results"),
		PlayerStats:  m.DB.Collection("player_stats"),
		Identities:   m.DB.Collection("identities"),
		LoginCodes:   m.DB.Collection("login_codes"),
		OAuthFlows:   m.DB.Collection("oauth_flows"),
		Feedback:     m.DB.Collection("feedback"),
	}
}
