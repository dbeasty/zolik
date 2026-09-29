package marias

import (
	"encoding/json"
	"fmt"
	"sort"

	"zolik/server/internal/module"
	"zolik/server/internal/tricks"
)

// New returns the module. Stateless: every method takes the state it works on.
func New() *Module { return &Module{} }

// GameState is the whole match. Opaque to the runtime — only this package
// reads it.
type GameState struct {
	Status  string   `json:"status"`  // "active" | "completed"
	Players []string `json:"players"` // seats, clockwise
	Seed    int64    `json:"seed"`

	// The table's options, resolved once at NewMatch so a match plays to the
	// rules it was created with.
	Deals          int    `json:"deals"`
	Tariff         Tariff `json:"tariff"`
	RedDoubles     bool   `json:"redDoubles,omitempty"`
	FlekLimit      int    `json:"flekLimit,omitempty"`
	ZLidu          bool   `json:"zLidu,omitempty"`
	ShowCardPoints bool   `json:"showCardPoints,omitempty"`
	Pause          bool   `json:"pause,omitempty"`

	// Deal is the deal in progress, counted from zero.
	Deal int `json:"deal"`
	// FirstChooser is the seat that chose trumps in the first deal; the role
	// moves one seat clockwise every deal.
	FirstChooser int `json:"firstChooser"`

	Phase   string `json:"phase"`
	Current string `json:"current"`

	Hands map[string][]string `json:"hands"`
	// Unseen are the chooser's five cards they may not look at until trumps
	// are named — and one of which they may name trumps from (z lidu).
	Unseen []string `json:"unseen,omitempty"`
	// TrumpCard is the card trumps were named by. Private to the chooser
	// until the contract is agreed.
	TrumpCard string   `json:"trumpCard,omitempty"`
	Talon     []string `json:"talon,omitempty"`

	Declarer string `json:"declarer,omitempty"`
	Game     string `json:"game,omitempty"`
	Sedma    bool   `json:"sedma,omitempty"`
	// ProtiSedma and ProtiSto are the defender who announced each, if any.
	ProtiSedma string `json:"protiSedma,omitempty"`
	ProtiSto   string `json:"protiSto,omitempty"`
	// Fleks counts the doublings on each part of the contract.
	Fleks map[string]int `json:"fleks,omitempty"`
	// Quiet counts passes in a row in the doubling round; a full circle of
	// them closes it.
	Quiet int `json:"quiet,omitempty"`

	Trick     []tricks.Play       `json:"trick,omitempty"`
	LastTrick []tricks.Play       `json:"lastTrick,omitempty"`
	TricksWon map[string]int      `json:"tricksWon,omitempty"`
	Points    map[string]int      `json:"points,omitempty"`    // card points, last trick included
	Marriages map[string][]string `json:"marriages,omitempty"` // suits announced, per player

	Scores       map[string]int      `json:"scores"`
	Rounds       []DealRecord        `json:"rounds,omitempty"`
	Intermission module.Intermission `json:"intermission,omitempty"`
}

// DealRecord is one settled deal: what was played and what it paid. Public —
// it names games, suits and players, never a card.
type DealRecord struct {
	Number   int            `json:"number"` // 1-based
	Declarer string         `json:"declarer"`
	Game     string         `json:"game"`
	Trump    string         `json:"trump,omitempty"`
	Parts    []PartResult   `json:"parts"`
	Deltas   map[string]int `json:"deltas"`
	Totals   map[string]int `json:"totals"`
}

// PartResult is one part of a contract, settled.
type PartResult struct {
	Part string `json:"part"`
	// ForDeclarer is whether the declarer's side won it.
	ForDeclarer bool `json:"forDeclarer"`
	// Units is what each defender paid or was paid for it.
	Units int `json:"units"`
}

// Phases of a deal, in the order they happen.
const (
	phaseTrump    = "trump"    // the chooser names trumps from their first seven
	phaseAnnounce = "announce" // the declarer names the game
	phaseTalon    = "talon"    // the declarer discards two
	phaseAnswer   = "answer"   // the others say dobrá, or take over
	phaseFlek     = "flek"     // the doubling round
	phasePlay     = "play"
)

// Games, lowest first.
const (
	gameHra   = "hra"
	gameSto   = "sto"
	gameBetl  = "betl"
	gameDurch = "durch"
)

// The parts a contract settles separately.
const (
	partGame       = "game"
	partSedma      = "sedma"
	partProtiSedma = "protiSedma"
	partProtiSto   = "protiSto"
	partQuietSeven = "quietSeven"
)

// Verbs this module accepts. VerbContinue belongs to module.Intermission.
const (
	VerbChooseTrump = "choose_trump"
	VerbAnnounce    = "announce"
	VerbDiscard     = "discard"
	VerbGood        = "good"
	VerbTakeOver    = "take_over"
	VerbFlek        = "flek"
	VerbProti       = "announce_proti"
	VerbPass        = "pass"
	VerbPlay        = "play_card"
)

// Offer ids. The ones that share a verb are told apart by id: the engine
// reads which game, part or counter-announcement from the offer pressed.
const (
	OfferTrump      = "trump"
	OfferZLidu      = "trumpZLidu"
	OfferHra        = "announce.hra"
	OfferHraSedma   = "announce.hraSedma"
	OfferSto        = "announce.sto"
	OfferStoSedma   = "announce.stoSedma"
	OfferBetl       = "announce.betl"
	OfferDurch      = "announce.durch"
	OfferTalon      = "talon"
	OfferGood       = "good"
	OfferBadBetl    = "bad.betl"
	OfferBadDurch   = "bad.durch"
	OfferFlekGame   = "flek.game"
	OfferFlekSedma  = "flek.sedma"
	OfferFlekPSedma = "flek.protiSedma"
	OfferFlekPSto   = "flek.protiSto"
	OfferProtiSedma = "proti.sedma"
	OfferProtiSto   = "proti.sto"
	OfferPass       = "pass"
	OfferPlay       = "play"
)

// Error codes. Stable keys, worded by the client's locale bundles.
const (
	ErrNotYourTurn   = "NOT_YOUR_TURN"
	ErrGameNotActive = "GAME_NOT_ACTIVE"
	ErrUnknownAction = "UNKNOWN_ACTION"
	ErrCardNotInHand = "CARD_NOT_IN_HAND"

	ErrNotNow         = "MARIAS_NOT_NOW"
	ErrNoZLidu        = "MARIAS_NO_Z_LIDU"
	ErrPickOneCard    = "MARIAS_PICK_ONE_CARD"
	ErrNoSeven        = "MARIAS_NO_TRUMP_SEVEN"
	ErrDiscardTwo     = "MARIAS_DISCARD_TWO"
	ErrSharpInTalon   = "MARIAS_SHARP_IN_TALON"
	ErrSevenInTalon   = "MARIAS_SEVEN_IN_TALON"
	ErrNotHigher      = "MARIAS_NOT_A_HIGHER_GAME"
	ErrNotYoursToFlek = "MARIAS_NOT_YOURS_TO_DOUBLE"
	ErrFlekLimit      = "MARIAS_DOUBLING_LIMIT"
	ErrNoSuchPart     = "MARIAS_NO_SUCH_PART"
	ErrProtiNotNow    = "MARIAS_PROTI_NOT_ALLOWED"
	ErrMustFollow     = "MARIAS_MUST_FOLLOW_SUIT"
	ErrMustBeat       = "MARIAS_MUST_BEAT"
	ErrMustTrump      = "MARIAS_MUST_TRUMP"
	ErrMustOvertrump  = "MARIAS_MUST_OVERTRUMP"
	ErrKeepSeven      = "MARIAS_KEEP_THE_SEVEN"
)

func decode(raw module.State) (*GameState, error) {
	var s GameState
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("marias: decode state: %w", err)
	}
	// Empty maps are omitted on the wire and come back nil; writing to one
	// would panic.
	if s.Fleks == nil {
		s.Fleks = map[string]int{}
	}
	if s.TricksWon == nil {
		s.TricksWon = map[string]int{}
	}
	if s.Points == nil {
		s.Points = map[string]int{}
	}
	if s.Marriages == nil {
		s.Marriages = map[string][]string{}
	}
	if s.Scores == nil {
		s.Scores = map[string]int{}
	}
	return &s, nil
}

func encode(s *GameState) (module.State, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("marias: encode state: %w", err)
	}
	return raw, nil
}

func errCode(code string) error { return module.Error{Code: code} }

// seat is a player's index at the table.
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

// chooser is the seat naming trumps this deal.
func (s *GameState) chooser() string {
	return s.Players[(s.FirstChooser+s.Deal)%len(s.Players)]
}

// trumpGame is whether the game has trumps at all.
func (s *GameState) trumpGame() bool { return s.Game == gameHra || s.Game == gameSto }

// trump is the trump suit in force, or tricks.NoTrump.
func (s *GameState) trump() byte {
	if !s.trumpGame() || s.TrumpCard == "" {
		return tricks.NoTrump
	}
	return tricks.Suit(s.TrumpCard)
}

// order is the rank order in force.
func (s *GameState) order() tricks.Order {
	if s.Game == gameBetl || s.Game == gameDurch {
		return orderPlain
	}
	return orderTrump
}

// trumpSeven is the seven of trumps, or "" in a game without them.
func (s *GameState) trumpSeven() string {
	if t := s.trump(); t != tricks.NoTrump {
		return "7" + string(t)
	}
	return ""
}

// sevenHolder is the player who announced a seven, bound to keep it for the
// last trick; "" if nobody did.
func (s *GameState) sevenHolder() string {
	switch {
	case s.Sedma:
		return s.Declarer
	case s.ProtiSedma != "":
		return s.ProtiSedma
	}
	return ""
}

func (s *GameState) defender(id string) bool { return id != s.Declarer && s.seat(id) >= 0 }

// gameRank orders games for a take-over: betl beats any trump game, durch
// beats everything.
func gameRank(g string) int {
	switch g {
	case gameBetl:
		return 1
	case gameDurch:
		return 2
	}
	return 0
}

// sortHand orders a hand by suit, then high to low — the way a player holds
// Mariáš cards. Display only; nothing depends on it.
func sortHand(hand []string) {
	suitOrder := map[byte]int{'H': 0, 'C': 1, 'S': 2, 'D': 3}
	sort.SliceStable(hand, func(i, j int) bool {
		a, b := hand[i], hand[j]
		if tricks.Suit(a) != tricks.Suit(b) {
			return suitOrder[tricks.Suit(a)] < suitOrder[tricks.Suit(b)]
		}
		return orderTrump.Beats(tricks.Rank(a), tricks.Rank(b))
	})
}

func hasCard(hand []string, card string) bool {
	for _, c := range hand {
		if c == card {
			return true
		}
	}
	return false
}

func removeCards(hand []string, cards ...string) []string {
	out := make([]string, 0, len(hand))
	gone := map[string]bool{}
	for _, c := range cards {
		gone[c] = true
	}
	for _, c := range hand {
		if gone[c] {
			delete(gone, c)
			continue
		}
		out = append(out, c)
	}
	return out
}

func cardPoints(card string) int {
	switch tricks.Rank(card) {
	case 'A', 'T':
		return pointsSharp
	}
	return 0
}
