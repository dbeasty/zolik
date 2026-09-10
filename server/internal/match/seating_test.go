package match_test

import (
	"context"
	"testing"

	"zolik/server/internal/canasta"
	"zolik/server/internal/match"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/prsi"
	"zolik/server/internal/ws"
	"zolik/server/internal/zolikmod"
)

// Reordering the seats is the whole of "form teams", so what these check is
// that the order a host chooses is the order the game is dealt in.

func seatingManager(t *testing.T) *match.Manager {
	t.Helper()
	hub, err := ws.NewHub(ws.NewConnRegistry(), "")
	if err != nil {
		t.Fatalf("building a hub: %v", err)
	}
	t.Cleanup(func() { _ = hub.Close() })
	mods := module.NewRegistry(zolikmod.New(), prsi.New(), canasta.New())
	return match.NewManager(newTestRepository(t), mods, hub)
}

func seatedTable(t *testing.T, m *match.Manager, seats int) models.Match {
	t.Helper()
	ctx := context.Background()
	host := models.Player{ID: "p1", Name: "p1", UserID: "p1"}
	created, err := m.Create(ctx, "canasta", module.MatchConfig{Variation: "classic"}, host)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	for i := 2; i <= seats; i++ {
		id := "p" + string(rune('0'+i))
		if _, err := m.Join(ctx, created.ID.Hex(), models.Player{ID: id, Name: id, UserID: id}); err != nil {
			t.Fatalf("Join %s: %v", id, err)
		}
	}
	out, err := m.Repo().Resolve(ctx, created.ID.Hex())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	return out
}

func TestSeatReordersTheTable(t *testing.T) {
	m := seatingManager(t)
	table := seatedTable(t, m, 4)

	// p1 wants p4 as a partner. Partners sit opposite, so p4 takes the third
	// chair — which is the point of the whole feature: saying where people sit
	// *is* saying who plays with whom.
	want := []string{"p1", "p2", "p4", "p3"}
	next, err := m.Seat(context.Background(), table.ID.Hex(), "p1", want)
	if err != nil {
		t.Fatalf("Seat: %v", err)
	}
	for i, id := range want {
		if next.Players[i].ID != id {
			t.Errorf("seat %d is %s, want %s", i, next.Players[i].ID, id)
		}
		if next.TurnOrder[i] != id {
			t.Errorf("turn order %d is %s, want %s", i, next.TurnOrder[i], id)
		}
	}

	// And the deal agrees: the module is handed the seats in this order, so the
	// table it deals is the table the host arranged.
	started, err := m.Start(context.Background(), table.ID.Hex())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	for i, id := range want {
		if started.TurnOrder[i] != id {
			t.Errorf("dealt turn order %d is %s, want %s", i, started.TurnOrder[i], id)
		}
	}
}

func TestSeatRefusesAnythingThatIsNotAPermutation(t *testing.T) {
	cases := []struct {
		name  string
		order []string
	}{
		{"a seat left out", []string{"p1", "p2", "p3"}},
		{"somebody twice", []string{"p1", "p2", "p3", "p3"}},
		{"a stranger", []string{"p1", "p2", "p3", "p9"}},
		{"too many", []string{"p1", "p2", "p3", "p4", "p1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := seatingManager(t)
			table := seatedTable(t, m, 4)
			before := append([]string(nil), table.TurnOrder...)

			if _, err := m.Seat(context.Background(), table.ID.Hex(), "p1", tc.order); err == nil {
				t.Fatal("accepted an order that is not a permutation of the table")
			}
			// Refused, not half-applied.
			after, err := m.Repo().Resolve(context.Background(), table.ID.Hex())
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			for i := range before {
				if after.TurnOrder[i] != before[i] {
					t.Fatalf("a refused seating still moved the table: %v", after.TurnOrder)
				}
			}
		})
	}
}

func TestOnlyTheHostSeatsAndOnlyBeforeTheDeal(t *testing.T) {
	m := seatingManager(t)
	table := seatedTable(t, m, 4)
	order := []string{"p1", "p2", "p4", "p3"}

	if _, err := m.Seat(context.Background(), table.ID.Hex(), "p2", order); module.CodeOf(err) != "NOT_THE_HOST" {
		t.Errorf("a non-host reseating the table got %v, want NOT_THE_HOST", err)
	}
	if _, err := m.Start(context.Background(), table.ID.Hex()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Afterwards the seating is what the hands were dealt against; moving it
	// would reassign cards already held.
	if _, err := m.Seat(context.Background(), table.ID.Hex(), "p1", order); module.CodeOf(err) != "MATCH_ALREADY_STARTED" {
		t.Errorf("reseating a dealt table got %v, want MATCH_ALREADY_STARTED", err)
	}
}
