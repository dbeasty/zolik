// Package blackjack implements casino Blackjack as a game module.
//
// It is the seventh game, and the first whose opponent is not at the table.
// Every other module here is players against each other; this one is players
// against the house, and the house is not a seat — it is a hand the engine
// plays by a rule with no choices in it. That turned out to need nothing new
// from the module protocol, which is the interesting part: the dealer is a
// zone nobody owns, the round is a `module.RoundResult` like any other, and
// "who won" is still the seat that finished with the most chips.
//
// What it did make plain is that the protocol's turn model was never a
// *rotation*. Two of this game's three phases wait on everybody at once —
// betting and insurance are simultaneous decisions at a real table, and there
// is no reason to serialise them here — and `Seat.Active` already says "the
// seats the module is waiting on", plural, because an intermission had needed
// exactly that. So a simultaneous phase is an ordinary state rather than a
// special case.
//
// Rules implemented (the common US shoe game):
//   - One to eight decks, dealt face up, reshuffled when the shoe runs low.
//   - A bet per seat per round, between the table minimum and that seat's
//     whole stack.
//   - Dealer up card and hole card; the dealer peeks for blackjack on an ace
//     or a ten, and the round settles there when it is one.
//   - Hit, stand, double down, split (to a declared number of hands), and
//     late surrender — each of them a house rule this table can turn on, off
//     or set a number for rather than a constant baked into the engine.
//   - Split aces take one card each and stand; twenty-one after a split is
//     twenty-one, never a blackjack.
//   - Insurance at 2:1 when the dealer shows an ace.
//   - The dealer draws to seventeen, hitting or standing on a soft one as the
//     table is set.
//   - Blackjack pays at the table's declared rate; the match is a fixed
//     number of rounds and the most chips wins it.
package blackjack

import (
	"encoding/json"
	"fmt"

	"zolik/server/internal/module"
)

// The phases a round walks through. Betting and insurance wait on every seat
// at once; play walks the seats in order, one hand at a time.
const (
	phaseBets      = "bets"
	phaseInsurance = "insurance"
	phasePlay      = "play"
)

// Verbs this module accepts. VerbContinue belongs to module.Intermission.
const (
	VerbBet       = "bet"
	VerbInsure    = "insure"
	VerbDecline   = "decline_insurance"
	VerbHit       = "hit"
	VerbStand     = "stand"
	VerbDouble    = "double"
	VerbSplit     = "split"
	VerbSurrender = "surrender"
)

// ParamAmount is the size of a bet: chips, not a multiple of anything, so a
// client renders one control for a range the engine computed.
const ParamAmount = "amount"

// Error codes. Stable keys a client renders from its locale bundle; the ones
// without a blackjack flavour are deliberately spelled the way the other
// modules already spell them, so a client words them once.
const (
	ErrWrongPlayerCount = "WRONG_PLAYER_COUNT"
	ErrGameNotActive    = "GAME_NOT_ACTIVE"
	ErrNotYourTurn      = "NOT_YOUR_TURN"
	ErrUnknownAction    = "UNKNOWN_ACTION"
	ErrWrongPhase       = "WRONG_PHASE"
	ErrNotInRound       = "SEAT_NOT_IN_HAND"
	ErrAmountRequired   = "AMOUNT_REQUIRED"
	ErrAmountNotNumber  = "AMOUNT_NOT_A_NUMBER"
	ErrNotEnoughChips   = "NOT_ENOUGH_CHIPS"
	ErrBetTooSmall      = "BET_BELOW_MINIMUM"
	ErrAlreadyBet       = "ALREADY_BET"
	ErrInsuranceClosed  = "INSURANCE_CLOSED"
	ErrCannotDouble     = "CANNOT_DOUBLE"
	ErrCannotSplit      = "CANNOT_SPLIT"
	ErrCannotSurrender  = "CANNOT_SURRENDER"
)

// How one hand finished. Stored rather than recomputed because settlement
// reads the dealer's hand and the shoe's history, and a round table drawn
// three deals later has neither.
const (
	outcomeBlackjack  = "blackjack"
	outcomeWin        = "win"
	outcomePush       = "push"
	outcomeLose       = "lose"
	outcomeBust       = "bust"
	outcomeSurrender  = "surrender"
	outcomeUnfinished = ""
)

// Hand is one box a seat is playing this round. A seat has one, or several
// once it has split.
type Hand struct {
	ID    string   `json:"id"`
	Cards []string `json:"cards"`
	// Bet is this hand's own stake — already taken out of the stack, and
	// doubled in place when the hand is doubled down.
	Bet     int  `json:"bet"`
	Doubled bool `json:"doubled,omitempty"`
	// Done is a hand with no decisions left: stood, doubled, bust,
	// surrendered, or twenty-one.
	Done        bool `json:"done,omitempty"`
	Surrendered bool `json:"surrendered,omitempty"`
	// Split marks a hand that came out of a split, which is what stops
	// twenty-one on it being paid as a blackjack. SplitAces additionally
	// stops it being hit at all.
	Split     bool `json:"split,omitempty"`
	SplitAces bool `json:"splitAces,omitempty"`
	// Outcome and Payout are written at settlement: what the hand was worth
	// and what came back to the stack, the stake included.
	Outcome string `json:"outcome,omitempty"`
	Payout  int    `json:"payout,omitempty"`
}

// Seat is one player, for the whole match.
type Seat struct {
	PlayerID string `json:"playerId"`
	// Stack is chips behind. A stake leaves it when the bet is placed and a
	// payout comes back into it at settlement, so it is always "what this
	// seat could bet next round".
	Stack int `json:"stack"`
	// Bet is this round's opening stake; zero means the seat is not in this
	// round. Kept alongside the hands because a split hand's own bet is a
	// copy of it, and insurance is priced off the opening stake.
	Bet   int    `json:"bet,omitempty"`
	Hands []Hand `json:"hands,omitempty"`
	// Insurance is what this seat put up against a dealer blackjack, and
	// InsuranceAnswered that it has made the decision either way. Both are
	// cleared every round.
	Insurance         int  `json:"insurance,omitempty"`
	InsuranceAnswered bool `json:"insuranceAnswered,omitempty"`
	// Staked is everything this seat has put up during the round — stakes,
	// doubles, split stakes, insurance. Accumulated rather than derived
	// because settlement pays into the stack hand by hand, and "what did this
	// round actually cost me" is otherwise unrecoverable once it has.
	Staked int `json:"staked,omitempty"`
	// Out is a seat that can no longer meet the table minimum. There is no
	// way back — chips only arrive by winning a bet — so it is settled once,
	// at the top of a round, rather than asked every time.
	Out bool `json:"out,omitempty"`
}

// inRound reports a seat playing this round.
func (s *Seat) inRound() bool { return s.Bet > 0 }

// GameState is the whole match. Opaque to the runtime — only this package
// reads it.
type GameState struct {
	Status    string `json:"status"` // "active" | "completed"
	Variation string `json:"variation,omitempty"`
	Phase     string `json:"phase"`

	Seats []Seat `json:"seats"`
	// Current is the seat index playing a hand, and CurrentHand which of its
	// hands. Both are -1 outside the play phase, where the module waits on
	// every seat at once rather than on one.
	Current     int `json:"current"`
	CurrentHand int `json:"currentHand"`

	Shoe []string `json:"shoe"`
	// Dealer holds the up card first and the hole card second. The hole card
	// is filtered out of every view until HoleShown, which is the only hidden
	// information in this game — every player card is dealt face up.
	Dealer    []string `json:"dealer,omitempty"`
	HoleShown bool     `json:"holeShown,omitempty"`
	// Peeked is the dealer having looked for blackjack, which is what makes
	// surrender *late* and stops a player doubling into a hand that was
	// already over.
	Peeked bool `json:"peeked,omitempty"`

	// The table, resolved at NewMatch from the variation and the lobby's
	// options, and then never read from the config again — a match plays to
	// the rules it was created with even if the defaults move under it.
	Decks            int  `json:"decks"`
	MinBet           int  `json:"minBet"`
	StartingStack    int  `json:"startingStack"`
	RoundLimit       int  `json:"roundLimit"`
	DealerHitsSoft17 bool `json:"dealerHitsSoft17,omitempty"`
	// BlackjackPayout is the win as a percentage of the stake: 150 is 3:2.
	// A percentage rather than a ratio pair because it is one number on the
	// wire, one number in an option, and the arithmetic is integer either way.
	BlackjackPayout  int  `json:"blackjackPayout"`
	AllowSurrender   bool `json:"allowSurrender,omitempty"`
	DoubleAfterSplit bool `json:"doubleAfterSplit,omitempty"`
	MaxSplits        int  `json:"maxSplits"`
	AllowInsurance   bool `json:"allowInsurance,omitempty"`

	RoundNumber int `json:"roundNumber"`

	Rounds []RoundSummary `json:"rounds,omitempty"`

	Pause bool                `json:"pause,omitempty"`
	Break module.Intermission `json:"break,omitempty"`

	Winners []string `json:"winners,omitempty"`
	Seed    int64    `json:"seed"`
}

// RoundSummary is what one settled round leaves behind.
//
// It carries no cards, and that is a rule rather than an economy: it is
// written onto the wire and into a permanent match record, and holdem's round
// log was held to the same line for the same reason. The dealer's *total* is
// the fact a player argues about afterwards, and a total is not a card.
type RoundSummary struct {
	Number int `json:"number"`
	// DealerTotal is the dealer's final total, or zero for a round that
	// settled before the dealer drew.
	DealerTotal     int  `json:"dealerTotal"`
	DealerBust      bool `json:"dealerBust,omitempty"`
	DealerBlackjack bool `json:"dealerBlackjack,omitempty"`
	// Outcomes is one entry per hand a seat played, in the order it played
	// them, so a split shows as the two results it really was.
	Outcomes map[string][]string `json:"outcomes,omitempty"`
	Deltas   map[string]int      `json:"deltas,omitempty"`
	Stacks   map[string]int      `json:"stacks"`
	Winners  []string            `json:"winners,omitempty"`
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

// order is every seat in turn order — the order an intermission readies
// through, and the order a round table reads down. Seats that have run out of
// chips are included: they are still at the table and still watching.
func order(s *GameState) []string {
	out := make([]string, 0, len(s.Seats))
	for i := range s.Seats {
		out = append(out, s.Seats[i].PlayerID)
	}
	return out
}

// upCard is the dealer's face-up card, or "" before the deal.
func (s *GameState) upCard() string {
	if len(s.Dealer) == 0 {
		return ""
	}
	return s.Dealer[0]
}

// shownDealer is the dealer's cards as the table can see them: everything
// once the hole card is turned, the up card alone before that.
func (s *GameState) shownDealer() []string {
	if s.HoleShown || len(s.Dealer) < 2 {
		return append([]string(nil), s.Dealer...)
	}
	return []string{s.Dealer[0]}
}

func decode(raw module.State) (*GameState, error) {
	var s GameState
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("blackjack: decode state: %w", err)
	}
	return &s, nil
}

func encode(s *GameState) (module.State, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("blackjack: encode state: %w", err)
	}
	return raw, nil
}

func errCode(code string) error { return module.Error{Code: code} }
