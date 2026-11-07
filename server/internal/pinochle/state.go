package pinochle

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
	Status    string   `json:"status"` // "active" | "completed"
	Players   []string `json:"players"`
	Seed      int64    `json:"seed"`
	Variation string   `json:"variation"`

	Scoring   int  `json:"scoring"`
	MinBid    int  `json:"minBid,omitempty"`
	Target    int  `json:"target"`
	PassCount int  `json:"passCount,omitempty"`
	MeldNeeds int  `json:"meldNeeds,omitempty"`
	AllPass   int  `json:"allPass,omitempty"`
	Pause     bool `json:"pause,omitempty"`

	// Deal counts hands from zero; the deal passes to the left each hand.
	Deal        int `json:"deal"`
	FirstDealer int `json:"firstDealer"`

	Phase   string              `json:"phase"`
	Current string              `json:"current"`
	Hands   map[string][]string `json:"hands"`

	// The auction.
	Bid     int             `json:"bid,omitempty"`
	Bidder  string          `json:"bidder,omitempty"`
	Passed  map[string]bool `json:"passed,omitempty"`
	LastBid map[string]int  `json:"lastBid,omitempty"` // each seat's last call; -1 a pass
	Stuck   bool            `json:"stuck,omitempty"`   // the dealer took it at the minimum

	Trump string `json:"trump,omitempty"`

	// Cutthroat's widow: face up for everyone from the end of the auction
	// until the bidder has buried. Buried is the bidder's alone.
	Widow  []string `json:"widow,omitempty"`
	Buried []string `json:"buried,omitempty"`

	// Partner pass: what each giver handed over, known to the two of them.
	Given map[string][]string `json:"given,omitempty"`

	// Melds, by player, laid at the start of play (three and four players).
	Melds map[string]Meld `json:"melds,omitempty"`

	// Two-handed: the stock, the turned-up trump under it, what each player
	// has melded, and whether the stock is gone.
	Stock  []string        `json:"stock,omitempty"`
	TurnUp string          `json:"turnUp,omitempty"`
	Used   map[string]Used `json:"used,omitempty"`
	Late   bool            `json:"late,omitempty"`
	// MeldLog is two-handed melds in the order they were laid, for the
	// record and the table.
	MeldLog []LaidMeld `json:"meldLog,omitempty"`

	Leader    string          `json:"leader,omitempty"`
	Trick     []tricks.Play   `json:"trick,omitempty"`
	LastTrick []tricks.Play   `json:"lastTrick,omitempty"`
	History   [][]tricks.Play `json:"history,omitempty"`

	// What each side has taken this hand, by side id.
	Taken    map[string]int `json:"taken,omitempty"`    // card points, last trick included
	Tricks   map[string]int `json:"tricks,omitempty"`   // tricks won
	Counters map[string]int `json:"counters,omitempty"` // counting cards won
	// MeldScore is meld already credited this hand, by side (two-handed and
	// the dix of the turn-up).
	MeldScore map[string]int `json:"meldScore,omitempty"`

	Scores       map[string]int      `json:"scores"` // by side
	Rounds       []HandRecord        `json:"rounds,omitempty"`
	Winners      []string            `json:"winners,omitempty"`
	Intermission module.Intermission `json:"intermission,omitempty"`
}

// LaidMeld is one two-handed meld.
type LaidMeld struct {
	Player string   `json:"player"`
	Item   MeldItem `json:"item"`
}

// HandRecord is one finished hand. Public: it names players, sides and
// numbers, never a card held.
type HandRecord struct {
	Number   int            `json:"number"` // 1-based
	Bidder   string         `json:"bidder,omitempty"`
	Bid      int            `json:"bid,omitempty"`
	Trump    string         `json:"trump,omitempty"`
	Stuck    bool           `json:"stuck,omitempty"`
	Made     bool           `json:"made,omitempty"`
	ThrownIn bool           `json:"thrownIn,omitempty"`
	Redealt  bool           `json:"redealt,omitempty"`
	Meld     map[string]int `json:"meld"`  // by side, as counted
	Taken    map[string]int `json:"taken"` // by side
	Delta    map[string]int `json:"delta"` // by side
	Totals   map[string]int `json:"totals"`
}

// Phases of a hand.
const (
	phaseAuction  = "auction"
	phaseTrump    = "trump"
	phaseBury     = "bury"
	phasePassIn   = "passIn"   // the bidder's partner passes to the bidder
	phasePassBack = "passBack" // the bidder passes back
	phasePlay     = "play"
	phaseMeld     = "meld" // two-handed: the trick's winner may meld
)

// Verbs this module accepts. VerbContinue belongs to module.Intermission.
const (
	VerbBid     = "bid"
	VerbPass    = "pass"
	VerbTrump   = "name_trump"
	VerbBury    = "bury"
	VerbGive    = "pass_cards"
	VerbPlay    = "play_card"
	VerbThrowIn = "throw_in"
	VerbMeld    = "meld"
	VerbDraw    = "draw"
)

// Offer ids that are not melds.
const (
	OfferBid      = "bid"
	OfferPass     = "pass"
	OfferTrump    = "trump"
	OfferBury     = "bury"
	OfferGive     = "give"
	OfferPlay     = "play"
	OfferThrowIn  = "throwIn"
	OfferDraw     = "draw"
	OfferExchange = "meld.exchange"
	OfferDix      = "meld.dix"
)

// ParamAmount is the bid figure; ParamSuit names trumps.
const (
	ParamAmount = "amount"
	ParamSuit   = "suit"
)

// Error codes. Stable keys, worded by the client's locale bundles.
const (
	ErrNotYourTurn   = "NOT_YOUR_TURN"
	ErrGameNotActive = "GAME_NOT_ACTIVE"
	ErrUnknownAction = "UNKNOWN_ACTION"
	ErrCardNotInHand = "CARD_NOT_IN_HAND"
	ErrWrongPlayers  = "WRONG_PLAYER_COUNT"
	ErrNotANumber    = "AMOUNT_NOT_A_NUMBER"

	ErrWrongPhase    = "PINOCHLE_NOT_NOW"
	ErrBidTooLow     = "PINOCHLE_BID_TOO_LOW"
	ErrBidStep       = "PINOCHLE_BID_STEP"
	ErrBidTooHigh    = "PINOCHLE_BID_TOO_HIGH"
	ErrAlreadyPassed = "PINOCHLE_ALREADY_PASSED"
	ErrNoSuchSuit    = "PINOCHLE_NO_SUCH_SUIT"
	ErrBuryCount     = "PINOCHLE_BURY_COUNT"
	ErrGiveCount     = "PINOCHLE_PASS_COUNT"
	ErrPickOneCard   = "PINOCHLE_PICK_ONE_CARD"
	ErrMustFollow    = "PINOCHLE_MUST_FOLLOW"
	ErrMustBeat      = "PINOCHLE_MUST_BEAT"
	ErrMustTrump     = "PINOCHLE_MUST_TRUMP"
	ErrNotBidder     = "PINOCHLE_ONLY_THE_BIDDER"
	ErrTooLate       = "PINOCHLE_PLAY_HAS_BEGUN"
	ErrNoSuchMeld    = "PINOCHLE_NO_SUCH_MELD"
	ErrNoExchange    = "PINOCHLE_NO_EXCHANGE"
)

func decode(raw module.State) (*GameState, error) {
	var s GameState
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("pinochle: decode state: %w", err)
	}
	for _, m := range []*map[string]int{&s.Taken, &s.Tricks, &s.Counters, &s.MeldScore, &s.Scores, &s.LastBid} {
		if *m == nil {
			*m = map[string]int{}
		}
	}
	if s.Hands == nil {
		s.Hands = map[string][]string{}
	}
	if s.Passed == nil {
		s.Passed = map[string]bool{}
	}
	if s.Used == nil {
		s.Used = map[string]Used{}
	}
	if s.Melds == nil {
		s.Melds = map[string]Meld{}
	}
	if s.Given == nil {
		s.Given = map[string][]string{}
	}
	return &s, nil
}

func encode(s *GameState) (module.State, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("pinochle: encode state: %w", err)
	}
	return raw, nil
}

func errCode(code string) error { return module.Error{Code: code} }

func (s *GameState) table() table { return tableOf(s.Variation) }

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

func (s *GameState) dealer() string { return s.Players[(s.FirstDealer+s.Deal)%len(s.Players)] }

// side is who a player scores with: seat mod 2 in a partnership, the seat
// itself otherwise.
func (s *GameState) side(id string) string { return sideOf(s.seat(id), s.table().partners) }

func sideOf(seat int, partners bool) string {
	if partners {
		return strconv.Itoa(seat % 2)
	}
	return strconv.Itoa(seat)
}

// sides is every side at the table, in seat order.
func (s *GameState) sides() []string {
	if s.table().partners {
		return []string{"0", "1"}
	}
	out := make([]string, len(s.Players))
	for i := range s.Players {
		out[i] = strconv.Itoa(i)
	}
	return out
}

// members is the players of one side.
func (s *GameState) members(side string) []string {
	var out []string
	for _, p := range s.Players {
		if s.side(p) == side {
			out = append(out, p)
		}
	}
	return out
}

// partner is a player's partner, or "" at a table without sides.
func (s *GameState) partner(id string) string {
	if !s.table().partners {
		return ""
	}
	return s.Players[(s.seat(id)+2)%4]
}

func (s *GameState) trump() byte {
	if s.Trump == "" {
		return tricks.NoTrump
	}
	return s.Trump[0]
}

// policy is what a follower owes the trick: in two-handed nothing at all
// while the stock lasts; otherwise follow, beat, trump and overtrump.
func (s *GameState) policy() (tricks.Policy, bool) {
	if s.Variation == VarTwoHanded && !s.Late {
		return tricks.FollowOnly, false
	}
	return tricks.FollowBeatTrump, true
}

// legal is the cards p may play now.
func (s *GameState) legal(p string) []string {
	if s.Phase != phasePlay || s.Current != p {
		return nil
	}
	pol, bound := s.policy()
	if !bound {
		return uniq(s.Hands[p])
	}
	return uniq(tricks.Legal(s.Hands[p], tricks.Trick{Plays: s.Trick}, s.trump(), s.table().order(), pol))
}

// uniq is a card list with each code once, in its first order: the two
// queens of spades are one choice, not two.
func uniq(cards []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range cards {
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	return out
}

func hasCards(hand []string, cards []string) bool {
	have := countOf(hand)
	for _, c := range cards {
		if have[c] == 0 {
			return false
		}
		have[c]--
	}
	return true
}

func removeCards(hand []string, cards []string) []string {
	drop := countOf(cards)
	out := make([]string, 0, len(hand))
	for _, c := range hand {
		if drop[c] > 0 {
			drop[c]--
			continue
		}
		out = append(out, c)
	}
	return out
}

// sortHand orders a hand suit by suit, trumps first once they are named,
// each suit high to low. Display only.
func sortHand(hand []string, trump byte, o tricks.Order) {
	suitRank := func(su byte) int {
		if su == trump {
			return -1
		}
		for i := 0; i < len(suits); i++ {
			if suits[i] == su {
				return i
			}
		}
		return len(suits)
	}
	sort.SliceStable(hand, func(i, j int) bool {
		a, b := hand[i], hand[j]
		if tricks.Suit(a) != tricks.Suit(b) {
			return suitRank(tricks.Suit(a)) < suitRank(tricks.Suit(b))
		}
		return o.Beats(tricks.Rank(a), tricks.Rank(b))
	})
}

func (s *GameState) sortAll() {
	for _, p := range s.Players {
		sortHand(s.Hands[p], s.trump(), s.table().order())
	}
}

// trickPoints is what a finished trick holds.
func (s *GameState) trickPoints(plays []tricks.Play) int {
	n := 0
	for _, pl := range plays {
		n += cardPoints(pl.Card, s.Scoring)
	}
	return n
}
