package rules

import (
	"time"
)

type Phase string

const (
	PhaseDraw      Phase = "draw"
	PhaseMeld      Phase = "meld"
	PhaseDiscard   Phase = "discard"
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

type MeldType string

const (
	MeldSet MeldType = "set"
	MeldRun MeldType = "run"
)

// LayOffSnapshot holds what a lay_off needs to be undone: the meld's exact
// prior contents/meta and the cards it took from the player's hand.
type LayOffSnapshot struct {
	PlayerID  string
	MeldID    string
	PrevCards []string
	PrevMeta  MeldInfo
	Cards     []string
}

// MeldLaidSnapshot holds what a lay_meld needs to be undone: the brand-new
// meld it created plus everything else that lay_meld call may have changed
// (going down for the first time, consuming the discard-pickup obligation,
// or advancing MeldsLaidThisTurn) so undoing it restores state exactly as it
// was beforehand — not just "cards back in hand."
type MeldLaidSnapshot struct {
	PlayerID                        string
	MeldID                          string
	Cards                           []string
	PrevRoundReqMet                 bool
	PrevMeldsLaidThisTurn           int
	PrevDiscardDrawnCardPendingMeld string
}

// TurnMeldSnapshot captures everything a player's meld phase can touch, taken
// right after they draw, so ValidateUndoTurn can roll the whole phase back to
// that point no matter how many lay_meld/lay_off/swap_joker actions (and their
// own individual undos) happened since — a single "always available" undo
// distinct from the last-action-only LastLayOff/LastMeldLaid snapshots above.
type TurnMeldSnapshot struct {
	PlayerID                    string
	Hands                       map[string][]string
	Melds                       map[string][][]string
	MeldMeta                    map[string][]MeldInfo
	RoundReqMet                 bool
	MeldsLaidThisTurn           int
	DiscardDrawnCardPendingMeld string
	DiscardDrawnCards           []string
	DiscardPile                 []string
	NextMeldSeq                 int
}

type MeldInfo struct {
	MeldID  string
	Type    MeldType
	OwnerID string
	// WildCount is how many jokers/flex-aces the meld currently uses — 0
	// means "clean" (see RulesConfig.StaticContract.RequireCleanRun). It is
	// re-derived whenever the meld grows, so a lay-off cannot leave it stale.
	WildCount int
}

type ActionType string

const (
	ActionDrawCard        ActionType = "draw_card"
	ActionLayMeld         ActionType = "lay_meld"
	ActionLayOff          ActionType = "lay_off"
	ActionSwapJoker       ActionType = "swap_joker"
	ActionDiscard         ActionType = "discard"
	ActionUndoDrawDiscard ActionType = "undo_draw_discard"
	ActionUndoLayOff      ActionType = "undo_lay_off"
	ActionUndoLayMeld     ActionType = "undo_lay_meld"
	ActionUndoTurn        ActionType = "undo_turn"
)

type DrawFrom string

const (
	DrawFromDeck    DrawFrom = "deck"
	DrawFromDiscard DrawFrom = "discard"
)

type Action struct {
	Type ActionType

	DrawFrom DrawFrom

	// Cards is used for lay_meld, and for lay_off to add more than one card
	// to the same meld in a single action (a single-card lay_off may use
	// Card instead; Cards takes precedence when both are set).
	Cards  []string
	MeldID string // for lay_off, swap_joker
	// Card is used for lay_off (single-card form), discard, swap_joker (the
	// natural replacement card from hand), and (only under
	// DiscardPickupAnyFromPile) to name which discard-pile card a
	// draw_card{from:"discard"} action targets — empty means "the top card".
	Card string
	// CardIndex disambiguates which physical card Card names when the hand
	// holds a duplicate value (two decks in play) — the hand-slot index the
	// caller believes Card sits at. Currently only consulted by discard.
	// nil (or out-of-range / non-matching) falls back to removing the first
	// matching value, same as before this field existed.
	CardIndex *int
	// Position is an optional lay_off hint for run melds: "front" or "end"
	// names which side of the run the dropped card(s) must extend, so a
	// drag onto one specific end of a run does what it visually looks like
	// instead of the server silently picking whichever placement uses the
	// fewest wilds. Empty means "either end" (sets always ignore it).
	Position string
}

type GameState struct {
	Status GameStatus
	// Rules is the fully-resolved ruleset this game runs under. Always
	// concrete on a persisted/started game; zero-value here falls back to
	// ProfileContinental via effectiveRules(state) so older callers/tests
	// that construct a GameState without setting it keep working.
	Rules RulesConfig
	// GameNumber is which deal of the match this is (1-7). One GameNumber
	// runs from deal through someone going out; it drives the
	// RoundRequirementFor() Sets/Runs pattern and the match ends once
	// GameNumber reaches 7 and that deal finishes.
	GameNumber int
	Phase      Phase
	Created    time.Time

	CurrentTurn string
	TurnOrder   []string
	// DealStarterID is whoever acted first in the current GameNumber. Used to
	// detect when play has come back around to them, i.e. a full lap of the
	// table (see Round below).
	DealStarterID string

	DrawPile       []string
	DiscardPile    []string // top = last element
	ReshuffleCount int

	Hands map[string][]string

	// Melds are stored by player/owner, but lay-off can target any meld.
	Melds    map[string][][]string
	MeldMeta map[string][]MeldInfo // ownerId -> []MeldInfo aligned with Melds[ownerId] index

	RoundReqMet map[string]bool
	// Round counts full laps around the table within the current GameNumber:
	// it starts at 1 when a deal begins and increments each time play comes
	// back around to DealStarterID. Distinct from GameNumber (which deal),
	// this is what gates Rules.DiscardDrawMinRound.
	//
	// Note there is deliberately no InitialMeldMinimum/DiscardDrawMinRound
	// field here: those are rules, and rules live in Rules above. They used to
	// be duplicated onto GameState, which meant the engine read one copy while
	// callers configured the other — see Rules' doc comment.
	Round int
	// MeldsLaidThisTurn counts lay_meld actions the current turn's actor has
	// made toward their (not-yet-met) initial round requirement since their
	// turn began. A player who has started but not finished their initial
	// meld combination this turn cannot discard (end their turn) until they
	// either complete it or... they must complete it — see ValidateDiscard.
	// Reset to 0 whenever a player's turn begins.
	MeldsLaidThisTurn int
	// DiscardDrawnCardPendingMeld: when a player draws from the discard pile
	// while they haven't yet met their round requirement, that specific
	// card must be laid down as part of completing their initial meld this
	// turn — it can't just be picked up and held. Holds the card string
	// while that obligation is outstanding, "" once satisfied (the card was
	// used in a lay_meld) or moot (round requirement already met). Reset
	// whenever a turn begins.
	DiscardDrawnCardPendingMeld string
	// DiscardDrawnCards holds the card(s) just taken from the discard pile
	// this turn, in their original pile order, so ValidateUndoDrawDiscard
	// can put them back and let the player draw again. Set on a discard-pile
	// pickup, cleared to nil once anything else happens this turn (a
	// lay_meld, a lay_off, or a fresh draw) — undo is only available in the
	// window right after the pickup, before the drawn cards could have
	// scattered into melds.
	DiscardDrawnCards []string

	// LastLayOff snapshots the most recent lay_off this turn so
	// ValidateUndoLayOff can revert it — cleared whenever anything else
	// happens this turn (a fresh draw, a lay_meld, a swap_joker, or another
	// lay_off, which replaces it with its own snapshot). Mirrors
	// DiscardDrawnCards' same-turn undo window.
	LastLayOff *LayOffSnapshot

	// LastMeldLaid snapshots the most recent brand-new lay_meld this turn so
	// ValidateUndoLayMeld can revert it — cleared whenever anything else
	// happens this turn (a fresh draw, a lay_off, a swap_joker, or another
	// lay_meld, which replaces it with its own snapshot). Same same-turn
	// undo window as LastLayOff/DiscardDrawnCards: available only in the
	// window right after that lay_meld, before anything else has had a
	// chance to build on top of it.
	LastMeldLaid *MeldLaidSnapshot

	// TurnMeldSnapshot snapshots the whole meld phase's starting point (right
	// after the player's draw) so ValidateUndoTurn can revert everything done
	// since — every meld, lay-off, and joker swap this turn — in one action,
	// any time before discard. Set on each draw, cleared once the turn ends.
	TurnMeldSnapshot *TurnMeldSnapshot

	DeckSeed int64

	// Scoring
	GameScores  map[string][]int
	TotalScores map[string]int

	WinnerID string
	IsDraw   bool

	NextMeldSeq int
}

type RulesErrorCode string

const (
	ErrNotYourTurn           RulesErrorCode = "NOT_YOUR_TURN"
	ErrWrongPhase            RulesErrorCode = "WRONG_PHASE"
	ErrCardNotInHand         RulesErrorCode = "CARD_NOT_IN_HAND"
	ErrInvalidMeld           RulesErrorCode = "INVALID_MELD"
	ErrRoundReqNotMet        RulesErrorCode = "ROUND_REQ_NOT_MET"
	ErrMeldBelowMinimum      RulesErrorCode = "MELD_BELOW_MINIMUM"
	ErrMeldNoContribution    RulesErrorCode = "MELD_NO_CONTRIBUTION"
	ErrTooManyWilds          RulesErrorCode = "TOO_MANY_WILDS"
	ErrAdjacentWilds         RulesErrorCode = "ADJACENT_WILDS"
	ErrAceBridge             RulesErrorCode = "ACE_BRIDGE"
	ErrDiscardPileEmpty      RulesErrorCode = "DISCARD_PILE_EMPTY"
	ErrDiscardLocked         RulesErrorCode = "DISCARD_LOCKED"
	ErrIncompleteInitialMeld RulesErrorCode = "INCOMPLETE_INITIAL_MELD"
	ErrDiscardCardNotMelded  RulesErrorCode = "DISCARD_CARD_NOT_MELDED"
	ErrGameSuspended         RulesErrorCode = "GAME_SUSPENDED"
	ErrGameNotActive         RulesErrorCode = "GAME_NOT_ACTIVE"
	ErrJokerDiscard          RulesErrorCode = "JOKER_DISCARD_FORBIDDEN"
	ErrBreaksCleanRun        RulesErrorCode = "BREAKS_CLEAN_RUN"
	ErrNothingToUndo         RulesErrorCode = "NOTHING_TO_UNDO"
	ErrNoJokerInMeld         RulesErrorCode = "NO_JOKER_IN_MELD"
	ErrJokerSwapMismatch     RulesErrorCode = "JOKER_SWAP_MISMATCH"
	ErrWrongRunEnd           RulesErrorCode = "WRONG_RUN_END"

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
