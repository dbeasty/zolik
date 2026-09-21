package match

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
)

// oldDocument is a match as the server stored it before moves and boards left
// the match document: board, log and checkpoints inline, as BSON binary.
type oldDocument struct {
	models.Match `bson:",inline"`
	State        json.RawMessage `bson:"state,omitempty"`
	ActionLog    []oldAction     `bson:"actionLog,omitempty"`
	Checkpoints  []oldCheckpoint `bson:"checkpoints,omitempty"`
}

type oldAction struct {
	Seq      int             `bson:"seq"`
	PlayerID string          `bson:"playerId"`
	Action   json.RawMessage `bson:"action"`
	At       time.Time       `bson:"at"`
}

type oldCheckpoint struct {
	Seq   int             `bson:"seq"`
	Round int             `bson:"round"`
	State json.RawMessage `bson:"state,omitempty"`
	At    time.Time       `bson:"at"`
}

func writeOld(t *testing.T, k *db.KDB, doc oldDocument) {
	t.Helper()
	raw, err := db.MarshalDoc(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := k.Insert(db.NSMatches, doc.ID.Hex(), raw); err != nil {
		t.Fatal(err)
	}
}

// A played game in the old layout comes across whole: every move with its
// round mark, the board it ended on, and a replay identical to one folded
// from the original log.
func TestMigrationCarriesAnOldMatchAcross(t *testing.T) {
	ctx := context.Background()
	for _, name := range []string{"canasta", "ginrummy", "prsi"} {
		t.Run(name, func(t *testing.T) {
			h := newLiveHarness(t)
			g := withRounds(t, name)
			played, final := playOut(t, g, 21)
			log := fixtures.logOf(played.ID)

			old := oldDocument{Match: played}
			old.Snapshots = nil
			old.Version = 7
			old.UpdatedAt = time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)
			old.State = json.RawMessage(final)
			for _, mv := range log {
				old.ActionLog = append(old.ActionLog, oldAction{Seq: mv.Seq, PlayerID: mv.PlayerID,
					Action: json.RawMessage(mv.Action), At: mv.At.Truncate(time.Millisecond)})
				if mv.Rounds > 0 {
					old.Checkpoints = append(old.Checkpoints, oldCheckpoint{Seq: mv.Seq, Round: mv.Rounds,
						State: json.RawMessage(`{"a":"board"}`)})
				}
			}
			writeOld(t, h.k, old)
			// A lobby that was never dealt carries no old fields at all.
			writeOld(t, h.k, oldDocument{Match: models.Match{ID: bson.NewObjectID(), ModuleID: name,
				Status: "lobby", Version: 1}})

			report, err := MigrateKDB(ctx, h.k, false)
			if err != nil {
				t.Fatalf("migrate: %v", err)
			}
			if report.Matches != 1 || report.Moves != len(log) || report.Skipped != 1 {
				t.Fatalf("report %s, want 1 match with %d moves and 1 skipped", report, len(log))
			}

			raw, _ := h.k.Get(db.NSMatches, played.ID.Hex())
			var left legacyParts
			_ = db.UnmarshalDoc(raw, &left)
			if left.present() {
				t.Error("the old fields are still on the match")
			}
			if env, _ := h.repo.FindByID(ctx, played.ID); !env.UpdatedAt.Equal(old.UpdatedAt) {
				t.Errorf("migration moved UpdatedAt from %v to %v; it is not activity", old.UpdatedAt, env.UpdatedAt)
			}
			if got := h.board(t, played.ID.Hex()); string(got) != string(final) {
				t.Error("the board rebuilt after migration differs from the old one")
			}
			moves, _ := h.repo.Moves(ctx, played.ID, 0, -1)
			if roundsClosed(moves) != roundsClosed(log) {
				t.Errorf("round marks: %d after, %d before", roundsClosed(moves), roundsClosed(log))
			}

			env, _ := h.repo.FindByID(ctx, played.ID)
			migrated, err := h.m.BuildReplay(ctx, env, "p1", ReplayOptions{Limit: MaxReplayFrames})
			if err != nil {
				t.Fatalf("replay after migration: %v", err)
			}
			original, err := replayManager().BuildReplay(ctx, played, "p1", ReplayOptions{Limit: MaxReplayFrames})
			if err != nil {
				t.Fatal(err)
			}
			if a, b := mustJSON(t, migrated.Frames), mustJSON(t, original.Frames); a != b {
				t.Error("the replay after migration differs from one folded from the original log")
			}
			if a, b := mustJSON(t, migrated.Chapters), mustJSON(t, original.Chapters); a != b {
				t.Errorf("chapters changed:\n after:  %s\n before: %s", a, b)
			}

			again, err := MigrateKDB(ctx, h.k, false)
			if err != nil || again.Matches != 0 || again.Skipped != 2 {
				t.Errorf("a second run: %s, %v — want nothing migrated", again, err)
			}
		})
	}
}
