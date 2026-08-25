package ui

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorilla/websocket"

	"zolik/client-tui/api"
	"zolik/client-tui/internal/render"
)

// One terminal screen, every game.
//
// It replaced five: a 503-line rummy table, 274 lines of rummy offer helpers,
// a deal-end screen, a match-end screen and a score table — all of which knew
// what a meld was. This one knows what a *zone* is, and draws whatever the
// server sends.
//
// The rule it obeys is the same one the graphical shell obeys: everything on
// screen is something the server said. Zones to lay out, seats to draw, offers
// to press, message keys to look up. It names no rank, suit, meld, blind or
// pot, which is why it plays Žolíky, Prší, Canasta and Hold'em without knowing
// which of them it is showing.

type matchModel struct {
	root    *Root
	matchID string
	state   api.MatchState
	ws      *websocket.Conn

	// cursor is which offer is highlighted; selected is which of the viewer's
	// own cards are picked, by index into the hand zone.
	cursor   int
	selected map[int]bool
	// params holds values in progress for the highlighted offer, keyed by
	// parameter name.
	params map[string]string
	status string
}

func newMatchModel(root *Root) matchModel {
	return matchModel{root: root, selected: map[int]bool{}, params: map[string]string{}}
}

// --- messages ---------------------------------------------------------------

type matchStateMsg struct{ state api.MatchState }
type matchErrMsg struct{ code, message string }
type matchClosedMsg struct{}

func (m matchModel) Init() tea.Cmd {
	return tea.Batch(m.connect(), m.listen())
}

func (m matchModel) connect() tea.Cmd {
	return func() tea.Msg {
		state, err := m.root.api.GetMatch(m.matchID)
		if err != nil {
			return matchErrMsg{code: "LOAD_FAILED", message: err.Error()}
		}
		return matchStateMsg{state: state}
	}
}

// listen pumps the socket. One message type matters — the whole board, already
// filtered for this viewer — so there is nothing to merge and no local
// projection that could drift.
func (m matchModel) listen() tea.Cmd {
	return func() tea.Msg {
		if m.ws == nil {
			return nil
		}
		for {
			_, data, err := m.ws.ReadMessage()
			if err != nil {
				return matchClosedMsg{}
			}
			var probe struct {
				Type    string `json:"type"`
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			if json.Unmarshal(data, &probe) != nil {
				continue
			}
			switch probe.Type {
			case "match_state":
				var s api.MatchState
				if json.Unmarshal(data, &s) == nil {
					return matchStateMsg{state: s}
				}
			case "error":
				return matchErrMsg{code: probe.Code, message: probe.Message}
			}
		}
	}
}

// --- update -----------------------------------------------------------------

func (m matchModel) update(msg tea.Msg) (matchModel, tea.Cmd) {
	switch msg := msg.(type) {
	case matchStateMsg:
		m.state = msg.state
		if m.cursor >= len(m.state.LegalActions) {
			m.cursor = 0
		}
		return m, m.listen()

	case matchErrMsg:
		m.status = msg.code
		if msg.message != "" {
			m.status += ": " + msg.message
		}
		return m, m.listen()

	case matchClosedMsg:
		m.status = "connection closed"
		return m, nil

	case tea.KeyMsg:
		return m.key(msg)
	}
	return m, nil
}

func (m matchModel) key(msg tea.KeyMsg) (matchModel, tea.Cmd) {
	offers := m.state.LegalActions

	switch msg.String() {
	case "q", "esc":
		if m.ws != nil {
			_ = m.ws.Close()
		}
		m.root.screen = ScreenMainMenu
		return m, nil

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.params = map[string]string{}
		}
		return m, nil

	case "down", "j":
		if m.cursor < len(offers)-1 {
			m.cursor++
			m.params = map[string]string{}
		}
		return m, nil

	// Adjust the highlighted offer's numeric parameter, if it has one. A
	// no-limit betting range cannot be a list of choices, so it is a number
	// nudged between the bounds the engine computed.
	case "+", "=", "right", "l":
		return m.nudge(1), nil
	case "-", "_", "left", "h":
		return m.nudge(-1), nil

	case "enter", " ":
		return m.send()
	}

	// Digits pick cards out of your own hand, for an offer that needs a
	// combination only a person can compose.
	if d, err := strconv.Atoi(msg.String()); err == nil && d >= 1 && d <= 9 {
		i := d - 1
		if m.selected[i] {
			delete(m.selected, i)
		} else {
			m.selected[i] = true
		}
	}
	return m, nil
}

// nudge moves the highlighted offer's int parameter within its declared range.
func (m matchModel) nudge(dir int) matchModel {
	o, ok := m.current()
	if !ok {
		return m
	}
	for _, p := range o.Params {
		if p.Kind != "int" {
			continue
		}
		step := p.Step
		if step <= 0 {
			step = 1
		}
		cur, _ := strconv.Atoi(m.paramValue(o, p))
		next := cur + dir*step
		if next < p.Min {
			next = p.Min
		}
		if next > p.Max {
			next = p.Max
		}
		m.params[p.Name] = strconv.Itoa(next)
	}
	return m
}

func (m matchModel) send() (matchModel, tea.Cmd) {
	o, ok := m.current()
	if !ok || !o.Enabled {
		m.status = "that move is not available"
		return m, nil
	}

	action, ready := api.SubmissionFor(o)
	if !ready {
		// The offer needs a combination this client cannot compose on its own,
		// so it uses the cards the player picked.
		cards := m.pickedCards(o)
		need := 1
		if o.Source != nil && o.Source.MinCards > 0 {
			need = o.Source.MinCards
		}
		if len(cards) < need {
			m.status = fmt.Sprintf("pick at least %d card(s) with the number keys", need)
			return m, nil
		}
		action = api.Action{OfferID: o.ID, Verb: o.Verb, Cards: cards}
		if o.Target != nil && o.Target.MeldID != "" {
			action.Target = o.Target.MeldID
		}
	}
	// Anything the player set by hand wins over the offer's own default.
	for _, p := range o.Params {
		if v, ok := m.params[p.Name]; ok {
			if action.Params == nil {
				action.Params = map[string]string{}
			}
			action.Params[p.Name] = v
		}
	}

	if m.ws == nil {
		m.status = "not connected"
		return m, nil
	}
	if err := m.root.api.SendWS(m.ws, action); err != nil {
		m.status = err.Error()
		return m, nil
	}
	m.selected = map[int]bool{}
	m.params = map[string]string{}
	m.status = ""
	return m, nil
}

func (m matchModel) current() (api.ActionOffer, bool) {
	if m.cursor < 0 || m.cursor >= len(m.state.LegalActions) {
		return api.ActionOffer{}, false
	}
	return m.state.LegalActions[m.cursor], true
}

// paramValue is what the player has set, or the offer's own default.
func (m matchModel) paramValue(o api.ActionOffer, p api.ParamSpec) string {
	if v, ok := m.params[p.Name]; ok {
		return v
	}
	v, _ := api.DefaultParam(p)
	return v
}

// myHand is the viewer's own hand zone, or nil.
func (m matchModel) myHand() *api.Zone {
	for i := range m.state.View.Zones {
		z := &m.state.View.Zones[i]
		if z.Kind == "hand" && z.OwnerID == m.root.session.UserID {
			return z
		}
	}
	return nil
}

// pickedCards is the selection, filtered to what the offer says it accepts.
func (m matchModel) pickedCards(o api.ActionOffer) []string {
	hand := m.myHand()
	if hand == nil {
		return nil
	}
	allowed := map[string]bool{}
	if o.Source != nil {
		for _, c := range o.Source.Cards {
			allowed[c] = true
		}
	}
	var out []string
	for i, cv := range hand.Cards {
		if !m.selected[i] {
			continue
		}
		if len(allowed) > 0 && !allowed[cv.Card] {
			continue
		}
		out = append(out, cv.Card)
	}
	return out
}

// --- view -------------------------------------------------------------------

func (m matchModel) view(width, height int) string {
	var b strings.Builder

	title := m.state.ModuleID
	if m.state.Variation != "" {
		title += " · " + m.state.Variation
	}
	b.WriteString(titleStyle.Render(title + "  [" + m.state.Status + "]"))
	b.WriteString("\n")

	if facts := factLine(m.state.View.Header); facts != "" {
		b.WriteString(mutedStyle.Render(facts) + "\n")
	}
	b.WriteString(m.seatLine() + "\n")

	for _, f := range m.state.View.Prompts {
		b.WriteString(promptStyle.Render("» "+factText(f)) + "\n")
	}

	// The board: everything that is not the viewer's own hand, then the hand
	// itself at the bottom where a player looks for it.
	for _, z := range m.state.View.Zones {
		if z.Kind == "hand" && z.OwnerID == m.root.session.UserID {
			continue
		}
		b.WriteString(m.zoneLine(z))
	}
	if hand := m.myHand(); hand != nil {
		b.WriteString("\n" + mutedStyle.Render(labelOf(hand.LabelKey, "Your hand")) + "\n")
		cards := make([]string, 0, len(hand.Cards))
		for _, c := range hand.Cards {
			cards = append(cards, c.Card)
		}
		var sel []int
		for i := range m.selected {
			sel = append(sel, i)
		}
		b.WriteString(render.RenderHandWithNumbers(cards, sel) + "\n")
	}

	b.WriteString("\n" + m.offerList() + "\n")

	if len(m.state.Standings) > 0 {
		b.WriteString("\n" + mutedStyle.Render("Standings") + "\n")
		for _, s := range m.state.Standings {
			b.WriteString(fmt.Sprintf("  %d. %-16s %d %s\n",
				s.Rank, api.PlayerName(m.state.Players, s.PlayerID), s.Score, labelOf(s.LabelKey, "")))
		}
	}
	for _, f := range m.state.View.Status {
		b.WriteString(mutedStyle.Render(factText(f)) + "\n")
	}

	if m.status != "" {
		b.WriteString("\n" + errStyle.Render(m.status) + "\n")
	}
	b.WriteString(mutedStyle.Render(
		"\n↑/↓ choose · 1-9 pick cards · ←/→ adjust · enter play · q quit"))
	return b.String()
}

func (m matchModel) seatLine() string {
	var parts []string
	for _, s := range m.state.View.Seats {
		name := api.PlayerName(m.state.Players, s.PlayerID)
		if s.PlayerID == m.root.session.UserID {
			name = "you"
		}
		tag := ""
		if s.Active {
			tag = "*"
		}
		bits := []string{}
		for _, f := range s.Facts {
			bits = append(bits, factText(f))
		}
		for _, k := range s.LabelKeys {
			bits = append(bits, labelOf(k, ""))
		}
		entry := tag + name
		if len(bits) > 0 {
			entry += " (" + strings.Join(bits, ", ") + ")"
		}
		parts = append(parts, entry)
	}
	return mutedStyle.Render(strings.Join(parts, "   "))
}

func (m matchModel) zoneLine(z api.Zone) string {
	name := labelOf(z.LabelKey, z.ID)
	switch {
	case len(z.Groups) > 0:
		var b strings.Builder
		b.WriteString(mutedStyle.Render(name) + "\n")
		for _, g := range z.Groups {
			badges := ""
			if len(g.BadgeKeys) > 0 {
				var bs []string
				for _, k := range g.BadgeKeys {
					bs = append(bs, labelOf(k, ""))
				}
				badges = "  " + strings.Join(bs, " ")
			}
			b.WriteString("  " + strings.Join(g.Cards, " ") + badges + "\n")
		}
		return b.String()
	case len(z.Cards) > 0:
		var cards []string
		for _, c := range z.Cards {
			cards = append(cards, c.Card)
		}
		return fmt.Sprintf("%s %s\n", mutedStyle.Render(name+":"), strings.Join(cards, " "))
	default:
		// A count and no cards is somebody else's hand, or a face-down pile.
		return fmt.Sprintf("%s %d\n", mutedStyle.Render(name+":"), z.Count)
	}
}

func (m matchModel) offerList() string {
	var b strings.Builder
	for i, o := range m.state.LegalActions {
		marker := "  "
		if i == m.cursor {
			marker = "> "
		}
		line := marker + labelOf("verb."+o.Verb, o.Verb)

		for _, f := range o.Facts {
			line += " " + factText(f)
		}
		for _, p := range o.Params {
			line += fmt.Sprintf("  [%s %s]", labelOf(p.LabelKey, p.Name), m.paramValue(o, p))
		}
		if o.Composite {
			line += "  (pick cards)"
		}

		if o.Enabled {
			b.WriteString(offerOnStyle.Render(line))
		} else {
			why := o.WhyNot
			b.WriteString(offerOffStyle.Render(line + "  — " + why))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// --- small helpers ----------------------------------------------------------

// labelOf renders a message key. The bundle is not in this client, so an
// unknown key degrades to its own last segment made readable — enough to keep
// a game this build has never seen legible.
func labelOf(key, fallback string) string {
	if key == "" {
		return fallback
	}
	last := key
	if i := strings.LastIndex(key, "."); i >= 0 {
		last = key[i+1:]
	}
	var out []rune
	for i, r := range last {
		if i > 0 && r >= 'A' && r <= 'Z' {
			out = append(out, ' ')
		}
		out = append(out, r)
	}
	s := strings.ToLower(string(out))
	if s == "" {
		return fallback
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func factText(f api.Fact) string {
	name := labelOf(f.LabelKey, "")
	if f.Value == "" {
		return name
	}
	if name == "" {
		return f.Value
	}
	return name + " " + f.Value
}

func factLine(facts []api.Fact) string {
	var parts []string
	for _, f := range facts {
		if s := factText(f); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "   ")
}

var (
	titleStyle    = lipgloss.NewStyle().Bold(true)
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#8b9cb3"))
	promptStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#fbbf24"))
	errStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171"))
	offerOnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#e8eef5"))
	offerOffStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#5c6b80"))
)
