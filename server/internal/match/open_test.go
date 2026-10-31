package match_test

import (
	"testing"
	"time"

	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// A table opened the ordinary way stays private: only a listed one is
// offered to strangers, and never to its own host, whose table is in their
// games already.
func TestOpenTablesOffersOnlyListedLobbiesToOthers(t *testing.T) {
	ctx := t.Context()
	m, _, _ := newReaperHarness(t)
	host := models.Player{ID: "host", Name: "Ann"}
	if _, err := m.Create(ctx, "prsi", module.MatchConfig{}, host); err != nil {
		t.Fatal(err)
	}
	listed, err := m.CreateListed(ctx, "prsi", module.MatchConfig{}, host)
	if err != nil {
		t.Fatal(err)
	}

	got, err := m.OpenTables(ctx, "prsi", "stranger")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].MatchID != listed.ID.Hex() {
		t.Fatalf("a stranger was offered %+v, want only the listed table", got)
	}
	if got[0].HostName != "Ann" || got[0].JoinCode != listed.JoinCode || got[0].Seated != 1 {
		t.Fatalf("the listing says %+v", got[0])
	}
	if own, _ := m.OpenTables(ctx, "prsi", "host"); len(own) != 0 {
		t.Fatalf("the host was offered their own table: %+v", own)
	}
}

// A listing is a seat on offer: once the last seat is taken it goes, so
// nobody is sent to a table that will refuse them as full.
func TestOpenTablesDropsAFullTable(t *testing.T) {
	ctx := t.Context()
	m, _, _ := newReaperHarness(t)
	listed, err := m.CreateListed(ctx, "prsi", module.MatchConfig{}, models.Player{ID: "host", Name: "Ann"})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; ; i++ {
		got, _ := m.OpenTables(ctx, "prsi", "stranger")
		if len(got) == 0 {
			break
		}
		if i > 10 {
			t.Fatal("the table never filled")
		}
		p := models.Player{ID: string(rune('a' + i)), Name: "P"}
		if _, err := m.Join(ctx, listed.JoinCode, p); err != nil {
			t.Fatalf("seat %d was listed but refused: %v", i+2, err)
		}
	}
	if _, err := m.Join(ctx, listed.JoinCode, models.Player{ID: "late", Name: "Late"}); module.CodeOf(err) != "MATCH_FULL" {
		t.Fatalf("a table no longer listed still took a player: %v", err)
	}
}

// The host withdraws a listing, and nobody else may change it.
func TestSetListedIsTheHostsAndWithdraws(t *testing.T) {
	ctx := t.Context()
	m, _, _ := newReaperHarness(t)
	lobby, err := m.Create(ctx, "prsi", module.MatchConfig{}, models.Player{ID: "host", Name: "Ann"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.SetListed(ctx, lobby.ID.Hex(), "stranger", true); module.CodeOf(err) != "NOT_THE_HOST" {
		t.Fatalf("a stranger listed somebody else's table: %v", err)
	}
	if _, err := m.SetListed(ctx, lobby.ID.Hex(), "host", true); err != nil {
		t.Fatal(err)
	}
	if got, _ := m.OpenTables(ctx, "prsi", "stranger"); len(got) != 1 {
		t.Fatalf("listing it offered %d tables", len(got))
	}
	if _, err := m.SetListed(ctx, lobby.ID.Hex(), "host", false); err != nil {
		t.Fatal(err)
	}
	if got, _ := m.OpenTables(ctx, "prsi", "stranger"); len(got) != 0 {
		t.Fatalf("a withdrawn table is still offered: %+v", got)
	}
}

// A listing lapses unless renewed, so a host who walked away from their
// lobby stops being offered. Pinned at the repository, where the cut-off is
// applied: nothing written since a moment from now is offered.
func TestFindOpenSkipsAListingNotRenewedSince(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)
	if _, err := m.CreateListed(ctx, "prsi", module.MatchConfig{}, models.Player{ID: "host", Name: "Ann"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.FindOpen(ctx, "prsi", time.Now().Add(-time.Minute), 10); len(got) != 1 {
		t.Fatalf("a fresh listing was not found: %d", len(got))
	}
	if got, _ := repo.FindOpen(ctx, "prsi", time.Now().Add(time.Minute), 10); len(got) != 0 {
		t.Fatalf("a listing older than the cut-off was found: %d", len(got))
	}
}
