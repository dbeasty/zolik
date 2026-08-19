package rules

import (
	"time"
)

type Phase string

const (
	PhaseDraw      Phase = "draw"
	PhaseMeld      Phase = "meld"
	PhaseDiscard   Phase = "discard"
	PhaseOffer     Phase = "offer"
	PhaseSuspended Phase = "suspended"
)

type GameStatus string

const (
	StatusLobby     GameStatus = "lobby"
	StatusActive    GameStatus = "active"
	StatusSuspended GameStatus = "suspended"
	StatusCompleted GameStatus = "completed"
	StatusAbandoned GameStatus = "abandoned"
)

type DiscardOffer struct {
	Card      string
	OfferedTo string
}

type MeldType string

const (
	MeldSet MeldType = "set"
	MeldRun MeldType = "run"
)

type MeldInfo struct {
	MeldID  string
	Type    MeldType
	OwnerID string
}

type ActionType string

const (
	ActionDrawCard     ActionType = "draw_card"
	ActionAcceptOffer  ActionType = "accept_offer"
	ActionDeclineOffer ActionType = "decline_offer"
	ActionLayMeld      ActionType = "lay_meld"
	ActionLayOff       ActionType = "lay_off"
	ActionDiscard      ActionType = "discard"
)

type DrawFrom string

const (
	DrawFromDeck    DrawFrom = "deck"
	DrawFromDiscard DrawFrom = "discard"
)

type Action struct {
	Type ActionType

	DrawFrom DrawFrom

	Cards  []string // for lay_meld
	MeldID string   // for lay_off
	Card   string   // for lay_off, discard
}

type GameState struct {
	Status  GameStatus
	Round   int
	Phase   Phase
	Created time.Time

	CurrentTurn string
	TurnOrder   []string

	DrawPile       []string
	DiscardPile    []string // top = last element
	ReshuffleCount int

	Hands map[string][]string

	// Melds are stored by player/owner, but lay-off can target any meld.
	Melds    map[string][][]string
	MeldMeta map[string][]MeldInfo // ownerId -> []MeldInfo aligned with Melds[ownerId] index

	RoundReqMet        map[string]bool
	InitialMeldMinimum int
	// DiscardDrawMinRound gates the "draw from discard pile" action: 0 or 1
	// means no restriction (allowed from round 1); N>1 means players can't
	// draw from discard until round N (they can still draw from the deck).
	DiscardDrawMinRound int
	// MeldsLaidThisTurn counts lay_meld actions the current turn's actor has
	// made toward their (not-yet-met) initial round requirement since their
	// turn began. A player who has started but not finished their initial
	// meld combination this turn cannot discard (end their turn) until they
	// either complete it or... they must complete it — see ValidateDiscard.
	// Reset to 0 whenever a player's turn begins (draw, or accept-offer).
	MeldsLaidThisTurn int

	Offer *DiscardOffer

	DeckSeed int64

	// Scoring
	RoundScores map[string][]int
	TotalScores map[string]int

	WinnerID string
	IsDraw   bool

	NextMeldSeq int
}

type RulesErrorCode string

const (
	ErrNotYourTurn       RulesErrorCode = "NOT_YOUR_TURN"
	ErrWrongPhase        RulesErrorCode = "WRONG_PHASE"
	ErrCardNotInHand     RulesErrorCode = "CARD_NOT_IN_HAND"
	ErrInvalidMeld       RulesErrorCode = "INVALID_MELD"
	ErrRoundReqNotMet    RulesErrorCode = "ROUND_REQ_NOT_MET"
	ErrMeldBelowMinimum  RulesErrorCode = "MELD_BELOW_MINIMUM"
	ErrMeldNoContribution RulesErrorCode = "MELD_NO_CONTRIBUTION"
	ErrTooManyWilds      RulesErrorCode = "TOO_MANY_WILDS"
	ErrAdjacentWilds     RulesErrorCode = "ADJACENT_WILDS"
	ErrAceBridge         RulesErrorCode = "ACE_BRIDGE"
	ErrNotOfferRecipient RulesErrorCode = "NOT_OFFER_RECIPIENT"
	ErrNoActiveOffer     RulesErrorCode = "NO_ACTIVE_OFFER"
	ErrDiscardPileEmpty  RulesErrorCode = "DISCARD_PILE_EMPTY"
	ErrDiscardLocked     RulesErrorCode = "DISCARD_LOCKED"
	ErrIncompleteInitialMeld RulesErrorCode = "INCOMPLETE_INITIAL_MELD"
	ErrGameSuspended     RulesErrorCode = "GAME_SUSPENDED"
	ErrGameNotActive     RulesErrorCode = "GAME_NOT_ACTIVE"

	// Not in spec list; required by your decision for empty deck+discard.
	ErrNoCardsLeft RulesErrorCode = "NO_CARDS_LEFT"
)

type RulesError struct {
	Code    RulesErrorCode
	Message string
}

func (e RulesError) Error() string {
	if e.Message != "" {
		return string(e.Code) + ": " + e.Message
	}
	return string(e.Code)
}

