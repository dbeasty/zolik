package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"zolik/client-tui/api"
)

// The terminal client's screens.
//
// Three, where there used to be six. `ScreenGame`, `ScreenRoundEnd` and
// `ScreenGameEnd` were rummy screens — a table with melds on it, a deal
// summary, a match summary — and all three collapsed into one `ScreenMatch`
// that draws whatever zones the server sends. `ScreenScoreTable` went too: it
// showed a rummy scorepad, and standings now arrive on every match's state.

type Screen int

const (
	ScreenMainMenu Screen = iota
	ScreenLobby
	ScreenMatch
	ScreenScoreTable
)

type Root struct {
	screen    Screen
	width     int
	height    int
	renderer  *lipgloss.Renderer
	serverURL string
	session   PlayerSession
	api       *api.Client

	menu       menuModel
	lobby      lobbyModel
	match      matchModel
	scoreTable scoreTableModel

	status string
}

func NewRoot(_ any, serverURL string, sess PlayerSession) *Root {
	c := api.New(serverURL)
	c.SetAuth(sess.AccessToken, sess.UserID)
	r := &Root{
		screen:    ScreenMainMenu,
		serverURL: serverURL,
		session:   sess,
		api:       c,
	}
	r.menu = newMenuModel(r)
	r.lobby = newLobbyModel(r)
	r.match = newMatchModel(r)
	r.scoreTable = newScoreTableModel(r)
	return r
}

func (r *Root) SetRenderer(renderer *lipgloss.Renderer) {
	r.renderer = renderer
}

func (r *Root) Init() tea.Cmd {
	switch r.screen {
	case ScreenLobby:
		return r.lobby.Init()
	case ScreenMatch:
		return r.match.Init()
	default:
		return r.menu.Init()
	}
}

func (r *Root) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.width = msg.Width
		r.height = msg.Height
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return r, tea.Quit
		}
	case statsMsg:
		if msg.err != "" {
			r.menu.statsText = "✗ " + msg.err
		} else {
			r.menu.statsText = fmt.Sprintf("Matches: %v  Won: %v",
				msg.data["matches"], msg.data["wins"])
		}
		return r, nil
	case scoreErrMsg:
		r.scoreTable.errMsg = msg.err
		return r, nil
	case scoreCreatedMsg:
		r.scoreTable.session = msg.id
		r.scoreTable.players = msg.players
		r.scoreTable.errMsg = ""
		return r, nil
	case scoreExportMsg:
		r.scoreTable.export = msg.text
		return r, nil
	}

	var cmd tea.Cmd
	switch r.screen {
	case ScreenMainMenu:
		r.menu, cmd = r.menu.update(msg)
	case ScreenLobby:
		r.lobby, cmd = r.lobby.update(msg)
	case ScreenMatch:
		r.match, cmd = r.match.update(msg)
	case ScreenScoreTable:
		r.scoreTable, cmd = r.scoreTable.update(msg)
	}
	return r, cmd
}

func (r *Root) View() string {
	var body string
	switch r.screen {
	case ScreenLobby:
		body = r.lobby.view(r.width, r.height)
	case ScreenMatch:
		body = r.match.view(r.width, r.height)
	case ScreenScoreTable:
		body = r.scoreTable.view(r.width, r.height)
	default:
		body = r.menu.view(r.width, r.height)
	}
	if r.status != "" {
		return body + "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA")).Render(r.status) + "\n"
	}
	return body
}
