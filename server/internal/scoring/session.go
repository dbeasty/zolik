package scoring

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type PlayerScore struct {
	Name   string `bson:"name" json:"name"`
	Scores []int  `bson:"scores" json:"scores"` // length up to 7
}

type ScoringSession struct {
	ID        bson.ObjectID `bson:"_id,omitempty" json:"id"`
	OwnerUser string        `bson:"ownerUser,omitempty" json:"ownerUser,omitempty"`

	Players []PlayerScore `bson:"players" json:"players"`
	Rounds  int           `bson:"rounds" json:"rounds"` // always 7 for spec

	// Version counts the writes, so a patch made against an old reading
	// is refused rather than quietly undoing a newer one. A pad stored
	// before versions existed reads as 0.
	Version int `bson:"version" json:"version"`

	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt" json:"updatedAt"`
}
