// Package obs sets up this process's logging, and does nothing else.
//
// The server used to log through the standard library's package-level
// `log.Printf` from seventeen files, with the interesting fields hand-formatted
// into the message ("match=%s player=%s suspended (%s)"). That has no levels,
// so nothing could be turned down on a busy host, and no structure, so reading
// `docker logs` meant grep and a guess.
//
// What makes replacing it cheap is that `slog.SetDefault` also redirects the
// standard log package's default logger through the slog handler. Every
// existing `log.Printf` call site keeps compiling and starts emitting a
// structured record at INFO without being edited — so this package plus one
// line in main() is the whole migration, and call sites move to attributes
// individually, when someone is in them for another reason.
package obs

import (
	"log/slog"
	"os"
	"strings"
)

// Options is how the process wants to be logged.
type Options struct {
	// Local selects the human-readable handler. It is the same APP_ENV test
	// the rest of the configuration makes ("" or "local" is a developer's
	// machine), passed in rather than re-read here so there is one place that
	// decides what local means.
	Local bool
	// Level is a slog level name — debug, info, warn, error — matched
	// case-insensitively. Anything else, including empty, means info.
	Level string
}

// Setup installs the process logger and returns it.
//
// JSON off a developer's machine, text on it. The distinction is not
// decoration: JSON is what makes `docker logs zolik-app-1 | jq 'select(.level
// == "WARN")'` work on the host, and it is unreadable at a terminal while
// writing code.
func Setup(opts Options) *slog.Logger {
	handlerOpts := &slog.HandlerOptions{Level: ParseLevel(opts.Level)}

	var h slog.Handler
	if opts.Local {
		h = slog.NewTextHandler(os.Stderr, handlerOpts)
	} else {
		h = slog.NewJSONHandler(os.Stdout, handlerOpts)
	}

	logger := slog.New(h)
	// This is the line that carries the seventeen unedited files: SetDefault
	// re-points the standard log package at this handler as well as slog's
	// own default.
	slog.SetDefault(logger)
	return logger
}

// ParseLevel maps a configured level name onto a slog level, defaulting to
// info. An unrecognised name is not an error: a typo in an environment
// variable should not stop a server booting, and the wrong verbosity is a
// visible enough symptom on its own.
func ParseLevel(name string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
