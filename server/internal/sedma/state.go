package sedma

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"zolik/server/internal/module"
	"zolik/server/internal/tricks"
)

// GameState is the whole match. Opaque to the runtime — only this package
// reads it.
type GameState struct {
	Status  string   `json:"status"` // "active" | "completed"
	Players []string `json:"players"`
	Seed    int64    `json:"seed"`

	Target         int  `json:"target"`
	ShowCardPoints bool `json:"showCardPoints,omitempty"`
	Pause          bool `json:"pause,omitempty"`

	// Deal is the hand in progress, counted from zero; the deal passes one
	// seat to the left each hand, from FirstDealer.
	Deal        int `json:"deal"`
	FirstDealer int `json:"firstDealer"`

	Phase   string              `json:"phase"`
	Current string              `json:"current"`
	Leader  string              `json:"leader"`
	Hands   map[string][]string `json:"hands"`
	Stock   []string            `json:"stock,omitempty"`

	// Trick is every card played to the trick in progress, round after
	// round while the leader continues it.
	Trick     []tricks.Play   `json:"trick,omitempty"`
	LastTrick []tricks.Play   `json:"lastTrick,omitempty"`
	History   [][]tricks.Play `json:"history,omitempty"`

	// Points and Cards are what each side has taken this hand, by side id.
	Points map[string]int `json:"points,omitempty"`
	Cards  map[string]int `json:"cards,omitempty"`

	Scores       map[string]int      `json:"scores"` // stakes, by player
	Rounds       []HandRecord        `json:"rounds,omitempty"`
	Intermission module.Intermission `json:"intermission,omitempty"`
}

// HandRecord is one finished hand. Public: it names players and counts,
// never a card.
type HandRecord struct {
	Number  int            `json:"number"` // 1-based
	Winners []string       `json:"winners,omitempty"`
	Stakes  int            `json:"stakes"`
	Kind    string         `json:"kind"`   // won, allPoints, allCards, tie, none
	Points  map[string]int `json:"points"` // each player's side's points
	Totals  map[string]int `json:"totals"`
}

// Phases of a hand.
const (
	phasePlay   = "play"   // someone owes the trick a card
	phaseDecide = "decide" // a round is complete: the leader continues or stops
)

// How a hand was won.
const (
	kindWon       = "won"
	kindAllPoints = "allPoints"
	kindAllCards  = "allCards"
	kindTie       = "tie"  // three players, two level on most points
	kindNone      = "none" // three players, thirty each
)

// Verbs this module accepts. VerbContinue belongs to module.Intermission.
const (
	VerbPlay = "play_card"
	VerbStop = "end_trick"
)

// Offer ids.
const (
	OfferPlay = "play"
	OfferStop = "stop"
)

// Error codes. Stable keys, worded by the client's locale bundles.
const (
	ErrNotYourTurn   = "NOT_YOUR_TURN"
	ErrGameNotActive = "GAME_NOT_ACTIVE"
	ErrUnknownAction = "UNKNOWN_ACTION"
	ErrCardNotInHand = "CARD_NOT_IN_HAND"

	ErrPickOneCard = "SEDMA_PICK_ONE_CARD"
	ErrMustCapture = "SEDMA_CONTINUE_WITH_RANK_OR_SEVEN"
	ErrNotDeciding = "SEDMA_NOTHING_TO_STOP"
)

func decode(raw module.State) (*GameState, error) {
	var s GameState
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("sedma: decode state: %w", err)
	}
	for _, m := range []*map[string]int{&s.Points, &s.Cards, &s.Scores} {
		if *m == nil {
			*m = map[string]int{}
		}
	}
	if s.Hands == nil {
		s.Hands = map[string][]string{}
	}
	return &s, nil
}

func encode(s *GameState) (module.State, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("sedma: encode state: %w", err)
	}
	return raw, nil
}

func errCode(code string) error { return module.Error{Code: code} }

func (s *GameState) seat(id string) int {
	for i, p := range s.Players {
		if p == id {
			return i
		}
	}
	return -1
}

func (s *GameState) next(id string) string {
	return s.Players[(s.seat(id)+1)%len(s.Players)]
}

// teams is whether the table plays in partnerships: four seats, partners
// opposite.
func (s *GameState) teams() bool { return len(s.Players) == 4 }

// side is the partnership a player plays for: seat mod 2 at four, else the
// seat itself.
func (s *GameState) side(id string) string { return sideOf(s.seat(id), len(s.Players)) }

func sideOf(seat, n int) string {
	if n == 4 {
		return strconv.Itoa(seat % 2)
	}
	return strconv.Itoa(seat)
}

// sides is every side at the table, in seat order.
func (s *GameState) sides() []string {
	if s.teams() {
		return []string{"0", "1"}
	}
	out := make([]string, len(s.Players))
	for i := range s.Players {
		out[i] = strconv.Itoa(i)
	}
	return out
}

func (s *GameState) dealer() string { return s.Players[(s.FirstDealer+s.Deal)%len(s.Players)] }

// led is the rank of the card that opened the trick in progress.
func (s *GameState) led() byte {
	if len(s.Trick) == 0 {
		return 0
	}
	return tricks.Rank(s.Trick[0].Card)
}

// winning is who is taking the trick in progress: the last player to have
// played the led rank or a seven, the leader until somebody does.
func winning(trick []tricks.Play) int {
	led := tricks.Rank(trick[0].Card)
	w := trick[0].Seat
	for _, pl := range trick[1:] {
		if captures(pl.Card, led) {
			w = pl.Seat
		}
	}
	return w
}

// continuers are the cards the leader may continue the trick with.
func (s *GameState) continuers() []string {
	var out []string
	for _, c := range s.Hands[s.Leader] {
		if captures(c, s.led()) {
			out = append(out, c)
		}
	}
	return out
}

// trickPoints is the points on the table in the trick in progress.
func (s *GameState) trickPoints() int {
	n := 0
	for _, pl := range s.Trick {
		n += cardPoints(pl.Card)
	}
	return n
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

// sortHand orders a hand by rank, sevens first, then aces and tens: the way
// a Sedma player sorts, by what captures. Display only.
func sortHand(hand []string) {
	rankOrder := map[byte]int{'7': 0, 'A': 1, 'T': 2, 'K': 3, 'Q': 4, 'J': 5, '9': 6, '8': 7}
	suitOrder := map[byte]int{'H': 0, 'C': 1, 'S': 2, 'D': 3}
	sort.SliceStable(hand, func(i, j int) bool {
		a, b := hand[i], hand[j]
		if tricks.Rank(a) != tricks.Rank(b) {
			return rankOrder[tricks.Rank(a)] < rankOrder[tricks.Rank(b)]
		}
		return suitOrder[tricks.Suit(a)] < suitOrder[tricks.Suit(b)]
	})
}
