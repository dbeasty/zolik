package ui

import (
	"strings"
	"testing"

	"zolik/client-tui/api"
)

// Over SSH the client's build *is* the server's, by construction (see
// ui.Build's doc comment) — this pins that the common case renders quietly,
// with no server half at all, and the mismatched case (the standalone
// cmd/play runner against some other server) names it.
func TestMenuView_ShowsOwnBuild(t *testing.T) {
	root := &Root{build: Build{Version: "1.1.1.2", Commit: "7feb025"}}
	root.menu = newMenuModel(root)

	got := root.menu.buildLine()

	if !strings.Contains(got, "1.1.1.2") || !strings.Contains(got, "7feb025") {
		t.Fatalf("buildLine() = %q, want the client's own version and commit", got)
	}
}

func TestMenuView_NamesTheServerOnlyWhenItDiffers(t *testing.T) {
	root := &Root{build: Build{Version: "1.1.1.2", Commit: "7feb025"}}
	root.menu = newMenuModel(root)

	root.serverBuild = &api.ServerBuild{Version: "1.1.1.2", Commit: "7feb025"}
	if got := root.menu.buildLine(); strings.Contains(got, "server") {
		t.Fatalf("buildLine() = %q, should stay silent about a matching server", got)
	}

	root.serverBuild = &api.ServerBuild{Version: "1.1.1.4", Commit: "abc1234"}
	got := root.menu.buildLine()
	if !strings.Contains(got, "server") || !strings.Contains(got, "1.1.1.4") || !strings.Contains(got, "abc1234") {
		t.Fatalf("buildLine() = %q, want it to name the differing server build", got)
	}
}
