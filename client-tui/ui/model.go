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

// Build is the client's own identity — its version and the commit it was
// built from. Over SSH this is literally the server's own buildinfo, passed
// in as plain data by the server that hosts the TUI (see
// client-tui/ssh.Deps.Build): "the client's build" *is* "the server's build"
// on that path by construction, not a fact that could drift from it. Only the
// standalone `cmd/play` dev runner has a build identity of its own.
type Build struct {
	Version string
	Commit  string
}

type Root struct {
	screen    Screen
	width     int
	height    int
	renderer  *lipgloss.Renderer
	serverURL string
	session   PlayerSession
	api       *api.Client
	build     Build

	// serverBuild is filled in by Root.Init's version fetch. nil until it
	// resolves (or forever, if the fetch fails) — the menu must render fine
	// either way.
	serverBuild *api.ServerBuild

	menu       menuModel
	lobby      lobbyModel
	match      matchModel
	scoreTable scoreTableModel

	status string
}

func NewRoot(_ any, serverURL string, sess PlayerSession, build Build) *Root {
	c := api.New(serverURL)
	c.SetAuth(sess.AccessToken, sess.UserID)
	r := &Root{
		screen:    ScreenMainMenu,
		serverURL: serverURL,
		session:   sess,
		api:       c,
		build:     build,
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
	// Not r.menu.Init(): that only runs when r.screen defaults to the menu
	// (model.go:130), and the version fetch needs to fire regardless of
	// which screen this session starts on.
	cmds := []tea.Cmd{r.loadServerBuild()}
	switch r.screen {
	case ScreenLobby:
		cmds = append(cmds, r.lobby.Init())
	case ScreenMatch:
		cmds = append(cmds, r.match.Init())
	default:
		cmds = append(cmds, r.menu.Init())
	}
	return tea.Batch(cmds...)
}

// loadServerBuild asks the server which build it's running, for the menu to
// render beside the client's own. Best-effort only: a failed fetch must never
// surface as an error, so serverBuild simply stays nil.
func (r *Root) loadServerBuild() tea.Cmd {
	return func() tea.Msg {
		build, err := r.api.GetVersion()
		if err != nil {
			return serverBuildMsg{}
		}
		return serverBuildMsg{build: &build}
	}
}

type serverBuildMsg struct {
	build *api.ServerBuild
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
	case serverBuildMsg:
		r.serverBuild = msg.build
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
