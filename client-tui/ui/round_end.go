package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"zolik/client-tui/api"
	"zolik/client-tui/internal/render"
)

type roundEndModel struct {
	root  *Root
	data  map[string]any
	state api.GameState
}

func newRoundEndModel(root *Root) roundEndModel {
	return roundEndModel{root: root}
}

func (m roundEndModel) Init() tea.Cmd { return nil }

func (m roundEndModel) update(msg tea.Msg) (roundEndModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "enter" {
		gid := m.root.game.gameID
		m.root.screen = ScreenGame
		m.root.game = newGameModel(m.root)
		m.root.game.gameID = gid
		return m, m.root.game.Init()
	}
	return m, nil
}

func (m roundEndModel) view(width, height int) string {
	winner := ""
	if w, ok := m.data["winnerId"].(string); ok {
		for _, p := range m.state.Players {
			if p.ID == w {
				winner = p.Name
				break
			}
		}
	}
	round := m.state.Round
	var b strings.Builder
	b.WriteString(render.HeaderBar.Render(fmt.Sprintf("ROUND %d COMPLETE", round)) + "\n\n")
	if winner != "" {
		b.WriteString(fmt.Sprintf("Winner: %s went out!\n\n", winner))
	}
	b.WriteString("RUNNING TOTALS\n")
	refs := make([]playerRef, 0, len(m.state.Players))
	for _, p := range m.state.Players {
		refs = append(refs, playerRef{ID: p.ID, Name: p.Name})
	}
	b.WriteString(formatScores(m.state.TotalScores, refs) + "\n\n")
	b.WriteString("[ENTER] Continue\n")
	return render.Box.Render(b.String())
}
