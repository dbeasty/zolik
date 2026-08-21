package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"zolik/client-tui/api"
)

type Screen int

const (
	ScreenMainMenu Screen = iota
	ScreenLobby
	ScreenGame
	ScreenRoundEnd
	ScreenGameEnd
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
	game       gameModel
	roundEnd   roundEndModel
	gameEnd    gameEndModel
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
	r.game = newGameModel(r)
	r.roundEnd = newRoundEndModel(r)
	r.gameEnd = newGameEndModel(r)
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
	case ScreenGame:
		return r.game.Init()
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
			r.menu.statsText = fmt.Sprintf("Games: %v  Won: %v", msg.data["gamesPlayed"], msg.data["gamesWon"])
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
	case ScreenGame:
		r.game, cmd = r.game.update(msg)
	case ScreenRoundEnd:
		r.roundEnd, cmd = r.roundEnd.update(msg)
	case ScreenGameEnd:
		r.gameEnd, cmd = r.gameEnd.update(msg)
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
	case ScreenGame:
		body = r.game.view(r.width, r.height)
	case ScreenRoundEnd:
		body = r.roundEnd.view(r.width, r.height)
	case ScreenGameEnd:
		body = r.gameEnd.view(r.width, r.height)
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
