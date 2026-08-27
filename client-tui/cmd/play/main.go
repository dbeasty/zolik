// Play runs the TUI locally on stdout (no SSH). Useful for UI development.
// Requires the game server on http://127.0.0.1:8090
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"zolik/client-tui/api"
	"zolik/client-tui/internal/buildinfo"
	"zolik/client-tui/ui"
)

func main() {
	base := "http://127.0.0.1:8090"
	if v := os.Getenv("ZOLIK_BASE_URL"); v != "" {
		base = v
	}
	c := api.New(base)
	if err := c.GuestLogin("Terminal"); err != nil {
		fmt.Fprintf(os.Stderr, "login: %v\n", err)
		os.Exit(1)
	}
	sess := ui.PlayerSession{
		AccessToken: c.Token,
		UserID:      c.UserID,
		Username:    "Terminal",
		IsGuest:     true,
	}
	version, commit := buildinfo.Resolved()
	m := ui.NewRoot(nil, base, sess, ui.Build{Version: version, Commit: commit})
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "run: %v\n", err)
		os.Exit(1)
	}
}
