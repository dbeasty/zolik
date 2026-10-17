package replica

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db/doccodec"
)

// What a client is handed from the replica does not depend on which shape the
// row was stored in: Extended JSON from an older build, or plain JSON from a
// newer one (internal/db/doccodec).
func TestClientJSONIsTheSameForBothStoredShapes(t *testing.T) {
	type row struct {
		Kind      string          `bson:"_kind"`
		Match     bson.ObjectID   `bson:"match"`
		When      time.Time       `bson:"completedAt"`
		Seats     []string        `bson:"seats"`
		Score     map[string]int  `bson:"score"`
		Nested    struct{ A int } `bson:"nested"`
		Big       int64           `bson:"big"`
		Ratio     float64         `bson:"ratio"`
		Untouched *string         `bson:"untouched"`
	}
	v := row{Kind: "match", Match: bson.NewObjectID(), When: time.Date(2026, 10, 15, 7, 1, 2, 345_000_000, time.UTC),
		Seats: []string{"a", "b"}, Score: map[string]int{"a": 3}, Big: 1 << 40, Ratio: 0.25}
	v.Nested.A = 9
	legacy, err := bson.MarshalExtJSON(v, false, false)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := doccodec.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	a, err := asClientJSON(legacy)
	if err != nil {
		t.Fatal(err)
	}
	b, err := asClientJSON(plain)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Fatalf("a client is sent\n%s\nfor a legacy row and\n%s\nfor a plain one", a, b)
	}
	if completedAt(a) != completedAt(b) {
		t.Fatalf("history sorts the two rows differently: %s vs %s", completedAt(a), completedAt(b))
	}
}
