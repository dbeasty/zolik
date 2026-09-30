package stats

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db"
)

func openIndexTestKDB(t *testing.T) *db.KDB {
	t.Helper()
	// On disk rather than in memory: a namespace per account is opened on
	// demand, and only a file-backed database can do that.
	k, err := db.OpenKDB(t.TempDir())
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	t.Cleanup(func() { _ = k.Close(context.Background()) })
	return k
}

func TestAFinishedMatchIsIndexedForEveryAccountThatPlayedIt(t *testing.T) {
	k := openIndexTestKDB(t)
	repo := NewKDBRepository(k)
	ctx := context.Background()

	ada := bson.NewObjectID().Hex()
	result := MatchResult{
		MatchID:     bson.NewObjectID(),
		ModuleID:    "canasta",
		Variation:   "samba",
		CompletedAt: time.Now().UTC(),
		SubjectKeys: []string{"user:" + ada, "guest:abcd", "ai:steady"},
	}
	if _, err := repo.InsertMatch(ctx, result); err != nil {
		t.Fatalf("insert: %v", err)
	}

	doc, err := k.Get(db.UserReadOnlyNS(ada), "matches/"+result.MatchID.Hex())
	if err != nil {
		t.Fatalf("the account's match index: %v", err)
	}
	var entry struct {
		Kind     string `bson:"_kind"`
		ModuleID string `bson:"moduleId"`
		Finished bool   `bson:"finished"`
	}
	if err := db.UnmarshalDoc(doc, &entry); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if entry.ModuleID != "canasta" || !entry.Finished || entry.Kind != db.KindMatch {
		t.Fatalf("index entry = %+v", entry)
	}

	// Recording the same match again is refused, and does not double-index it.
	if _, err := repo.InsertMatch(ctx, result); err != ErrAlreadyRecorded {
		t.Fatalf("second insert: got %v, want ErrAlreadyRecorded", err)
	}
}

func TestLifetimeStatsAreMirroredWhereADeviceCanReadButNotWriteThem(t *testing.T) {
	k := openIndexTestKDB(t)
	repo := NewKDBRepository(k)
	ctx := context.Background()

	ada := bson.NewObjectID().Hex()
	ps := PlayerStats{SubjectKey: "user:" + ada, Subject: Subject{Kind: SubjectUser}, Overall: Tally{Matches: 3}}
	if err := repo.UpsertPlayerStats(ctx, ps); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	doc, err := k.Get(db.UserReadOnlyNS(ada), "stats")
	if err != nil {
		t.Fatalf("mirrored stats: %v", err)
	}
	var got struct {
		Overall struct {
			Matches int `bson:"matches"`
		} `bson:"overall"`
		Kind string `bson:"_kind"`
	}
	if err := db.UnmarshalDoc(doc, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Overall.Matches != 3 || got.Kind != db.KindStats {
		t.Fatalf("mirrored stats = %+v", got)
	}

	// A guest's aggregates stay where they are: there is no account namespace
	// to mirror them into.
	guest := PlayerStats{SubjectKey: "guest:abcd", Subject: Subject{Kind: SubjectGuest}, Overall: Tally{Matches: 1}}
	if err := repo.UpsertPlayerStats(ctx, guest); err != nil {
		t.Fatalf("guest upsert: %v", err)
	}
}
