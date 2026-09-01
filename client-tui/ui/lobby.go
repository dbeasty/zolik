package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"zolik/client-tui/api"
)

// Opening or joining a table.
//
// The picker is rendered from /modules, so adding a game is a server-only
// change here too: register a module and it appears in this list with its
// variations and its player range. What this replaced hard-coded one game and
// one knob — the create screen's only setting was a rummy initial-meld
// minimum.

type lobbyMode int

const (
	lobbyPick lobbyMode = iota // choosing which game to open
	lobbyWait                  // opened or joined, waiting to start
	lobbyJoin                  // typing a join code
)

type lobbyModel struct {
	root    *Root
	mode    lobbyMode
	matchID string

	modules []api.Module
	cursor  int
	// variation is which ruleset is picked for the highlighted module.
	variation map[string]int
	// bots is how many bots each module's table opens with. It holds only
	// what was typed; a module nobody has adjusted answers through botsFor
	// with the smallest legal table.
	bots map[string]int

	joinCode string
	input    string
	players  []api.Player
	hostID   string
	status   string
	errMsg   string
}

func newLobbyModel(root *Root) lobbyModel {
	return lobbyModel{root: root, variation: map[string]int{}, bots: map[string]int{}}
}

type lobbyModulesMsg struct{ modules []api.Module }
type lobbyErrMsg struct{ err string }
type lobbyCreatedMsg struct{ matchID, joinCode string }
type lobbyStateMsg struct{ state api.MatchState }
type lobbyTickMsg struct{}

func (m lobbyModel) Init() tea.Cmd {
	if m.mode == lobbyJoin {
		return nil
	}
	return m.loadModules()
}

func (m lobbyModel) loadModules() tea.Cmd {
	return func() tea.Msg {
		mods, err := m.root.api.Modules()
		if err != nil {
			return lobbyErrMsg{err: err.Error()}
		}
		return lobbyModulesMsg{modules: mods}
	}
}

func (m lobbyModel) create(mod api.Module, variation string, bots int) tea.Cmd {
	return func() tea.Msg {
		id, code, err := m.root.api.CreateMatch(mod.ID, variation, nil)
		if err != nil {
			return lobbyErrMsg{err: err.Error()}
		}
		// Seat the bots the player asked for, then start. How many of them
		// are legal is the module's business — botsFor reads its range; this
		// loop only counts.
		for i := 0; i < bots; i++ {
			if err := m.root.api.AddBot(id, ""); err != nil {
				return lobbyErrMsg{err: err.Error()}
			}
		}
		if err := m.root.api.StartMatch(id); err != nil {
			return lobbyErrMsg{err: err.Error()}
		}
		return lobbyCreatedMsg{matchID: id, joinCode: code}
	}
}

func (m lobbyModel) join(code string) tea.Cmd {
	return func() tea.Msg {
		id, err := m.root.api.JoinMatch(code)
		if err != nil {
			return lobbyErrMsg{err: err.Error()}
		}
		return lobbyCreatedMsg{matchID: id}
	}
}

func (m lobbyModel) poll() tea.Cmd {
	id := m.matchID
	return func() tea.Msg {
		time.Sleep(1500 * time.Millisecond)
		s, err := m.root.api.GetMatch(id)
		if err != nil {
			return lobbyErrMsg{err: err.Error()}
		}
		return lobbyStateMsg{state: s}
	}
}

func (m lobbyModel) update(msg tea.Msg) (lobbyModel, tea.Cmd) {
	switch msg := msg.(type) {
	case lobbyModulesMsg:
		m.modules = msg.modules
		return m, nil

	case lobbyErrMsg:
		m.errMsg = msg.err
		return m, nil

	case lobbyCreatedMsg:
		m.matchID, m.joinCode, m.mode, m.errMsg = msg.matchID, msg.joinCode, lobbyWait, ""
		return m, m.poll()

	case lobbyStateMsg:
		m.players = msg.state.Players
		m.hostID = msg.state.HostID
		if msg.state.Status != "lobby" {
			// Started. Everything from here is the match screen's job.
			m.root.match = newMatchModel(m.root)
			m.root.match.matchID = m.matchID
			ws, err := m.root.api.DialWS(m.matchID)
			if err != nil {
				m.errMsg = err.Error()
				return m, nil
			}
			m.root.match.ws = ws
			m.root.screen = ScreenMatch
			return m, m.root.match.Init()
		}
		return m, m.poll()

	case tea.KeyMsg:
		return m.key(msg)
	}
	return m, nil
}

func (m lobbyModel) key(msg tea.KeyMsg) (lobbyModel, tea.Cmd) {
	switch m.mode {
	case lobbyJoin:
		switch msg.String() {
		case "esc":
			m.root.screen = ScreenMainMenu
			return m, nil
		case "enter":
			if strings.TrimSpace(m.input) == "" {
				m.errMsg = "enter a join code"
				return m, nil
			}
			return m, m.join(strings.TrimSpace(m.input))
		case "backspace":
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
			return m, nil
		default:
			if len(msg.String()) == 1 {
				m.input += strings.ToUpper(msg.String())
			}
			return m, nil
		}

	case lobbyWait:
		if msg.String() == "esc" || msg.String() == "q" {
			m.root.screen = ScreenMainMenu
		}
		return m, nil

	default: // lobbyPick
		switch msg.String() {
		case "esc", "q":
			m.root.screen = ScreenMainMenu
			return m, nil
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "j":
			if m.cursor < len(m.modules)-1 {
				m.cursor++
			}
			return m, nil
		case "left", "h", "right", "l":
			// Cycle this module's shipped rulesets.
			if mod, ok := m.selected(); ok && len(mod.Variations) > 1 {
				d := 1
				if msg.String() == "left" || msg.String() == "h" {
					d = len(mod.Variations) - 1
				}
				m.variation[mod.ID] = (m.variation[mod.ID] + d) % len(mod.Variations)
			}
			return m, nil
		case "+", "=", "-", "_":
			// How many bots the table opens with. Clamped by botsFor, so
			// holding a key down settles at the module's own limit rather
			// than running off into a table it will not deal.
			if mod, ok := m.selected(); ok {
				d := 1
				if msg.String() == "-" || msg.String() == "_" {
					d = -1
				}
				m.bots[mod.ID] = clampBots(mod, botsFor(mod, m.bots)+d)
			}
			return m, nil
		case "enter":
			mod, ok := m.selected()
			if !ok {
				return m, nil
			}
			v := ""
			if len(mod.Variations) > 0 {
				v = mod.Variations[m.variation[mod.ID]].ID
			}
			m.status = "opening a table…"
			return m, m.create(mod, v, botsFor(mod, m.bots))
		}
	}
	return m, nil
}

// botsFor is how many bots this module's table opens with: the count the
// player typed, clamped to what the module will seat. Never fewer than its
// minimum table needs once the host's own seat is counted, never more than its
// maximum leaves room for — and an unset module answers with the smallest
// legal table, which is what pressing enter has always given.
func botsFor(mod api.Module, picked map[string]int) int {
	n, ok := picked[mod.ID]
	if !ok {
		n = mod.MinPlayers - 1
	}
	return clampBots(mod, n)
}

// clampBots holds a count inside the module's declared range.
func clampBots(mod api.Module, n int) int {
	if lo := mod.MinPlayers - 1; n < lo {
		n = lo
	}
	if hi := mod.MaxPlayers - 1; n > hi {
		n = hi
	}
	if n < 1 {
		n = 1
	}
	return n
}

func (m lobbyModel) selected() (api.Module, bool) {
	if m.cursor < 0 || m.cursor >= len(m.modules) {
		return api.Module{}, false
	}
	return m.modules[m.cursor], true
}

func (m lobbyModel) view(width, height int) string {
	var b strings.Builder

	switch m.mode {
	case lobbyJoin:
		b.WriteString(titleStyle.Render("Join a table") + "\n\n")
		b.WriteString("Join code: " + m.input + "_\n")

	case lobbyWait:
		b.WriteString(titleStyle.Render("Waiting to start") + "\n\n")
		if m.joinCode != "" {
			b.WriteString("Join code: " + titleStyle.Render(m.joinCode) + "\n\n")
		}
		b.WriteString(mutedStyle.Render(fmt.Sprintf("Players (%d)", len(m.players))) + "\n")
		for i, p := range m.players {
			bot := ""
			if p.IsAI {
				bot = " (bot)"
			}
			host := ""
			if p.ID == m.hostID {
				host = " ★"
			}
			b.WriteString(fmt.Sprintf("  %d. %s%s%s\n", i+1, p.Name, bot, host))
		}

	default:
		b.WriteString(titleStyle.Render("Games") + "\n")
		b.WriteString(mutedStyle.Render("Everything this server can host") + "\n\n")
		for i, mod := range m.modules {
			marker := "  "
			if i == m.cursor {
				marker = "> "
			}
			line := fmt.Sprintf("%s%-16s %d–%d players", marker, mod.Label, mod.MinPlayers, mod.MaxPlayers)
			if len(mod.Variations) > 0 {
				line += "   [" + mod.Variations[m.variation[mod.ID]].Label + "]"
			}
			line += fmt.Sprintf("   %d bots", botsFor(mod, m.bots))
			if i == m.cursor {
				b.WriteString(offerOnStyle.Render(line) + "\n")
			} else {
				b.WriteString(offerOffStyle.Render(line) + "\n")
			}
		}
		b.WriteString(mutedStyle.Render("\n↑/↓ choose · ←/→ ruleset · +/− bots · enter play against bots · q back"))
	}

	if m.status != "" {
		b.WriteString("\n" + mutedStyle.Render(m.status))
	}
	if m.errMsg != "" {
		b.WriteString("\n" + errStyle.Render(m.errMsg))
	}
	return b.String()
}
