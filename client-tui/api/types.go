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
	// "zolik_classic"). Needed to label the deal correctly: only Continental
	// has a fixed seven-deal match with a per-deal contract.
	RulesProfile string `json:"rulesProfile"`
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
