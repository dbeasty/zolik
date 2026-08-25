// Package legacy holds the Žolíky documents that predate the module runtime,
// and the one-shot that moves them onto it.
//
// Nothing in the running server reads this package. It exists so the migration
// can still decode a `games` document after everything that used to write one
// has been deleted — a migration that cannot read the old shape is not a
// migration, it is a data-loss notice.
//
// The document below is `models.Game` as it last stood: a Žolíky-shaped Mongo
// record with `drawPile`, `melds`, `meldMeta` and `roundReqMet` as first-class
// columns, plus a duplicate of the engine's own config. It is, field for field,
// `rules.GameState` plus an envelope — which is exactly the thirty-field
// hand-mapping the module split removed, preserved here only long enough to
// read it once.
//
// Delete this package once no deployment has a `games` collection left.
package legacy

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/models"
)

type Game struct {
	ID     bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Status string        `bson:"status" json:"status"`
	// GameNumber keeps the legacy "round" Mongo key (no migration needed for
	// existing documents) but is exposed to the API as "game" — see
	// rules.GameState.GameNumber for what this represents.
	GameNumber  int      `bson:"round" json:"game"`
	Phase       string   `bson:"phase" json:"phase"`
	JoinCode    string   `bson:"joinCode" json:"joinCode,omitempty"`
	HostID      string   `bson:"hostId" json:"hostId,omitempty"`
	CurrentTurn string   `bson:"currentTurn" json:"currentTurn"`
	TurnOrder   []string `bson:"turnOrder" json:"turnOrder"`
	// RulesProfile names the resolved ruleset this game runs under — see
	// rules.ResolveProfile. Set at lobby creation; empty/unknown defaults to
	// "zolik_classic".
	RulesProfile string `bson:"rulesProfile" json:"rulesProfile"`
	// Rules is the fully-resolved ruleset this game runs under, frozen at
	// creation — see rules.RulesConfig. It is persisted (rather than
	// re-derived from RulesProfile on every load) so that per-game house-rule
	// overrides survive a reload, and so that editing a shipped profile
	// constant can never retroactively change the rules of a game already in
	// progress. Nil only on documents written before this field existed;
	// toRulesState migrates those from RulesProfile plus the two legacy
	// scalar columns below.
	Rules *RulesConfig `bson:"rules,omitempty" json:"-"`
	// DealStarterID/Round are new fields (fresh bson keys, so old documents
	// simply default to "" / 0 until their next deal) — see
	// rules.GameState.DealStarterID / Round for what they represent.
	DealStarterID  string                `bson:"dealStarterId" json:"-"`
	Round          int                   `bson:"tableRound" json:"round"`
	DrawPile       []string              `bson:"drawPile" json:"-"`
	DiscardPile    []string              `bson:"discardPile" json:"discardPile"`
	ReshuffleCount int                   `bson:"reshuffleCount" json:"reshuffleCount"`
	DeckSeed       int64                 `bson:"deckSeed" json:"deckSeed"`
	Hands          map[string][]string   `bson:"hands" json:"-"`
	Melds          map[string][][]string `bson:"melds" json:"melds"`
	MeldMeta       map[string][]MeldInfo `bson:"meldMeta" json:"meldMeta"`
	GameScores     map[string][]int      `bson:"roundScores" json:"gameScores"`
	TotalScores    map[string]int        `bson:"totalScores" json:"totalScores"`
	WinnerID       string                `bson:"winnerId,omitempty" json:"winnerId,omitempty"`
	IsDraw         bool                  `bson:"isDraw,omitempty" json:"isDraw,omitempty"`
	RoundReqMet    map[string]bool       `bson:"roundReqMet" json:"roundReqMet"`
	// InitialMeldMinimum/DiscardDrawMinRound are legacy columns kept only so
	// pre-Rules documents can still be migrated on load (see Rules above).
	// Live reads and writes go through Rules; these are mirrored on save so a
	// rollback to an older server build still finds sane values.
	InitialMeldMinimum          int               `bson:"initialMeldMinimum" json:"initialMeldMinimum"`
	DiscardDrawMinRound         int               `bson:"discardDrawMinRound" json:"discardDrawMinRound"`
	MeldsLaidThisTurn           int               `bson:"meldsLaidThisTurn" json:"-"`
	DiscardDrawnCardPendingMeld string            `bson:"discardDrawnCardPendingMeld" json:"-"`
	DiscardTakenCard            string            `bson:"discardTakenCard" json:"-"`
	DiscardDrawnCards           []string          `bson:"discardDrawnCards" json:"-"`
	LastLayOff                  *LayOffSnapshot   `bson:"lastLayOff,omitempty" json:"-"`
	LastMeldLaid                *MeldLaidSnapshot `bson:"lastMeldLaid,omitempty" json:"-"`
	TurnMeldSnapshot            *TurnMeldSnapshot `bson:"turnMeldSnapshot,omitempty" json:"-"`
	Players                     []models.Player   `bson:"players" json:"players"`
	ActionLog                   []Action          `bson:"actionLog" json:"-"`
	// RawActionLog mirrors ActionLog but stores the raw player input (one
	// entry per accepted player action, vs. ActionLog's one-or-more derived
	// StateEvents per action) so a stored game can be replayed through
	// rules.ReplayActions — see DealInitialState and game.RespondTakeback.
	RawActionLog []RawAction `bson:"rawActionLog,omitempty" json:"-"`
	// DealInitialState snapshots rules state immediately after the current
	// deal (GameNumber) was dealt, before any player action — the
	// deterministic replay anchor for takeback and log verification. Reset
	// every time a new deal begins.
	DealInitialState *DealSnapshot `bson:"dealInitialState,omitempty" json:"-"`
	// PendingTakeback is set while a takeback request awaits the other
	// active players' approval; nil otherwise.
	PendingTakeback *TakebackRequest `bson:"pendingTakeback,omitempty" json:"pendingTakeback,omitempty"`
	NextMeldSeq     int              `bson:"nextMeldSeq" json:"nextMeldSeq"`
	SuspendedAt     *time.Time       `bson:"suspendedAt" json:"suspendedAt,omitempty"`
	AbandonAt       *time.Time       `bson:"abandonAt" json:"abandonAt,omitempty"`
	PreSuspendPhase string           `bson:"preSuspendPhase,omitempty" json:"-"`
	CreatedAt       time.Time        `bson:"createdAt" json:"createdAt"`
	CompletedAt     *time.Time       `bson:"completedAt" json:"completedAt,omitempty"`
	Version         int64            `bson:"version" json:"-"`
}

type MeldInfo struct {
	MeldID    string `bson:"meldId" json:"meldId"`
	Type      string `bson:"type" json:"type"`
	OwnerID   string `bson:"ownerId" json:"ownerId"`
	WildCount int    `bson:"wildCount" json:"wildCount"`
}

// LayOffSnapshot mirrors rules.LayOffSnapshot for persistence — see there
// for what it's used for.
type LayOffSnapshot struct {
	PlayerID  string   `bson:"playerId" json:"-"`
	MeldID    string   `bson:"meldId" json:"-"`
	PrevCards []string `bson:"prevCards" json:"-"`
	PrevMeta  MeldInfo `bson:"prevMeta" json:"-"`
	Cards     []string `bson:"cards" json:"-"`

	PrevDiscardTakenCard string `bson:"prevDiscardTakenCard" json:"-"`
	PrevOwnerReqMet      bool   `bson:"prevOwnerReqMet" json:"-"`
}

// MeldLaidSnapshot mirrors rules.MeldLaidSnapshot for persistence — see
// there for what it's used for.
type MeldLaidSnapshot struct {
	PlayerID                        string   `bson:"playerId" json:"-"`
	MeldID                          string   `bson:"meldId" json:"-"`
	Cards                           []string `bson:"cards" json:"-"`
	PrevRoundReqMet                 bool     `bson:"prevRoundReqMet" json:"-"`
	PrevMeldsLaidThisTurn           int      `bson:"prevMeldsLaidThisTurn" json:"-"`
	PrevDiscardDrawnCardPendingMeld string   `bson:"prevDiscardDrawnCardPendingMeld" json:"-"`
	PrevDiscardTakenCard            string   `bson:"prevDiscardTakenCard" json:"-"`
}

// TurnMeldSnapshot mirrors rules.TurnMeldSnapshot for persistence — see there
// for what it's used for.
type TurnMeldSnapshot struct {
	PlayerID                    string                `bson:"playerId" json:"-"`
	Hands                       map[string][]string   `bson:"hands" json:"-"`
	Melds                       map[string][][]string `bson:"melds" json:"-"`
	MeldMeta                    map[string][]MeldInfo `bson:"meldMeta" json:"-"`
	RoundReqMet                 bool                  `bson:"roundReqMet" json:"-"`
	AllRoundReqMet              map[string]bool       `bson:"allRoundReqMet,omitempty" json:"-"`
	MeldsLaidThisTurn           int                   `bson:"meldsLaidThisTurn" json:"-"`
	DiscardDrawnCardPendingMeld string                `bson:"discardDrawnCardPendingMeld" json:"-"`
	DiscardTakenCard            string                `bson:"discardTakenCard" json:"-"`
	DiscardDrawnCards           []string              `bson:"discardDrawnCards" json:"-"`
	DiscardPile                 []string              `bson:"discardPile" json:"-"`
	NextMeldSeq                 int                   `bson:"nextMeldSeq" json:"-"`
}

// RulesConfig mirrors rules.RulesConfig for persistence. Every field of the
// engine's config is stored, so a game's ruleset is reconstructed exactly as
// it was resolved at creation rather than re-derived from a profile name.
type RulesConfig struct {
	Profile string `bson:"profile" json:"profile"`

	DealSize   int `bson:"dealSize" json:"dealSize"`
	MinSetSize int `bson:"minSetSize" json:"minSetSize"`
	MinRunSize int `bson:"minRunSize" json:"minRunSize"`

	InitialMeldMinimum  int `bson:"initialMeldMinimum" json:"initialMeldMinimum"`
	DiscardDrawMinRound int `bson:"discardDrawMinRound" json:"discardDrawMinRound"`

	DiscardPickupMode      string `bson:"discardPickupMode" json:"discardPickupMode"`
	JokerDiscardRestricted bool   `bson:"jokerDiscardRestricted" json:"jokerDiscardRestricted"`

	FixedDealCount int                 `bson:"fixedDealCount" json:"fixedDealCount"`
	StaticContract ContractRequirement `bson:"staticContract" json:"staticContract"`

	MatchEndMode string `bson:"matchEndMode" json:"matchEndMode"`
	TargetScore  int    `bson:"targetScore" json:"targetScore"`
}

// ContractRequirement mirrors rules.ContractRequirement for persistence.
type ContractRequirement struct {
	Sets            int  `bson:"sets" json:"sets"`
	Runs            int  `bson:"runs" json:"runs"`
	RequireCleanRun bool `bson:"requireCleanRun" json:"requireCleanRun"`
}

type Action struct {
	Seq       int                    `bson:"seq" json:"seq"`
	Timestamp time.Time              `bson:"timestamp" json:"timestamp"`
	Type      string                 `bson:"type" json:"type"`
	PlayerID  string                 `bson:"playerId" json:"playerId"`
	Data      map[string]interface{} `bson:"data" json:"data"`
	// TurnSeq is the Seq of the RawAction that produced this event (a single
	// player action can emit several events, e.g. a reshuffling draw), so a
	// takeback can drop every Action with TurnSeq beyond the target turn.
	// Zero for synthetic entries with no player input (suspend/resume).
	TurnSeq int `bson:"turnSeq,omitempty" json:"-"`
}

// ActionInput is the raw player action that produced a RawAction log entry
// — enough to reconstruct a rules.Action and replay it via
// rules.ReplayActions. Mirrors rules.Action's fields as plain data so
// models stays independent of the rules package.
type ActionInput struct {
	Type      string   `bson:"type"`
	DrawFrom  string   `bson:"drawFrom,omitempty"`
	Cards     []string `bson:"cards,omitempty"`
	MeldID    string   `bson:"meldId,omitempty"`
	Card      string   `bson:"card,omitempty"`
	CardIndex *int     `bson:"cardIndex,omitempty"`
	Position  string   `bson:"position,omitempty"`
}

// RawAction is one accepted player action, stored verbatim so the game can
// be replayed later (log verification, AI training data, takeback).
type RawAction struct {
	Seq       int         `bson:"seq" json:"seq"`
	Timestamp time.Time   `bson:"timestamp" json:"timestamp"`
	PlayerID  string      `bson:"playerId" json:"playerId"`
	Input     ActionInput `bson:"input" json:"input"`
}

// DealSnapshot is the rules-relevant game state captured immediately after
// a deal, before any player action — the deterministic starting point a
// takeback (or any other replay) applies RawActionLog entries against.
// SinceSeq is the RawActionLog Seq in effect when the snapshot was taken
// (0 for a fresh deal with no actions yet), so replay knows which
// RawActionLog entries belong to this deal.
type DealSnapshot struct {
	GameNumber          int                   `bson:"gameNumber"`
	SinceSeq            int                   `bson:"sinceSeq"`
	Phase               string                `bson:"phase"`
	CurrentTurn         string                `bson:"currentTurn"`
	TurnOrder           []string              `bson:"turnOrder"`
	DealStarterID       string                `bson:"dealStarterId"`
	Round               int                   `bson:"round"`
	DrawPile            []string              `bson:"drawPile"`
	DiscardPile         []string              `bson:"discardPile"`
	ReshuffleCount      int                   `bson:"reshuffleCount"`
	DeckSeed            int64                 `bson:"deckSeed"`
	Hands               map[string][]string   `bson:"hands"`
	Melds               map[string][][]string `bson:"melds"`
	MeldMeta            map[string][]MeldInfo `bson:"meldMeta"`
	RoundReqMet         map[string]bool       `bson:"roundReqMet"`
	InitialMeldMinimum  int                   `bson:"initialMeldMinimum"`
	DiscardDrawMinRound int                   `bson:"discardDrawMinRound"`
	GameScores          map[string][]int      `bson:"gameScores"`
	TotalScores         map[string]int        `bson:"totalScores"`
	NextMeldSeq         int                   `bson:"nextMeldSeq"`
}

// TakebackRequest tracks a pending "undo back to RawAction Seq ToSeq"
// proposal awaiting the rest of the active players' consent. Approvals
// holds one entry per active (non-AI) player who has responded true;
// AI players auto-approve and are never added here (see game.RequestTakeback).
type TakebackRequest struct {
	RequesterID string          `bson:"requesterId" json:"requesterId"`
	ToSeq       int             `bson:"toSeq" json:"toSeq"`
	Approvals   map[string]bool `bson:"approvals" json:"approvals"`
	CreatedAt   time.Time       `bson:"createdAt" json:"createdAt"`
}
