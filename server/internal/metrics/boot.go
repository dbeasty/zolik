package metrics

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"sync"
	"time"
)

// BootRecorder writes this process's boot row and works out whether the
// previous one died.
//
// The asymmetry is the whole design: a process that is OOM-killed cannot write
// "I died", so nothing this process does can record its own crash. What it can
// do is leave a row open and let its successor draw the conclusion. Every
// crash is therefore reported one restart late, which is fine — an operator
// reads this after the fact by definition — and is the only version of it that
// is honest.
type BootRecorder struct {
	store Store
	sink  Sink

	once sync.Once
	id   string
}

// NewBootRecorder builds a recorder. The sink is where the boot counters go,
// so a process with no metrics recorder still writes its rows and simply
// counts nothing.
func NewBootRecorder(store Store, sink Sink) *BootRecorder {
	if sink == nil {
		sink = Nop()
	}
	return &BootRecorder{store: store, sink: sink}
}

// Start records this process's boot and reports on its predecessor.
//
// The order matters and is not arbitrary. The predecessor is read *before*
// this process's own row is written, so that a bug in the exclusion cannot
// make a process diagnose itself as a crash — and the read excludes this id
// as well, belt and braces, because the two happen close enough together that
// a retry could interleave them.
//
// Errors are logged and swallowed. A server that would not start because it
// could not write a statistics row would be trading the thing people use for
// the thing that describes it.
func (b *BootRecorder) Start(ctx context.Context, version string) {
	b.once.Do(func() { b.id = newBootID() })

	prev, ok, err := b.store.LatestOpenBoot(ctx, b.id)
	if err != nil {
		slog.Warn("could not read the previous boot record", "error", err)
	}

	now := time.Now().UTC()
	row := Boot{ID: b.id, Version: version, StartedAt: now}
	if err := b.store.InsertBoot(ctx, row); err != nil {
		slog.Warn("could not write this boot record; a crash after this point will go unreported",
			"boot", b.id, "error", err)
		return
	}
	b.sink.Add(BootsTotal, 1)
	slog.Info("started", "boot", b.id, "version", version)

	if !ok {
		return
	}

	// The previous process left its row open, so it did not run its shutdown
	// path: OOM kill, SIGKILL, a panic that took the process, or the host
	// going down underneath it.
	b.sink.Add(BootsUnclean, 1)
	slog.Warn("the previous run did not exit cleanly",
		"previousBoot", prev.ID,
		"previousVersion", prev.Version,
		"previousStartedAt", prev.StartedAt,
		"ranFor", now.Sub(prev.StartedAt).Round(time.Second))

	// Stamped unclean, and counted so a crash-looping container does not
	// report a fresh crash on every restart — one dead process is one crash,
	// however many times its successor is restarted. The stop time is the best
	// guess available: nobody recorded when it actually died, and leaving the
	// row open would have every later boot re-examine it.
	if err := b.store.CloseBoot(ctx, prev.ID, now, ReasonUnclean, true); err != nil {
		slog.Warn("could not stamp the previous boot record as counted; it may be counted again",
			"previousBoot", prev.ID, "error", err)
	}
}

// Stop closes this process's boot row. Called from the shutdown path, before
// the database is torn down — a slow teardown must not be what loses the fact
// that this was a clean exit.
func (b *BootRecorder) Stop(ctx context.Context, reason Reason) {
	if b.id == "" {
		return
	}
	if err := b.store.CloseBoot(ctx, b.id, time.Now().UTC(), reason, false); err != nil {
		slog.Warn("could not close this boot record; the next start will read this as a crash",
			"boot", b.id, "error", err)
		return
	}
	slog.Info("stopped", "boot", b.id, "reason", reason)
}

// ID is this process's boot id, for logs that want to correlate to it.
func (b *BootRecorder) ID() string { return b.id }

// newBootID mints an id that sorts by time, so the rows read in order even
// when something has to look at them raw.
func newBootID() string {
	var buf [8]byte
	_, _ = rand.Read(buf[:])
	return time.Now().UTC().Format("20060102T150405") + "-" + hex.EncodeToString(buf[:])
}
