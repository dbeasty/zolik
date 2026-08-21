package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
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
	DiscardDrawnCards           []string          `bson:"discardDrawnCards" json:"-"`
	LastLayOff                  *LayOffSnapshot   `bson:"lastLayOff,omitempty" json:"-"`
	LastMeldLaid                *MeldLaidSnapshot `bson:"lastMeldLaid,omitempty" json:"-"`
	TurnMeldSnapshot            *TurnMeldSnapshot `bson:"turnMeldSnapshot,omitempty" json:"-"`
	Players                     []Player          `bson:"players" json:"players"`
	ActionLog                   []Action          `bson:"actionLog" json:"-"`
	NextMeldSeq                 int               `bson:"nextMeldSeq" json:"nextMeldSeq"`
	SuspendedAt                 *time.Time        `bson:"suspendedAt" json:"suspendedAt,omitempty"`
	AbandonAt                   *time.Time        `bson:"abandonAt" json:"abandonAt,omitempty"`
	PreSuspendPhase             string            `bson:"preSuspendPhase,omitempty" json:"-"`
	CreatedAt                   time.Time         `bson:"createdAt" json:"createdAt"`
	CompletedAt                 *time.Time        `bson:"completedAt" json:"completedAt,omitempty"`
	Version                     int64             `bson:"version" json:"-"`
}

type Player struct {
	ID           string `bson:"id" json:"id"`
	Name         string `bson:"name" json:"name"`
	IsAI         bool   `bson:"isAI" json:"isAI"`
	AIDifficulty string `bson:"aiDifficulty" json:"aiDifficulty,omitempty"`
	ConnectionID string `bson:"connectionId" json:"connectionId,omitempty"`
	UserID       string `bson:"userId" json:"userId,omitempty"`
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
}

// TurnMeldSnapshot mirrors rules.TurnMeldSnapshot for persistence — see there
// for what it's used for.
type TurnMeldSnapshot struct {
	PlayerID                    string                `bson:"playerId" json:"-"`
	Hands                       map[string][]string   `bson:"hands" json:"-"`
	Melds                       map[string][][]string `bson:"melds" json:"-"`
	MeldMeta                    map[string][]MeldInfo `bson:"meldMeta" json:"-"`
	RoundReqMet                 bool                  `bson:"roundReqMet" json:"-"`
	MeldsLaidThisTurn           int                   `bson:"meldsLaidThisTurn" json:"-"`
	DiscardDrawnCardPendingMeld string                `bson:"discardDrawnCardPendingMeld" json:"-"`
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
}

type User struct {
	ID           bson.ObjectID   `bson:"_id,omitempty" json:"id"`
	Username     string          `bson:"username" json:"username"`
	Email        string          `bson:"email,omitempty" json:"email,omitempty"`
	PasswordHash string          `bson:"passwordHash,omitempty" json:"-"`
	AuthProvider string          `bson:"authProvider" json:"authProvider"`
	CreatedAt    time.Time       `bson:"createdAt" json:"createdAt"`
	LastSeenAt   time.Time       `bson:"lastSeenAt" json:"lastSeenAt"`
	Preferences  UserPreferences `bson:"preferences" json:"preferences"`
}

type UserPreferences struct {
	Language  string `bson:"language" json:"language"`
	CardStyle string `bson:"cardStyle" json:"cardStyle"`
}

type Statistics struct {
	ID                   bson.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID               bson.ObjectID `bson:"userId" json:"userId"`
	GamesPlayed          int           `bson:"gamesPlayed" json:"gamesPlayed"`
	GamesWon             int           `bson:"gamesWon" json:"gamesWon"`
	RoundsPlayed         int           `bson:"roundsPlayed" json:"roundsPlayed"`
	RoundsWon            int           `bson:"roundsWon" json:"roundsWon"`
	TotalPenaltyScore    int           `bson:"totalPenaltyScore" json:"totalPenaltyScore"`
	AverageScorePerRound float64       `bson:"averageScorePerRound" json:"averageScorePerRound"`
	BestGame             int           `bson:"bestGame" json:"bestGame"`
	LongestWinStreak     int           `bson:"longestWinStreak" json:"longestWinStreak"`
	CurrentStreak        int           `bson:"currentStreak" json:"currentStreak"`
	GoOutCount           int           `bson:"goOutCount" json:"goOutCount"`
	MatchHistory         []MatchRef    `bson:"matchHistory" json:"matchHistory"`
}

type MatchRef struct {
	GameID    bson.ObjectID `bson:"gameId" json:"gameId"`
	Date      time.Time     `bson:"date" json:"date"`
	FinalRank int           `bson:"finalRank" json:"finalRank"`
	Score     int           `bson:"score" json:"score"`
	Players   int           `bson:"players" json:"players"`
}

type Session struct {
	ID        bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Token     string        `bson:"token" json:"token"`
	GuestName string        `bson:"guestName" json:"guestName"`
	UserID    string        `bson:"userId,omitempty" json:"userId,omitempty"`
	CreatedAt time.Time     `bson:"createdAt" json:"createdAt"`
	ExpiresAt time.Time     `bson:"expiresAt" json:"expiresAt"`
}
