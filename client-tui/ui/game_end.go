package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"zolik/client-tui/api"
	"zolik/client-tui/internal/render"
)

type gameEndModel struct {
	root  *Root
	data  map[string]any
	state api.GameState
}

func newGameEndModel(root *Root) gameEndModel {
	return gameEndModel{root: root}
}

func (m gameEndModel) Init() tea.Cmd { return nil }

func (m gameEndModel) update(msg tea.Msg) (gameEndModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "enter" {
		m.root.screen = ScreenMainMenu
	}
	return m, nil
}

func (m gameEndModel) view(width, height int) string {
	var b strings.Builder
	b.WriteString(render.HeaderBar.Render("GAME COMPLETE") + "\n\n")
	refs := make([]playerRef, 0, len(m.state.Players))
	for _, p := range m.state.Players {
		refs = append(refs, playerRef{ID: p.ID, Name: p.Name})
	}
	b.WriteString("FINAL SCORES\n")
	b.WriteString(formatScores(m.state.TotalScores, refs) + "\n")
	if m.state.IsDraw {
		b.WriteString("\nDraw game.\n")
	}
	b.WriteString("\n[ENTER] Return to menu\n")
	return render.Box.Render(b.String())
}
