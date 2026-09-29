// Package learn is the shared machinery for bots that are trained rather than
// written: the contract a game implements to be learnable, the forward pass a
// trained network runs in, the bot that plays one, the duplicate bench that
// decides whether it is any good, and the environment a trainer drives.
//
// Nothing here knows a rule of any game. A game's rules stay in its module and
// its engine stays the only authority on what is legal — exactly as it is for
// the hand-written bots — and what a game adds to be learnable is three
// translations: a position into numbers (Encode), the legal moves into
// numbered candidates (Candidates), and an outcome into a reward (Reward).
// The trainer (ml/, in Python) never re-implements a game; it only ever sees
// what those three produce.
//
// The policy scores candidates rather than choosing from a fixed output
// layer, because the games disagree about how many moves there are. Hold'em
// has a handful at every decision; Canasta has anywhere from one to dozens,
// and which ones exist depends on the hand. A network that scores "this state
// with this move" works for both without either pretending to be the other.
package learn

import (
	"fmt"
	"sort"
	"sync"

	"zolik/server/internal/module"
)

// Benchable is what a game needs to be measured: enough to deal a table, play
// it out with any bots, and say how each seat did.
//
// Split from Game because a game can be benched long before it can be learned,
// and the bench is what tells the learning work whether it is going anywhere.
type Benchable interface {
	// Name is the registry key, the same as the module id.
	Name() string
	Module() module.GameModule
	// Config is the table a bench or a trainer deals for this many seats.
	Config(seats int, variation string) module.MatchConfig
	// Heuristic is the hand-written bot, which a trained one falls back to and
	// is measured against.
	Heuristic() module.Bot
	// Outcome is how one seat finished the match, in the game's own unit —
	// chips won for Hold'em, the partnership's score for Canasta. The bench
	// only ever compares outcomes of the same game, so the units need not
	// agree across games.
	Outcome(s module.State, seat string) (float64, error)
}

// Game is a game a network can learn.
type Game interface {
	Benchable

	// StateDim and CandDim are the fixed widths of Encode's vector and of
	// every candidate's feature vector. A network is trained for one pair and
	// refuses to load against another.
	StateDim() int
	CandDim() int

	// Encode is the position as one seat may know it — its own hidden cards
	// and everything public, never anyone else's. Every adapter carries a test
	// that two states differing only in cards this seat cannot see encode
	// identically.
	Encode(s module.State, seat string) ([]float32, error)

	// Candidates are the concrete moves this seat may make now, each with the
	// action that performs it. Built only from enabled offers: a candidate is
	// legal because the engine offered it, never because the adapter decided
	// so. Empty means the adapter has no opinion, and the caller falls back.
	Candidates(s module.State, seat string, offers []module.ActionOffer) ([]Candidate, error)

	// Reward is what one applied action earned the seat, and whether an
	// episode — a hand, a deal — ended with it. Partners share a reward, so a
	// Canasta seat is rewarded for its partner's meld.
	Reward(before, after module.State, seat string) (reward float32, episodeDone bool, err error)
}

// Candidate is one legal move and what the network is told about it.
type Candidate struct {
	Action   module.Action
	Features []float32
}

var (
	regMu    sync.RWMutex
	registry = map[string]Benchable{}
)

// Register makes a game available to cmd/gamebench and cmd/gameenv. Adapters
// call it from init, and the commands import them for that side effect.
func Register(g Benchable) {
	regMu.Lock()
	defer regMu.Unlock()
	if _, dup := registry[g.Name()]; dup {
		panic("learn: game registered twice: " + g.Name())
	}
	registry[g.Name()] = g
}

// Lookup finds a registered game.
func Lookup(name string) (Benchable, error) {
	regMu.RLock()
	defer regMu.RUnlock()
	if g, ok := registry[name]; ok {
		return g, nil
	}
	return nil, fmt.Errorf("learn: no game %q (have %v)", name, names())
}

// LookupGame finds a registered game that can also be learned.
func LookupGame(name string) (Game, error) {
	b, err := Lookup(name)
	if err != nil {
		return nil, err
	}
	g, ok := b.(Game)
	if !ok {
		return nil, fmt.Errorf("learn: %q can be benched but not learned yet", name)
	}
	return g, nil
}

func names() []string {
	out := make([]string, 0, len(registry))
	for n := range registry {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
