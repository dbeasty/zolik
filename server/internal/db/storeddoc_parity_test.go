package db

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db/doccodec/doccodectest"
	"zolik/server/internal/models"
)

// Every document this package stores on the kdb backend reads the same through
// doccodec as it did through the driver's Extended JSON codec, from a legacy row
// and from a plain one. A type added to what this package stores belongs here.
func TestStoredDocumentsReadTheSameThroughDoccodec(t *testing.T) {
	doccodectest.Check(t,
		expiryProbe{}, Reservation{},
		// mirrorPreferences' row, spelled as it is there.
		struct {
			models.UserPreferences `bson:",inline"`
			UpdatedAt              time.Time `bson:"updatedAt"`
		}{},
		// KDBUpdateUserFields' patch.
		bson.M{},
	)
}
