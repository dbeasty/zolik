package gamemcp

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
)

// Table is one match in progress: the state, who sits where, and what has
// happened. Every field is plain data so a table can be written to a file
// between two `game-mcp call` invocations and read back (see Service.Save);
// what is derived from it — bots, loaded models — is rebuilt on load.
type Table struct {
	ID        string             `json:"id"`
	Game      string             `json:"game"`
	Config    module.MatchConfig `json:"config"`
	Seed      int64              `json:"seed"`
	Seats     []SeatInfo         `json:"seats"`
	State     module.State       `json:"state"`
	Actions   int                `json:"actions"`
	Log       []LogEntry         `json:"log"`
	Illegal   int                `json:"illegal,omitempty"`
	Stalled   string             `json:"stalled,omitempty"`
	AutoPlay  bool               `json:"autoPlay"`
	RoundsLen int                `json:"roundsLen"`

	g       *game
	bots    map[string]module.Bot
	skills  map[string]module.Skill
	stepper learn.Stepper
	svc     *Service
}

// SeatInfo is one seat and who plays it.
type SeatInfo struct {
	Index  int    `json:"index"`
	Player string `json:"player"`
	Name   string `json:"name"`
	Claude bool   `json:"claude"`
	// Spec is the bot that plays a seat nobody drives: a skill, a style, or
	// net:<path>[@temperature]. Empty for a claude seat.
	Spec string `json:"spec,omitempty"`
}

// LogEntry is one action as the table saw it.
type LogEntry struct {
	N      int    `json:"n"`
	Seat   int    `json:"seat"`
	Name   string `json:"name"`
	Claude bool   `json:"claude,omitempty"`
	Auto   bool   `json:"auto,omitempty"`
	// Public is what anyone at the table may read about the action: the
	// module's own narration of its events as a spectator receives them,
	// or, for a game that does not narrate, the action and its public events.
	Public []string `json:"public"`

	// The rest is kept for replay_log's reveal, which only a finished match
	// may ask for: the action as sent, every event unprojected, and what a
	// trained model thought of the decision.
	Action module.Action  `json:"action"`
	Events []module.Event `json:"events,omitempty"`
	Audit  *Audit         `json:"audit,omitempty"`
}

// Audit is a trained network's view of one of its own decisions.
type Audit struct {
	Chosen     string       `json:"chosen"`
	ChosenProb float64      `json:"chosenProb"`
	Candidates int          `json:"candidates"`
	Top        []ScoredMove `json:"top"`

	firsts []module.Action
	probs  []float64
}

// ScoredMove is a move and the probability a model gives it.
type ScoredMove struct {
	Index int     `json:"index"`
	Text  string  `json:"text"`
	Prob  float64 `json:"prob"`
}

// budget bounds how far bots may play between two decisions of a claude
// seat: a whole Hold'em match of bots is a few hundred actions, a Canasta
// deal a few hundred more, and a runaway is a bug worth stopping on.
const budget = 20000

func (t *Table) players() []module.PlayerRef {
	out := make([]module.PlayerRef, len(t.Seats))
	for i, s := range t.Seats {
		out[i] = module.PlayerRef{ID: s.Player, Name: s.Name, IsAI: !s.Claude}
	}
	return out
}

func (t *Table) names() namer {
	n := namer{}
	for _, s := range t.Seats {
		n[s.Player] = s.Name
	}
	return n
}

func (t *Table) seatOf(player string) *SeatInfo {
	for i := range t.Seats {
		if t.Seats[i].Player == player {
			return &t.Seats[i]
		}
	}
	return nil
}

func (t *Table) seat(i int) (*SeatInfo, error) {
	if i < 0 || i >= len(t.Seats) {
		return nil, fmt.Errorf("no seat %d at table %s (seats 0..%d)", i, t.ID, len(t.Seats)-1)
	}
	return &t.Seats[i], nil
}

// bind rebuilds everything derived from the table's data: the game and a
// bot for every bot seat.
func (t *Table) bind(svc *Service) error {
	g, err := lookupGame(t.Game)
	if err != nil {
		return err
	}
	t.g, t.svc = g, svc
	t.bots, t.skills = map[string]module.Bot{}, map[string]module.Skill{}
	for _, s := range t.Seats {
		if s.Claude {
			continue
		}
		bot, skill, err := svc.botFor(g, s.Spec)
		if err != nil {
			return fmt.Errorf("seat %d: %w", s.Index, err)
		}
		t.bots[s.Player], t.skills[s.Player] = bot, skill
	}
	return nil
}

// botFor resolves a bot spec: a skill, a style the game supplies, or
// net:<path>[@temperature] for a trained model (temperature 0 — its favourite
// move every time — unless given).
func (svc *Service) botFor(g *game, spec string) (module.Bot, module.Skill, error) {
	for _, s := range module.Skills {
		if string(s) == spec {
			return g.heuristic(), s, nil
		}
	}
	if bot, ok := g.styles()[spec]; ok {
		return bot, module.SkillHard, nil
	}
	if path, ok := strings.CutPrefix(spec, "net:"); ok {
		if g.g == nil {
			return nil, "", fmt.Errorf("%s has no learn adapter, so a network cannot play it", g.m.Descriptor().ID)
		}
		temp := 0.0
		if i := strings.LastIndex(path, "@"); i >= 0 {
			t, err := strconv.ParseFloat(path[i+1:], 64)
			if err != nil {
				return nil, "", fmt.Errorf("bad temperature in %q: %w", spec, err)
			}
			path, temp = path[:i], t
		}
		p, lg, err := svc.policy(g, path)
		if err != nil {
			return nil, "", err
		}
		return learn.NetBot{Game: lg, Policy: p, Fallback: g.heuristic(), Temperature: temp}, module.SkillHard, nil
	}
	return nil, "", fmt.Errorf("unknown opponent %q: use easy, medium, hard, a style (%v) or net:<path>[@temperature]", spec, keys(g.styles()))
}

// policy loads a model once per path, and checks it was trained for this
// game's encoder or an earlier one it only appended to. The game it returns
// is the game as the network sees it (learn.Policy.GameFor): the encoder it
// was trained on, which is a prefix of today's for an older model.
func (svc *Service) policy(g *game, path string) (*learn.Policy, learn.Game, error) {
	if p, ok := svc.policies[path]; ok {
		lg, ok := p.GameFor(g.g)
		if !ok {
			return nil, nil, fmt.Errorf("%s was trained for another game or encoder", path)
		}
		return p, lg, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	n, err := learn.LoadNet(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", path, err)
	}
	p := learn.NewPolicy(n)
	lg, ok := p.GameFor(g.g)
	if !ok {
		return nil, nil, fmt.Errorf("%s was trained for %q (%d/%d), not this %s encoder (%d/%d) or one it grew from",
			path, n.Game, n.StateDim, n.CandDim, g.g.Name(), g.g.StateDim(), g.g.CandDim())
	}
	svc.policies[path] = p
	return p, lg, nil
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// --- whose turn ---------------------------------------------------------------

func (t *Table) finished() (bool, []string) {
	done, winners, err := t.g.m.Finished(t.State)
	if err != nil {
		return false, nil
	}
	return done, winners
}

// awaited is every seat the module is waiting on.
func (t *Table) awaited() []string {
	if done, _ := t.finished(); done {
		return nil
	}
	ps := t.players()
	return module.AwaitedSeats(t.g.m, t.State, ps[0].ID, ps)
}

func (t *Table) isAwaited(player string) bool {
	for _, p := range t.awaited() {
		if p == player {
			return true
		}
	}
	return false
}

// --- moves --------------------------------------------------------------------

// Move is one legal move for a seat, numbered as observe lists it.
type Move struct {
	Index int    `json:"index"`
	Text  string `json:"text"`
	// Steps is every action the move makes, in order. Most moves are one;
	// Žolíky's first lay-down, laid meld by meld, is several.
	Steps    []module.Action `json:"steps"`
	features []float32
}

// moves is what player may do now: the learn adapter's candidates where the
// game has one (concrete, legal, and whole — a composite meld or a pile
// pickup together with the opening that spends it), and the enabled offers
// otherwise. Empty when the seat is not being waited on.
func (t *Table) moves(player string) ([]Move, error) {
	if !t.isAwaited(player) {
		return nil, nil
	}
	offers, err := t.g.m.LegalActions(t.State, player)
	if err != nil {
		return nil, err
	}
	vm, err := t.g.m.View(t.State, player)
	if err != nil {
		return nil, err
	}
	d := describer{names: t.names(), vm: vm, offers: offers}
	var out []Move
	if t.g.g != nil {
		view := learn.Positions(t.g.g)
		pos, err := view.Position(t.State)
		if err != nil {
			return nil, err
		}
		cands, err := view.CandidatesFor(pos, player, offers)
		if err != nil {
			return nil, err
		}
		for i, c := range cands {
			steps := c.Steps()
			out = append(out, Move{Index: i, Steps: steps, Text: d.steps(steps), features: c.Features})
		}
	}
	if len(out) == 0 {
		for i, a := range enumerate(offers) {
			out = append(out, Move{Index: i, Steps: []module.Action{a}, Text: d.steps([]module.Action{a})})
		}
	}
	return out, nil
}

// maxEnumerated caps the offer-list fallback, which multiplies cards by
// parameter values.
const maxEnumerated = 80

// enumerate is every concrete submission the enabled offers describe: one
// per card an offer accepts one of, one per value of each parameter. Undos
// only when nothing else is enabled; composite offers never, since only the
// player can compose them (play takes an explicit action for those).
func enumerate(offers []module.ActionOffer) []module.Action {
	var out, undos []module.Action
	for _, o := range offers {
		if !o.Enabled || o.Composite {
			continue
		}
		base := module.Action{OfferID: o.ID, Verb: o.Verb}
		if o.Target != nil && o.Target.MeldID != "" {
			base.Target = o.Target.MeldID
		}
		var cardSets [][]string
		switch {
		case o.Source != nil && len(o.Source.Submit) > 0:
			cardSets = [][]string{o.Source.Submit}
		case o.Source != nil && o.Source.MinCards <= 1 && o.Source.MaxCards == 1 && len(o.Source.Cards) > 0:
			seen := map[string]bool{}
			for _, c := range o.Source.Cards {
				if !seen[c] {
					seen[c] = true
					cardSets = append(cardSets, []string{c})
				}
			}
		case o.Source != nil && o.Source.MinCards > 0:
			if len(o.Source.Cards) < o.Source.MinCards {
				continue
			}
			cardSets = [][]string{o.Source.Cards[:o.Source.MinCards]}
		default:
			cardSets = [][]string{nil}
		}
		params := []map[string]string{nil}
		for _, p := range o.Params {
			var next []map[string]string
			for _, prev := range params {
				for _, v := range paramValues(p) {
					m := map[string]string{}
					for k, x := range prev {
						m[k] = x
					}
					m[p.Name] = v
					next = append(next, m)
				}
			}
			params = next
		}
		for _, cs := range cardSets {
			for _, ps := range params {
				a := base
				a.Cards = append([]string(nil), cs...)
				a.Params = ps
				if o.Undo {
					undos = append(undos, a)
				} else {
					out = append(out, a)
				}
			}
		}
	}
	if len(out) == 0 {
		out = undos
	}
	if len(out) > maxEnumerated {
		out = out[:maxEnumerated]
	}
	return out
}

func paramValues(p module.ParamSpec) []string {
	var out []string
	seen := map[string]bool{}
	add := func(v string) {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	for _, c := range p.Choices {
		add(c.Value)
	}
	if p.Kind == module.ParamKindInt {
		for _, v := range []int{p.Min, p.Default, p.Max} {
			if v >= p.Min && v <= p.Max && (v != 0 || p.Min == 0) {
				add(strconv.Itoa(v))
			}
		}
	}
	return out
}

// --- playing --------------------------------------------------------------------

// apply makes one action for player and records it.
func (t *Table) apply(player string, a module.Action, auto bool, audit *Audit) error {
	before := t.State
	next, events, err := t.g.m.Apply(t.State, player, a)
	if err != nil {
		return err
	}
	t.State = next
	t.record(player, a, events, before, auto, audit)
	return nil
}

func (t *Table) record(player string, a module.Action, events []module.Event, before module.State, auto bool, audit *Audit) {
	t.Actions++
	s := t.seatOf(player)
	e := LogEntry{N: len(t.Log) + 1, Seat: s.Index, Name: s.Name, Claude: s.Claude, Auto: auto,
		Action: a, Events: events, Audit: audit}
	e.Public = t.publicLines(player, a, events, before)
	if rl := module.RoundsFor(t.g.m, t.State); rl != nil {
		for _, r := range rl.Rounds[min(t.RoundsLen, len(rl.Rounds)):] {
			e.Public = append(e.Public, t.roundLine(r))
		}
		t.RoundsLen = len(rl.Rounds)
	}
	if done, winners := t.finished(); done {
		e.Public = append(e.Public, "Match over. "+t.winnersText(winners))
	}
	t.Log = append(t.Log, e)
}

func (t *Table) winnersText(winners []string) string {
	if len(winners) == 0 {
		return "Nobody won."
	}
	n := t.names()
	names := make([]string, len(winners))
	for i, w := range winners {
		names[i] = n.token(w)
	}
	return "Won by " + strings.Join(names, ", ") + "."
}

func (t *Table) roundLine(r module.RoundResult) string {
	n := t.names()
	var b strings.Builder
	fmt.Fprintf(&b, "Round %d over", r.Number)
	if len(r.Winners) > 0 {
		b.WriteString(" (" + n.list(r.Winners) + ")")
	}
	if r.Headline != nil {
		b.WriteString(": " + n.fact(*r.Headline))
	}
	var parts []string
	for _, sc := range r.Scores {
		delta, total := sc.Delta, sc.Total
		if sc.Shown != nil {
			delta = *sc.Shown
		}
		if sc.ShownTotal != nil {
			total = *sc.ShownTotal
		}
		parts = append(parts, fmt.Sprintf("%s %+d (total %d)", n.token(sc.PlayerID), delta, total))
	}
	if len(parts) > 0 {
		b.WriteString(". Scores: " + strings.Join(parts, ", "))
	}
	for _, f := range r.Facts {
		b.WriteString(". " + n.fact(f))
	}
	return b.String()
}

// spectator is the viewer id nobody sits at: what it is shown and sent is
// what the whole table may know.
const spectator = ""

// publicLines is the action as anyone at the table may read it.
//
// The module's own narration of its events, projected for a spectator, is
// the answer wherever the module narrates: it is what the table's move strip
// shows, and ProjectEvent has already taken out anything private (the card
// drawn blind from the stock). A module that does not narrate gets the
// action's verb and parameters, its cards only where a spectator's board now
// shows them face up, and its projected events.
func (t *Table) publicLines(player string, a module.Action, events []module.Event, before module.State) []string {
	n := t.names()
	visible := t.spectatorCards(before, t.State)
	var lines, raw []string
	for _, ev := range events {
		pe, ok := module.ProjectEvent(t.g.m, ev, spectator)
		if !ok {
			continue
		}
		if mv, ok := module.NarrateEvent(t.g.m, t.State, pe); ok {
			lines = append(lines, n.fact(redact(mv.Fact, visible)))
			continue
		}
		// An event by the actor restates the action, which is said below.
		if p, _ := pe.Data["playerId"].(string); p == player {
			continue
		}
		raw = append(raw, n.event(pe))
	}
	if len(lines) > 0 {
		return lines
	}
	if _, narrates := t.g.m.(module.EventNarrator); narrates {
		raw = nil // a narrating module left these out on purpose
	}
	shown := a
	for _, c := range a.Cards {
		if !visible[c] {
			shown.Cards = nil
			break
		}
	}
	d := describer{names: n}
	line := n.token(player) + ": " + d.action(shown)
	if len(shown.Cards) == 0 && len(a.Cards) > 0 {
		line += fmt.Sprintf(" (%d cards)", len(a.Cards))
	}
	return append([]string{line}, raw...)
}

// redact takes out of a narrated fact any card no spectator's board shows
// face up, before or after the move. Narration is written for the move
// strip, and Žolíky's names the card a player goes out on even when the
// rules lay that card face down (GoOutDiscardFaceDown); the board is the
// authority on what is hidden, so the board wins.
func redact(f module.Fact, visible map[string]bool) module.Fact {
	hide := func(v any) any {
		if c, ok := v.(string); ok && isCardCode(c) && !visible[c] {
			return "a face-down card"
		}
		return v
	}
	out := f
	out.Params = map[string]any{}
	for k, v := range f.Params {
		switch x := v.(type) {
		case []any:
			ys := make([]any, len(x))
			for i, e := range x {
				ys[i] = hide(e)
			}
			out.Params[k] = ys
		case []string:
			ys := make([]any, len(x))
			for i, e := range x {
				ys[i] = hide(e)
			}
			out.Params[k] = ys
		default:
			out.Params[k] = hide(v)
		}
	}
	if s, ok := hide(f.Value).(string); ok {
		out.Value = s
	}
	return out
}

// spectatorCards is every card face up on a spectator's board before or after.
func (t *Table) spectatorCards(states ...module.State) map[string]bool {
	out := map[string]bool{}
	for _, s := range states {
		vm, err := t.g.m.View(s, spectator)
		if err != nil {
			continue
		}
		for _, z := range vm.Zones {
			for _, c := range z.Cards {
				if !c.FaceDown && c.Card != "" {
					out[c.Card] = true
				}
			}
			for _, gr := range z.Groups {
				for _, c := range gr.Cards {
					out[c] = true
				}
			}
		}
	}
	return out
}

// event is a projected event as a line, for a module that does not narrate.
func (n namer) event(ev module.Event) string {
	var parts []string
	for _, k := range sortedKeys(ev.Data) {
		if k == "playerId" {
			continue
		}
		parts = append(parts, humanise(k)+" "+n.token(ev.Data[k]))
	}
	who := ""
	if p, ok := ev.Data["playerId"].(string); ok {
		who = n.token(p) + " "
	}
	text := who + strings.ReplaceAll(ev.Type, "_", " ")
	if len(parts) > 0 {
		text += ": " + strings.Join(parts, ", ")
	}
	return text
}

// advance lets the bots play until a claude seat has a real decision, the
// match ends, or something stops it. A claude seat with exactly one legal
// move — a draw from the stock with nothing on the pile worth taking, the
// ready-up between deals — plays it here when AutoPlay is on.
func (t *Table) advance() error {
	for i := 0; i < budget; i++ {
		if t.Stalled != "" {
			return nil
		}
		if done, _ := t.finished(); done {
			return nil
		}
		awaited := t.awaited()
		if len(awaited) == 0 {
			t.Stalled = "nobody on turn"
			return nil
		}
		moved := false
		for _, p := range awaited {
			if !t.seatOf(p).Claude {
				if err := t.botMove(p); err != nil {
					return err
				}
				moved = true
				break
			}
		}
		if moved {
			continue
		}
		if !t.AutoPlay {
			return nil
		}
		for _, p := range awaited {
			ms, err := t.moves(p)
			if err != nil {
				return err
			}
			if len(ms) == 1 {
				if err := t.playMove(p, ms[0], true); err != nil {
					return err
				}
				moved = true
				break
			}
		}
		if !moved {
			return nil
		}
	}
	t.Stalled = "action budget"
	return nil
}

func (t *Table) botMove(player string) error {
	offers, err := t.g.m.LegalActions(t.State, player)
	if err != nil {
		return err
	}
	bot := t.bots[player]
	seat := module.BotSeat{PlayerID: player, Skill: t.skills[player], Seed: module.SeatSeed(t.Seed, player, "bot")}
	var audit *Audit
	if nb, ok := bot.(learn.NetBot); ok {
		audit = t.audit(nb, player, offers)
	}
	before := t.State
	var events []module.Event
	played, _, refused, ok := t.stepper.Step(t.State, bot, seat, offers, func(a module.Action) error {
		next, evs, err := t.g.m.Apply(t.State, player, a)
		if err != nil {
			return err
		}
		t.State, events = next, evs
		return nil
	})
	t.Illegal += refused
	if !ok {
		t.Stalled = "no move for " + t.seatOf(player).Name
		return nil
	}
	if audit != nil {
		audit.settle(played, describer{names: t.names()}.action(played))
	}
	t.record(player, played, events, before, false, audit)
	return nil
}

// playMove makes every step of a claude seat's move.
func (t *Table) playMove(player string, m Move, auto bool) error {
	for i, a := range m.Steps {
		if err := t.apply(player, a, auto, nil); err != nil {
			if i == 0 {
				return refusal(err)
			}
			return fmt.Errorf("step %d of %q was refused after the first %d stood: %w", i+1, m.Text, i, refusal(err))
		}
	}
	t.stepper.Played(player)
	return nil
}

// refusal is an engine error in words, with its code.
func refusal(err error) error {
	code := module.CodeOf(err)
	if code == "" {
		return err
	}
	if text, ok := messages["err."+code]; ok {
		return fmt.Errorf("%s (%s)", text, code)
	}
	return fmt.Errorf("%s (%s)", err.Error(), code)
}

// audit scores a network's own decision, for replay_log's reveal.
func (t *Table) audit(nb learn.NetBot, player string, offers []module.ActionOffer) *Audit {
	view := learn.Positions(nb.Game)
	pos, err := view.Position(t.State)
	if err != nil {
		return nil
	}
	cands, err := view.CandidatesFor(pos, player, offers)
	if err != nil || len(cands) < 2 {
		return nil
	}
	obs, err := view.EncodeFor(pos, player)
	if err != nil {
		return nil
	}
	feats := make([][]float32, len(cands))
	for i, c := range cands {
		feats[i] = c.Features
	}
	probs := softmax(nb.Policy.Logits(obs, feats))
	vm, _ := t.g.m.View(t.State, player)
	d := describer{names: t.names(), vm: vm, offers: offers}
	out := &Audit{Candidates: len(cands)}
	for _, i := range topK(probs, 5) {
		out.Top = append(out.Top, ScoredMove{Index: i, Text: d.steps(cands[i].Steps()), Prob: probs[i]})
	}
	out.firsts, out.probs = make([]module.Action, len(cands)), probs
	for i, c := range cands {
		out.firsts[i] = c.Action
	}
	return out
}

// settle records what the network played and the probability it gave every
// candidate that starts that way (a multi-step move is played one step per
// call, so the step is what can be matched).
func (a *Audit) settle(played module.Action, text string) {
	a.Chosen = text
	for i, f := range a.firsts {
		if sameAction(f, played) {
			a.ChosenProb += a.probs[i]
		}
	}
	a.firsts, a.probs = nil, nil
}
