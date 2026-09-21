package match_test

import (
	"context"
	"sync"
	"testing"

	"zolik/server/internal/db"
	"zolik/server/internal/match"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/prsi"
	"zolik/server/internal/ws"
)

// closedLog records every LobbyClosed the runtime reports.
type closedLog struct {
	mu  sync.Mutex
	ids []string
}

func (c *closedLog) LobbyClosed(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ids = append(c.ids, id)
}

func (c *closedLog) count(id string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, x := range c.ids {
		if x == id {
			n++
		}
	}
	return n
}

func observedManager(t *testing.T) (*match.Manager, *closedLog) {
	t.Helper()
	k, err := db.OpenKDB(t.TempDir())
	if err != nil {
		t.Fatalf("opening kdb: %v", err)
	}
	t.Cleanup(func() { _ = k.Close(context.Background()) })
	hub, err := ws.NewHub(ws.NewConnRegistry(), "")
	if err != nil {
		t.Fatal(err)
	}
	m := match.NewManager(match.NewKDBRepository(k), module.NewRegistry(prsi.New()), hub)
	log := &closedLog{}
	m.SetLobbyObserver(log)
	return m, log
}

func human(id string) models.Player { return models.Player{ID: id, Name: id, UserID: id} }

// An invite sent about a table has to be withdrawn the moment nobody else can
// sit down at it — dealt, full, or gone — or it sends people to a door that
// answers MATCH_ALREADY_STARTED.
func TestLobbyObserverHearsWhenATableStopsTakingPlayers(t *testing.T) {
	ctx := context.Background()

	t.Run("started", func(t *testing.T) {
		m, log := observedManager(t)
		lobby, err := m.Create(ctx, "prsi", module.MatchConfig{}, human("host"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := m.Join(ctx, lobby.ID.Hex(), human("guest")); err != nil {
			t.Fatal(err)
		}
		if log.count(lobby.ID.Hex()) != 0 {
			t.Fatal("a seat left open is not a closed lobby")
		}
		if _, err := m.Start(ctx, lobby.ID.Hex()); err != nil {
			t.Fatal(err)
		}
		if log.count(lobby.ID.Hex()) != 1 {
			t.Fatal("starting must close the lobby")
		}
	})

	t.Run("full", func(t *testing.T) {
		m, log := observedManager(t)
		lobby, err := m.Create(ctx, "prsi", module.MatchConfig{}, human("host"))
		if err != nil {
			t.Fatal(err)
		}
		_, max := m.Registry().Get("prsi").Descriptor().SeatRange("")
		for i := 1; i < max; i++ {
			if _, err := m.Join(ctx, lobby.ID.Hex(), human(string(rune('a'+i)))); err != nil {
				t.Fatal(err)
			}
		}
		if log.count(lobby.ID.Hex()) != 1 {
			t.Fatalf("taking the last of %d seats must close the lobby once", max)
		}
	})

	t.Run("deleted", func(t *testing.T) {
		m, log := observedManager(t)
		lobby, err := m.Create(ctx, "prsi", module.MatchConfig{}, human("host"))
		if err != nil {
			t.Fatal(err)
		}
		if err := m.DeleteAsHost(ctx, lobby.ID.Hex(), "host"); err != nil {
			t.Fatal(err)
		}
		if log.count(lobby.ID.Hex()) != 1 {
			t.Fatal("deleting a lobby must close it")
		}
	})
}
