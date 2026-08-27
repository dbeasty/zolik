// Package buildinfo carries the identity of this binary: the number a human
// bumps in /VERSION plus the commits-since-bump and short hash that
// scripts/version.sh computes. Version and Commit are empty in source and set
// at link time:
//
//	-X zolik/server/internal/buildinfo.Version=1.1.1.2
//	-X zolik/server/internal/buildinfo.Commit=7feb025
//
// Named buildinfo, not version — models.Match.Version already means document
// optimistic concurrency, and internal/match leans on that name.
package buildinfo

import "runtime/debug"

var (
	Version string
	Commit  string
)

// Resolved fills in what the linker didn't. A plain `go build`/`go run`
// inside a git tree stamps runtime/debug.BuildInfo's vcs.revision on its own,
// so this rescues an unstamped `go run ./cmd/server`. It cannot rescue the
// Docker build, where .git is absent from the image; that's what the -X
// build args are for. Neither mechanism can supply the "1.1.1" part, which
// isn't a VCS fact.
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
