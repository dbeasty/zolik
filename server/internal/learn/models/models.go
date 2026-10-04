// Package models holds the trained networks that ship inside the server
// binary, and what is known about each: where it was trained, when, and what
// it scored against the hand-written Hard bot.
//
// They are compiled in with go:embed rather than read from disk so that a
// release carries its models and a rollback restores the ones it shipped
// with: there is no second artefact to deploy, forget, or leave behind at the
// wrong version.
//
// Shipping a model changes nothing on its own. Whether a Hard seat plays it is
// an operator's switch (learn.HardModels, driven from the admin console), and
// that switch is off for every game until somebody turns it on.
//
// This package imports nothing of the server's, so learn can depend on it
// without a cycle and every game module reaches its model through learn.
package models

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"sort"
)

//go:embed zolik.bin
var zolik []byte

//go:embed canasta.bin
var canasta []byte

//go:embed holdem.bin
var holdem []byte

// Info is what the console shows about a shipped model.
type Info struct {
	// Game is the learn registry name, the same as the module id.
	Game string `json:"game"`
	// Title is the game's name as people write it.
	Title string `json:"title"`
	// SourceRun is the training run the bytes were copied from.
	SourceRun string `json:"sourceRun"`
	// Trained is the day the run wrote its final.bin, YYYY-MM-DD.
	Trained string `json:"trained"`
	// Encoder names the observation layout the network was trained for. The
	// authoritative check is learn.Policy.Fits; this is for people.
	Encoder string `json:"encoder"`
	// Benchmark is the headline result against the Hard heuristic.
	Benchmark string `json:"benchmark"`
	// Bytes and SHA256 identify exactly which file shipped.
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type shipped struct {
	bytes []byte
	info  Info
}

var hard = map[string]shipped{
	"zolik": {zolik, Info{
		Title:     "Žolíky",
		SourceRun: "zolik-adapter-v2/ml/runs/zolik-v2",
		Trained:   "2026-09-30",
		Encoder:   "Žolíky encoder v2 (900 state / 62 candidate)",
		Benchmark: "+117 vs Hard at 4 seats, 35-point minimum; +160 at 2-seat classic",
	}},
	"canasta": {canasta, Info{
		Title:     "Canasta",
		SourceRun: "zolik-learn-core/ml/runs/canasta-long",
		Trained:   "2026-09-29",
		Encoder:   "Canasta encoder (383 state / 40 candidate)",
		Benchmark: "+7,003 vs Hard at 2 seats; +3,084 at 4 seats",
	}},
	"holdem": {holdem, Info{
		Title:     "Hold'em",
		SourceRun: "zolik-learn-core/ml/runs/holdem-long",
		Trained:   "2026-09-29",
		Encoder:   "Hold'em encoder (295 state / 14 candidate)",
		Benchmark: "+21.4 BB per match heads-up vs Hard",
	}},
}

func init() {
	for name, s := range hard {
		sum := sha256.Sum256(s.bytes)
		s.info.Game = name
		s.info.Bytes = len(s.bytes)
		s.info.SHA256 = hex.EncodeToString(sum[:])
		hard[name] = s
	}
}

// Hard returns the model shipped for a game's Hard seats and what is known
// about it. The slice is the embedded variable itself — it never moves, which
// is what lets learn.LoadEmbedded cache the parsed network by its address —
// and must not be written.
func Hard(game string) ([]byte, Info, bool) {
	s, ok := hard[game]
	if !ok {
		return nil, Info{}, false
	}
	return s.bytes, s.info, true
}

// Games lists every game with a shipped Hard model, sorted.
func Games() []string {
	out := make([]string, 0, len(hard))
	for name := range hard {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
