package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

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
			}
		}
		think := time.Duration(a.cfg.BotThinkMinMS) * time.Millisecond
		a.monitor = capacity.New(sample, capacity.DefaultThresholds(), think)
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
	_ = enc.Encode(a.capacityMonitor().Status())
}
