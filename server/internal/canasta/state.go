// Package canasta implements Classic (American) Canasta as a game module.
//
// It is the third module behind the runtime, and the first one that is a
// *rummy* and still not the rummy engine. `architecture.md` §1 predicted this
// game would be a configuration problem — a profile of `internal/rules`. It is
// not, and `docs/canasta-plan.md` §1 records why: partnership-owned melds,
// capture of the whole discard pile, a pile that can be frozen, cards that
// leave your hand on sight, wild limits inside a meld and multi-deal scoring
// are rules with no knob behind them. Expressing them as `RulesConfig` values
// would put a second game's clauses inside the engine Žolíky depends on, which
// is the coupling the module seam exists to prevent.
//
// Rules implemented (Hoyle / pagat.com classic canasta):
//   - 108 cards: two decks plus four jokers. Two or four players; at four,
//     seats 0+2 play seats 1+3.
//   - Melds are sets of equal rank, 3–7 cards, at most three wilds and at
//     least two naturals, owned by the partnership. Seven cards is a canasta:
//     natural (no wilds) 500, mixed 300.
//   - Jokers and 2s are wild. Red threes lay themselves down for 100 each
//     (800 for all four) and count against a partnership with no canasta.
//     Black threes block the pile when discarded and may only be melded on
//     the way out.
//   - A turn is draw (or take the whole discard pile) → meld → discard.
//   - The pile may be taken only by using its top card at once, and is frozen
//     against everyone by a buried wild and against a partnership that has
//     not yet made its initial meld.
//   - The initial meld minimum rises with the partnership's own score:
//     15 / 50 / 90 / 120.
//   - Going out needs the partnership's canasta quota. 100, or 200 concealed.
//   - Deals repeat until a partnership passes the target score.
package canasta

import (
	"encoding/json"
	"fmt"

	"zolik/server/internal/module"
)

// Verbs this module accepts.
const (
	VerbDraw     = "draw"
	VerbTakePile = "take_pile"
	VerbLayMeld  = "lay_meld"
	VerbLayOff   = "lay_off"
	VerbDiscard  = "discard"
)

// Turn phases. Two, not three: melding and discarding are the same phase,
// because in Canasta a discard is simply the move that ends it.
const (
	phaseDraw = "draw"
	phaseMeld = "meld"
)

// Error codes. Stable keys a client renders from its locale bundle, never
// sentences — the same contract the other two modules keep.
const (
	ErrNotYourTurn        = "NOT_YOUR_TURN"
	ErrGameNotActive      = "GAME_NOT_ACTIVE"
	ErrWrongPhase         = "WRONG_PHASE"
	ErrCardNotInHand      = "CARD_NOT_IN_HAND"
	ErrUnknownAction      = "UNKNOWN_ACTION"
	ErrNothingToDraw      = "NOTHING_TO_DRAW"
	ErrPileEmpty          = "PILE_EMPTY"
	ErrPileBlocked        = "PILE_BLOCKED"
	ErrPileFrozen         = "PILE_FROZEN"
	ErrTopCardUnusable    = "TOP_CARD_UNUSABLE"
	ErrMeldTooSmall       = "MELD_TOO_SMALL"
	ErrMeldTooLarge       = "MELD_TOO_LARGE"
	ErrMeldMixedRanks     = "MELD_MIXED_RANKS"
	ErrTooManyWilds       = "TOO_MANY_WILDS"
	ErrNotEnoughNaturals  = "NOT_ENOUGH_NATURALS"
	ErrRankAlreadyMelded  = "RANK_ALREADY_MELDED"
	ErrCannotMeldThree    = "CANNOT_MELD_THREE"
	ErrNoSuchMeld         = "NO_SUCH_MELD"
	ErrNotYourMeld        = "NOT_YOUR_MELD"
	ErrMeldClosed         = "MELD_CLOSED"
	ErrWrongRank          = "WRONG_RANK"
	ErrInitialMeldNotMet  = "INITIAL_MELD_NOT_MET"
	ErrMustMeldFirst      = "MUST_MELD_FIRST"
	ErrCannotDiscardThree = "CANNOT_DISCARD_RED_THREE"
	ErrCannotGoOutYet     = "CANNOT_GO_OUT_YET"
	ErrMustKeepACard      = "MUST_KEEP_A_CARD"
)

// Meld is one partnership's set of a single rank.
//
// Owned by the team rather than the player: either partner may extend it, and
// that single fact is most of why Canasta could not be a `RulesConfig` profile.
type Meld struct {
	ID     string   `json:"id"`
	TeamID int      `json:"teamId"`
	Rank   string   `json:"rank"`
	Cards  []string `json:"cards"`
}

func meldID(teamID int, rank string) string {
	return fmt.Sprintf("t%d-%s", teamID, rank)
}

func (m Meld) naturals() int {
	n := 0
	for _, c := range m.Cards {
		if !isWild(c) {
			n++
		}
	}
	return n
}

func (m Meld) wilds() int { return len(m.Cards) - m.naturals() }

// isCanasta reports whether this meld has reached seven cards, which is both
// the unit of progress and the licence to go out.
func (m Meld) isCanasta() bool { return len(m.Cards) >= canastaSize }

// isNatural reports a canasta with no wilds in it — worth 500 rather than 300.
func (m Meld) isNatural() bool { return m.wilds() == 0 }

// closed reports a meld that can take no more cards. A canasta is complete at
// seven; adding an eighth is not a bigger canasta, it is a rule nobody plays.
func (m Meld) closed() bool { return len(m.Cards) >= canastaSize }

// Team is a partnership: the scoring unit, and the owner of melds.
type Team struct {
	ID      int      `json:"id"`
	Players []string `json:"players"`
	// Score is carried across deals and is what the initial meld minimum and
	// the target are measured against.
	Score int    `json:"score"`
	Melds []Meld `json:"melds,omitempty"`
	// RedThrees are this deal's, laid face up the moment they appear.
	RedThrees []string `json:"redThrees,omitempty"`
	// HasMelded is whether the initial meld minimum has been satisfied this
	// deal. It gates lay-offs, unfreezes the pile for this team, and is reset
	// every deal.
	HasMelded bool `json:"hasMelded"`
}

func (t *Team) meld(rank string) *Meld {
	for i := range t.Melds {
		if t.Melds[i].Rank == rank {
			return &t.Melds[i]
		}
	}
	return nil
}

func (t *Team) meldByID(id string) *Meld {
	for i := range t.Melds {
		if t.Melds[i].ID == id {
			return &t.Melds[i]
		}
	}
	return nil
}

func (t *Team) canastas() int {
	n := 0
	for _, m := range t.Melds {
		if m.isCanasta() {
			n++
		}
	}
	return n
}

// GameState is the whole match — every deal of it. Opaque to the runtime.
type GameState struct {
	Status    string   `json:"status"` // "active" | "completed"
	Variation string   `json:"variation,omitempty"`
	Players   []string `json:"players"`
	TurnOrder []string `json:"turnOrder"`
	Current   string   `json:"current"`
	Phase     string   `json:"phase"`

	Teams  []Team         `json:"teams"`
	TeamOf map[string]int `json:"teamOf"`

	DrawPile    []string            `json:"drawPile"`
	DiscardPile []string            `json:"discardPile"` // top = last
	Hands       map[string][]string `json:"hands"`

	// Frozen is the pile-wide freeze: a wild buried in the pile, or a red
	// three turned up to start the deal. It outlives turns and is cleared
	// only when the pile is taken.
	Frozen bool `json:"frozen,omitempty"`

	// LaidThisTurn is the card value melded so far this turn, which is what
	// the initial meld minimum is measured against — the minimum is a
	// property of a *turn*, not of a single meld.
	LaidThisTurn int `json:"laidThisTurn,omitempty"`
	// MeldsAtTurnStart records whether the partnership had anything on the
	// table when this turn began. It is what makes a concealed go-out
	// detectable after the fact.
	MeldsAtTurnStart bool `json:"meldsAtTurnStart,omitempty"`
	TookPileThisTurn bool `json:"tookPileThisTurn,omitempty"`

	// Rules resolved at deal time, so a match cannot change shape underneath
	// a deal in progress.
	HandSize        int `json:"handSize"`
	TargetScore     int `json:"targetScore"`
	CanastasToGoOut int `json:"canastasToGoOut"`

	DealNumber int `json:"dealNumber"`
	// Dealer is the seat index that dealt, rotating each deal so the
	// first-player advantage moves around.
	Dealer int `json:"dealer"`

	// LastDeal is how the previous deal scored, for a scoreboard.
	LastDeal *DealResult `json:"lastDeal,omitempty"`
	// Deals is every deal that has been scored, oldest first.
	//
	// LastDeal is kept beside it rather than replaced by it: the board shows
	// one deal's settlement and the history shows all of them, and a reader of
	// either should not have to know about the other. Empty for a match that
	// was already in flight when this field arrived, which is why the round
	// numbers here are the deal's own and not a renumbering from one — a gap is
	// better read as a gap than papered over.
	Deals []DealResult `json:"deals,omitempty"`
	// Break is the pause between deals, and Pause whether this table takes one.
	//
	// Pause is resolved once, at NewMatch, and never re-read from the lobby's
	// options afterwards — which is what lets a match already in flight when
	// this shipped play out under the rules it was dealt under, with no
	// migration at all.
	Pause bool                `json:"pause,omitempty"`
	Break module.Intermission `json:"break,omitempty"`

	WinnerTeam int    `json:"winnerTeam"`
	WinnerID   string `json:"winnerId,omitempty"`
	Seed       int64  `json:"seed"`
}

// DealResult is one deal's arithmetic, kept so a client can show why the
// score moved without recomputing anything.
type DealResult struct {
	DealNumber int          `json:"dealNumber"`
	WentOut    string       `json:"wentOut,omitempty"`
	Concealed  bool         `json:"concealed,omitempty"`
	Exhausted  bool         `json:"exhausted,omitempty"`
	Teams      []TeamResult `json:"teams"`
}

// TeamResult breaks a deal's score into the parts a player argues about.
type TeamResult struct {
	TeamID    int `json:"teamId"`
	MeldCards int `json:"meldCards"`
	Canastas  int `json:"canastas"`
	RedThrees int `json:"redThrees"`
	GoingOut  int `json:"goingOut"`
	InHand    int `json:"inHand"`
	Total     int `json:"total"`
	Running   int `json:"running"`
}

func (s *GameState) team(playerID string) *Team {
	id, ok := s.TeamOf[playerID]
	if !ok {
		return nil
	}
	for i := range s.Teams {
		if s.Teams[i].ID == id {
			return &s.Teams[i]
		}
	}
	return nil
}

func (s *GameState) top() string {
	if len(s.DiscardPile) == 0 {
		return ""
	}
	return s.DiscardPile[len(s.DiscardPile)-1]
}

func (s *GameState) nextPlayer(from string) string {
	for i, p := range s.TurnOrder {
		if p == from {
			return s.TurnOrder[(i+1)%len(s.TurnOrder)]
		}
	}
	if len(s.TurnOrder) > 0 {
		return s.TurnOrder[0]
	}
	return ""
}

// allMelds is every meld on the table, both partnerships'. Used by the view
// and by lay-off validation, which has to be able to say "that is not yours".
func (s *GameState) allMelds() []Meld {
	var out []Meld
	for i := range s.Teams {
		out = append(out, s.Teams[i].Melds...)
	}
	return out
}

func (s *GameState) findMeld(id string) (*Team, *Meld) {
	for i := range s.Teams {
		if m := s.Teams[i].meldByID(id); m != nil {
			return &s.Teams[i], m
		}
	}
	return nil, nil
}

func decode(raw module.State) (*GameState, error) {
	var s GameState
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("canasta: decode state: %w", err)
	}
	return &s, nil
}

func encode(s *GameState) (module.State, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("canasta: encode state: %w", err)
	}
	return raw, nil
}
