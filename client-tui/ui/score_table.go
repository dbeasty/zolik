package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"zolik/client-tui/internal/render"
)

type scoreTableModel struct {
	root     *Root
	players  []string
	session  string
	round    int
	input    string
	typing   bool
	export   string
	errMsg   string
}

func newScoreTableModel(root *Root) scoreTableModel {
	return scoreTableModel{
		root:    root,
		players: []string{"P1", "P2", "P3", "P4"},
		round:   1,
	}
}

func (m scoreTableModel) Init() tea.Cmd { return nil }

func (m scoreTableModel) update(msg tea.Msg) (scoreTableModel, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.typing {
		switch key.String() {
		case "enter":
			m.typing = false
			return m, m.createSession()
		case "backspace":
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
		default:
			if len(key.Runes) > 0 {
				m.input += string(key.Runes)
			}
		}
		return m, nil
	}
	switch key.String() {
	case "esc", "q":
		m.root.screen = ScreenMainMenu
	case "n":
		m.typing = true
		m.input = strings.Join(m.players, ",")
	case "e":
		if m.session != "" {
			return m, m.exportSession()
		}
	}
	return m, nil
}

func (m scoreTableModel) createSession() tea.Cmd {
	return func() tea.Msg {
		names := strings.Split(m.input, ",")
		var trimmed []string
		for _, n := range names {
			n = strings.TrimSpace(n)
			if n != "" {
				trimmed = append(trimmed, n)
			}
		}
		if len(trimmed) < 4 {
			return scoreErrMsg{err: "need 4-8 player names, comma-separated"}
		}
		id, err := m.root.api.CreateScoringSession(trimmed)
		if err != nil {
			return scoreErrMsg{err: err.Error()}
		}
		return scoreCreatedMsg{id: id, players: trimmed}
	}
}

func (m scoreTableModel) exportSession() tea.Cmd {
	return func() tea.Msg {
		text, err := m.root.api.ExportScoringSession(m.session)
		if err != nil {
			return scoreErrMsg{err: err.Error()}
		}
		return scoreExportMsg{text: text}
	}
}

type scoreErrMsg struct{ err string }
type scoreCreatedMsg struct {
	id      string
	players []string
}
type scoreExportMsg struct{ text string }

func (m scoreTableModel) view(width, height int) string {
	var b strings.Builder
	b.WriteString(render.HeaderBar.Render("OFFLINE SCORE TABLE") + "\n\n")
	if m.session == "" {
		b.WriteString("No session yet.\n")
		b.WriteString("[N] New session (comma-separated names)\n")
		if m.typing {
			b.WriteString("Names: " + m.input + "_\n")
		}
	} else {
		b.WriteString(fmt.Sprintf("Session: %s\n", m.session))
		b.WriteString(fmt.Sprintf("Players: %s\n", strings.Join(m.players, ", ")))
		b.WriteString("[E] Export scorecard\n")
	}
	if m.export != "" {
		b.WriteString("\n--- export ---\n" + m.export + "\n")
	}
	if m.errMsg != "" {
		b.WriteString(render.StatusErr.Render(m.errMsg) + "\n")
	}
	b.WriteString("\n[ESC] Back\n")
	return render.Box.Render(b.String())
}
