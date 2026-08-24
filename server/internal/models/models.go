package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Player struct {
	ID           string `bson:"id" json:"id"`
	Name         string `bson:"name" json:"name"`
	IsAI         bool   `bson:"isAI" json:"isAI"`
	AIDifficulty string `bson:"aiDifficulty" json:"aiDifficulty,omitempty"`
	ConnectionID string `bson:"connectionId" json:"connectionId,omitempty"`
	UserID       string `bson:"userId" json:"userId,omitempty"`
}

type User struct {
	ID           bson.ObjectID   `bson:"_id,omitempty" json:"id"`
	Username     string          `bson:"username" json:"username"`
	Email        string          `bson:"email,omitempty" json:"email,omitempty"`
	PasswordHash string          `bson:"passwordHash,omitempty" json:"-"`
	AuthProvider string          `bson:"authProvider" json:"authProvider"`
	CreatedAt    time.Time       `bson:"createdAt" json:"createdAt"`
	LastSeenAt   time.Time       `bson:"lastSeenAt" json:"lastSeenAt"`
	Preferences  UserPreferences `bson:"preferences" json:"preferences"`
}

type UserPreferences struct {
	Language  string `bson:"language" json:"language"`
	CardStyle string `bson:"cardStyle" json:"cardStyle"`
}

// Statistics and MatchRef used to live here: a per-user counters document
// seeded at registration and read by the stats endpoint, but never written by
// anything. Lifetime records now live in internal/stats, keyed by a subject
// that covers registered users and AI difficulties alike, and are derived
// from the immutable match records written when a match completes.

type Session struct {
	ID        bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Token     string        `bson:"token" json:"token"`
	GuestName string        `bson:"guestName" json:"guestName"`
	UserID    string        `bson:"userId,omitempty" json:"userId,omitempty"`
	CreatedAt time.Time     `bson:"createdAt" json:"createdAt"`
	ExpiresAt time.Time     `bson:"expiresAt" json:"expiresAt"`
}
