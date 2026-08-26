package rules

import (
	"time"
)

type Phase string

const (
	PhaseDraw Phase = "draw"
	// PhaseIntermission: the deal has been scored and the table is waiting to
	// be told to deal the next one.
	PhaseIntermission Phase = "intermission"
	PhaseMeld         Phase = "meld"
	PhaseDiscard      Phase = "discard"
	PhaseSuspended    Phase = "suspended"
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
	// PrevDiscardTakenCard restores GameState.DiscardTakenCard, which this
	// lay_off may have spent by putting that card on the table. Without it,
	// undoing the lay_off would hand the card back with no marker attached
	// and reopen the take-it-and-give-it-straight-back loop the marker
	// exists to close.
	PrevDiscardTakenCard string
	// PrevOwnerReqMet restores the meld owner's RoundReqMet. A lay-off can
	// put the *owner* down (their meld grew past the contract or the point
	// floor) even when the owner is not the player laying off, so undoing
	// it has to take that back too — otherwise a lay-off and its undo leave
	// an opponent permanently down for free.
	PrevOwnerReqMet bool
	// ReclaimedJokers holds any jokers this lay-off swapped out of the meld
	// because a card in Cards took the joker's exact place, moved into the
	// player's hand alongside the ordinary addition. PrevCards already has
	// them back in the meld on undo; this is what lets undo also pull them
	// back out of the hand they were moved into, instead of leaving them
	// duplicated in both places.
	ReclaimedJokers []string
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
	PrevDiscardTakenCard            string
}

// TurnMeldSnapshot captures everything a player's meld phase can touch, taken
// right after they draw, so ValidateUndoTurn can roll the whole phase back to
// that point no matter how many lay_meld/lay_off/swap_joker actions (and their
// own individual undos) happened since — a single "always available" undo
// distinct from the last-action-only LastLayOff/LastMeldLaid snapshots above.
type TurnMeldSnapshot struct {
	PlayerID string
	Hands    map[string][]string
	Melds    map[string][][]string
	MeldMeta map[string][]MeldInfo
	// RoundReqMet is the acting player's own down-status. Kept as-is so a
	// snapshot persisted by an older build still restores that much.
	RoundReqMet bool
	// AllRoundReqMet is every player's down-status. A lay_off or a joker
	// swap during this turn can put the *meld's owner* down, who need not
	// be the acting player, so rolling the turn back has to roll their flag
	// back too — the single bool above cannot express that. Nil on a
	// snapshot written before this field existed; see ValidateUndoTurn.
	AllRoundReqMet              map[string]bool
	MeldsLaidThisTurn           int
	DiscardDrawnCardPendingMeld string
	DiscardTakenCard            string
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
	// DiscardTakenCard is the card this player specifically asked for when
	// they took from the discard pile this turn — the same card
	// DiscardDrawnCardPendingMeld tracks, but kept regardless of whether
	// they are down. It exists to stop a card being taken and handed
	// straight back on the same turn, which is a null move: the state after
	// it is the state before it, so a table of players all doing it never
	// progresses (observed live — one QH circulated between a player and an
	// agent while the deck still held dozens of cards). The obligation
	// field can't do this job on its own because it is deliberately empty
	// for a player who is already down, and DiscardDrawnCards can't either
	// because that one is cleared the moment anything else happens this
	// turn, to close the undo window.
	//
	// Cleared when the card leaves the hand legitimately (into a lay_meld,
	// a lay_off or a joker swap), when the pickup is undone, when the
	// player draws from the deck instead, and when the turn ends. Only the
	// requested card is tracked: under DiscardPickupAnyFromPile a pickup
	// also sweeps up every card above it, and those carry no obligation and
	// no null-move risk (that turn is a net gain of cards, not a no-op).
	DiscardTakenCard string
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
	// DealWinners is who went out in each deal, oldest first, parallel to the
	// per-player GameScores arrays.
	//
	// It has to be recorded rather than recovered, because a deal's winner is
	// not derivable from the scores: EndGame gives the go-out zero, and
	// HandPenaltyTotalWithMelds can also return zero for a non-winner whose
	// whole hand lays off. An empty string is a deal that ended with nobody
	// named — and so is a deal played before this field existed, which is why
	// a reader must treat the two the same.
	DealWinners []string
	// PendingDealStarter is who leads the deal the table is paused before.
	//
	// Remembered rather than worked out again on resume: who leads the next
	// deal is decided by who went out of this one, and that is gone the moment
	// the table is dealt again.
	PendingDealStarter string

	WinnerID string
	IsDraw   bool

	NextMeldSeq int
}

type RulesErrorCode string

const (
	ErrNotYourTurn RulesErrorCode = "NOT_YOUR_TURN"
	ErrWrongPhase  RulesErrorCode = "WRONG_PHASE"
	// ErrMustDrawFirst is the specific case of ErrWrongPhase that players
	// actually hit: trying to meld at the start of your turn, before drawing.
	// "Not available right now" is true but tells them nothing; naming the
	// missing step is the difference between a dead control and an
	// instruction — and it is a rule, so it belongs here rather than being
	// inferred by a client from the phase.
	ErrMustDrawFirst  RulesErrorCode = "MUST_DRAW_FIRST"
	ErrCardNotInHand  RulesErrorCode = "CARD_NOT_IN_HAND"
	ErrInvalidMeld    RulesErrorCode = "INVALID_MELD"
	ErrRoundReqNotMet RulesErrorCode = "ROUND_REQ_NOT_MET"
	// ErrNeedCleanRun: down in every other respect, but the house rule
	// wants a joker-free run and there isn't one. Distinct from
	// ErrRoundReqNotMet because no further set can ever satisfy it.
	ErrNeedCleanRun          RulesErrorCode = "NEED_CLEAN_RUN"
	ErrMeldBelowMinimum      RulesErrorCode = "MELD_BELOW_MINIMUM"
	ErrMeldNoContribution    RulesErrorCode = "MELD_NO_CONTRIBUTION"
	ErrTooManyWilds          RulesErrorCode = "TOO_MANY_WILDS"
	ErrAdjacentWilds         RulesErrorCode = "ADJACENT_WILDS"
	ErrAceBridge             RulesErrorCode = "ACE_BRIDGE"
	ErrDiscardPileEmpty      RulesErrorCode = "DISCARD_PILE_EMPTY"
	ErrDiscardLocked         RulesErrorCode = "DISCARD_LOCKED"
	ErrIncompleteInitialMeld RulesErrorCode = "INCOMPLETE_INITIAL_MELD"
	ErrDiscardCardNotMelded  RulesErrorCode = "DISCARD_CARD_NOT_MELDED"
	ErrDiscardTakenCard      RulesErrorCode = "DISCARD_TAKEN_CARD_FORBIDDEN"
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
