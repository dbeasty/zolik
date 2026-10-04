package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"zolik/server/internal/admin"
	"zolik/server/internal/botgov"
	"zolik/server/internal/botsettings"
	"zolik/server/internal/db"
	"zolik/server/internal/learn"
)

func hardSwitch(game string) bool {
	for _, s := range learn.HardModels() {
		if s.Game == game {
			return s.Enabled
		}
	}
	return false
}

// A change is stored, moves the switch, and is what the next boot comes back
// to — the boot here being a second store over the same KDB directory and a
// switch reset to its default.
func TestHardModelChangeSurvivesARestart(t *testing.T) {
	t.Cleanup(func() { _, _ = learn.SetHardModel("zolik", false) })
	ctx := context.Background()
	dir := t.TempDir()

	k, err := db.OpenKDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	store := botsettings.NewKDBStore(k)
	from, err := setHardModel(ctx, store, admin.HardModelChange{Game: "zolik", Enabled: true, By: "operator", At: time.Now()})
	if err != nil || from {
		t.Fatalf("enabling: from=%v err=%v", from, err)
	}
	if !hardSwitch("zolik") {
		t.Fatal("the switch did not move")
	}
	rows, err := hardModelRows(ctx, store)
	if err != nil {
		t.Fatal(err)
	}
	var zolik admin.HardModelRow
	for _, r := range rows {
		if r.Game == "zolik" {
			zolik = r
		}
	}
	if !zolik.Enabled || !zolik.Fits || zolik.UpdatedBy != "operator" || zolik.UpdatedAt == nil || zolik.Title != "Žolíky" {
		t.Errorf("row after change: %+v", zolik)
	}
	_ = k.Close(ctx)

	// "Restart": the process default is off, then the store is loaded.
	_, _ = learn.SetHardModel("zolik", false)
	k, err = db.OpenKDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close(ctx)
	loadHardModels(ctx, botsettings.NewKDBStore(k))
	if !hardSwitch("zolik") {
		t.Error("after a restart the stored setting was not applied")
	}
	for _, g := range []string{"canasta", "holdem"} {
		if hardSwitch(g) {
			t.Errorf("%s came back on without anyone turning it on", g)
		}
	}
}

func TestHardModelChangeRefusesAnUnknownGame(t *testing.T) {
	k, err := db.OpenKDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close(context.Background())
	store := botsettings.NewKDBStore(k)
	if _, err := setHardModel(context.Background(), store, admin.HardModelChange{Game: "prsi", Enabled: true}); !errors.Is(err, admin.ErrUnknownGame) {
		t.Errorf("got %v, want ErrUnknownGame", err)
	}
	stored, _ := store.HardModels(context.Background())
	if len(stored) != 0 {
		t.Errorf("a refused change was stored: %v", stored)
	}
}

// A governor mode set in the console is stored, moves the governor at once,
// and outranks BOT_GOVERNOR at the next boot — the boot here being a second
// store over the same KDB directory and a governor built from the
// environment's value.
func TestGovernorModeChangeSurvivesARestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	k, err := db.OpenKDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	store := botsettings.NewKDBStore(k)
	gov := botgov.New(botgov.Config{Mode: botgov.Observe, Cores: 1})

	// Nothing stored: the environment's mode stands.
	loadGovernorMode(ctx, store, gov)
	if gov.Mode() != botgov.Observe {
		t.Fatalf("with nothing stored the mode moved to %s", gov.Mode())
	}

	from, err := changeGovernorMode(ctx, store, gov, admin.GovernorChange{Mode: "enforce", By: "operator", At: time.Now()})
	if err != nil || from != "observe" || gov.Mode() != botgov.Enforce {
		t.Fatalf("change: from=%s err=%v now=%s", from, err, gov.Mode())
	}
	if err := k.Close(ctx); err != nil {
		t.Fatal(err)
	}

	k2, err := db.OpenKDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = k2.Close(ctx) })
	rebooted := botgov.New(botgov.Config{Mode: botgov.Observe, Cores: 1})
	loadGovernorMode(ctx, botsettings.NewKDBStore(k2), rebooted)
	if rebooted.Mode() != botgov.Enforce {
		t.Fatalf("after a restart the mode is %s, want the console's enforce", rebooted.Mode())
	}
	stored, _ := botsettings.NewKDBStore(k2).Governor(ctx)
	if stored.UpdatedBy != "operator" || stored.UpdatedAt.IsZero() {
		t.Fatalf("stored change = %+v", stored)
	}
}
