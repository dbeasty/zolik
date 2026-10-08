// Package gamemcp lets a model — Claude through MCP, or a Claude Code session
// through the command line — sit at a table of any game the server hosts,
// against the heuristic bots or a trained network, and ask for coaching.
//
// It is a driver of the same kind as internal/learn's bench and environment:
// every move goes through the module's Apply, every legal move comes from the
// module's offers (through the learn adapter's Candidates where the game has
// one), and every board a seat is shown is the module's own View for that
// seat — the one place hidden information is filtered. Nothing here knows a
// rule of any game.
package gamemcp

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"zolik/server/internal/blackjack"
	"zolik/server/internal/canasta"
	"zolik/server/internal/ferbl"
	"zolik/server/internal/ginrummy"
	"zolik/server/internal/holdem"
	"zolik/server/internal/klondike"
	"zolik/server/internal/lastcard"
	"zolik/server/internal/learn"
	"zolik/server/internal/marias"
	"zolik/server/internal/module"
	"zolik/server/internal/okobere"
	"zolik/server/internal/prsi"
	"zolik/server/internal/rummytiles"
	"zolik/server/internal/sedma"
	"zolik/server/internal/snaps"
	"zolik/server/internal/zolikmod"
)

// registry is every game the server hosts, as cmd/server registers them.
var registry = module.NewRegistry(zolikmod.New(), prsi.New(), canasta.New(), holdem.New(),
	ginrummy.New(), rummytiles.New(), blackjack.New(), marias.New(), klondike.New(), lastcard.New(), ferbl.New(), okobere.New(), sedma.New(), snaps.New())

// game is one module, with its learn adapter when it has one.
type game struct {
	m module.GameModule
	// g is the learn adapter: concrete candidates and a network's encoder.
	// Nil for a game nobody has taught a network yet; those are played from
	// the offer list.
	g learn.Game
	// b is the benchable registration, for its heuristic and styles, when
	// the game has one.
	b learn.Benchable
}

func lookupGame(id string) (*game, error) {
	m := registry.Get(id)
	if m == nil {
		return nil, fmt.Errorf("no game %q (have %s)", id, strings.Join(registry.IDs(), ", "))
	}
	out := &game{m: m}
	if b, err := learn.Lookup(id); err == nil {
		out.b = b
		out.g, _ = b.(learn.Game)
	}
	return out, nil
}

// heuristic is the hand-written bot for a seat: the learn registration's
// (which is what the bench measures against) or the module's own.
func (g *game) heuristic() module.Bot {
	if g.b != nil {
		return g.b.Heuristic()
	}
	return module.BotFor(g.m)
}

func (g *game) styles() map[string]module.Bot {
	if st, ok := g.b.(learn.Styled); ok && g.b != nil {
		return st.Styles()
	}
	return nil
}

// modelEnv is the variable naming a game's local trained model, the same one
// learn.LocalHard reads on the branches that have it.
func modelEnv(gameID string) string {
	return "ZOLIK_LEARNED_MODEL_" + strings.ToUpper(gameID)
}

// defaultModel is the model path the environment names for a game, if any.
func defaultModel(gameID string) string { return os.Getenv(modelEnv(gameID)) }

// --- list_games ---------------------------------------------------------------

// GameInfo is one game as list_games describes it.
type GameInfo struct {
	ID         string          `json:"id"`
	Label      string          `json:"label"`
	MinSeats   int             `json:"minSeats"`
	MaxSeats   int             `json:"maxSeats"`
	Variations []VariationInfo `json:"variations"`
	Options    []OptionInfo    `json:"options"`
	// Learnable games have concrete candidate moves (multi-step ones
	// included) and can seat a trained network and be coached by one.
	Learnable bool     `json:"learnable"`
	Opponents []string `json:"opponents" jsonschema:"the bot specs this game accepts, besides net:<path>[@temperature] on a learnable game"`
	// ModelEnv is the environment variable coach reads a default model from.
	ModelEnv string `json:"modelEnv,omitempty"`
	ModelSet bool   `json:"modelSet,omitempty"`
}

// VariationInfo is one shipped ruleset.
type VariationInfo struct {
	ID       string         `json:"id"`
	Label    string         `json:"label"`
	MinSeats int            `json:"minSeats"`
	MaxSeats int            `json:"maxSeats"`
	Defaults map[string]int `json:"defaults,omitempty"`
}

// OptionInfo is one table option and the values it takes.
type OptionInfo struct {
	Name   string   `json:"name"`
	Label  string   `json:"label"`
	Help   string   `json:"help,omitempty"`
	Values []int    `json:"values"`
	Labels []string `json:"labels"`
}

func listGames() []GameInfo {
	var out []GameInfo
	for _, id := range registry.IDs() {
		g, _ := lookupGame(id)
		d := g.m.Descriptor()
		info := GameInfo{ID: d.ID, Label: d.Label, MinSeats: d.MinPlayers, MaxSeats: d.MaxPlayers,
			Learnable: g.g != nil, Variations: []VariationInfo{}, Options: []OptionInfo{}}
		for _, v := range d.Variations {
			lo, hi := d.SeatRange(v.ID)
			info.Variations = append(info.Variations, VariationInfo{ID: v.ID, Label: v.Label, MinSeats: lo, MaxSeats: hi, Defaults: v.Defaults})
		}
		for _, o := range d.Options {
			oi := OptionInfo{Name: o.Name, Label: o.Label, Help: o.Help, Values: []int{}, Labels: []string{}}
			for _, c := range o.Choices {
				oi.Values = append(oi.Values, c.Value)
				oi.Labels = append(oi.Labels, c.Label)
			}
			info.Options = append(info.Options, oi)
		}
		for _, s := range module.Skills {
			info.Opponents = append(info.Opponents, string(s))
		}
		styles := make([]string, 0)
		for name := range g.styles() {
			styles = append(styles, name)
		}
		sort.Strings(styles)
		info.Opponents = append(info.Opponents, styles...)
		if g.g != nil {
			info.ModelEnv = modelEnv(id)
			info.ModelSet = defaultModel(id) != ""
		}
		out = append(out, info)
	}
	return out
}
