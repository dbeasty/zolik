package rummytiles

import (
	"encoding/json"
	"fmt"

	"zolik/server/internal/module"
)

// Verbs this module accepts.
const (
	VerbPlace     = "place"
	VerbAdd       = "add"
	VerbTake      = "take"
	VerbSplit     = "split"
	VerbSwapJoker = "swap_joker"
	VerbResetTurn = "reset_turn"
	VerbCommit    = "commit"
	VerbDraw      = "draw"
)

// Error codes. Stable keys, rendered by the client's locale bundle.
const (
	ErrWrongPlayerCount  = "WRONG_PLAYER_COUNT"
	ErrGameNotActive     = "GAME_NOT_ACTIVE"
	ErrNotYourTurn       = "NOT_YOUR_TURN"
	ErrUnknownAction     = "UNKNOWN_ACTION"
	ErrTileNotInHand     = "TILE_NOT_IN_HAND"
	ErrTileDoesNotFit    = "TILE_DOES_NOT_FIT"
	ErrNoSuchSet         = "NO_SUCH_SET"
	ErrInitialMeldOnly   = "INITIAL_MELD_ONLY"
	ErrTableNotValid     = "TABLE_NOT_VALID"
	ErrTrayNotEmpty      = "TRAY_NOT_EMPTY"
	ErrNothingPlayed     = "NOTHING_PLAYED"
	ErrInitialMeldLow    = "INITIAL_MELD_TOO_LOW"
	ErrNotARun           = "NOT_A_RUN"
	ErrBadSplitPosition  = "BAD_SPLIT_POSITION"
	ErrNoJokerInSet      = "NO_JOKER_IN_SET"
	ErrJokerSwapMismatch = "JOKER_SWAP_MISMATCH"
	ErrNothingToDraw     = "NOTHING_TO_DRAW"
)

// Set is one group or run, on the table or in a player's workspace.
type Set struct {
	ID    string   `json:"id"`
	Kind  string   `json:"kind"` // "group" | "run" — informational; commit revalidates regardless.
	Cards []string `json:"cards"`
}

// Workspace is the scratch copy of the table the player on turn edits with
// small moves — see the package doc on why the turn is shaped this way. It
// exists only while a turn is in progress; NewMatch and every commit/draw
// leave it nil.
type Workspace struct {
	Sets []Set `json:"sets"`
	// Tray holds tiles taken off a set (via take, or freed by swap_joker) and
	// not yet placed anywhere. Commit requires it empty — see §3.3.
	Tray []string `json:"tray,omitempty"`
	// NewSetIDs are sets created this turn via place. Only a new set may be
	// touched by add/take/split/swap_joker before the player's initial meld —
	// their own in-progress lay is never "the table" in that sense.
	NewSetIDs map[string]bool `json:"newSetIds,omitempty"`
	// PlayedFromHand is every tile moved out of the hand this turn, in the
	// order it left. reset_turn hands them straight back; nothing else reads
	// order, only membership and count.
	PlayedFromHand []string `json:"playedFromHand,omitempty"`
}

// GameState is the whole match. Opaque to the runtime — only this package
// reads it.
type GameState struct {
	Status    string `json:"status"` // "active" | "completed"
	Variation string `json:"variation"`

	Players []string `json:"players"` // seating order, stable for the match
	Current string   `json:"current"`

	Hands map[string][]string `json:"hands"`
	Pool  []string            `json:"pool"`
	Sets  []Set               `json:"sets"`

	Workspace *Workspace `json:"workspace,omitempty"`

	// InitialMeld records, per player, whether they have made their 30-point
	// lay yet this round. Reset every round.
	InitialMeld map[string]bool `json:"initialMeld"`

	NextSetID int `json:"nextSetId"`

	RoundNumber int            `json:"roundNumber"`
	Scores      map[string]int `json:"scores"`

	Intermission module.Intermission `json:"intermission,omitempty"`
	Pause        bool                `json:"pause,omitempty"`

	TargetScore int `json:"targetScore"`
	RoundLimit  int `json:"roundLimit"`
	// PoolExhaustionLowestWins resolves the one rule the physical sets
	// disagree on: if the pool runs dry and nobody can play, does the lowest
	// hand value win, or does nobody?
	PoolExhaustionLowestWins bool `json:"poolExhaustionLowestWins"`

	Seed int64 `json:"seed"`

	WinnerID string        `json:"winnerId,omitempty"`
	Rounds   []RoundResult `json:"rounds,omitempty"`
}

// RoundResult is one round's settlement, kept for Rounds().
type RoundResult struct {
	Number int            `json:"number"`
	Kind   string         `json:"kind"` // "out" | "pool_exhausted"
	Winner string         `json:"winner,omitempty"`
	Deltas map[string]int `json:"deltas,omitempty"`
	Totals map[string]int `json:"totals,omitempty"`
}

func decode(raw module.State) (*GameState, error) {
	var s GameState
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("rummytiles: decode state: %w", err)
	}
	return &s, nil
}

func encode(s *GameState) (module.State, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("rummytiles: encode state: %w", err)
	}
	return raw, nil
}

func errCode(code string) error { return module.Error{Code: code} }

func nextPlayer(players []string, from string) string {
	for i, p := range players {
		if p == from {
			return players[(i+1)%len(players)]
		}
	}
	return ""
}

func setIndex(sets []Set, id string) int {
	for i, s := range sets {
		if s.ID == id {
			return i
		}
	}
	return -1
}
