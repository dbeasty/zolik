package ui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorilla/websocket"

	"zolik/client-tui/api"
	"zolik/client-tui/internal/render"
)

type gameModel struct {
	root       *Root
	gameID     string
	state      api.GameState
	ws         *websocket.Conn
	selected   map[int]bool
	cursor     int
	status     string
	cmdMode    bool
	cmdInput   textinput.Model
	spinner    spinner.Model
	layoffPick bool
	swapPick   bool
	meldLabels map[string]string // meldId -> letter
	roundData  map[string]any
}

func newGameModel(root *Root) gameModel {
	ti := textinput.New()
	ti.Prompt = ": "
	ti.CharLimit = 120
	return gameModel{
		root:     root,
		selected: map[int]bool{},
		cmdInput: ti,
		spinner:  spinner.New(),
	}
}

func (m gameModel) Init() tea.Cmd {
	if m.gameID == "" {
		return nil
	}
	return tea.Batch(m.dialWS(), m.spinner.Tick)
}

func (m gameModel) dialWS() tea.Cmd {
	return func() tea.Msg {
		conn, err := m.root.api.DialWS(m.gameID)
		if err != nil {
			return wsErrMsg{err: err.Error()}
		}
		return wsConnectedMsg{conn: conn}
	}
}

func (m gameModel) readWS() tea.Cmd {
	return func() tea.Msg {
		if m.ws == nil {
			return wsErrMsg{err: "no connection"}
		}
		_, data, err := m.ws.ReadMessage()
		if err != nil {
			return wsErrMsg{err: err.Error()}
		}
		return wsFrameMsg{raw: data}
	}
}

type wsConnectedMsg struct {
	conn *websocket.Conn
}

type wsErrMsg struct {
	err string
}

type wsFrameMsg struct {
	raw []byte
}

func (m gameModel) update(msg tea.Msg) (gameModel, tea.Cmd) {
	switch msg := msg.(type) {
	case wsConnectedMsg:
		m.ws = msg.conn
		return m, m.readWS()
	case wsErrMsg:
		if msg.err != "" {
			m.status = "✗ " + msg.err
		}
		return m, nil
	case wsFrameMsg:
		next, cmd := m.handleWS(msg.raw)
		return next, tea.Batch(cmd, next.readWS())
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	if m.cmdMode {
		var cmd tea.Cmd
		m.cmdInput, cmd = m.cmdInput.Update(msg)
		if key, ok := msg.(tea.KeyMsg); ok && key.String() == "enter" {
			m.runCommand(strings.TrimSpace(m.cmdInput.Value()))
			m.cmdInput.SetValue("")
			m.cmdMode = false
		}
		if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
			m.cmdMode = false
		}
		return m, cmd
	}

	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	if m.layoffPick {
		r := key.String()
		if len(r) == 1 && r[0] >= 'a' && r[0] <= 'z' {
			for id, letter := range m.meldLabels {
				if letter == r {
					cards := selectedCards(m.state.MyHand, m.selected)
					if len(cards) > 0 {
						return m, m.send(api.WSAction{Type: "lay_off", MeldID: id, Cards: cards})
					}
				}
			}
			m.layoffPick = false
		}
	}

	if m.swapPick {
		r := key.String()
		if len(r) == 1 && r[0] >= 'a' && r[0] <= 'z' {
			for id, letter := range m.meldLabels {
				if letter == r {
					cards := selectedCards(m.state.MyHand, m.selected)
					if len(cards) == 1 {
						return m, m.send(api.WSAction{Type: "swap_joker", MeldID: id, Card: cards[0]})
					}
				}
			}
			m.swapPick = false
		}
	}

	switch key.String() {
	case ":":
		m.cmdMode = true
		m.cmdInput.Focus()
	case "q":
		m.root.screen = ScreenMainMenu
	case "left", "h":
		if m.cursor > 0 {
			m.cursor--
		}
	case "right":
		if m.cursor < len(m.state.MyHand)-1 {
			m.cursor++
		}
	case " ":
		if len(m.state.MyHand) > 0 {
			m.selected[m.cursor] = !m.selected[m.cursor]
		}
	case "m", "M":
		cards := selectedCards(m.state.MyHand, m.selected)
		if len(cards) > 0 {
			return m, m.send(api.WSAction{Type: "lay_meld", Cards: cards})
		}
	case "l", "L":
		cards := selectedCards(m.state.MyHand, m.selected)
		if len(cards) > 0 {
			m.layoffPick = true
			m.buildMeldLabels()
			m.status = "Pick meld letter for lay-off"
		}
	case "j":
		cards := selectedCards(m.state.MyHand, m.selected)
		if len(cards) == 1 {
			m.swapPick = true
			m.buildMeldLabels()
			m.status = "Pick meld letter to swap joker for " + cards[0]
		}
	case "d", "D":
		cards := selectedCards(m.state.MyHand, m.selected)
		if len(cards) == 1 {
			return m, m.send(api.WSAction{Type: "discard", Card: cards[0]})
		}
	case "g", "G":
		return m, m.send(api.WSAction{Type: "draw_card", From: "deck"})
	case "t", "T":
		return m, m.send(api.WSAction{Type: "draw_card", From: "discard"})
	default:
		if len(key.Runes) == 1 {
			d := key.Runes[0]
			if d >= '1' && d <= '9' {
				idx := int(d - '1')
				if idx < len(m.state.MyHand) {
					m.selected[idx] = !m.selected[idx]
					m.cursor = idx
				}
			}
			if d == '0' && len(m.state.MyHand) >= 10 {
				m.selected[9] = !m.selected[9]
				m.cursor = 9
			}
		}
	}
	return m, nil
}

func (m gameModel) handleWS(raw []byte) (gameModel, tea.Cmd) {
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return m, nil
	}
	t, _ := envelope["type"].(string)
	switch t {
	case "game_state":
		var st api.GameState
		if err := json.Unmarshal(raw, &st); err == nil {
			m.state = st
			m.selected = map[int]bool{}
		}
	case "error":
		msg, _ := envelope["message"].(string)
		m.status = "✗ " + msg
	case "deal_ended":
		m.roundData = envelope
		m.root.screen = ScreenRoundEnd
		m.root.roundEnd = newRoundEndModel(m.root)
		m.root.roundEnd.data = envelope
		m.root.roundEnd.state = m.state
	case "game_ended":
		m.root.screen = ScreenGameEnd
		m.root.gameEnd = newGameEndModel(m.root)
		m.root.gameEnd.data = envelope
		m.root.gameEnd.state = m.state
	default:
		if t == "reshuffle" {
			m.status = "! Deck recycled"
		}
		if t == "game_suspended" {
			m.status = "! Game suspended"
		}
	}
	return m, nil
}

func (m gameModel) send(action api.WSAction) tea.Cmd {
	return func() tea.Msg {
		if m.ws == nil {
			return wsErrMsg{err: "not connected"}
		}
		if err := m.root.api.SendWS(m.ws, action); err != nil {
			return wsErrMsg{err: err.Error()}
		}
		return nil
	}
}

func (m *gameModel) buildMeldLabels() {
	m.meldLabels = map[string]string{}
	letters := "abcdefghijklmnopqrstuvwxyz"
	i := 0
	for owner, metas := range m.state.MeldMeta {
		for _, meta := range metas {
			if i < len(letters) {
				m.meldLabels[meta.MeldID] = string(letters[i])
				i++
			}
			_ = owner
		}
	}
}

func (m gameModel) runCommand(line string) {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return
	}
	switch parts[0] {
	case "select":
		m.selected = map[int]bool{}
		for _, p := range parts[1:] {
			var idx int
			fmt.Sscanf(p, "%d", &idx)
			if idx >= 1 && idx <= len(m.state.MyHand) {
				m.selected[idx-1] = true
			}
		}
	case "meld":
		cards := selectedCards(m.state.MyHand, m.selected)
		if len(cards) > 0 {
			_ = m.send(api.WSAction{Type: "lay_meld", Cards: cards})()
		}
	case "discard":
		if len(parts) >= 2 {
			var idx int
			fmt.Sscanf(parts[1], "%d", &idx)
			if idx >= 1 && idx <= len(m.state.MyHand) {
				_ = m.send(api.WSAction{Type: "discard", Card: m.state.MyHand[idx-1]})()
			}
		}
	case "draw":
		_ = m.send(api.WSAction{Type: "draw_card", From: "deck"})()
	case "take":
		_ = m.send(api.WSAction{Type: "draw_card", From: "discard"})()
	case "help":
		m.status = "Keys: arrows, space, M meld, L layoff, J swap joker, D discard, G draw, T take, : commands"
	case "sort":
		if len(parts) < 2 {
			return
		}
		m.state.MyHand = sortHand(m.state.MyHand, parts[1])
	}
}

func (m gameModel) view(width, height int) string {
	if m.state.Type == "" && m.gameID != "" {
		return "Connecting to game...\n"
	}
	compact := render.UseCompact(width)
	var b strings.Builder

	roundLbl := roundRequirementLabel(m.state.Game)
	header := fmt.Sprintf("ŽOLÍKY │ Game %d of 7: %s │ Round %d │ Deck: %d │ ",
		m.state.Game, roundLbl, m.state.Round, m.state.DeckCount)
	if m.state.CurrentTurn == m.root.session.UserID {
		header += "Your turn"
	} else {
		name := m.state.CurrentTurn
		for _, p := range m.state.Players {
			if p.ID == m.state.CurrentTurn {
				name = p.Name
				break
			}
		}
		if isAI(m.state.Players, m.state.CurrentTurn) {
			header += name + " is thinking... " + m.spinner.View()
		} else {
			header += name + "'s turn"
		}
	}
	b.WriteString(render.HeaderBar.Render(header) + "\n\n")

	b.WriteString(render.SectionLabel.Render("OPPONENTS") + "\n")
	for _, p := range m.state.Players {
		if p.ID == m.root.session.UserID {
			continue
		}
		cnt := m.state.CardCounts[p.ID]
		line := fmt.Sprintf("  %s (%d cards)", p.Name, cnt)
		if p.IsAI {
			line += " 🤖"
		}
		b.WriteString(line + "\n")
	}

	b.WriteString("\n" + render.SectionLabel.Render("TABLE — Melds") + "\n")
	for _, p := range m.state.Players {
		melds := m.state.Melds[p.ID]
		metas := m.state.MeldMeta[p.ID]
		for i, cards := range melds {
			label := p.Name
			if i < len(metas) {
				label = fmt.Sprintf("[%s] %s", p.Name, metas[i].MeldID)
			}
			if compact {
				b.WriteString(label + " " + render.RenderHandCompact(cards, nil) + "\n")
			} else {
				b.WriteString(render.RenderMeld(cards, "", label) + "\n")
			}
		}
	}

	b.WriteString("\n" + render.SectionLabel.Render("PILES") + "\n")
	top := "—"
	if len(m.state.DiscardPile) > 0 {
		top = m.state.DiscardPile[len(m.state.DiscardPile)-1]
	}
	if compact {
		b.WriteString("Discard: " + render.RenderHandCompact([]string{top}, nil) + "\n")
		b.WriteString(fmt.Sprintf("Draw pile: × %d\n", m.state.DeckCount))
	} else {
		b.WriteString("Discard: " + render.RenderCard(top, false, false) + "\n")
		b.WriteString(fmt.Sprintf("Draw: %s × %d\n", render.RenderCardBack(), m.state.DeckCount))
	}

	b.WriteString("\n" + render.SectionLabel.Render("YOUR HAND") + "\n")
	sel := selectedIndexes(m.selected)
	if compact {
		b.WriteString(render.RenderHandCompact(m.state.MyHand, sel) + "\n")
	} else {
		b.WriteString(render.RenderHandWithNumbers(m.state.MyHand, sel) + "\n")
	}

	if m.state.InitialMeldMinimum > 0 && !m.state.RoundReqMet[m.root.session.UserID] {
		cards := selectedCards(m.state.MyHand, m.selected)
		nv := approximateNaturalValue(cards)
		ok := nv >= m.state.InitialMeldMinimum
		flag := "✗"
		if ok {
			flag = "✓"
		}
		b.WriteString(fmt.Sprintf("Natural value: %d (min %d) %s\n", nv, m.state.InitialMeldMinimum, flag))
	}

	refs := make([]playerRef, 0, len(m.state.Players))
	for _, p := range m.state.Players {
		refs = append(refs, playerRef{ID: p.ID, Name: p.Name, IsAI: p.IsAI})
	}
	b.WriteString("\nSCORES │ " + formatScores(m.state.TotalScores, refs) + "\n")
	if m.status != "" {
		b.WriteString("\n" + m.status + "\n")
	}
	if m.cmdMode {
		b.WriteString(m.cmdInput.View() + "\n")
	} else {
		b.WriteString("\n[?] help  [:] command  [Q] quit\n")
	}
	return render.Box.Render(b.String())
}

func isAI(players []api.Player, id string) bool {
	for _, p := range players {
		if p.ID == id {
			return p.IsAI
		}
	}
	return false
}

func sortHand(hand []string, mode string) []string {
	out := append([]string(nil), hand...)
	switch mode {
	case "suit":
		// simple suit sort
		for i := 0; i < len(out); i++ {
			for j := i + 1; j < len(out); j++ {
				if suitKey(out[i]) > suitKey(out[j]) {
					out[i], out[j] = out[j], out[i]
				}
			}
		}
	case "rank":
		for i := 0; i < len(out); i++ {
			for j := i + 1; j < len(out); j++ {
				if rankKey(out[i]) > rankKey(out[j]) {
					out[i], out[j] = out[j], out[i]
				}
			}
		}
	}
	return out
}

func suitKey(c string) string {
	if len(c) < 2 {
		return c
	}
	if c[0] == 'T' {
		return string(c[1])
	}
	return string(c[len(c)-1])
}

func rankKey(c string) string {
	if strings.HasPrefix(c, "JOKER") {
		return "Z"
	}
	return c
}
