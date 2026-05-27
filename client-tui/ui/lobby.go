package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"zolik/client-tui/api"
	"zolik/client-tui/internal/render"
)

type lobbyMode int

const (
	lobbyCreate lobbyMode = iota
	lobbyJoin
)

type lobbyModel struct {
	root       *Root
	mode       lobbyMode
	gameID     string
	joinCode   string
	input      string
	typing     bool
	players    []api.LobbyPlayer
	errMsg     string
	initialMin int
	aiDiff     string
	hostID     string
}

func newLobbyModel(root *Root) lobbyModel {
	return lobbyModel{
		root:   root,
		aiDiff: "medium",
	}
}

func (m lobbyModel) Init() tea.Cmd {
	if m.mode == lobbyCreate {
		return m.createGame()
	}
	return nil
}

func (m lobbyModel) createGame() tea.Cmd {
	return func() tea.Msg {
		min := m.initialMin
		var p *int
		if min > 0 {
			p = &min
		}
		id, code, err := m.root.api.CreateGame(p)
		if err != nil {
			return lobbyErrMsg{err: err.Error()}
		}
		return lobbyCreatedMsg{gameID: id, joinCode: code}
	}
}

type lobbyCreatedMsg struct {
	gameID   string
	joinCode string
}

type lobbyErrMsg struct {
	err string
}

type lobbyTickMsg struct {
	info api.LobbyGame
	err  string
}

func (m lobbyModel) update(msg tea.Msg) (lobbyModel, tea.Cmd) {
	switch msg := msg.(type) {
	case lobbyCreatedMsg:
		m.gameID = msg.gameID
		m.joinCode = msg.joinCode
		m.hostID = m.root.session.UserID
		return m, tea.Batch(m.pollLobby(), tea.Every(time.Second, func(t time.Time) tea.Msg {
			return pollTickMsg{}
		}))
	case lobbyErrMsg:
		m.errMsg = msg.err
		return m, nil
	case pollTickMsg:
		if m.gameID == "" {
			return m, nil
		}
		return m, m.pollLobby()
	case lobbyTickMsg:
		if msg.err != "" {
			m.errMsg = msg.err
			return m, nil
		}
		m.players = msg.info.Players
		if msg.info.Status == "active" {
			m.root.screen = ScreenGame
			m.root.game = newGameModel(m.root)
			m.root.game.gameID = m.gameID
			return m, m.root.game.Init()
		}
		return m, nil
	}

	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.typing {
		switch key.String() {
		case "enter":
			m.typing = false
			if m.mode == lobbyJoin && m.input != "" {
				return m, m.joinCodeCmd(m.input)
			}
		case "backspace":
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
		case "esc":
			m.typing = false
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
	case "j":
		if m.mode == lobbyJoin {
			m.typing = true
			m.input = ""
		}
	case "a":
		if m.gameID != "" && m.hostID == m.root.session.UserID {
			return m, m.addAI()
		}
	case "s":
		if m.gameID != "" && m.hostID == m.root.session.UserID {
			return m, m.startGame()
		}
	case "i":
		switch m.initialMin {
		case 0:
			m.initialMin = 35
		case 35:
			m.initialMin = 50
		default:
			m.initialMin = 0
		}
	case "e", "m", "h":
		switch key.String() {
		case "e":
			m.aiDiff = "easy"
		case "m":
			m.aiDiff = "medium"
		case "h":
			m.aiDiff = "hard"
		}
	}
	return m, nil
}

type pollTickMsg struct{}

func (m lobbyModel) pollLobby() tea.Cmd {
	return func() tea.Msg {
		info, err := m.root.api.GetLobby(m.gameID)
		if err != nil {
			return lobbyTickMsg{err: err.Error()}
		}
		return lobbyTickMsg{info: info}
	}
}

func (m lobbyModel) joinCodeCmd(code string) tea.Cmd {
	return func() tea.Msg {
		id, err := m.root.api.JoinGame(code)
		if err != nil {
			return lobbyErrMsg{err: err.Error()}
		}
		return lobbyCreatedMsg{gameID: id, joinCode: strings.ToUpper(code)}
	}
}

func (m lobbyModel) addAI() tea.Cmd {
	return func() tea.Msg {
		if err := m.root.api.AddAI(m.gameID, m.aiDiff); err != nil {
			return lobbyErrMsg{err: err.Error()}
		}
		return pollTickMsg{}
	}
}

func (m lobbyModel) startGame() tea.Cmd {
	return func() tea.Msg {
		if err := m.root.api.StartGame(m.gameID); err != nil {
			return lobbyErrMsg{err: err.Error()}
		}
		return pollTickMsg{}
	}
}

func (m lobbyModel) view(width, height int) string {
	title := "NEW GAME LOBBY"
	if m.mode == lobbyJoin {
		title = "JOIN GAME"
	}
	var b strings.Builder
	b.WriteString(render.HeaderBar.Render(title) + "\n")
	if m.joinCode != "" {
		b.WriteString(fmt.Sprintf("Join code: %s\n", m.joinCode))
	}
	if m.errMsg != "" {
		b.WriteString(render.StatusErr.Render("✗ "+m.errMsg) + "\n")
	}
	b.WriteString(fmt.Sprintf("\nPlayers (%d/8):\n", len(m.players)))
	for i, p := range m.players {
		line := fmt.Sprintf("  %d. %s", i+1, p.Name)
		if p.ID == m.root.session.UserID {
			line += " (you)"
		}
		if p.ID == m.hostID {
			line += " — host"
		}
		if p.IsAI {
			line += " 🤖"
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n[A] Add AI   Difficulty: " + m.aiDiff + "  (E/M/H keys)\n")
	b.WriteString(fmt.Sprintf("[I] Initial meld minimum: %d\n", m.initialMin))
	b.WriteString("[S] Start game (min 4 players, host only)\n")
	if m.mode == lobbyJoin {
		b.WriteString("[J] Enter join code\n")
		if m.typing {
			b.WriteString("Code: " + m.input + "_\n")
		}
	}
	b.WriteString("[ESC] Back to menu\n")
	return render.Box.Render(b.String())
}
