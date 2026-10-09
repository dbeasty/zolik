package match_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"zolik/server/internal/canasta"
	"zolik/server/internal/db"
	"zolik/server/internal/ginrummy"
	"zolik/server/internal/lastcard"
	"zolik/server/internal/marias"
	"zolik/server/internal/match"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/prsi"
	"zolik/server/internal/rummytiles"
	"zolik/server/internal/ws"
	"zolik/server/internal/zolikmod"
)

// Every game that pauses for a missing player has to keep going once a bot
// stands in: through its rounds, the pause between them, and every phase a
// seat can be dropped in. A stand-in is the module's own bot, so this is
// mostly the runtime's half — the seat is a person's, and nothing about that
// may stop the bot loop from playing it.
func TestAStandInKeepsEveryPausingGamePlaying(t *testing.T) {
	if testing.Short() {
		t.Skip("plays several games of each")
	}
	games := []struct {
		name string
		mod  module.GameModule
		cfg  module.MatchConfig
	}{
		{"zolik", zolikmod.New(), module.MatchConfig{}},
		{"prsi", prsi.New(), module.MatchConfig{}},
		{"lastcard", lastcard.New(), module.MatchConfig{}},
		{"canasta", canasta.New(), module.MatchConfig{}},
		{"canasta/samba", canasta.New(), module.MatchConfig{Variation: "samba"}},
		{"ginrummy", ginrummy.New(), module.MatchConfig{}},
		{"marias", marias.New(), module.MatchConfig{Options: module.Options{"deals": 9}}},
		{"marias/licitovany", marias.New(), module.MatchConfig{Variation: "licitovany", Options: module.Options{"deals": 9}}},
		{"rummytiles", rummytiles.New(), module.MatchConfig{}},
	}
	for _, g := range games {
		for run := 0; run < 3; run++ {
			t.Run(fmt.Sprintf("%s/%d", g.name, run), func(t *testing.T) {
				if g.mod.Descriptor().Variation(g.cfg.Variation) == nil && g.cfg.Variation != "" {
					t.Skipf("no variation %q", g.cfg.Variation)
				}
				playWithAStandIn(t, g.mod, g.cfg)
			})
		}
	}
}

// playWithAStandIn seats Ada and whoever else the game needs, puts a bot in
// for each of the others straight away, and plays Ada's seat from the test
// with her game's own bot, until the match ends or Ada has made enough
// decisions. It fails if the table ever sits waiting on a seat a bot is
// playing.
func playWithAStandIn(t *testing.T, mod module.GameModule, cfg module.MatchConfig) {
	t.Helper()
	ctx := t.Context()
	k, err := db.OpenKDB(t.TempDir())
	if err != nil {
		t.Fatalf("opening kdb: %v", err)
	}
	t.Cleanup(func() { _ = k.Close(context.Background()) })
	hub, err := ws.NewHub(ws.NewConnRegistry(), "")
	if err != nil {
		t.Fatalf("hub: %v", err)
	}
	t.Cleanup(func() { _ = hub.Close() })
	m := match.NewManager(match.NewKDBRepository(k), module.NewRegistry(mod), hub)
	m.SetBotPace(time.Millisecond, 2*time.Millisecond)

	id := mod.Descriptor().ID
	created, err := m.Create(ctx, id, cfg, models.Player{ID: "p1", Name: "Ada"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	matchID := created.ID.Hex()
	// Everybody but Ada is away and played for, at a table as small as the
	// game allows.
	away := []string{"p2"}
	if min, _ := mod.Descriptor().SeatRange(cfg.Variation); min > 2 {
		away = append(away, "p3")
	}
	for _, pid := range away {
		if _, err := m.Join(ctx, matchID, models.Player{ID: pid, Name: "Away " + pid}); err != nil {
			t.Fatalf("join: %v", err)
		}
	}
	if _, err := m.Start(ctx, matchID); err != nil {
		t.Fatalf("start: %v", err)
	}
	sitDown(t, m, matchID, "p1")
	for _, pid := range away {
		if _, err := m.SetStandIn(ctx, matchID, "p1", pid, true, ""); err != nil {
			t.Fatalf("stand-in: %v", err)
		}
	}

	bot := module.BotFor(mod)
	var last []byte
	changed := time.Now()
	decisions := 0
	for decisions < 150 {
		cur, err := m.Current(ctx, matchID)
		if err != nil {
			t.Fatalf("current: %v", err)
		}
		if cur.Status == "completed" {
			return
		}
		if cur.Status != "active" {
			t.Fatalf("status = %q with a stand-in in and Ada at the table", cur.Status)
		}
		if !bytes.Equal(cur.State, last) {
			last, changed = append(last[:0], cur.State...), time.Now()
		}
		if m.Awaits(ctx, matchID, "p1") {
			offers, err := mod.LegalActions(module.State(cur.State), "p1")
			if err != nil {
				t.Fatalf("offers: %v", err)
			}
			seat := module.BotSeat{PlayerID: "p1", Seed: cur.Seed}
			candidates := module.ChooseActions(offers, nil)
			if a, ok := bot.Act(module.State(cur.State), seat, offers); ok {
				candidates = append([]module.Action{a}, candidates...)
			}
			for _, a := range candidates {
				if m.HandleAction(ctx, matchID, "p1", a) == nil {
					decisions++
					break
				}
			}
			continue
		}
		if time.Since(changed) > 5*time.Second {
			awaited := module.AwaitedSeats(mod, module.State(cur.State), "p1", nil)
			t.Fatalf("the table sat for 5s waiting on %v after %d of Ada's decisions", awaited, decisions)
		}
		time.Sleep(5 * time.Millisecond)
	}
	for _, pid := range away {
		if seat(t, m, matchID, pid).StandIn == nil {
			t.Errorf("the stand-in left %s's seat while they were away", pid)
		}
	}
}
