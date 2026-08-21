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

	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt" json:"updatedAt"`
}
