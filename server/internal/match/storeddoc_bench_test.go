package match

import (
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db/doccodec"
	"zolik/server/internal/models"
)

// A board snapshot as one is stored: a module's state (here about 6 KB) inside
// the row. The legacy shape holds it as an escaped string, the plain one as
// itself.
func BenchmarkSnapshotDecode(b *testing.B) {
	state := `{"seed":4611686018427387903,"hands":[` + strings.Repeat(`["AS","KD","QH","JC","10S","9D","8H","7C","Joker"],`, 60) + `[]],"turn":2}`
	v := snapshotRecord{Match: bson.NewObjectID(), Kind: kindSnapshot, Seq: 120, State: models.JSONDoc(state)}
	legacy, _ := bson.MarshalExtJSON(v, false, false)
	plain, _ := doccodec.Marshal(v)
	b.Logf("state %d B; stored legacy %d B, plain %d B", len(state), len(legacy), len(plain))
	for _, c := range []struct {
		name string
		data []byte
		dec  func([]byte, any) error
	}{
		{"extjson-driver", legacy, func(d []byte, v any) error { return bson.UnmarshalExtJSON(d, false, v) }},
		{"extjson-doccodec", legacy, doccodec.Unmarshal},
		{"json-doccodec", plain, doccodec.Unmarshal},
	} {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(c.data)))
			for b.Loop() {
				var s snapshotRecord
				if err := c.dec(c.data, &s); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
