package learn

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"zolik/server/internal/learn/models"
	"zolik/server/internal/module"
)

// HardModel is what a learnable module's Bot() returns: its heuristic, or —
// when an operator has switched the game's shipped model on — a bot that plays
// Hard seats with that model and every other seat with the heuristic.
//
// The switch is read on every call, and module.BotFor calls Bot() on every
// bot move, so a change in the admin console takes effect at the next move of
// every table with no restart. Easy and Medium seats never change.
//
// Precedence, highest first:
//
//  1. ZOLIK_LEARNED_MODEL_<GAME> names a model file (LocalHard). That is a
//     person testing a model on their own machine, and it wins whatever the
//     switch says; the console shows that the environment overrides it.
//  2. The switch is on and the shipped model fits this game's encoder: Hard
//     seats play the model.
//  3. Otherwise, the heuristic, exactly as before this existed.
//
// Cheap per move: an environment read, a map read and an atomic load when the
// switch is off; when it is on, one more sync.Map read for the bot built the
// first time. The model is parsed once per process (LoadEmbedded).
func HardModel(game Game, heuristic module.Bot) module.Bot {
	m := hardModels[game.Name()]
	if m == nil {
		return LocalHard(game, heuristic)
	}
	if os.Getenv(m.envKey) != "" {
		return LocalHard(game, heuristic)
	}
	if !m.enabled.Load() {
		return heuristic
	}
	return m.bot(game, heuristic)
}

// ErrUnknownGame is a switch asked for a game that ships no model.
var ErrUnknownGame = errors.New("learn: no shipped model for that game")

// ErrModelDoesNotFit is a switch asked to seat a model that would not play:
// it does not load, or it was trained for another encoder.
var ErrModelDoesNotFit = errors.New("learn: the shipped model does not fit this game's encoder")

// SetHardModel turns a game's shipped model on or off for Hard seats, and
// reports what the switch was before. Turning on a model that does not fit is
// refused and leaves the switch off: HardModel would fall back to the
// heuristic anyway, but a console that says "on" for a bot that is not playing
// is worse than one that says no.
//
// The switch lives in this process only. Production is one instance
// (deploy/compose/zolik.yml); a deployment of several would need each
// change broadcast — the Redis hub channel is the obvious carrier — so every
// instance flips together. The persisted setting is loaded into each at boot
// either way.
func SetHardModel(game string, on bool) (was bool, err error) {
	m := hardModels[game]
	if m == nil {
		return false, fmt.Errorf("%w: %q", ErrUnknownGame, game)
	}
	if on {
		if fits, why := m.fits(); !fits {
			return m.enabled.Load(), fmt.Errorf("%w: %s", ErrModelDoesNotFit, why)
		}
	}
	return m.enabled.Swap(on), nil
}

// HardModelStatus is one game's switch as the console shows it.
type HardModelStatus struct {
	Game     string `json:"game"`
	Enabled  bool   `json:"enabled"`
	Embedded bool   `json:"embedded"`
	Fits     bool   `json:"fits"`
	// Problem says why a model does not fit, when it does not.
	Problem string `json:"problem,omitempty"`
	// EnvOverride is the model file ZOLIK_LEARNED_MODEL_<GAME> names, which
	// Hard seats play whatever the switch says.
	EnvOverride string      `json:"envOverride,omitempty"`
	Model       models.Info `json:"model"`
}

// HardModels reports every game that ships a model, sorted by name.
func HardModels() []HardModelStatus {
	out := make([]HardModelStatus, 0, len(hardModels))
	for name, m := range hardModels {
		fits, why := m.fits()
		out = append(out, HardModelStatus{
			Game:        name,
			Enabled:     m.enabled.Load(),
			Embedded:    len(m.bytes) > 0,
			Fits:        fits,
			Problem:     why,
			EnvOverride: localModelPath(name),
			Model:       m.info,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Game < out[j].Game })
	return out
}

// --- the table ----------------------------------------------------------------

type hardModel struct {
	name string
	// envKey is ZOLIK_LEARNED_MODEL_<GAME>, built once so the per-move check
	// does not build the string every time.
	envKey  string
	bytes   []byte
	info    models.Info
	enabled atomic.Bool
	// bots holds the bySkill built for each heuristic, so the per-move path
	// allocates nothing after the first.
	bots sync.Map
}

// hardModels is built once from what the binary ships and never grows after
// init (tests add entries before they play). Only the enabled flags change.
var hardModels = map[string]*hardModel{}

func init() {
	for _, name := range models.Games() {
		b, info, _ := models.Hard(name)
		hardModels[name] = newHardModel(name, b, info)
	}
}

func newHardModel(name string, b []byte, info models.Info) *hardModel {
	return &hardModel{name: name, envKey: localModelEnv(name), bytes: b, info: info}
}

func (m *hardModel) bot(game Game, heuristic module.Bot) module.Bot {
	// A heuristic is a comparable value in every module (an empty struct),
	// which is what lets it key the cache. One that is not is built afresh,
	// which is correct and merely slower.
	if !reflect.TypeOf(heuristic).Comparable() {
		return m.resolve(game, heuristic)
	}
	if b, ok := m.bots.Load(heuristic); ok {
		return b.(module.Bot)
	}
	b, _ := m.bots.LoadOrStore(heuristic, m.resolve(game, heuristic))
	return b.(module.Bot)
}

func (m *hardModel) resolve(game Game, heuristic module.Bot) module.Bot {
	hard := HardBot(game, m.bytes, heuristic)
	if _, ok := hard.(NetBot); !ok {
		warnOnce("hard:"+game.Name(), "learn: shipped model does not fit this game; Hard seats play the heuristic", "game", game.Name())
		return heuristic
	}
	return bySkill{hard: hard, other: heuristic}
}

// fits reports whether the shipped model would play its game, and why not.
func (m *hardModel) fits() (bool, string) {
	if len(m.bytes) == 0 {
		return false, "no model is embedded"
	}
	g, err := LookupGame(m.name)
	if err != nil {
		return false, err.Error()
	}
	p, err := LoadEmbedded(m.name, m.bytes)
	if err != nil {
		return false, err.Error()
	}
	if !p.Fits(g) {
		return false, fmt.Sprintf("model is %d/%d, encoder is %d/%d",
			p.net.StateDim, p.net.CandDim, g.StateDim(), g.CandDim())
	}
	if p.net.Game != "" && p.net.Game != g.Name() {
		return false, fmt.Sprintf("model was trained for %q", p.net.Game)
	}
	return true, ""
}

// localModelPath is the model file the environment names for a game, if any.
func localModelPath(game string) string { return os.Getenv(localModelEnv(game)) }

func localModelEnv(game string) string { return "ZOLIK_LEARNED_MODEL_" + strings.ToUpper(game) }
