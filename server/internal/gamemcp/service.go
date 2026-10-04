package gamemcp

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
)

// Service holds the open tables and answers every tool. The MCP server and
// the command line are two front doors onto one of these.
//
// Calls are serialised: a table is a sequence of moves, and nothing here is
// slow enough for a lock to cost anything.
type Service struct {
	mu       sync.Mutex
	tables   map[string]*Table
	next     int
	policies map[string]*learn.Policy
}

// NewService is an empty service.
func NewService() *Service {
	return &Service{tables: map[string]*Table{}, policies: map[string]*learn.Policy{}}
}

func (svc *Service) table(id string) (*Table, error) {
	t, ok := svc.tables[id]
	if !ok {
		open := keys(svc.tables)
		sort.Strings(open)
		return nil, fmt.Errorf("no table %q (open: %v)", id, open)
	}
	return t, nil
}

// --- list_games -------------------------------------------------------------------

// ListGamesIn takes nothing.
type ListGamesIn struct{}

// ListGamesOut is every game.
type ListGamesOut struct {
	Games []GameInfo `json:"games"`
	Text  string     `json:"text"`
}

func (svc *Service) ListGames(ListGamesIn) (ListGamesOut, error) {
	gs := listGames()
	var b strings.Builder
	for _, g := range gs {
		fmt.Fprintf(&b, "%s — %s, %d-%d seats", g.ID, g.Label, g.MinSeats, g.MaxSeats)
		if g.Learnable {
			b.WriteString(", learnable (net opponents and coach)")
		}
		b.WriteString("\n")
		for _, v := range g.Variations {
			fmt.Fprintf(&b, "  variation %s (%s), %d-%d seats\n", v.ID, v.Label, v.MinSeats, v.MaxSeats)
		}
		for _, o := range g.Options {
			var vals []string
			for i, v := range o.Values {
				vals = append(vals, fmt.Sprintf("%d=%s", v, o.Labels[i]))
			}
			fmt.Fprintf(&b, "  option %s (%s): %s\n", o.Name, o.Label, strings.Join(vals, ", "))
		}
		fmt.Fprintf(&b, "  opponents: %s", strings.Join(g.Opponents, ", "))
		if g.Learnable {
			fmt.Fprintf(&b, ", net:<model path>[@temperature]; default coach model from %s (set: %v)", g.ModelEnv, g.ModelSet)
		}
		b.WriteString("\n")
	}
	return ListGamesOut{Games: gs, Text: b.String()}, nil
}

// --- new_table ----------------------------------------------------------------------

// NewTableIn deals a new match.
type NewTableIn struct {
	Game      string         `json:"game" jsonschema:"game id from list_games, e.g. zolik, canasta, holdem"`
	Variation string         `json:"variation,omitempty" jsonschema:"variation id; the game's first when empty"`
	Options   map[string]int `json:"options,omitempty" jsonschema:"table options by name, e.g. {\"initialMeldMinimum\":35,\"requireCleanRun\":1}"`
	Seats     int            `json:"seats" jsonschema:"number of seats at the table"`
	// ClaudeSeats are the seats driven by tool calls; seat numbers are 0-based.
	ClaudeSeats []int `json:"claude_seats" jsonschema:"0-based seats driven by play calls, e.g. [0]"`
	// Opponents is one bot spec per other seat, in seat order, or a single
	// spec for all of them.
	Opponents []string `json:"opponents,omitempty" jsonschema:"bot spec per non-claude seat in seat order, or one spec for all: easy, medium, hard, a style, or net:<model path>[@temperature] (temperature 0 when omitted); default hard"`
	Seed      *int64   `json:"seed,omitempty" jsonschema:"deal seed; the same seed and seating replay the same match"`
	// AutoPlay plays a claude seat's move for it when it has exactly one.
	AutoPlay *bool `json:"auto_play,omitempty" jsonschema:"play a claude seat's only legal move automatically (default true)"`
}

// NewTableOut is the table and the first claude seat's observation.
type NewTableOut struct {
	Table       string      `json:"table"`
	Seed        int64       `json:"seed"`
	Seats       []SeatInfo  `json:"seats"`
	Log         []string    `json:"log"`
	Observation Observation `json:"observation"`
}

func (svc *Service) NewTable(in NewTableIn) (NewTableOut, error) {
	g, err := lookupGame(in.Game)
	if err != nil {
		return NewTableOut{}, err
	}
	d := g.m.Descriptor()
	variation := in.Variation
	if variation == "" && len(d.Variations) > 0 {
		variation = d.Variations[0].ID
	}
	if variation != "" && d.Variation(variation) == nil {
		var ids []string
		for _, v := range d.Variations {
			ids = append(ids, v.ID)
		}
		return NewTableOut{}, fmt.Errorf("%s has no variation %q (have %v)", in.Game, variation, ids)
	}
	lo, hi := d.SeatRange(variation)
	if in.Seats < lo || in.Seats > hi {
		return NewTableOut{}, fmt.Errorf("%s %s seats %d-%d, not %d", in.Game, variation, lo, hi, in.Seats)
	}
	opts := map[string]*int{}
	for k, v := range in.Options {
		opts[k] = &v
	}
	if err := d.ValidateOptions(opts); err != nil {
		return NewTableOut{}, err
	}
	if len(in.ClaudeSeats) == 0 {
		return NewTableOut{}, fmt.Errorf("claude_seats is empty: name at least one seat to play")
	}
	claude := map[int]bool{}
	for _, i := range in.ClaudeSeats {
		if i < 0 || i >= in.Seats || claude[i] {
			return NewTableOut{}, fmt.Errorf("claude seat %d is not a seat of 0..%d, or is named twice", i, in.Seats-1)
		}
		claude[i] = true
	}
	others := in.Seats - len(claude)
	specs := in.Opponents
	switch {
	case len(specs) == 0:
		specs = []string{string(module.SkillHard)}
		fallthrough
	case len(specs) == 1:
		for len(specs) < others {
			specs = append(specs, specs[0])
		}
	}
	if len(specs) != others {
		return NewTableOut{}, fmt.Errorf("%d opponent specs for %d bot seats", len(specs), others)
	}
	seed := time.Now().UnixNano() % 1_000_000_000
	if in.Seed != nil {
		seed = *in.Seed
	}
	svc.next++
	t := &Table{ID: fmt.Sprintf("t%d", svc.next), Game: in.Game, Seed: seed, AutoPlay: in.AutoPlay == nil || *in.AutoPlay,
		Config: module.MatchConfig{Variation: variation, Options: module.Options(in.Options)}}
	for i, bot := 0, 0; i < in.Seats; i++ {
		s := SeatInfo{Index: i, Player: fmt.Sprintf("p%d", i), Claude: claude[i]}
		if s.Claude {
			s.Name = "claude"
			if len(claude) > 1 {
				s.Name = fmt.Sprintf("claude%d", i)
			}
		} else {
			s.Name, s.Spec = fmt.Sprintf("bot%d", i), specs[bot]
			bot++
		}
		t.Seats = append(t.Seats, s)
	}
	if err := t.bind(svc); err != nil {
		return NewTableOut{}, err
	}
	t.State, err = g.m.NewMatch(t.Config, t.players(), seed)
	if err != nil {
		return NewTableOut{}, fmt.Errorf("deal: %w", err)
	}
	t.Log = []LogEntry{}
	if err := t.advance(); err != nil {
		return NewTableOut{}, err
	}
	svc.tables[t.ID] = t
	first := slices.Sorted(func(yield func(int) bool) {
		for i := range claude {
			if !yield(i) {
				return
			}
		}
	})[0]
	obs, err := t.observe(first)
	if err != nil {
		return NewTableOut{}, err
	}
	return NewTableOut{Table: t.ID, Seed: seed, Seats: t.Seats, Log: publicLog(t.Log), Observation: obs}, nil
}

func publicLog(entries []LogEntry) []string {
	out := []string{}
	for _, e := range entries {
		for _, l := range e.Public {
			if e.Auto {
				l += " (only legal move, played automatically)"
			}
			out = append(out, l)
		}
	}
	return out
}

// --- observe ---------------------------------------------------------------------------

// ObserveIn asks what a seat sees.
type ObserveIn struct {
	Table string `json:"table" jsonschema:"table id from new_table"`
	Seat  int    `json:"seat" jsonschema:"0-based seat to observe as"`
}

func (svc *Service) Observe(in ObserveIn) (Observation, error) {
	t, err := svc.table(in.Table)
	if err != nil {
		return Observation{}, err
	}
	return t.observe(in.Seat)
}

// --- play ------------------------------------------------------------------------------

// PlayIn makes a move for a claude seat.
type PlayIn struct {
	Table string `json:"table" jsonschema:"table id"`
	Seat  int    `json:"seat" jsonschema:"0-based claude seat making the move"`
	Move  *int   `json:"move,omitempty" jsonschema:"index into the moves listed by the last observation"`
	// Action is an explicit engine action, for a move the list does not
	// carry (a composite meld in a game with no learn adapter, an undo).
	Action *module.Action `json:"action,omitempty" jsonschema:"explicit engine action {verb, cards, target, params, offerId} instead of a move index; the engine validates it"`
}

// PlayOut is what happened and what the seat sees now.
type PlayOut struct {
	Played      string      `json:"played"`
	Log         []string    `json:"log" jsonschema:"public account of every action since this move, bots included"`
	Observation Observation `json:"observation"`
}

func (svc *Service) Play(in PlayIn) (PlayOut, error) {
	t, err := svc.table(in.Table)
	if err != nil {
		return PlayOut{}, err
	}
	s, err := t.seat(in.Seat)
	if err != nil {
		return PlayOut{}, err
	}
	if !s.Claude {
		return PlayOut{}, fmt.Errorf("seat %d is played by a bot (%s)", s.Index, s.Spec)
	}
	if done, _ := t.finished(); done {
		return PlayOut{}, fmt.Errorf("the match is over")
	}
	if t.Stalled != "" {
		return PlayOut{}, fmt.Errorf("the table has stopped: %s", t.Stalled)
	}
	if !t.isAwaited(s.Player) {
		var who []string
		for _, p := range t.awaited() {
			who = append(who, fmt.Sprintf("seat %d (%s)", t.seatOf(p).Index, t.seatOf(p).Name))
		}
		return PlayOut{}, fmt.Errorf("it is not seat %d's turn: waiting for %s", s.Index, strings.Join(who, ", "))
	}
	mark := len(t.Log)
	var played string
	switch {
	case in.Move != nil && in.Action != nil:
		return PlayOut{}, fmt.Errorf("give a move index or an action, not both")
	case in.Move != nil:
		moves, err := t.moves(s.Player)
		if err != nil {
			return PlayOut{}, err
		}
		if *in.Move < 0 || *in.Move >= len(moves) {
			return PlayOut{}, fmt.Errorf("no move %d: the list has %d moves (0..%d); observe to see them", *in.Move, len(moves), len(moves)-1)
		}
		m := moves[*in.Move]
		if err := t.playMove(s.Player, m, false); err != nil {
			return PlayOut{}, err
		}
		played = m.Text
	case in.Action != nil:
		if err := t.apply(s.Player, *in.Action, false, nil); err != nil {
			return PlayOut{}, refusal(err)
		}
		t.stepper.Played(s.Player)
		played = describer{names: t.names()}.action(*in.Action)
	default:
		return PlayOut{}, fmt.Errorf("give a move index (from observe) or an explicit action")
	}
	if err := t.advance(); err != nil {
		return PlayOut{}, err
	}
	obs, err := t.observe(in.Seat)
	if err != nil {
		return PlayOut{}, err
	}
	log := publicLog(t.Log[mark:])
	obs.Text = "Played: " + played + "\n" + logBlock(log) + "\n" + obs.Text
	return PlayOut{Played: played, Log: log, Observation: obs}, nil
}

func logBlock(log []string) string {
	if len(log) == 0 {
		return ""
	}
	return "Since then:\n  " + strings.Join(log, "\n  ") + "\n"
}

// --- replay_log ----------------------------------------------------------------------------

// ReplayIn asks for the match so far.
type ReplayIn struct {
	Table  string `json:"table" jsonschema:"table id"`
	Reveal bool   `json:"reveal,omitempty" jsonschema:"also show every action as sent, every event unfiltered, what a trained bot thought of each of its decisions, and all hands; only once the match is finished"`
}

// ReplayOut is the public history, and with reveal the private one too.
type ReplayOut struct {
	Table   string       `json:"table"`
	Entries []ReplayLine `json:"entries"`
	Text    string       `json:"text"`
	// Open is the board with nothing hidden, where the module has one.
	Open *module.ViewModel `json:"open,omitempty"`
}

// ReplayLine is one action.
type ReplayLine struct {
	N      int      `json:"n"`
	Seat   int      `json:"seat"`
	Name   string   `json:"name"`
	Public []string `json:"public"`
	// Revealed only.
	Action *module.Action `json:"action,omitempty"`
	Events []module.Event `json:"events,omitempty"`
	Audit  *Audit         `json:"audit,omitempty"`
}

func (svc *Service) Replay(in ReplayIn) (ReplayOut, error) {
	t, err := svc.table(in.Table)
	if err != nil {
		return ReplayOut{}, err
	}
	if in.Reveal {
		if done, _ := t.finished(); !done {
			return ReplayOut{}, fmt.Errorf("reveal is only for a finished match; this one is still being played")
		}
	}
	out := ReplayOut{Table: t.ID, Entries: []ReplayLine{}}
	var b strings.Builder
	d := describer{names: t.names()}
	for _, e := range t.Log {
		l := ReplayLine{N: e.N, Seat: e.Seat, Name: e.Name, Public: e.Public}
		fmt.Fprintf(&b, "%4d  %s\n", e.N, strings.Join(e.Public, " / "))
		if in.Reveal {
			a := e.Action
			l.Action, l.Events, l.Audit = &a, e.Events, e.Audit
			fmt.Fprintf(&b, "        sent: %s\n", d.action(a))
			if e.Audit != nil {
				fmt.Fprintf(&b, "        model: played %q at p=%.3f of %d candidates; its top: ", e.Audit.Chosen, e.Audit.ChosenProb, e.Audit.Candidates)
				var tops []string
				for _, s := range e.Audit.Top {
					tops = append(tops, fmt.Sprintf("%s %.3f", s.Text, s.Prob))
				}
				b.WriteString(strings.Join(tops, "; ") + "\n")
			}
		}
		out.Entries = append(out.Entries, l)
	}
	if in.Reveal {
		if vm, ok := module.OpenViewFor(t.g.m, t.State); ok {
			out.Open = &vm
		}
		b.WriteString("\nFinal hands:\n")
		for _, s := range t.Seats {
			vm, err := t.g.m.View(t.State, s.Player)
			if err != nil {
				continue
			}
			for _, z := range vm.Zones {
				if z.Kind == module.ZoneHand && z.OwnerID == s.Player {
					fmt.Fprintf(&b, "  %s: %s\n", s.Name, cardsText(viewCards(z.Cards)))
				}
			}
		}
	}
	out.Text = b.String()
	return out, nil
}

// --- rules -----------------------------------------------------------------------------------

// RulesIn names the table whose rules to state.
type RulesIn struct {
	Game      string         `json:"game" jsonschema:"game id"`
	Variation string         `json:"variation,omitempty" jsonschema:"variation id; the game's first when empty"`
	Options   map[string]int `json:"options,omitempty" jsonschema:"table options, so the rules match the table"`
}

// RulesOut is the written rules.
type RulesOut struct {
	Text     string               `json:"text"`
	Sections []module.RuleSection `json:"sections"`
}

func (svc *Service) Rules(in RulesIn) (RulesOut, error) {
	g, err := lookupGame(in.Game)
	if err != nil {
		return RulesOut{}, err
	}
	d := g.m.Descriptor()
	variation := in.Variation
	if variation == "" && len(d.Variations) > 0 {
		variation = d.Variations[0].ID
	}
	secs := module.RulesFor(g.m, module.MatchConfig{Variation: variation, Options: module.Options(in.Options)})
	if secs == nil {
		return RulesOut{Text: d.Label + " has no written rules yet.", Sections: []module.RuleSection{}}, nil
	}
	n := namer{}
	var b strings.Builder
	fmt.Fprintf(&b, "%s — %s\n", d.Label, variation)
	for _, s := range secs {
		b.WriteString("\n## " + label(s.TitleKey, nil) + "\n")
		for _, it := range s.Items {
			b.WriteString("- " + n.fact(it.Fact) + "\n")
		}
	}
	return RulesOut{Text: b.String(), Sections: secs}, nil
}

// --- close_table -------------------------------------------------------------------------------

// CloseIn names a table to close.
type CloseIn struct {
	Table string `json:"table" jsonschema:"table id"`
}

// CloseOut confirms it.
type CloseOut struct {
	Closed string `json:"closed"`
}

func (svc *Service) Close(in CloseIn) (CloseOut, error) {
	if _, err := svc.table(in.Table); err != nil {
		return CloseOut{}, err
	}
	delete(svc.tables, in.Table)
	return CloseOut{Closed: in.Table}, nil
}

// --- persistence -------------------------------------------------------------------------------

type snapshot struct {
	Next   int      `json:"next"`
	Tables []*Table `json:"tables"`
}

// Save writes every open table, for a later Load to carry on from.
func (svc *Service) Save(w io.Writer) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	snap := snapshot{Next: svc.next}
	ids := keys(svc.tables)
	sort.Strings(ids)
	for _, id := range ids {
		snap.Tables = append(snap.Tables, svc.tables[id])
	}
	return json.NewEncoder(w).Encode(snap)
}

// Load replaces the open tables with the ones r holds.
func (svc *Service) Load(r io.Reader) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	var snap snapshot
	if err := json.NewDecoder(r).Decode(&snap); err != nil {
		return err
	}
	svc.tables, svc.next = map[string]*Table{}, snap.Next
	for _, t := range snap.Tables {
		if err := t.bind(svc); err != nil {
			return fmt.Errorf("table %s: %w", t.ID, err)
		}
		svc.tables[t.ID] = t
	}
	return nil
}
