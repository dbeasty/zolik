package snaps

import (
	"encoding/json"
	"fmt"
	"sort"

	"zolik/server/internal/module"
	"zolik/server/internal/tricks"
)

// GameState is the whole match. Opaque to the runtime — only this package
// reads it.
type GameState struct {
	Status    string   `json:"status"` // "active" | "completed"
	Variation string   `json:"variation,omitempty"`
	Players   []string `json:"players"` // two seats
	Seed      int64    `json:"seed"`

	// The table's options, resolved once at NewMatch.
	Target         int  `json:"target"`
	ShowCardPoints bool `json:"showCardPoints,omitempty"`
	Pause          bool `json:"pause,omitempty"`

	// Deal is the deal in progress, counted from zero; the deal alternates
	// from FirstDealer.
	Deal        int `json:"deal"`
	FirstDealer int `json:"firstDealer"`

	Current string              `json:"current"`
	Hands   map[string][]string `json:"hands"`
	// Talon is the stock, top card first. While it is open its last card is
	// the turned-up trump, half under the rest; closing turns it face down.
	Talon []string `json:"talon,omitempty"`
	Trump string   `json:"trump"` // the trump suit

	// ClosedBy is whoever closed the talon, and the other player's tricks
	// and points at that moment, which is what a closer is paid on.
	ClosedBy        string `json:"closedBy,omitempty"`
	ClosedOppTricks int    `json:"closedOppTricks,omitempty"`
	ClosedOppPoints int    `json:"closedOppPoints,omitempty"`
	// StrictFrom is the trick from which the strict rules applied (the
	// talon closed or used up), or -1 while they have not. Public: it is
	// what a bot reads voids from.
	StrictFrom int `json:"strictFrom"`

	Trick     []tricks.Play   `json:"trick,omitempty"`
	LastTrick []tricks.Play   `json:"lastTrick,omitempty"`
	History   [][]tricks.Play `json:"history,omitempty"`
	TricksWon map[string]int  `json:"tricksWon,omitempty"`
	Points    map[string]int  `json:"points,omitempty"` // card points taken
	// Marriages are the suits each player has announced.
	Marriages map[string][]string `json:"marriages,omitempty"`
	// Known are cards the table has seen go into a hand and not come out
	// yet: the turned-up trump taken by an exchange or as the last card of
	// the talon, and the other half of an announced marriage.
	Known map[string][]string `json:"known,omitempty"`

	Scores       map[string]int      `json:"scores"` // game points
	Rounds       []DealRecord        `json:"rounds,omitempty"`
	Intermission module.Intermission `json:"intermission,omitempty"`
}

// DealRecord is one finished deal. Public: it names players and reasons,
// never a card.
type DealRecord struct {
	Number int    `json:"number"` // 1-based
	Winner string `json:"winner,omitempty"`
	// GamePoints is what the winner scored; 0 for a drawn deal.
	GamePoints int            `json:"gamePoints"`
	Reason     string         `json:"reason"`
	ClosedBy   string         `json:"closedBy,omitempty"`
	Points     map[string]int `json:"points"` // each side's count at the end
	Totals     map[string]int `json:"totals"`
}

// Why a deal ended.
const (
	reasonOut          = "out"          // a player reached 66
	reasonCloserFailed = "closerFailed" // the closer ran out of cards short of 66
	reasonLastTrick    = "lastTrick"    // Šnaps: nobody reached 66; the last trick wins
	reasonHigher       = "higher"       // Šedesát šest: nobody reached 66; the higher total wins
	reasonDraw         = "draw"         // Šedesát šest: 65 each
)

// Verbs this module accepts. VerbContinue belongs to module.Intermission.
const (
	VerbPlay     = "play_card"
	VerbExchange = "exchange_trump"
	VerbClose    = "close_talon"
)

// Offer ids.
const (
	OfferPlay     = "play"
	OfferExchange = "exchange"
	OfferClose    = "close"
)

// Error codes. Stable keys, worded by the client's locale bundles.
const (
	ErrNotYourTurn   = "NOT_YOUR_TURN"
	ErrGameNotActive = "GAME_NOT_ACTIVE"
	ErrUnknownAction = "UNKNOWN_ACTION"
	ErrCardNotInHand = "CARD_NOT_IN_HAND"

	ErrPickOneCard     = "SNAPS_PICK_ONE_CARD"
	ErrMustFollow      = "SNAPS_MUST_FOLLOW_SUIT"
	ErrMustBeat        = "SNAPS_MUST_BEAT"
	ErrMustTrump       = "SNAPS_MUST_TRUMP"
	ErrNotOnLead       = "SNAPS_NOT_ON_LEAD"
	ErrTalonClosed     = "SNAPS_TALON_CLOSED"
	ErrNoExchangeCard  = "SNAPS_NO_EXCHANGE_CARD"
	ErrExchangeNoTrick = "SNAPS_EXCHANGE_NEEDS_A_TRICK"
)

func decode(raw module.State) (*GameState, error) {
	var s GameState
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("snaps: decode state: %w", err)
	}
	// Empty maps are left out of the JSON; give them back.
	for _, m := range []*map[string]int{&s.TricksWon, &s.Points, &s.Scores} {
		if *m == nil {
			*m = map[string]int{}
		}
	}
	for _, m := range []*map[string][]string{&s.Hands, &s.Marriages, &s.Known} {
		if *m == nil {
			*m = map[string][]string{}
		}
	}
	return &s, nil
}

func encode(s *GameState) (module.State, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("snaps: encode state: %w", err)
	}
	return raw, nil
}

func errCode(code string) error { return module.Error{Code: code} }

func (s *GameState) rules() ruleset { return rulesFor(s.Variation) }

// seat is a player's index at the table.
func (s *GameState) seat(id string) int {
	for i, p := range s.Players {
		if p == id {
			return i
		}
	}
	return -1
}

func (s *GameState) other(id string) string {
	if s.Players[0] == id {
		return s.Players[1]
	}
	return s.Players[0]
}

// dealer deals this deal; the other player leads to its first trick.
func (s *GameState) dealer() string { return s.Players[(s.FirstDealer+s.Deal)%2] }

// open is whether the talon still has cards to draw and is not closed.
func (s *GameState) open() bool { return s.ClosedBy == "" && len(s.Talon) > 0 }

// strict is whether the strict rules of play apply.
func (s *GameState) strict() bool { return !s.open() }

// turnedUp is the face-up trump card under the talon, or "".
func (s *GameState) turnedUp() string {
	if !s.open() {
		return ""
	}
	return s.Talon[len(s.Talon)-1]
}

// total is a player's count toward 66: their card points, and their
// marriages once they have taken a trick.
func (s *GameState) total(p string) int {
	n := s.Points[p]
	if s.TricksWon[p] > 0 {
		for _, suit := range s.Marriages[p] {
			n += s.marriageValue(suit)
		}
	}
	return n
}

func (s *GameState) marriageValue(suit string) int {
	if suit == s.Trump {
		return marriageTrump
	}
	return marriagePlain
}

// marriagesAllowed is whether a marriage may be announced now.
func (s *GameState) marriagesAllowed() bool {
	return !s.rules().strictMarriages || s.open()
}

// legal is the cards p may play now.
func (s *GameState) legal(p string) []string {
	hand := s.Hands[p]
	if !s.strict() || len(s.Trick) == 0 {
		return append([]string(nil), hand...)
	}
	return tricks.Legal(hand, tricks.Trick{Plays: s.Trick}, s.Trump[0], order, tricks.FollowBeatTrump)
}

func hasCard(hand []string, card string) bool {
	for _, c := range hand {
		if c == card {
			return true
		}
	}
	return false
}

func removeCard(hand []string, card string) []string {
	out := make([]string, 0, len(hand))
	done := false
	for _, c := range hand {
		if c == card && !done {
			done = true
			continue
		}
		out = append(out, c)
	}
	return out
}

// partner is the other card of a marriage: the svršek for a král and the
// other way round, or "" for any other card.
func partner(card string) string {
	switch tricks.Rank(card) {
	case 'K':
		return "Q" + string(tricks.Suit(card))
	case 'Q':
		return "K" + string(tricks.Suit(card))
	}
	return ""
}

// sortHand orders a hand by suit, then high to low. Display only.
func sortHand(hand []string) {
	suitOrder := map[byte]int{'H': 0, 'C': 1, 'S': 2, 'D': 3}
	sort.SliceStable(hand, func(i, j int) bool {
		a, b := hand[i], hand[j]
		if tricks.Suit(a) != tricks.Suit(b) {
			return suitOrder[tricks.Suit(a)] < suitOrder[tricks.Suit(b)]
		}
		return order.Beats(tricks.Rank(a), tricks.Rank(b))
	})
}
