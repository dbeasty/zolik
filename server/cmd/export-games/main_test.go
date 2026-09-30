package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"zolik/server/internal/db"
	"zolik/server/internal/learn"
	"zolik/server/internal/learnexport"
	"zolik/server/internal/match"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

func TestMongoHostMustBeThisMachine(t *testing.T) {
	for _, tc := range []struct {
		uri   string
		local bool
	}{
		{"mongodb://localhost:27017", true},
		{"mongodb://127.0.0.1:27017/zolik?replicaSet=rs0", true},
		{"mongodb://user:p%40ss@localhost:27017,127.0.0.1:27018/zolik", true},
		{"mongodb://[::1]:27017", true},
		{"mongodb://%2Ftmp%2Fmongodb-27017.sock", true},
		{"mongodb://db.example.com:27017", false},
		{"mongodb://localhost:27017,db.example.com:27017", false},
		{"mongodb://user:secret@10.0.0.5:27017", false},
		{"mongodb+srv://cluster0.example.mongodb.net", false},
	} {
		shown, err := checkMongoHost(tc.uri, false)
		if (err == nil) != tc.local {
			t.Errorf("%s: err = %v, want local = %v", tc.uri, err, tc.local)
		}
		if strings.Contains(shown, "p%40ss") || (err != nil && strings.Contains(err.Error(), "secret")) {
			t.Errorf("%s: the credentials were shown", tc.uri)
		}
		if _, err := checkMongoHost(tc.uri, true); err != nil {
			t.Errorf("%s: refused with -allow-remote: %v", tc.uri, err)
		}
	}
}

// Nothing is read from a store nobody named.
func TestNoStoreIsNoDefault(t *testing.T) {
	t.Setenv("FEATURE_FLAG_DB_ENGINE", "")
	t.Setenv("MONGO_URI", "")
	t.Setenv("KDB_PATH", "")
	err := run(t.Context(), []string{"-game", "holdem", "-salt", "s", "-out", filepath.Join(t.TempDir(), "x.jsonl")}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "no store") {
		t.Fatalf("ran with no store named: %v", err)
	}
}

// The whole command against a KDB root it did not write: every finished
// hold'em match exported, gzipped, and the root left exactly as it was.
func TestExportFromAKDBRootLeavesItUntouched(t *testing.T) {
	root := filepath.Join(t.TempDir(), "kdb")
	want := seedStore(t, root, 3)
	before := treeDigest(t, root)

	out := filepath.Join(t.TempDir(), "holdem.jsonl.gz")
	var summary bytes.Buffer
	t.Setenv("FEATURE_FLAG_DB_ENGINE", "")
	t.Setenv("MONGO_URI", "")
	err := run(t.Context(), []string{"-game", "holdem", "-salt", "pepper", "-kdb", root, "-out", out}, &summary)
	if err != nil {
		t.Fatal(err)
	}
	if after := treeDigest(t, root); after != before {
		t.Fatal("the KDB root changed under a read-only export")
	}

	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	lines := 0
	sc := bufio.NewScanner(gz)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var r learnexport.Record
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			t.Fatal(err)
		}
		if r.Game != "holdem" || r.Chosen < 0 {
			t.Fatalf("record %d: %+v", lines, r)
		}
		lines++
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if lines != want {
		t.Fatalf("%d records written, %d human decisions stored", lines, want)
	}
	for _, line := range []string{"matches read:       3", fmt.Sprintf("records written:    %d", want)} {
		if !strings.Contains(summary.String(), line) {
			t.Fatalf("summary lacks %q:\n%s", line, summary.String())
		}
	}
}

// seedStore plays n hold'em matches with one human seat each and stores them
// in a fresh KDB root, then closes it. It returns how many human decisions
// were stored.
func seedStore(t *testing.T, root string, n int) int {
	t.Helper()
	k, err := db.OpenKDB(root)
	if err != nil {
		t.Fatal(err)
	}
	repo := match.NewKDBRepository(k)
	ctx := t.Context()
	g, err := learn.LookupGame("holdem")
	if err != nil {
		t.Fatal(err)
	}
	mod := g.Module()
	humans := 0
	for i := 0; i < n; i++ {
		players := []models.Player{{ID: "human-seat", Name: "A Person", UserID: "user-1"}, {ID: "bot:1", Name: "Bot", IsAI: true}}
		refs := []module.PlayerRef{{ID: "human-seat", Name: "A Person"}, {ID: "bot:1", Name: "Bot", IsAI: true}}
		seed := int64(100 + i)
		cfg := g.Config(2, "")
		deal, err := mod.NewMatch(cfg, refs, seed)
		if err != nil {
			t.Fatal(err)
		}
		state := deal
		var moves []models.MatchAction
		for {
			if done, _, _ := mod.Finished(state); done {
				break
			}
			actor := module.ActiveSeat(mod, state, refs[0].ID, refs)
			offers, err := mod.LegalActions(state, actor)
			if err != nil {
				t.Fatal(err)
			}
			a, ok := g.Heuristic().Act(state, module.BotSeat{PlayerID: actor, Seed: seed}, offers)
			if !ok {
				t.Fatalf("no move for %s", actor)
			}
			next, _, err := mod.Apply(state, actor, a)
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(a)
			moves = append(moves, models.MatchAction{Seq: len(moves) + 1, PlayerID: actor, Action: raw, At: time.Now().UTC()})
			if actor == "human-seat" {
				humans++
			}
			state = next
		}
		ended := time.Now().UTC()
		m, err := repo.Insert(ctx, models.Match{ModuleID: "holdem", Variation: cfg.Variation, Options: cfg.Options,
			Status: "completed", Players: players, Seed: seed, StartedAt: &ended, EndedAt: &ended})
		if err != nil {
			t.Fatal(err)
		}
		next := m
		next.Snapshots = []int{len(moves)} // no deal stored: rebuilt from the seed
		if err := repo.CommitSnapshot(ctx, m.ID, m.Version, next, moves, len(moves), models.JSONDoc(state)); err != nil {
			t.Fatal(err)
		}
	}
	if err := k.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	return humans
}

// treeDigest hashes every regular file under root, names and contents.
func treeDigest(t *testing.T, root string) string {
	t.Helper()
	h := sha256.New()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		fmt.Fprintf(h, "%s\x00%d\x00", rel, len(b))
		h.Write(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}
