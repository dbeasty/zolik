package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"zolik/server/internal/botgov"
	"zolik/server/internal/botstats"
	"zolik/server/internal/capacity"
)

// capacityMonitor builds the resource monitor once, on first use. Lazy for the
// same reason the match runtime is: it reads the runtime's bot recorder, and a
// test that builds an App without starting it has no use for either.
func (a *App) capacityMonitor() *capacity.Monitor {
	if !a.cfg.BotMonitorEnabled {
		return nil
	}
	a.monitorOnce.Do(func() {
		bots := a.matchManager().BotStats()
		gate := a.admission
		sample := func() capacity.Reading {
			s := gate.Snapshot()
			p95, n := bots.RecentQuantile(botstats.SourceLoop, 0.95)
			return capacity.Reading{
				// Admission reports zero stall where PSI cannot be read, which
				// is the reading that moves nothing — the same as unreadable.
				CPUStall: s.CPUStall, CPUOK: true,
				MemFrac: s.MemoryFraction, MemOK: s.MemoryLimit > 0,
				ActP95: p95, ActCount: n,
				Thermal: int(a.thermal.Load()),
			}
		}
		think := time.Duration(a.cfg.BotThinkMinMS) * time.Millisecond
		a.monitor = capacity.New(sample, capacity.DefaultThresholds(), think)
		if pin, ok := capacity.ParseLevel(a.cfg.BotMonitorPin); ok && a.cfg.DebugEndpointsEnabled {
			a.monitor.Pin(pin)
			slog.Warn("capacity monitor pinned", "level", pin.String())
		}
	})
	return a.monitor
}

// logCapacity writes one line per level change. Observe mode: this is all
// that happens on a change today.
func logCapacity(ctx context.Context, events <-chan capacity.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-events:
			attrs := []any{
				"from", ev.Previous.String(), "to", ev.Level.String(), "reason", ev.Reason,
				"cpuStall", ev.Reading.CPUStall, "memoryFraction", ev.Reading.MemFrac,
				"botActP95", ev.Reading.ActP95.String(), "botActs", ev.Reading.ActCount,
			}
			if ev.Level > ev.Previous {
				slog.Warn("capacity level", attrs...)
			} else {
				slog.Info("capacity level", attrs...)
			}
		}
	}
}

func (a *App) capacityReport(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(map[string]any{
		"monitor":  a.capacityMonitor().Status(),
		"governor": a.matchManager().Governor().Status(),
	})
}

// newGovernor builds the bot governor from config: the cores Go schedules
// onto (sized from the container's CPU limit since Go 1.25), the average bot
// pause, and this machine's speed against the one the costs were measured on.
//
// Built whatever BOT_GOVERNOR says, off included: the admin console can turn
// it on at run time, and an off governor costs one atomic load per bot move.
func newGovernor(cfg Config) *botgov.Governor {
	mode := botgov.ParseMode(cfg.BotGovernor)
	speed := botgov.Calibrate()
	think := time.Duration(cfg.BotThinkMinMS+cfg.BotThinkMaxMS) * time.Millisecond / 2
	cores := capacity.CPUQuota()
	g := botgov.New(botgov.Config{
		Mode:  mode,
		Cores: cores,
		Think: think,
		Speed: speed,
	})
	st := g.Status()
	slog.Info("bot governor", "mode", mode.String(), "cores", cores, "gomaxprocs", runtime.GOMAXPROCS(0),
		"speed", speed, "capacityCores", st.CapacityCores, "leasedClasses", len(st.Classes))
	return g
}

// followLevels hands every level change to the governor.
func followLevels(ctx context.Context, gov *botgov.Governor, events <-chan capacity.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-events:
			gov.SetLevel(ev.Level)
		}
	}
}

// SetThermalPressure is the device's thermal state as its OS reports it: 0
// nominal or fair, 1 serious, 2 critical. The mobile host calls it whenever
// the state changes; the monitor reads it on its next sample.
func (a *App) SetThermalPressure(level int) {
	if level < 0 {
		level = 0
	}
	if level > 2 {
		level = 2
	}
	a.thermal.Store(int32(level))
}

// botCapacity answers GET /bots/capacity: whether bots are playing at full
// strength right now, for a lobby or a table to say so. Public, and nothing
// in it is about any one table.
func (a *App) botCapacity(w http.ResponseWriter, _ *http.Request) {
	st := a.matchManager().Governor().Status()
	level := a.capacityMonitor().Level()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"level": level,
		// Simplifying is whether a bot seated now might play below its
		// skill: the governor enforces and the server is short of room.
		"simplifying":  st.Mode == botgov.Enforce && level != capacity.Green,
		"reducedSeats": st.ReducedSeats,
	})
}

func (a *App) pinCapacity(w http.ResponseWriter, req *http.Request) {
	level, ok := capacity.ParseLevel(req.URL.Query().Get("level"))
	mon := a.capacityMonitor()
	if !ok || mon == nil {
		http.Error(w, "level must be green, amber or red, with the monitor on", http.StatusBadRequest)
		return
	}
	mon.Pin(level)
	mon.Step(time.Now())
	slog.Warn("capacity monitor pinned", "level", level.String())
	a.capacityReport(w, req)
}
