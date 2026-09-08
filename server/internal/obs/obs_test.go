package obs_test

import (
	"log"
	"log/slog"
	"testing"

	"zolik/server/internal/obs"
)

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug":   slog.LevelDebug,
		"DEBUG":   slog.LevelDebug,
		" warn ":  slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
		"info":    slog.LevelInfo,
		"":        slog.LevelInfo,
		"loud":    slog.LevelInfo,
	}
	for in, want := range cases {
		if got := obs.ParseLevel(in); got != want {
			t.Errorf("ParseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

// The whole reason Phase 0 is one package and one line: SetDefault has to
// carry the seventeen files still calling log.Printf. If a future Go release
// or a refactor stopped routing the standard logger through the slog handler,
// every one of those call sites would silently go back to unstructured output
// on stderr and nobody would notice until they went looking for a field.
func TestSetupRoutesTheStandardLogPackage(t *testing.T) {
	obs.Setup(obs.Options{Local: true})

	if log.Flags() != 0 {
		t.Errorf("standard log still has its own flags (%d): slog did not take it over", log.Flags())
	}
	if log.Prefix() != "" {
		t.Errorf("standard log still has prefix %q", log.Prefix())
	}
}

func TestSetupHonoursLevel(t *testing.T) {
	logger := obs.Setup(obs.Options{Local: true, Level: "error"})
	if logger.Enabled(t.Context(), slog.LevelWarn) {
		t.Error("warn is enabled at level=error")
	}
	if !logger.Enabled(t.Context(), slog.LevelError) {
		t.Error("error is not enabled at level=error")
	}
}
