package lora

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
	Status  string   `json:"status"` // "active" | "completed"
	Players []string `json:"players"`
	Seed    int64    `json:"seed"`

	Talies   int  `json:"talies"`
	Maturita bool `json:"maturita,omitempty"`
	Pause    bool `json:"pause,omitempty"`

	// FirstLeader leads the first tálie; each later one is led by the next
	// seat. Talie and Game say where the match is: Game indexes sequence,
	// and len(sequence) is the maturita.
	FirstLeader int `json:"firstLeader"`
	Talie       int `json:"talie"`
	Game        int `json:"game"`
	// Attempt counts the maturita attempts that have failed this tálie.
	Attempt int `json:"attempt,omitempty"`
	// Deal counts every deal of the match, for seeding the shuffle.
	Deal int `json:"deal"`

	// Contract is the game being played: sequence[Game], or the one the
	// maturant chose ("" while they choose).
	Contract string              `json:"contract"`
	Phase    string              `json:"phase"`
	Current  string              `json:"current"`
	Leader   string              `json:"leader"`
	Hands    map[string][]string `json:"hands"`

	// Trick games.
	Trick     []tricks.Play  `json:"trick,omitempty"`
	LastTrick []tricks.Play  `json:"lastTrick,omitempty"`
	Tricks    int            `json:"tricks,omitempty"` // completed this deal
	Won       map[string]int `json:"won,omitempty"`    // tricks taken, by player
	// Voids is the suits each player has shown they hold none of: they did
	// not follow, or led a red card when only red was allowed. Public.
	Voids map[string]string `json:"voids,omitempty"`

	// Laying-out games: every card laid this deal, and the quart just
	// finished (kvarty) for the board to show.
	Laid   []string       `json:"laid,omitempty"`
	Quart  []tricks.Play  `json:"quart,omitempty"`
	Turned bool           `json:"turned,omitempty"` // desítky: laid a card this turn
	Tuks   map[string]int `json:"tuks,omitempty"`

	// Points is this deal's penalty so far; Scores the match's.
	Points map[string]int `json:"points,omitempty"`
	Scores map[string]int `json:"scores"`

	// Failed is the maturant who failed their last attempt, ending the
	// match: they finish last whatever their score.
	Failed string `json:"failed,omitempty"`

	Rounds       []DealRecord        `json:"rounds,omitempty"`
	Intermission module.Intermission `json:"intermission,omitempty"`
}

// DealRecord is one finished deal. Public: it names players and counts,
// never a card still hidden.
type DealRecord struct {
	Number   int            `json:"number"` // 1-based, across the match
	Talie    int            `json:"talie"`  // 1-based
	Contract string         `json:"contract"`
	Maturant string         `json:"maturant,omitempty"` // a maturita deal
	Passed   bool           `json:"passed,omitempty"`   // the maturita held
	Deltas   map[string]int `json:"deltas"`
	Totals   map[string]int `json:"totals"`
}

// Phases of a deal.
const (
	phaseChoose = "choose" // the maturant names their game
	phasePlay   = "play"   // a trick game
	phaseQuart  = "quart"  // kvarty: the leader lays a quart
	phaseTens   = "tens"   // desítky: the player on turn lays or says ťuk
)

// Verbs this module accepts. VerbContinue belongs to module.Intermission.
const (
	VerbPlay   = "play_card"
	VerbDone   = "end_turn"
	VerbTuk    = "tuk"
	VerbChoose = "choose_game"
)

// Offer ids.
const (
	OfferPlay   = "play"
	OfferDone   = "done"
	OfferTuk    = "tuk"
	OfferChoose = "choose"
)

// ParamGame is the choose offer's parameter: the game's id.
const ParamGame = "game"

// Error codes. Stable keys, worded by the client's locale bundles.
const (
	ErrNotYourTurn   = "NOT_YOUR_TURN"
	ErrGameNotActive = "GAME_NOT_ACTIVE"
	ErrUnknownAction = "UNKNOWN_ACTION"
	ErrCardNotInHand = "CARD_NOT_IN_HAND"
	ErrMustFollow    = "LORA_MUST_FOLLOW_SUIT"

	ErrPickOneCard   = "LORA_PICK_ONE_CARD"
	ErrNoRedLead     = "LORA_NO_RED_LEAD"
	ErrCannotLay     = "LORA_CANNOT_LAY"
	ErrMustLay       = "LORA_MUST_LAY"
	ErrNothingLaid   = "LORA_NOTHING_LAID"
	ErrNotChoosing   = "LORA_NOT_CHOOSING"
	ErrNoSuchGame    = "LORA_NO_SUCH_GAME"
	ErrPrPoThirdOnly = "LORA_PRPO_THIRD_ATTEMPT"
)

func decode(raw module.State) (*GameState, error) {
	var s GameState
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("lora: decode state: %w", err)
	}
	for _, m := range []*map[string]int{&s.Won, &s.Tuks, &s.Points, &s.Scores} {
		if *m == nil {
			*m = map[string]int{}
		}
	}
	if s.Voids == nil {
		s.Voids = map[string]string{}
	}
	if s.Hands == nil {
		s.Hands = map[string][]string{}
	}
	return &s, nil
}

func encode(s *GameState) (module.State, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("lora: encode state: %w", err)
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

// talieLeader leads every game of the tálie and takes its maturita.
func (s *GameState) talieLeader() string {
	return s.Players[(s.FirstLeader+s.Talie)%len(s.Players)]
}

// dealer is the seat before the leader.
func (s *GameState) dealer() string {
	return s.Players[(s.FirstLeader+s.Talie+len(s.Players)-1)%len(s.Players)]
}

// maturita is whether the deal in progress is the tálie's maturita.
func (s *GameState) maturita() bool { return s.Game >= len(sequence) }

// maturant is who must come through the maturita clean, or "".
func (s *GameState) maturant() string {
	if s.maturita() {
		return s.talieLeader()
	}
	return ""
}

// choosable is whether the maturant may choose game g now: any game but
// první–poslední, which only the third attempt may choose.
func (s *GameState) choosable(g string) bool {
	if g == gamePrPo {
		return s.Attempt == maxAttempts-1
	}
	for _, x := range sequence {
		if x == g {
			return true
		}
	}
	return false
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
	for _, c := range hand {
		if c != card {
			out = append(out, c)
		}
	}
	return out
}

// holder is who holds card, or "".
func (s *GameState) holder(card string) string {
	for _, p := range s.Players {
		if hasCard(s.Hands[p], card) {
			return p
		}
	}
	return ""
}

// sortHand orders a hand by suit, then low to high within it. Display only.
func sortHand(hand []string) {
	sort.SliceStable(hand, func(i, j int) bool {
		a, b := hand[i], hand[j]
		if tricks.Suit(a) != tricks.Suit(b) {
			return indexOf(suits, tricks.Suit(a)) < indexOf(suits, tricks.Suit(b))
		}
		return rankIndex(a) < rankIndex(b)
	})
}

func indexOf(str string, b byte) int {
	for i := 0; i < len(str); i++ {
		if str[i] == b {
			return i
		}
	}
	return -1
}
