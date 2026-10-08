// Package klondike implements Solitaire (Klondike) as a game module.
//
// It is the first game with one seat, no opponent and no bot, and the first
// whose secret is not a hand but the deck itself: the face-down cards in the
// tableau and the stock are the only thing the player is playing against, so
// they are never sent — a column's View carries Group.Hidden, a count, and
// nothing names a card until it has been turned up.
//
// Rules implemented (docs/solitaire-plan.md §1):
//   - Seven columns of one to seven cards, only the top card of each face up;
//     the other 24 cards are the stock.
//   - The stock turns one or three cards at a time to the waste, and the waste
//     is turned back over as a new stock up to the table's redeal limit.
//   - Columns build down in alternating colours; any face-up run moves as a
//     unit; only a king (or a run headed by one) fills an empty column; the
//     card exposed under a run turns up by itself.
//   - Four foundations are built up from ace to king by suit; a card may come
//     back down to the tableau when the table allows it.
//   - Won when all 52 cards are on the foundations; lost when the player gives
//     up or the game can no longer go anywhere.
package klondike

import (
	"encoding/json"
	"fmt"

	"zolik/server/internal/module"
)

// Verbs this module accepts.
const (
	VerbDraw       = "draw"
	VerbRecycle    = "recycle"
	VerbMove       = "move"
	VerbAutoFinish = "autofinish"
	VerbUndo       = "undo"
	VerbGiveUp     = "giveup"
)

// Offer ids that do not depend on the position.
const (
	OfferDraw       = "draw"
	OfferRecycle    = "recycle"
	OfferAutoFinish = "autofinish"
	OfferUndo       = "undo"
	OfferGiveUp     = "giveup"
)

// Match status.
const (
	statusActive = "active"
	statusWon    = "won"
	statusLost   = "lost"
)

// idleLimit is how many moves in a row may achieve nothing — no card to a
// foundation, none turned up, none played off the waste — before the game
// calls itself stuck. Without it a position that has only shuffling left could
// be shuffled for ever; with it every game ends, and no one playing for a
// result gets near it.
const idleLimit = 250

// undoDepth bounds the history a state carries.
const undoDepth = 200

// Column is one tableau column: Down is the face-down cards (bottom first),
// Up is the face-up run (bottom first, so Up[0] is its head).
type Column struct {
	Down []string `json:"down"`
	Up   []string `json:"up"`
}

// step is one move an undo can take back: the cards, where they went and
// where they came from, and the counters as they stood before. Only a move
// that turned nothing up is ever recorded, so taking it back is just carrying
// the cards home again.
type step struct {
	Cards []string `json:"c"`
	From  string   `json:"f"`
	To    string   `json:"t"`
	Score int      `json:"s"`
	Moves int      `json:"m"`
	Idle  int      `json:"i"`
	Moved bool     `json:"mv,omitempty"`
}

// GameState is everything a Klondike match is. Opaque outside this package.
type GameState struct {
	Variation  string `json:"variation,omitempty"`
	Draw       int    `json:"draw"`
	Redeals    int    `json:"redeals"`
	Scoring    int    `json:"scoring"`
	TakeBack   bool   `json:"takeBack"`
	AutoFinish bool   `json:"autoFinish"`

	Seed   int64  `json:"seed"`
	Player string `json:"player"`
	Status string `json:"status"`

	Cols  []Column   `json:"cols"`
	Found [][]string `json:"found"`
	Stock []string   `json:"stock"` // top is last
	Waste []string   `json:"waste"` // top is last

	Score    int `json:"score"`
	Moves    int `json:"moves"`
	Recycles int `json:"recycles"`
	// Idle counts moves since anything last got somewhere.
	Idle int `json:"idle"`
	// Moved is whether a card has moved since the waste was last turned over.
	// A pass through the stock with no card moved changes nothing, so turning
	// it over again is not progress.
	Moved bool `json:"moved"`

	// Undo is the history a player may step back through. It is emptied by
	// anything that shows a card the player had not seen — a column turning
	// up, a draw — because taking those back would let them look and hide it.
	Undo []step `json:"undo,omitempty"`
	// BestFound is the most cards the foundations have held. A card going up
	// again after being taken down is not progress, or one card could be
	// carried up and down for ever.
	BestFound int `json:"bestFound,omitempty"`
}

func (s *GameState) unlimitedRedeals() bool { return s.Redeals >= redealsUnlimited }

func (s *GameState) redealsLeft() int {
	if s.unlimitedRedeals() {
		return -1
	}
	if left := s.Redeals - s.Recycles; left > 0 {
		return left
	}
	return 0
}

func (s *GameState) canRecycle() bool {
	return len(s.Stock) == 0 && len(s.Waste) > 0 && (s.unlimitedRedeals() || s.Recycles < s.Redeals)
}

func (s *GameState) foundationCount() int {
	n := 0
	for _, f := range s.Found {
		n += len(f)
	}
	return n
}

func cloneList(a []string) []string { return append([]string{}, a...) }

func cloneLists(a [][]string) [][]string {
	out := make([][]string, len(a))
	for i := range a {
		out[i] = cloneList(a[i])
	}
	return out
}

func decode(raw module.State) (*GameState, error) {
	var s GameState
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("klondike: decode state: %w", err)
	}
	return &s, nil
}

func encode(s *GameState) (module.State, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("klondike: encode state: %w", err)
	}
	return raw, nil
}

func errCode(code string) error { return module.Error{Code: code} }

// takeOff removes the top n cards of a pile, column or foundation by id.
func (s *GameState) takeOff(id string, n int) {
	switch {
	case id == zoneWaste:
		s.Waste = s.Waste[:len(s.Waste)-n]
	case foundIndex(id) >= 0:
		f := foundIndex(id)
		s.Found[f] = s.Found[f][:len(s.Found[f])-n]
	case colIndex(id) >= 0:
		c := &s.Cols[colIndex(id)]
		c.Up = c.Up[:len(c.Up)-n]
	}
}

// putOn lays cards on top of a pile, column or foundation by id.
func (s *GameState) putOn(id string, cards []string) {
	switch {
	case id == zoneWaste:
		s.Waste = append(s.Waste, cards...)
	case foundIndex(id) >= 0:
		f := foundIndex(id)
		s.Found[f] = append(s.Found[f], cards...)
	case colIndex(id) >= 0:
		c := &s.Cols[colIndex(id)]
		c.Up = append(c.Up, cards...)
	}
}
