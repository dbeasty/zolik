package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"zolik/client-tui/internal/render"
)

type menuModel struct {
	root          *Root
	pendingScreen Screen
	statsText     string
}

func newMenuModel(root *Root) menuModel {
	return menuModel{root: root}
}

func (m menuModel) Init() tea.Cmd { return nil }

func (m menuModel) update(msg tea.Msg) (menuModel, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "1":
		m.root.screen = ScreenLobby
		m.root.lobby = newLobbyModel(m.root)
		m.root.lobby.mode = lobbyPick
	case "2":
		m.root.screen = ScreenLobby
		m.root.lobby = newLobbyModel(m.root)
		m.root.lobby.mode = lobbyJoin
	case "3":
		m.root.screen = ScreenScoreTable
		m.root.scoreTable = newScoreTableModel(m.root)
	case "4":
		return m, m.loadStats()
	case "q", "Q":
		return m, tea.Quit
	}
	return m, nil
}

func (m menuModel) loadStats() tea.Cmd {
	return func() tea.Msg {
		stats, err := m.root.api.GetStats()
		if err != nil {
			return statsMsg{err: err.Error()}
		}
		return statsMsg{data: stats}
	}
}

type statsMsg struct {
	data map[string]any
	err  string
}

// buildLine reports the client's own build, and the server's only when it
// differs. Over SSH the two are identical by construction (see ui.Build), so
// printing both would be noise; from the standalone cmd/play runner, talking
// to a possibly-different server, a mismatch is the whole reason to show it.
func (m menuModel) buildLine() string {
	line := fmt.Sprintf("  zolik %s · %s", m.root.build.Version, m.root.build.Commit)
	sb := m.root.serverBuild
	if sb != nil && (sb.Version != m.root.build.Version || sb.Commit != m.root.build.Commit) {
		line += fmt.Sprintf("  (server %s · %s)", sb.Version, sb.Commit)
	}
	return line
}

func (m menuModel) view(width, height int) string {
	title := render.HeaderBar.Render("ŽOLÍKY — Continental Rummy")
	logo := `
    ██╗ ██████╗ ██╗     ██╗██╗  ██╗
    ╚══╝██╔═══╝ ██║     ██║██║ ██╔╝
    ███╗██║███╗ ██║     ██║█████╔╝
    ╚══╝██║╚██║ ██║     ██║██╔═██╗
    ████║╚████║ ████╗   ██║██║  ██╗
    ╚═══╝ ╚═══╝ ╚═══╝   ╚═╝╚═╝  ╚═╝`

	var b strings.Builder
	b.WriteString(render.Box.Render(title + "\n" + logo + "\n\n"))
	b.WriteString(m.buildLine() + "\n\n")
	b.WriteString("  [1] Play\n")
	b.WriteString("  [2] Join a Table\n")
	b.WriteString("  [3] Score Table (offline)\n")
	b.WriteString("  [4] View Stats\n")
	b.WriteString("  [Q] Quit\n\n")
	b.WriteString(fmt.Sprintf("  Logged in as: %s\n", m.root.session.Username))
	if m.statsText != "" {
		b.WriteString("\n" + m.statsText + "\n")
	}
	if m.root.session.IsGuest {
		b.WriteString("\n  (Guest — stats require a registered account)\n")
	}
	return b.String()
}
