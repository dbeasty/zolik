package learn

import (
	"log/slog"
	"os"
	"sync"

	"zolik/server/internal/module"
)

// LocalHard lets a person play a trained model at the Hard setting before it
// has earned shipping: set ZOLIK_LEARNED_MODEL_<GAME> (the game's registry
// name, upper-cased, so ZOLIK_LEARNED_MODEL_ZOLIK) to a model file and a
// server started with it seats that model wherever a lobby asks for a Hard
// bot. Easy and Medium stay the heuristic, so the two can be told apart at one
// table.
//
// Unset — every deployment — it returns heuristic unchanged: nothing about
// production changes by this function existing. A file that does not load, or
// was trained for another encoder, is logged and ignored rather than crashing
// the server, and play falls back to the heuristic exactly as HardBot does.
func LocalHard(game Game, heuristic module.Bot) module.Bot {
	path := localModelPath(game.Name())
	if path == "" {
		return heuristic
	}
	// A module's Bot() is asked for on every move, so the file is read and
	// parsed once per path, and every later move gets the same bot back.
	key := game.Name() + "\x00" + path
	if b, ok := local.Load(key); ok {
		return b.(module.Bot)
	}
	b, _ := local.LoadOrStore(key, resolveLocal(game, path, heuristic))
	return b.(module.Bot)
}

var local sync.Map

func resolveLocal(game Game, path string, heuristic module.Bot) module.Bot {
	b, err := os.ReadFile(path)
	if err != nil {
		warnOnce(game.Name(), "learn: cannot read local model; playing the heuristic", "path", path, "err", err)
		return heuristic
	}
	hard := HardBot(game, b, heuristic)
	if _, ok := hard.(NetBot); !ok {
		warnOnce(game.Name(), "learn: local model does not fit this game; playing the heuristic", "path", path)
		return heuristic
	}
	infoOnce(game.Name(), "learn: Hard seats play a trained model", "game", game.Name(), "path", path)
	return bySkill{hard: hard, other: heuristic}
}

// bySkill plays Hard seats with one bot and every other seat with another.
type bySkill struct{ hard, other module.Bot }

// Heuristic is the hand-written bot behind the model. Player hints ask for it
// (match.Manager.Hint): a hint is a suggestion a person reads and learns
// from, and the heuristic's is the one this game's hints have always given,
// whichever bot sits in the Hard seats today.
func (b bySkill) Heuristic() module.Bot { return b.other }

func (b bySkill) Act(s module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	if seat.Skill == module.SkillHard {
		return b.hard.Act(s, seat, offers)
	}
	return b.other.Act(s, seat, offers)
}

// Bot() is asked for on every move, so the log lines are said once per game
// rather than once per move.
var said sync.Map

func infoOnce(game, msg string, args ...any) {
	if _, dup := said.LoadOrStore(game, true); !dup {
		slog.Info(msg, args...)
	}
}

func warnOnce(game, msg string, args ...any) {
	if _, dup := said.LoadOrStore(game, true); !dup {
		slog.Warn(msg, args...)
	}
}
