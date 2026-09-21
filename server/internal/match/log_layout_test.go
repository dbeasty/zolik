package match

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// storeGame writes a game played in memory into the store move by move, as
// HandleAction would have — the real states, the real checkpoints — leaving
// the last `held` moves unwritten. It returns the stored match and the moves
// it held back.
func storeGame(t *testing.T, s logStore, g replayable, seed int64, held int) (models.Match, []models.MatchAction) {
	t.Helper()
	ctx := context.Background()
	played, _ := playOut(t, g, seed)
	log := fixtures.logOf(played.ID)
	if len(log) <= held {
		t.Fatalf("%s played only %d moves", g.name, len(log))
	}

	deal, err := g.mod.NewMatch(g.cfg, g.players, seed)
	if err != nil {
		t.Fatal(err)
	}
	m, err := s.repo.Insert(ctx, models.Match{
		ModuleID: g.name, Variation: g.cfg.Variation, Options: g.cfg.Options,
		Status: "active", Players: played.Players, Seed: seed, State: models.JSONDoc(deal),
		LogFormat: newMatchLogFormat,
	})
	if err != nil {
		t.Fatal(err)
	}
	return appendMoves(t, s, g, m, log[:len(log)-held]), log[len(log)-held:]
}

func appendMoves(t *testing.T, s logStore, g replayable, m models.Match, moves []models.MatchAction) models.Match {
	t.Helper()
	ctx := context.Background()
	for _, e := range moves {
		var a module.Action
		if err := json.Unmarshal(e.Action, &a); err != nil {
			t.Fatal(err)
		}
		next, _, err := g.mod.Apply(module.State(m.State), e.PlayerID, a)
		if err != nil {
			t.Fatalf("move %d no longer applies: %v", e.Seq, err)
		}
		m.State = models.JSONDoc(next)
		cp, board, ok := checkpointClosedBy(m, e.Seq, module.RoundsFor(g.mod, next))
		var cpp *models.MatchCheckpoint
		if ok {
			cpp = &cp
		}
		if err := s.repo.AppendAction(ctx, m.ID, m.Version, m, e, cpp, board); err != nil {
			t.Fatalf("append %d: %v", e.Seq, err)
		}
		if m, err = s.repo.FindByID(ctx, m.ID); err != nil {
			t.Fatal(err)
		}
	}
	return m
}

// A match written in the original layout replays frame for frame the same
// after the move that carries it into pages — history, boards, every hidden
// card projected for the viewer — and the new move follows on after it.
func TestReplayIsUnchangedByTheMoveToPages(t *testing.T) {
	for _, name := range []string{"canasta", "prsi", "ginrummy"} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			s := openLogStore(t)
			g := withRounds(t, name)
			m, held := storeGame(t, s, g, 21, 1)
			mgr := &Manager{repo: s.repo, registry: replayManager().registry, replayEnabled: true}

			before, err := mgr.BuildReplay(ctx, m, "p1", ReplayOptions{Limit: MaxReplayFrames})
			if err != nil {
				t.Fatalf("replay before: %v", err)
			}

			withSwitches(t, "", true)
			m = appendMoves(t, s, g, m, held)
			if raw, _ := s.k.Get(db.NSMatches, m.ID.Hex()); !strings.Contains(string(raw), `"logFormat":"pages"`) {
				t.Fatalf("the move did not carry the match into pages")
			}

			after, err := mgr.BuildReplay(ctx, m, "p1", ReplayOptions{Limit: MaxReplayFrames})
			if err != nil {
				t.Fatalf("replay after: %v", err)
			}
			if after.Total != before.Total+1 {
				t.Fatalf("total %d after one more move, was %d", after.Total, before.Total)
			}
			for i, f := range before.Frames {
				want, _ := json.Marshal(f)
				got, _ := json.Marshal(after.Frames[i])
				if string(got) != string(want) {
					t.Fatalf("frame %d changed across the move to pages:\n before %s\n after  %s", i, want, got)
				}
			}
			for _, from := range []int{0, before.Total / 2, before.Total - 1} {
				page, err := mgr.BuildReplay(ctx, m, "p1", ReplayOptions{From: from, Limit: 5})
				if err != nil {
					t.Fatalf("page from %d: %v", from, err)
				}
				for i, f := range page.Frames {
					want, _ := json.Marshal(after.Frames[from+i])
					got, _ := json.Marshal(f)
					if string(got) != string(want) {
						t.Fatalf("a page from %d disagrees with the whole replay at frame %d", from, from+i)
					}
				}
			}
		})
	}
}

// What the layouts cost per move: the bytes a move commits — which is what
// the store keeps in its history and what a restart decodes back into memory —
// for a board the size of a four-player canasta state and a 70-byte action.
// The original layout rewrites the whole log on every move; pages rewrite one
// page. Disk is logged too, but compresses the repeated log away, which is
// exactly why it hid the problem.
func TestPagesWriteAFixedAmountPerMove(t *testing.T) {
	if testing.Short() {
		t.Skip("writes thousands of commits")
	}
	const moves = 1000
	state := models.JSONDoc(fmt.Sprintf(`{"board":%q}`, strings.Repeat("QC,QS,5D,", 400)))
	committed := map[string]float64{}
	for _, format := range []string{"", models.LogFormatPages} {
		dir := t.TempDir()
		k, err := db.OpenKDB(dir)
		if err != nil {
			t.Fatal(err)
		}
		repo := NewKDBRepository(k)
		ctx := context.Background()
		m, err := repo.Insert(ctx, models.Match{ModuleID: "canasta", Status: "active", State: state, LogFormat: format})
		if err != nil {
			t.Fatal(err)
		}
		written := func(seq int) int64 {
			doc, err := k.Get(db.NSMatches, m.ID.Hex())
			if err != nil {
				t.Fatal(err)
			}
			n := int64(len(doc))
			if format == models.LogFormatPages {
				page, err := k.Get(db.NSMatchLog, pageKey(m.ID.Hex(), pageOf(seq)))
				if err != nil {
					t.Fatal(err)
				}
				n += int64(len(page))
			}
			return n
		}
		var bytes, disk500 int64
		for i := 1; i <= moves; i++ {
			entry := models.MatchAction{Seq: i, PlayerID: "0efb065d6f1eaca97ca5b759fc3fb34d",
				Action: models.JSONDoc(`{"offerId":"lay_meld:5","verb":"lay_meld","cards":["5D","5H","2D"]}`),
				At:     time.Now().UTC()}
			if err := repo.AppendAction(ctx, m.ID, m.Version, m, entry, nil, nil); err != nil {
				t.Fatal(err)
			}
			if m, err = repo.FindByID(ctx, m.ID); err != nil {
				t.Fatal(err)
			}
			if i > moves/2 {
				bytes += written(i)
			}
			if i == moves/2 {
				disk500 = dirBytes(t, filepath.Join(dir, "ns"))
			}
		}
		committed[format] = float64(bytes) / float64(moves/2)
		disk := float64(dirBytes(t, filepath.Join(dir, "ns"))-disk500) / float64(moves/2)
		_ = k.Close(ctx)
		t.Logf("layout %-7q: %7.1f KB committed per move, %5.1f KB on disk, over moves 501-1000",
			format, committed[format]/1024, disk/1024)
	}
	if pages := committed[models.LogFormatPages]; pages > 16*1024 {
		t.Errorf("pages committed %.1f KB per move; a move should cost about one state and one page", pages/1024)
	}
	if committed[""] < 10*committed[models.LogFormatPages] {
		t.Errorf("pages (%.0f B) saved less than 10x over the original layout (%.0f B) past move 500",
			committed[models.LogFormatPages], committed[""])
	}
}

func dirBytes(t *testing.T, root string) int64 {
	t.Helper()
	var total int64
	err := filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err == nil {
			total += info.Size()
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return total
}
