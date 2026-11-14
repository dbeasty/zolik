package blackpeter

import (
	"encoding/json"
	"fmt"

	"zolik/server/internal/module"
)

// Seat is one player: their hand this round, and the marks they carry.
type Seat struct {
	PlayerID string   `json:"playerId"`
	Hand     []string `json:"hand,omitempty"`
	// Marks are rounds lost: each one is a round this seat was left holding
	// Black Peter.
	Marks int `json:"marks,omitempty"`
	// Out is a seat that has got rid of every card this round.
	Out bool `json:"out,omitempty"`
}

// GameState is the whole match. Opaque to the runtime — only this package
// reads it.
type GameState struct {
	Status string `json:"status"`
	Seats  []Seat `json:"seats"`
	Seed   int64  `json:"seed"`

	ByColour   bool `json:"byColour,omitempty"`
	RoundLimit int  `json:"roundLimit"`
	Pause      bool `json:"pause,omitempty"`

	// Round counts rounds finished; the one being played is Round+1.
	Round int `json:"round"`
	// Dealer deals this round; the seat on their left draws first.
	Dealer  int    `json:"dealer"`
	Current string `json:"current,omitempty"`
	// Draws counts cards taken this round. It seeds where an unchosen card
	// is taken from and where a taken card lands in its new hand.
	Draws int `json:"draws,omitempty"`
	// Pairs are the pairs put down this round, oldest first: public.
	Pairs [][]string `json:"pairs,omitempty"`
	// OutOrder is who got rid of their cards this round, in order.
	OutOrder []string `json:"outOrder,omitempty"`

	Rounds       []RoundRecord       `json:"rounds,omitempty"`
	Intermission module.Intermission `json:"intermission,omitempty"`
}

// RoundRecord is one finished round. Public: who went out, and who lost.
type RoundRecord struct {
	Number   int            `json:"number"`
	Loser    string         `json:"loser"`
	OutOrder []string       `json:"outOrder,omitempty"`
	Marks    map[string]int `json:"marks"`
}

// Verbs this module accepts. VerbContinue belongs to module.Intermission.
const VerbDraw = "draw"

// Offer ids, and the draw's one optional parameter.
const (
	OfferDraw = "draw"
	// ParamPick is the position, from 0, of the face-down card taken, in the
	// order the hand is drawn on the table. Absent, a card is taken at
	// random — which, with every card face down, is the same choice.
	ParamPick = module.PickParam
)

// Error codes. Stable keys, worded by the client's locale bundles.
const (
	ErrNotYourTurn   = "NOT_YOUR_TURN"
	ErrGameNotActive = "GAME_NOT_ACTIVE"
	ErrUnknownAction = "UNKNOWN_ACTION"
	ErrPlayerCount   = "WRONG_PLAYER_COUNT"

	ErrOut    = "BLACKPETER_OUT"
	ErrNoCard = "BLACKPETER_NO_SUCH_CARD"
)

func decode(raw module.State) (*GameState, error) {
	var s GameState
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("blackpeter: decode state: %w", err)
	}
	return &s, nil
}

func encode(s *GameState) (module.State, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("blackpeter: encode state: %w", err)
	}
	return raw, nil
}

func errCode(code string) error { return module.Error{Code: code} }

func (s *GameState) seat(id string) *Seat {
	for i := range s.Seats {
		if s.Seats[i].PlayerID == id {
			return &s.Seats[i]
		}
	}
	return nil
}

func (s *GameState) index(id string) int {
	for i := range s.Seats {
		if s.Seats[i].PlayerID == id {
			return i
		}
	}
	return -1
}

// players is every player id, in seat order.
func (s *GameState) players() []string {
	out := make([]string, len(s.Seats))
	for i, st := range s.Seats {
		out[i] = st.PlayerID
	}
	return out
}

// holders is how many seats still hold cards.
func (s *GameState) holders() int {
	n := 0
	for _, st := range s.Seats {
		if len(st.Hand) > 0 {
			n++
		}
	}
	return n
}

// leftOf is the next seat on i's left still holding cards, or -1.
func (s *GameState) leftOf(i int) int {
	n := len(s.Seats)
	for k := 1; k < n; k++ {
		if j := (i + k) % n; len(s.Seats[j].Hand) > 0 {
			return j
		}
	}
	return -1
}

// source is the seat the given one would take a card from: the nearest on
// its left still holding cards.
func (s *GameState) source(id string) int {
	i := s.index(id)
	if i < 0 {
		return -1
	}
	return s.leftOf(i)
}
