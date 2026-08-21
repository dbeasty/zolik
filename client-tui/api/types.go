package api

type GameState struct {
	Type   string `json:"type"`
	Status string `json:"status"`
	// Game is which deal of the match this is (1-7); drives the
	// initial-meld pattern (see roundRequirementLabel).
	Game int `json:"game"`
	// Round is laps around the table within the current deal; gates
	// DiscardDrawMinRound below.
	Round               int                   `json:"round"`
	Phase               string                `json:"phase"`
	CurrentTurn         string                `json:"currentTurn"`
	MyHand              []string              `json:"myHand"`
	DiscardPile         []string              `json:"discardPile"`
	DeckCount           int                   `json:"deckCount"`
	ReshuffleCount      int                   `json:"reshuffleCount"`
	CardCounts          map[string]int        `json:"cardCounts"`
	Melds               map[string][][]string `json:"melds"`
	MeldMeta            map[string][]MeldMeta `json:"meldMeta"`
	Players             []Player              `json:"players"`
	RoundReqMet         map[string]bool       `json:"roundReqMet"`
	TotalScores         map[string]int        `json:"totalScores"`
	WinnerID            string                `json:"winnerId"`
	IsDraw              bool                  `json:"isDraw"`
	InitialMeldMinimum  int                   `json:"initialMeldMinimum"`
	DiscardDrawMinRound int                   `json:"discardDrawMinRound"`
	// RulesProfile names the variation this game runs ("continental" |
	// "zolik_classic"). Display only — never switch behaviour on it; read
	// Rules and Contract below instead, so a profile this client has never
	// heard of still renders correctly.
	RulesProfile string `json:"rulesProfile"`

	// LegalActions is what this player may do right now, decided by the
	// server's ruleset. Read it through ui/offers.go rather than re-deriving
	// legality from the fields above — see docs/extensibility-plan.md Phase 1.
	LegalActions []ActionOffer `json:"legalActions"`
	// Rules is the game's fully-resolved ruleset.
	Rules ResolvedRules `json:"rules"`
	// Contract is what a player must lay down to go down on this deal. It
	// replaces the copy of the Continental contract table this client used
	// to carry in ui/helpers.go.
	Contract Contract `json:"contract"`
}

// ActionOffer mirrors the server's rules.ActionOffer.
type ActionOffer struct {
	ID      string `json:"id"`
	Verb    string `json:"verb"`
	Enabled bool   `json:"enabled"`
	// WhyNot is the engine's error code for a disabled offer — a stable
	// key, not a sentence, so the wording stays owned by the client.
	WhyNot string    `json:"whyNot"`
	Source *Selector `json:"source"`
	Target *Selector `json:"target"`
}

type Selector struct {
	Zone       string      `json:"zone"`
	OwnerID    string      `json:"ownerId"`
	MeldID     string      `json:"meldId"`
	Cards      []string    `json:"cards"`
	Placements []Placement `json:"placements"`
	MinCards   int         `json:"minCards"`
	MaxCards   int         `json:"maxCards"`
}

// Placement is one card an offer accepts, plus which end(s) of a run it may
// extend. No positions means "send no position hint".
type Placement struct {
	Card      string   `json:"card"`
	Positions []string `json:"positions"`
}

// ResolvedRules mirrors the server's RulesMsg — the ruleset this game
// actually runs under, rather than constants re-typed per profile name.
type ResolvedRules struct {
	Profile                string `json:"profile"`
	DealSize               int    `json:"dealSize"`
	MinSetSize             int    `json:"minSetSize"`
	MinRunSize             int    `json:"minRunSize"`
	InitialMeldMinimum     int    `json:"initialMeldMinimum"`
	DiscardDrawMinRound    int    `json:"discardDrawMinRound"`
	DiscardPickupMode      string `json:"discardPickupMode"`
	JokerDiscardRestricted bool   `json:"jokerDiscardRestricted"`
	FixedDealCount         int    `json:"fixedDealCount"`
	MatchEndMode           string `json:"matchEndMode"`
	TargetScore            int    `json:"targetScore"`
}

// Contract is the sets/runs/clean-run combination required to go down.
type Contract struct {
	Sets            int  `json:"sets"`
	Runs            int  `json:"runs"`
	RequireCleanRun bool `json:"requireCleanRun"`
}

type MeldMeta struct {
	MeldID string `json:"meldId"`
	Type   string `json:"type"`
}

type Player struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	IsAI         bool   `json:"isAI"`
	AIDifficulty string `json:"aiDifficulty,omitempty"`
}

type LobbyGame struct {
	ID             string        `json:"id"`
	Status         string        `json:"status"`
	Game           int           `json:"game"`
	Round          int           `json:"round"`
	Phase          string        `json:"phase"`
	CurrentTurn    string        `json:"currentTurn"`
	Players        []LobbyPlayer `json:"players"`
	DiscardPileTop any           `json:"discardPileTop"`
}

type LobbyPlayer struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	IsAI bool   `json:"isAI"`
}

type WSAction struct {
	Type   string   `json:"type"`
	From   string   `json:"from,omitempty"`
	Cards  []string `json:"cards,omitempty"`
	MeldID string   `json:"meldId,omitempty"`
	Card   string   `json:"card,omitempty"`
}

// MeldPreview is the server's answer to a preview_meld frame: what the
// currently selected cards would be if played. It replaces this client's own
// card-scoring table (the old approximateNaturalValue), so the number shown
// while choosing is computed by the same code that judges the submission —
// see server/internal/rules/preview.go.
type MeldPreview struct {
	Cards []string `json:"cards"`
	Valid bool     `json:"valid"`
	// Type is the meld kind ("set"/"run"). Keyed meldType, not type: the
	// envelope's own type field carries the frame name.
	Type           string `json:"meldType"`
	NaturalValue   int    `json:"naturalValue"`
	WildCount      int    `json:"wildCount"`
	WhyNot         string `json:"whyNot"`
	Playable       bool   `json:"playable"`
	WhyNotPlayable string `json:"whyNotPlayable"`

	InitialMeldMinimum int  `json:"initialMeldMinimum"`
	MeetsMinimum       bool `json:"meetsMinimum"`
}
