// Package buildinfo carries the identity of the standalone `cmd/play` dev
// runner. Version and Commit are empty in source and set at link time:
//
//	-X zolik/client-tui/internal/buildinfo.Version=1.1.1.2
//	-X zolik/client-tui/internal/buildinfo.Commit=7feb025
//
// The TUI's other binary — the one embedded in the game server over SSH — has
// no package of its own: its build identity *is* the server's, passed in as
// plain data (see ui.Build), not a second linked copy that could drift from
// it with nothing able to catch it.
package buildinfo

import "runtime/debug"

var (
	Version string
	Commit  string
)

// Resolved fills in what the linker didn't, the same way the server's
// buildinfo.Resolved does: falls back to the VCS stamp `go build`/`go run`
// leaves in a git tree, then to values that are obviously not a real release.
func Resolved() (version, commit string) {
	v, c := Version, Commit
	if v == "" {
		v = "0.0.0-dev"
	}
	if c == "" {
		c = "unknown"
		if bi, ok := debug.ReadBuildInfo(); ok {
			var rev string
			var dirty bool
			for _, s := range bi.Settings {
				switch s.Key {
				case "vcs.revision":
					rev = s.Value
				case "vcs.modified":
					dirty = s.Value == "true"
				}
			}
			if rev != "" {
				if len(rev) > 7 {
					rev = rev[:7]
				}
				c = rev
				if dirty {
					c += "-dirty"
				}
			}
		}
	}
	return v, c
}
