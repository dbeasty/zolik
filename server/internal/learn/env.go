package learn

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"zolik/server/internal/module"
)

// The environment a trainer drives: many tables at once, played by the real
// engine, stopping only where a learning seat has a real decision to make.
//
// Everything that is not the learner acts here, in Go — the heuristic bots,
// the extra styles a game supplies, and frozen checkpoints of the learner
// itself through NetBot. The trainer is asked only for the learner's choices,
// and is told only what Encode and Candidates say, so it can never learn from
// a card its seat could not see.
//
// A table is never idle: after every step each one has been advanced to its
// next learner decision, starting a fresh match (next seed) whenever one ends.
// A decision with a single candidate is not a decision and is taken here
// rather than costing a round trip.

// Styled is implemented by a game that supplies opponents beyond its skill
// ladder — a maniac, a rock — for the training pool.
type Styled interface {
	Styles() map[string]module.Bot
}

// Learner is the seat spec for a seat the trainer plays.
const Learner = "learner"

// TableSpec is one table: how many seats, and who plays each. A spec is
// "learner", a skill ("easy", "medium", "hard"), a style name the game
// supplies, or "net:<path>[@temperature]" for a frozen checkpoint.
type TableSpec struct {
	Plan []string `json:"plan"`
}

// Event is reward reaching a learner seat. Done closes that seat's episode.
type Event struct {
	Seat   string  `json:"seat"`
	Reward float32 `json:"reward"`
	Done   bool    `json:"done,omitempty"`
}

// Observation is one table's pending decision and what happened since the
// last one.
type Observation struct {
	Seat    string      `json:"seat"`
	Obs     []float32   `json:"obs"`
	Cands   [][]float32 `json:"cands"`
	Events  []Event     `json:"events,omitempty"`
	Matches int         `json:"matches"` // matches finished on this table so far
	Illegal int         `json:"illegal"`
	Stalls  int         `json:"stalls"`
}

// Env is a batch of tables.
type Env struct {
	g         Game
	variation string
	budget    int
	stride    int64
	nets      map[string]*Net
	tables    []*table
}

type table struct {
	plan    map[string]string
	players []module.PlayerRef
	cfg     module.MatchConfig
	seed    int64
	state   module.State
	actions int
	pending []Candidate
	actor   string
	events  []Event
	matches int
	illegal int
	stalls  int
}

// NewEnv deals every table. Table i plays seeds first+i, first+i+len(specs), ...
func NewEnv(g Game, variation string, specs []TableSpec, first int64, budget int) (*Env, error) {
	e := &Env{g: g, variation: variation, budget: budget, stride: int64(len(specs)), nets: map[string]*Net{}}
	for i, spec := range specs {
		if len(spec.Plan) < 2 {
			return nil, fmt.Errorf("learn: table %d has %d seats", i, len(spec.Plan))
		}
		t := &table{plan: map[string]string{}, players: Players(len(spec.Plan)), seed: first + int64(i)}
		t.cfg = g.Config(len(spec.Plan), variation)
		for j, p := range t.players {
			if _, err := e.botFor(spec.Plan[j]); err != nil && spec.Plan[j] != Learner {
				return nil, err
			}
			t.plan[p.ID] = spec.Plan[j]
		}
		if err := e.deal(t); err != nil {
			return nil, err
		}
		e.tables = append(e.tables, t)
	}
	return e, nil
}

// Observe advances every table to its next learner decision and reports them.
func (e *Env) Observe() ([]Observation, error) {
	out := make([]Observation, len(e.tables))
	for i, t := range e.tables {
		if err := e.advance(t); err != nil {
			return nil, fmt.Errorf("table %d: %w", i, err)
		}
		obs, err := e.g.Encode(t.state, t.actor)
		if err != nil {
			return nil, err
		}
		cands := make([][]float32, len(t.pending))
		for j, c := range t.pending {
			cands[j] = c.Features
		}
		out[i] = Observation{Seat: t.actor, Obs: obs, Cands: cands, Events: t.events,
			Matches: t.matches, Illegal: t.illegal, Stalls: t.stalls}
		t.events = nil
	}
	return out, nil
}

// Step applies one choice per table — an index into that table's last
// candidates — and returns the next observations.
func (e *Env) Step(choices []int) ([]Observation, error) {
	if len(choices) != len(e.tables) {
		return nil, fmt.Errorf("learn: %d choices for %d tables", len(choices), len(e.tables))
	}
	for i, t := range e.tables {
		c := choices[i]
		if t.pending == nil || c < 0 || c >= len(t.pending) {
			return nil, fmt.Errorf("learn: table %d: choice %d of %d", i, c, len(t.pending))
		}
		a := t.pending[c].Action
		t.pending = nil
		if err := e.apply(t, t.actor, a); err != nil {
			// The adapter built a candidate the engine refused. Count it and let
			// the heuristic make the move, exactly as the bench does.
			t.illegal++
			if err := e.heuristicMove(t, t.actor); err != nil {
				return nil, err
			}
		}
	}
	return e.Observe()
}

// advance plays non-learner seats, and trivial learner decisions, until a
// learner has a real choice.
func (e *Env) advance(t *table) error {
	for t.pending == nil {
		if done, _, err := e.g.Module().Finished(t.state); err != nil {
			return err
		} else if done || t.actions >= e.budget {
			if !done {
				t.stalls++
			}
			if err := e.nextMatch(t); err != nil {
				return err
			}
			continue
		}
		actor := module.ActiveSeat(e.g.Module(), t.state, t.players[0].ID, t.players)
		if actor == "" {
			t.stalls++
			if err := e.nextMatch(t); err != nil {
				return err
			}
			continue
		}
		offers, err := e.g.Module().LegalActions(t.state, actor)
		if err != nil {
			return err
		}
		if t.plan[actor] != Learner {
			bot, _ := e.botFor(t.plan[actor])
			if err := e.botMove(t, actor, bot, skillOf(t.plan[actor]), offers); err != nil {
				return err
			}
			continue
		}
		cands, err := e.g.Candidates(t.state, actor, offers)
		if err != nil {
			return err
		}
		switch len(cands) {
		case 0:
			if err := e.heuristicMove(t, actor); err != nil {
				return err
			}
		case 1:
			if err := e.apply(t, actor, cands[0].Action); err != nil {
				t.illegal++
				if err := e.heuristicMove(t, actor); err != nil {
					return err
				}
			}
		default:
			t.pending, t.actor = cands, actor
		}
	}
	return nil
}

func (e *Env) deal(t *table) error {
	s, err := e.g.Module().NewMatch(t.cfg, t.players, t.seed)
	if err != nil {
		return err
	}
	t.state, t.actions = s, 0
	return nil
}

// nextMatch closes every learner's episode and deals the next seed.
func (e *Env) nextMatch(t *table) error {
	for _, p := range t.players {
		if t.plan[p.ID] == Learner {
			t.events = append(t.events, Event{Seat: p.ID, Done: true})
		}
	}
	t.matches++
	t.seed += e.stride
	return e.deal(t)
}

// apply makes one move and credits every learner seat with what it earned.
func (e *Env) apply(t *table, actor string, a module.Action) error {
	next, _, err := e.g.Module().Apply(t.state, actor, a)
	if err != nil {
		return err
	}
	for _, p := range t.players {
		if t.plan[p.ID] != Learner {
			continue
		}
		r, done, err := e.g.Reward(t.state, next, p.ID)
		if err != nil {
			return err
		}
		if r != 0 || done {
			t.events = append(t.events, Event{Seat: p.ID, Reward: r, Done: done})
		}
	}
	t.state = next
	t.actions++
	return nil
}

func (e *Env) botMove(t *table, actor string, bot module.Bot, skill module.Skill, offers []module.ActionOffer) error {
	seat := module.BotSeat{PlayerID: actor, Skill: skill, Seed: module.SeatSeed(t.seed, actor, "bot")}
	if a, ok := bot.Act(t.state, seat, offers); ok {
		if err := e.apply(t, actor, a); err == nil {
			return nil
		}
		t.illegal++
	}
	for _, a := range module.ChooseActions(offers, nil) {
		if err := e.apply(t, actor, a); err == nil {
			return nil
		}
	}
	// Nobody can move this seat: the match is a stall, and the table moves on.
	t.stalls++
	return e.nextMatch(t)
}

func (e *Env) heuristicMove(t *table, actor string) error {
	offers, err := e.g.Module().LegalActions(t.state, actor)
	if err != nil {
		return err
	}
	return e.botMove(t, actor, e.g.Heuristic(), module.SkillHard, offers)
}

// botFor resolves a non-learner seat spec.
func (e *Env) botFor(spec string) (module.Bot, error) {
	switch {
	case spec == Learner:
		return nil, errors.New("learn: learner has no bot")
	case skillOf(spec) != "":
		return e.g.Heuristic(), nil
	case strings.HasPrefix(spec, "net:"):
		path, temp := strings.TrimPrefix(spec, "net:"), 1.0
		if i := strings.LastIndex(path, "@"); i >= 0 {
			t, err := strconv.ParseFloat(path[i+1:], 64)
			if err != nil {
				return nil, fmt.Errorf("learn: %q: %w", spec, err)
			}
			path, temp = path[:i], t
		}
		n, ok := e.nets[path]
		if !ok {
			b, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			if n, err = LoadNet(b); err != nil {
				return nil, fmt.Errorf("learn: %s: %w", path, err)
			}
			e.nets[path] = n
		}
		nb := NetBot{Game: e.g, Net: n, Fallback: e.g.Heuristic(), Temperature: temp}
		if !nb.usable() {
			return nil, fmt.Errorf("learn: %s was trained for another encoder", path)
		}
		return nb, nil
	}
	if s, ok := e.g.(Styled); ok {
		if b, ok := s.Styles()[spec]; ok {
			return b, nil
		}
	}
	return nil, fmt.Errorf("learn: unknown seat %q", spec)
}

func skillOf(spec string) module.Skill {
	for _, s := range module.Skills {
		if string(s) == spec {
			return s
		}
	}
	return ""
}

// --- the wire -----------------------------------------------------------------
//
// One JSON object per line each way. Requests:
//
//	{"op":"reset","game":"holdem","variation":"","seed":1,"budget":20000,"tables":[{"plan":["learner","hard"]}]}
//	{"op":"step","choices":[2]}
//	{"op":"info"}                  -> {"stateDim":..,"candDim":..}
//
// Every reply to reset and step is {"tables":[Observation...]}; a failure is
// {"error":"..."}. A failed reset keeps the previous environment; a failed step
// may have moved some tables, and the trainer should reset.

type request struct {
	Op        string      `json:"op"`
	Game      string      `json:"game"`
	Variation string      `json:"variation"`
	Seed      int64       `json:"seed"`
	Budget    int         `json:"budget"`
	Tables    []TableSpec `json:"tables"`
	Choices   []int       `json:"choices"`
}

type reply struct {
	Tables   []Observation `json:"tables,omitempty"`
	StateDim int           `json:"stateDim,omitempty"`
	CandDim  int           `json:"candDim,omitempty"`
	Error    string        `json:"error,omitempty"`
}

// Serve answers requests until r closes.
func Serve(r io.Reader, w io.Writer) error {
	in := bufio.NewScanner(r)
	in.Buffer(make([]byte, 1<<20), 64<<20)
	out := bufio.NewWriter(w)
	enc := json.NewEncoder(out)
	var env *Env
	var game Game
	for in.Scan() {
		var req request
		var rep reply
		if err := json.Unmarshal(in.Bytes(), &req); err != nil {
			rep.Error = err.Error()
		} else {
			switch req.Op {
			case "reset":
				g, err := LookupGame(req.Game)
				if err == nil {
					budget := req.Budget
					if budget <= 0 {
						budget = 20000
					}
					var e *Env
					if e, err = NewEnv(g, req.Variation, req.Tables, req.Seed, budget); err == nil {
						if rep.Tables, err = e.Observe(); err == nil {
							env, game = e, g
						}
					}
				}
				if err != nil {
					rep.Error = err.Error()
				}
			case "step":
				if env == nil {
					rep.Error = "step before reset"
				} else if obs, err := env.Step(req.Choices); err != nil {
					rep.Error = err.Error()
				} else {
					rep.Tables = obs
				}
			case "info":
				g, err := LookupGame(req.Game)
				if err != nil && game != nil {
					g, err = game, nil
				}
				if err != nil {
					rep.Error = err.Error()
				} else {
					rep.StateDim, rep.CandDim = g.StateDim(), g.CandDim()
				}
			default:
				rep.Error = fmt.Sprintf("unknown op %q", req.Op)
			}
		}
		if err := enc.Encode(rep); err != nil {
			return err
		}
		if err := out.Flush(); err != nil {
			return err
		}
	}
	return in.Err()
}
