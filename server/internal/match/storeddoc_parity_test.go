package match

import (
	"testing"

	"zolik/server/internal/db/doccodec/doccodectest"
)

// Every document this package stores on the kdb backend reads the same through
// doccodec as it did through the driver's Extended JSON codec, from a legacy row
// and from a plain one. A type added to what this package stores belongs here.
func TestStoredDocumentsReadTheSameThroughDoccodec(t *testing.T) {
	doccodectest.Check(t,
		moveRecord{}, snapshotRecord{}, logProbe{}, matchIDProbe{}, legacyParts{}, LocalSave{},
		// RememberPass's row, and passOf's view of it.
		map[string]any{},
		struct {
			Pass string `bson:"pass"`
		}{},
	)
}
