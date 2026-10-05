package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"zolik/server/internal/admin"
	"zolik/server/internal/botgov"
	"zolik/server/internal/botsettings"
	"zolik/server/internal/learn"
)

// Whether AI seats play the trained models this binary ships.
//
// The switch itself is learn's, in process: an atomic flag per game, read on
// every bot move. This file is the rest — loading the operator's stored
// choice into it at boot, and handing the console a way to change both the
// store and the switch. Production is one instance, so that is enough; more
// than one would need each change broadcast (the Redis hub channel) so every
// instance flips together, and each would still load the store at boot.

// loadHardModels applies the stored settings to the switch. Anything it cannot
// apply is logged and left off: the default is the heuristic, and a boot must
// never fail over which bot sits in a seat.
func loadHardModels(ctx context.Context, store botsettings.Store) {
	if store == nil {
		return
	}
	stored, err := store.HardModels(ctx)
	if err != nil {
		slog.Warn("bots: could not read the Hard model settings; every AI seat plays the Hard heuristic", "error", err)
		return
	}
	for game, s := range stored {
		if !s.Enabled {
			continue
		}
		if _, err := learn.SetHardModel(game, true); err != nil {
			// A rollback to a build whose model no longer fits, or which
			// ships none for this game, lands here.
			slog.Warn("bots: stored setting not applied; AI seats play the Hard heuristic",
				"game", game, "error", err)
			continue
		}
		slog.Info("bots: AI seats play the shipped model", "game", game,
			"since", s.UpdatedAt, "by", s.UpdatedBy)
	}
}

// hardModelRows is the console's Bots card: the switch, the model, and the
// last stored change, per game.
func hardModelRows(ctx context.Context, store botsettings.Store) ([]admin.HardModelRow, error) {
	var stored map[string]botsettings.HardModel
	if store != nil {
		var err error
		if stored, err = store.HardModels(ctx); err != nil {
			return nil, err
		}
	}
	statuses := learn.HardModels()
	rows := make([]admin.HardModelRow, 0, len(statuses))
	for _, s := range statuses {
		row := admin.HardModelRow{
			Game:        s.Game,
			Title:       s.Model.Title,
			Enabled:     s.Enabled,
			Embedded:    s.Embedded,
			Fits:        s.Fits,
			Problem:     s.Problem,
			EnvOverride: s.EnvOverride,
			Model:       s.Model,
		}
		if st, ok := stored[s.Game]; ok {
			row.UpdatedBy = st.UpdatedBy
			if !st.UpdatedAt.IsZero() {
				at := st.UpdatedAt
				row.UpdatedAt = &at
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// setHardModel persists a change and then moves the switch, refusing a game
// that ships no model and a model that would not play before either happens.
func setHardModel(ctx context.Context, store botsettings.Store, c admin.HardModelChange) (bool, error) {
	var current *learn.HardModelStatus
	for _, s := range learn.HardModels() {
		if s.Game == c.Game {
			current = &s
			break
		}
	}
	if current == nil {
		return false, admin.ErrUnknownGame
	}
	if c.Enabled && !current.Fits {
		return current.Enabled, errors.Join(admin.ErrModelDoesNotFit, errors.New(current.Problem))
	}
	if store != nil {
		if err := store.SetHardModel(ctx, c.Game, botsettings.HardModel{
			Enabled: c.Enabled, UpdatedBy: c.By, UpdatedAt: c.At.UTC().Truncate(time.Millisecond),
		}); err != nil {
			return current.Enabled, err
		}
	}
	was, err := learn.SetHardModel(c.Game, c.Enabled)
	switch {
	case errors.Is(err, learn.ErrUnknownGame):
		return was, admin.ErrUnknownGame
	case errors.Is(err, learn.ErrModelDoesNotFit):
		return was, errors.Join(admin.ErrModelDoesNotFit, err)
	}
	return was, err
}

// --- the governor ------------------------------------------------------------

// loadGovernorMode applies the operator's stored governor mode, if they ever
// set one; otherwise BOT_GOVERNOR stands. Anything it cannot read is logged
// and left as configured: a boot must never fail over this.
func loadGovernorMode(ctx context.Context, store botsettings.Store, gov *botgov.Governor) {
	if store == nil || gov == nil {
		return
	}
	stored, err := store.Governor(ctx)
	if err != nil {
		slog.Warn("bots: could not read the stored governor mode; BOT_GOVERNOR stands", "error", err)
		return
	}
	if stored.Mode == "" {
		return
	}
	mode := botgov.ParseMode(stored.Mode)
	was := gov.SetMode(mode)
	slog.Info("bots: governor mode from the console", "mode", mode.String(), "environment", was.String(),
		"since", stored.UpdatedAt, "by", stored.UpdatedBy)
}

// governorView is the console's governor panel: the mode and where it came
// from, and what the governor and the monitor are doing right now.
func (a *App) governorView(ctx context.Context) (admin.GovernorView, error) {
	gov := a.matchManager().Governor()
	v := admin.GovernorView{
		Mode:       gov.Mode().String(),
		EnvDefault: botgov.ParseMode(a.cfg.BotGovernor).String(),
		Source:     "environment",
		Governor:   gov.Status(),
		Monitor:    a.capacityMonitor().Status(),
	}
	if a.botSettings != nil {
		stored, err := a.botSettings.Governor(ctx)
		if err != nil {
			return v, err
		}
		if stored.Mode != "" {
			v.Source, v.UpdatedBy = "console", stored.UpdatedBy
			if !stored.UpdatedAt.IsZero() {
				at := stored.UpdatedAt
				v.UpdatedAt = &at
			}
		}
	}
	return v, nil
}

// setGovernorMode persists a change and then moves the governor, reporting
// the mode it had.
func (a *App) setGovernorMode(ctx context.Context, c admin.GovernorChange) (string, error) {
	return changeGovernorMode(ctx, a.botSettings, a.matchManager().Governor(), c)
}

func changeGovernorMode(ctx context.Context, store botsettings.Store, gov *botgov.Governor, c admin.GovernorChange) (string, error) {
	if gov == nil {
		return "", errors.New("the bot governor is not running")
	}
	mode := botgov.ParseMode(c.Mode)
	if store != nil {
		if err := store.SetGovernor(ctx, botsettings.Governor{
			Mode: mode.String(), UpdatedBy: c.By, UpdatedAt: c.At.UTC().Truncate(time.Millisecond),
		}); err != nil {
			return gov.Mode().String(), err
		}
	}
	return gov.SetMode(mode).String(), nil
}
