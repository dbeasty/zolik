package stats

import (
	"fmt"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db/doccodec"
)

// A match result as one is stored: the row the sign-in scan and every history
// list read, many times over.
func benchMatchResult() MatchResult {
	m := MatchResult{
		ID: bson.NewObjectID(), MatchID: bson.NewObjectID(), ModuleID: "zolik", Variation: "zolik_classic",
		StartedAt: time.Now().Add(-20 * time.Minute), CompletedAt: time.Now(), DurationSeconds: 1200,
		Composition: Composition{Players: 3, Users: 1, Guests: 1, AIs: 1},
		Winners:     []string{"p1"}, RecordedAt: time.Now(),
	}
	for i := 0; i < 3; i++ {
		m.Participants = append(m.Participants, Standing{PlayerID: fmt.Sprintf("p%d", i), Name: fmt.Sprintf("Player %d", i), Seat: i, Score: 100 * i})
		m.SubjectKeys = append(m.SubjectKeys, fmt.Sprintf("guest:%032x", i))
	}
	return m
}

func BenchmarkMatchResultDecode(b *testing.B) {
	v := benchMatchResult()
	legacy, _ := bson.MarshalExtJSON(v, false, false)
	plain, _ := doccodec.Marshal(v)
	b.Run("extjson-driver", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			var m MatchResult
			if err := bson.UnmarshalExtJSON(legacy, false, &m); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("extjson-doccodec", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			var m MatchResult
			if err := doccodec.Unmarshal(legacy, &m); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("json-doccodec", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			var m MatchResult
			if err := doccodec.Unmarshal(plain, &m); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkMatchResultEncode(b *testing.B) {
	v := benchMatchResult()
	b.Run("extjson-driver", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := bson.MarshalExtJSON(v, false, false); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("json-doccodec", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := doccodec.Marshal(v); err != nil {
				b.Fatal(err)
			}
		}
	})
}
