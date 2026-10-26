package sietemedio

import (
	"encoding/json"
	"fmt"

	"zolik/server/internal/module"
)

// The phases a round walks through. Stakes wait on every staking seat at
// once — a table puts its money down together; play walks the seats in turn
// from the banker's left; the bank is the banker's own turn, kept a phase of
// its own so the board can turn their card over at the moment the game does.
const (
	phaseStakes = "stakes"
	phasePlay   = "play"
	phaseBank   = "bank"
)

// Verbs this module accepts. VerbContinue belongs to module.Intermission.
const (
	VerbStake = "stake"
	VerbHit   = "hit"
	VerbStand = "stand"
)

// ParamAmount is the size of a stake: chips, so a client renders one control
// for a range the engine computed.
const ParamAmount = "amount"

// Error codes. Stable keys a client words from its locale bundle; the ones
// without a siete y medio flavour are spelled the way the other modules
// already spell them, so a client words them once.
const (
	ErrWrongPlayerCount = "WRONG_PLAYER_COUNT"
	ErrGameNotActive    = "GAME_NOT_ACTIVE"
	ErrNotYourTurn      = "NOT_YOUR_TURN"
	ErrUnknownAction    = "UNKNOWN_ACTION"
	ErrWrongPhase       = "WRONG_PHASE"
	// ErrAlreadyLeft is a second "leave" from a seat already got up, or getting up.
	ErrAlreadyLeft     = "ALREADY_LEFT"
	ErrNotInRound      = "SEAT_NOT_IN_HAND"
	ErrAmountRequired  = "AMOUNT_REQUIRED"
	ErrAmountNotNumber = "AMOUNT_NOT_A_NUMBER"
	ErrNotEnoughChips  = "NOT_ENOUGH_CHIPS"
	ErrStakeTooSmall   = "STAKE_BELOW_MINIMUM"
	ErrAlreadyStaked   = "ALREADY_STAKED"
	// ErrBankCannotCover is la banca responde: the bank answers every stake on
	// the table out of its own stack, and this one would put it past what it
	// holds. The only refusal in this engine that is about another seat's
	// chips.
	ErrBankCannotCover = "BANK_CANNOT_COVER"
	// ErrBankerDoesNotStake is the banker trying to put a stake up. The bank
	// covers the stakes; it does not make one.
	ErrBankerDoesNotStake = "BANKER_DOES_NOT_STAKE"
)

// How one hand finished. Stored rather than recomputed because settlement
// reads the banker's hand and this table's options, and a round table drawn
// three rounds later has neither.
const (
	outcomeSeven      = "seven" // 7½, paid at this table's rate
	outcomeWin        = "win"
	outcomePush       = "push"
	outcomeLose       = "lose"
	outcomeBust       = "bust"
	outcomeUnfinished = ""
)

// Seat is one player, for the whole match. There are no splits in this game,
// so a seat's hand is the seat's own field rather than a list of boxes.
type Seat struct {
	PlayerID string `json:"playerId"`
	// Stack is chips behind. A stake leaves it when it goes up and a payout
	// comes back at settlement, so it is always "what this seat could stake
	// next round" — and, for the banker, what the bank can answer for.
	Stack int `json:"stack"`
	// Stake is this round's stake; zero means this seat is not playing against
	// the bank this round, which is also true of the banker.
	Stake int      `json:"stake,omitempty"`
	Cards []string `json:"cards,omitempty"`
	// Stood is a seat with no decisions left: stood, bust, or on 7½.
	Stood bool `json:"stood,omitempty"`
	// Shown is a hand the whole table may see — a seat that went over turns
	// its cards up, and so does the banker once their turn starts.
	Shown bool `json:"shown,omitempty"`
	// NoRoom is a seat the bank could not answer a minimum stake for this
	// round. It is not waited on and is settled as not playing: a seat shown a
	// stake control whose only legal value does not exist is a dead position.
	NoRoom bool `json:"noRoom,omitempty"`
	// Outcome and Payout are written at settlement: what the hand was worth
	// and what came back to the stack, the stake included.
	Outcome string `json:"outcome,omitempty"`
	Payout  int    `json:"payout,omitempty"`
	// Out is a seat that can no longer cover the table minimum. Chips only
	// arrive by winning, so it is settled once at the top of a round rather
	// than asked every time.
	Out bool `json:"out,omitempty"`
	// Left is a player who got up from the table, their chips frozen in Stack
	// as their result. Leaving is a seat that asked to go with a stake down:
	// it plays the round out — sat out, if its player has gone — and gets up
	// when the round settles.
	Left    bool `json:"left,omitempty"`
	Leaving bool `json:"leaving,omitempty"`
	// Bot is a seat the server plays, recorded at the deal so a table with no
	// person left at it can end instead of dealing on to nobody.
	Bot bool `json:"bot,omitempty"`
}

// inRound reports a seat playing against the bank this round.
func (s *Seat) inRound() bool { return s.Stake > 0 }

// GameState is the whole match. Opaque to the runtime — only this package
// reads it.
type GameState struct {
	Status    string `json:"status"` // "active" | "completed"
	Variation string `json:"variation,omitempty"`
	Phase     string `json:"phase"`

	Seats []Seat `json:"seats"`
	// Banker is the seat index holding the bank. Unlike every other opponent
	// on this server it is state rather than a property of the match: the bank
	// is won and lost while the match runs.
	Banker int `json:"banker"`
	// Current is the seat whose turn it is, and -1 outside the play phases,
	// where the module waits on every staking seat at once rather than on one.
	Current int `json:"current"`

	Deck []string `json:"deck"`

	// The table, resolved at NewMatch from the variation and the lobby's
	// options and then never read from the config again — a match plays to the
	// rules it was created with even if the defaults move under it.
	MinStake      int  `json:"minStake"`
	StartingStack int  `json:"startingStack"`
	RoundLimit    int  `json:"roundLimit"`
	TiesToBank    bool `json:"tiesToBank,omitempty"`
	SevenDoubles  bool `json:"sevenDoubles,omitempty"`
	BankPass      int  `json:"bankPass"`
	FirstCardDown bool `json:"firstCardDown,omitempty"`

	RoundNumber int            `json:"roundNumber"`
	Rounds      []RoundSummary `json:"rounds,omitempty"`

	Pause bool                `json:"pause,omitempty"`
	Break module.Intermission `json:"break,omitempty"`

	Winners []string `json:"winners,omitempty"`
	Seed    int64    `json:"seed"`
}

// RoundSummary is what one settled round leaves behind.
//
// It carries no cards, and that is a rule rather than an economy: it goes onto
// the wire and into a permanent match record. The banker's *total* is the fact
// everybody at the table was watching and nobody can reconstruct afterwards,
// and a total is not a card.
type RoundSummary struct {
	Number int `json:"number"`
	// Banker is who held the bank for this round, and BankTotal what they made
	// of it, in halves.
	Banker    string `json:"banker"`
	BankTotal int    `json:"bankTotal"`
	BankBust  bool   `json:"bankBust,omitempty"`
	BankSeven bool   `json:"bankSeven,omitempty"`
	// Totals is every playing seat's own total, in halves, so a round table can
	// say what each hand was worth without naming a card.
	Totals   map[string]int    `json:"totals,omitempty"`
	Outcomes map[string]string `json:"outcomes,omitempty"`
	Deltas   map[string]int    `json:"deltas,omitempty"`
	Stacks   map[string]int    `json:"stacks"`
	Winners  []string          `json:"winners,omitempty"`
}

func (s *GameState) seatIndex(playerID string) int {
	for i := range s.Seats {
		if s.Seats[i].PlayerID == playerID {
			return i
		}
	}
	return -1
}

func (s *GameState) seat(playerID string) *Seat {
	if i := s.seatIndex(playerID); i >= 0 {
		return &s.Seats[i]
	}
	return nil
}

// banker is the seat holding the bank, or nil before the cut.
func (s *GameState) banker() *Seat {
	if s.Banker < 0 || s.Banker >= len(s.Seats) {
		return nil
	}
	return &s.Seats[s.Banker]
}

// isBanker reports a seat index holding the bank.
func (s *GameState) isBanker(i int) bool { return i == s.Banker }

// order is every seat in turn order — the order an intermission readies
// through, and the order a round table reads down. Seats out of chips are
// included: they are still at the table and still watching.
func order(s *GameState) []string {
	out := make([]string, 0, len(s.Seats))
	for i := range s.Seats {
		// A player who got up is not reading the results, and waiting on them
		// to go on would wait for ever.
		if s.Seats[i].Left {
			continue
		}
		out = append(out, s.Seats[i].PlayerID)
	}
	return out
}

// playOrder is the seats that act this round, in the order they act: the
// players from the banker's left round to the banker's right, and the banker
// last. That is the order a real table plays in, and the reason the bank has
// the advantage it does — it answers every hand knowing what it has to beat.
func playOrder(s *GameState) []int {
	out := make([]int, 0, len(s.Seats))
	n := len(s.Seats)
	for step := 1; step <= n; step++ {
		i := (s.Banker + step) % n
		if i == s.Banker {
			continue
		}
		out = append(out, i)
	}
	return append(out, s.Banker)
}

// liability is what the bank may have to pay out on a stake of one chip: two
// where a 7½ collects double, one otherwise.
//
// It is the multiplier the stake range is built from, so a stake can never be
// accepted that the bank might not be able to answer. A bank that could be
// overdrawn is a settlement with a negative stack in it, which is not a state
// this game has.
func (s *GameState) liability() int {
	if s.SevenDoubles {
		return 2
	}
	return 1
}

// bankExposure is what the bank has already promised this round.
func bankExposure(s *GameState) int {
	out := 0
	for i := range s.Seats {
		if s.Seats[i].inRound() {
			out += s.Seats[i].Stake * s.liability()
		}
	}
	return out
}

// maxStakeFor is the largest stake this seat may put up: what it holds, capped
// by what the bank can still answer for.
func maxStakeFor(s *GameState, i int) int {
	b := s.banker()
	if b == nil || s.isBanker(i) {
		return 0
	}
	room := (b.Stack - bankExposure(s)) / s.liability()
	if room < 0 {
		room = 0
	}
	if seat := &s.Seats[i]; seat.Stack < room {
		return seat.Stack
	}
	return room
}

// canBank reports a seat able to hold the bank: it must be able to answer at
// least one minimum stake, or the table has nothing to play for.
func canBank(s *GameState, i int) bool {
	st := &s.Seats[i]
	if st.Left || st.Leaving {
		return false
	}
	return st.Stack >= s.MinStake*s.liability()
}

func decode(raw module.State) (*GameState, error) {
	var s GameState
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("sietemedio: decode state: %w", err)
	}
	return &s, nil
}

func encode(s *GameState) (module.State, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("sietemedio: encode state: %w", err)
	}
	return raw, nil
}

func errCode(code string) error { return module.Error{Code: code} }
