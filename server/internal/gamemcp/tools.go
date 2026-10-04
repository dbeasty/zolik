package gamemcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// tool is one tool, once: its name and description for tools/list, and its
// handler for both front doors.
type tool struct {
	name, desc string
	// call decodes args (strictly: an unknown field is a mistake worth
	// hearing about), runs the tool and returns its typed result.
	call func(svc *Service, args json.RawMessage) (any, error)
	// add registers it on an MCP server.
	add func(svc *Service, s *mcp.Server)
}

func def[In, Out any](name, desc string, f func(*Service, In) (Out, error)) tool {
	run := func(svc *Service, in In) (Out, error) {
		svc.mu.Lock()
		defer svc.mu.Unlock()
		return f(svc, in)
	}
	return tool{
		name: name, desc: desc,
		call: func(svc *Service, args json.RawMessage) (any, error) {
			var in In
			if len(bytes.TrimSpace(args)) > 0 && string(bytes.TrimSpace(args)) != "null" {
				dec := json.NewDecoder(bytes.NewReader(args))
				dec.DisallowUnknownFields()
				if err := dec.Decode(&in); err != nil {
					return nil, fmt.Errorf("%s: bad arguments: %w", name, err)
				}
			}
			return run(svc, in)
		},
		add: func(svc *Service, s *mcp.Server) {
			mcp.AddTool(s, &mcp.Tool{Name: name, Description: desc},
				func(_ context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
					out, err := run(svc, in)
					if err != nil {
						var zero Out
						return nil, zero, err
					}
					// The text a model reads first is the rendering, not a
					// JSON dump of the structured result (which still goes
					// out as structuredContent).
					res := &mcp.CallToolResult{}
					if tx := textOf(out); tx != "" {
						res.Content = []mcp.Content{&mcp.TextContent{Text: tx}}
					}
					return res, out, nil
				})
		},
	}
}

// textOf is a result's rendering for a reader, where it has one.
func textOf(v any) string {
	switch x := v.(type) {
	case ListGamesOut:
		return x.Text
	case NewTableOut:
		return fmt.Sprintf("Table %s (seed %d)\n", x.Table, x.Seed) + logBlock(x.Log) + "\n" + x.Observation.Text
	case Observation:
		return x.Text
	case PlayOut:
		return x.Observation.Text
	case CoachOut:
		return x.Text
	case ReplayOut:
		return x.Text
	case RulesOut:
		return x.Text
	case CloseOut:
		return "Closed " + x.Closed
	}
	return ""
}

var tools = []tool{
	def("list_games", "List the games this server hosts: variations, table options and their allowed values, seat ranges, the opponents each accepts, and whether a trained model can play and coach it.",
		func(svc *Service, in ListGamesIn) (ListGamesOut, error) { return svc.ListGames(in) }),
	def("new_table", "Deal a new match. claude_seats are the seats you drive with play; every other seat is a bot (easy, medium, hard, a style, or net:<model path>[@temperature] for a trained model). Bots move automatically until one of your seats is on turn. Returns the table id and your first seat's observation. Deterministic from the seed.",
		func(svc *Service, in NewTableIn) (NewTableOut, error) { return svc.NewTable(in) }),
	def("observe", "What one seat may see and nothing more (the module's own per-seat view): its hand, the table, the pile, the other seats' card counts, the scores, whose turn it is, and the numbered legal moves.",
		func(svc *Service, in ObserveIn) (Observation, error) { return svc.Observe(in) }),
	def("play", "Make a move for one of your seats, by its number in the last observation's move list (or as an explicit engine action). The bots then play until one of your seats is on turn again. Returns the public log of what everyone did in between and the new observation. An illegal move is refused with the reason.",
		func(svc *Service, in PlayIn) (PlayOut, error) { return svc.Play(in) }),
	def("coach", "Score every legal move of one of your seats' current decision with a trained model (probabilities at temperature 1; default model from ZOLIK_LEARNED_MODEL_<GAME>), and say what the hand-written Hard bot would play. Useful to coach a player or to audit the model.",
		func(svc *Service, in CoachIn) (CoachOut, error) { return svc.Coach(in) }),
	def("replay_log", "The public move history of a table so far. With reveal (only once the match is finished) also every action as sent, every event unfiltered, what a trained bot thought of each of its own decisions, and all hands.",
		func(svc *Service, in ReplayIn) (ReplayOut, error) { return svc.Replay(in) }),
	def("rules", "The written rules of a game, resolved for a variation and table options, so you can play it correctly.",
		func(svc *Service, in RulesIn) (RulesOut, error) { return svc.Rules(in) }),
	def("close_table", "Close a table and forget it.",
		func(svc *Service, in CloseIn) (CloseOut, error) { return svc.Close(in) }),
}

// ToolNames lists the tools, sorted.
func ToolNames() []string {
	out := make([]string, len(tools))
	for i, t := range tools {
		out[i] = t.name
	}
	sort.Strings(out)
	return out
}

// Call runs one tool by name with JSON arguments.
func (svc *Service) Call(name string, args json.RawMessage) (any, error) {
	for _, t := range tools {
		if t.name == name {
			return t.call(svc, args)
		}
	}
	return nil, fmt.Errorf("no tool %q (have %v)", name, ToolNames())
}

// Version is what the server reports in initialize.
const Version = "0.1.0"

// NewMCPServer is an MCP server with every tool, over svc.
func NewMCPServer(svc *Service) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "zolik-games", Version: Version}, &mcp.ServerOptions{
		Instructions: "Play, inspect and coach card games on the zolik rules engine. Start with list_games and rules, " +
			"deal with new_table, then alternate observe/play; coach scores your options with a trained model. " +
			"Moves are numbered 0.. in each observation; seats are 0-based.",
	})
	for _, t := range tools {
		t.add(svc, s)
	}
	return s
}
