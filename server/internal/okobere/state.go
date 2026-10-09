package okobere

import (
	"encoding/json"
	"fmt"

	"zolik/server/internal/module"
)

// Seat is one player: their chips and, in a round, their cards and stake.
type Seat struct {
	PlayerID string   `json:"playerId"`
	Stack    int      `json:"stack"`
	Hand     []string `json:"hand,omitempty"`
	// Bet is the stake on the table this round, already off the stack.
	Bet int `json:"bet,omitempty"`
	// Done is a punter finished for the round: stood, bust, oko, or sat
	// out with nothing to bet.
	Done bool `json:"done,omitempty"`
	// Result is how the round ended for this seat, once it has.
	Result string `json:"result,omitempty"`
	// Delta is what the round paid this seat, net of the stake.
	Delta int `json:"delta,omitempty"`
}

// GameState is the whole match. Opaque to the runtime — only this package
// reads it.
type GameState struct {
	Status string `json:"status"`
	Seats  []Seat `json:"seats"`
	Seed   int64  `json:"seed"`

	StartingStack int  `json:"startingStack"`
	MinBet        int  `json:"minBet"`
	BankSize      int  `json:"bankSize"`
	RoundsPerBank int  `json:"roundsPerBank"`
	BankTurns     int  `json:"bankTurns"` // each seat holds the bank this many times
	OkoPays       int  `json:"okoPays"`   // multiple of the stake an oko collects
	Pause         bool `json:"pause,omitempty"`

	// Round counts every round dealt; Turn counts bank turns begun, and the
	// banker is the seat Turn points at, from FirstBanker.
	Round        int `json:"round"`
	Turn         int `json:"turn"`
	FirstBanker  int `json:"firstBanker"`
	RoundsInBank int `json:"roundsInBank"`
	// Bank is the chips the banker has at stake: what punters may bet
	// against, and where lost stakes go.
	Bank int `json:"bank"`

	Phase   string   `json:"phase"`
	Current string   `json:"current"`
	Deck    []string `json:"deck,omitempty"`

	Rounds       []RoundRecord       `json:"rounds,omitempty"`
	Intermission module.Intermission `json:"intermission,omitempty"`
}

// RoundRecord is one settled round. Public: totals and results, no cards.
type RoundRecord struct {
	Number      int               `json:"number"`
	Banker      string            `json:"banker"`
	BankerTotal int               `json:"bankerTotal"`
	BankerOko   bool              `json:"bankerOko,omitempty"`
	BankBroken  bool              `json:"bankBroken,omitempty"`
	Results     map[string]string `json:"results"`
	Deltas      map[string]int    `json:"deltas"`
	Stacks      map[string]int    `json:"stacks"`
}

// Phases of a round.
const (
	phasePunters = "punters" // each punter in turn: bet, then draw or stand
	phaseBank    = "bank"    // the banker draws
)

// Results, per seat per round.
const (
	resultWon    = "won"
	resultLost   = "lost"
	resultBust   = "bust"
	resultOko    = "oko"
	resultSatOut = "satOut"
	resultBanker = "banker"
)

// Verbs this module accepts. VerbContinue belongs to module.Intermission.
const (
	VerbBet   = "bet"
	VerbCard  = "take_card"
	VerbStand = "stand"
)

// Offer ids, and the bet's one parameter.
const (
	OfferBet    = "bet"
	OfferCard   = "card"
	OfferStand  = "stand"
	ParamAmount = "amount"
)

// Error codes. Stable keys, worded by the client's locale bundles.
const (
	ErrNotYourTurn   = "NOT_YOUR_TURN"
	ErrGameNotActive = "GAME_NOT_ACTIVE"
	ErrUnknownAction = "UNKNOWN_ACTION"

	ErrBetFirst     = "OKO_BET_FIRST"
	ErrAlreadyBet   = "OKO_ALREADY_BET"
	ErrBetAmount    = "OKO_BET_NOT_A_NUMBER"
	ErrBetTooSmall  = "OKO_BET_BELOW_MINIMUM"
	ErrBetTooLarge  = "OKO_BET_ABOVE_STACK"
	ErrBankTooSmall = "OKO_BET_ABOVE_BANK"
	ErrDeckEmpty    = "OKO_DECK_EMPTY"
)

func decode(raw module.State) (*GameState, error) {
	var s GameState
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("okobere: decode state: %w", err)
	}
	return &s, nil
}

func encode(s *GameState) (module.State, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("okobere: encode state: %w", err)
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

func (s *GameState) bankerIndex() int { return (s.FirstBanker + s.Turn) % len(s.Seats) }

func (s *GameState) banker() *Seat { return &s.Seats[s.bankerIndex()] }

// players is every player id, in seat order.
func (s *GameState) players() []string {
	out := make([]string, len(s.Seats))
	for i, st := range s.Seats {
		out[i] = st.PlayerID
	}
	return out
}

// committed is what the bank stands to pay on the stakes already down.
func (s *GameState) committed() int {
	n := 0
	for i, st := range s.Seats {
		if i != s.bankerIndex() && st.Result == "" {
			n += st.Bet
		}
	}
	return n
}

// bankFree is how much more the bank can cover this round.
func (s *GameState) bankFree() int { return max(0, s.Bank-s.committed()) }

// maxBet is the most this seat may stake now: its stack, and what the bank
// has free.
func (s *GameState) maxBet(st *Seat) int { return min(st.Stack, s.bankFree()) }
