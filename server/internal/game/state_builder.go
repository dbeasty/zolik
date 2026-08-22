package game

import (
	"zolik/server/internal/models"
	"zolik/server/internal/rules"
)

type PlayerMsg struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	IsAI         bool   `json:"isAI"`
	AIDifficulty string `json:"aiDifficulty,omitempty"`
}

type MeldMetaMsg struct {
	MeldID string `json:"meldId"`
	Type   string `json:"type"`
}

type GameStateMsg struct {
	Type                        string                   `json:"type"`
	Status                      string                   `json:"status"`
	Game                        int                      `json:"game"`
	Round                       int                      `json:"round"`
	Phase                       string                   `json:"phase"`
	CurrentTurn                 string                   `json:"currentTurn"`
	MyHand                      []string                 `json:"myHand"`
	DiscardPile                 []string                 `json:"discardPile"`
	DeckCount                   int                      `json:"deckCount"`
	ReshuffleCount              int                      `json:"reshuffleCount"`
	CardCounts                  map[string]int           `json:"cardCounts"`
	Melds                       map[string][][]string    `json:"melds"`
	MeldMeta                    map[string][]MeldMetaMsg `json:"meldMeta"`
	Players                     []PlayerMsg              `json:"players"`
	RoundReqMet                 map[string]bool          `json:"roundReqMet"`
	TotalScores                 map[string]int           `json:"totalScores"`
	WinnerID                    string                   `json:"winnerId,omitempty"`
	IsDraw                      bool                     `json:"isDraw,omitempty"`
	InitialMeldMinimum          int                      `json:"initialMeldMinimum"`
	DiscardDrawMinRound         int                      `json:"discardDrawMinRound"`
	DiscardLocked               bool                     `json:"discardLocked"`
	DiscardDrawnCardPendingMeld string                   `json:"discardDrawnCardPendingMeld,omitempty"`
	RulesProfile                string                   `json:"rulesProfile"`

	// LegalActions is what this viewer may do right now, computed by the
	// ruleset itself (rules.LegalActions). It is the reason a client no
	// longer has to re-derive legality from the raw facts above — see
	// docs/extensibility-plan.md Phase 1. Always the complete offer set,
	// disabled entries included, each with the engine's own reason.
	LegalActions []rules.ActionOffer `json:"legalActions"`
	// Rules is the game's fully-resolved ruleset. Shipping it means a client
	// reads "minimum run length" instead of switching on the profile *name*
	// and re-typing each profile's constants — so a third profile needs no
	// client change at all.
	Rules RulesMsg `json:"rules"`
	// Contract is what this player must lay down to go down, resolved for
	// *this* deal. Replaces the deal-number-to-contract tables both clients
	// carry copies of.
	Contract ContractMsg `json:"contract"`

	// Deprecated: derived from LegalActions below and kept only so a client
	// build predating the offer list keeps working. New client code must read
	// the offers — these will be removed once no such client is deployed.
	CanUndoDiscardDraw bool `json:"canUndoDiscardDraw,omitempty"`
	CanUndoLayOff      bool `json:"canUndoLayOff,omitempty"`
	CanUndoLayMeld     bool `json:"canUndoLayMeld,omitempty"`
	CanUndoTurn        bool `json:"canUndoTurn,omitempty"`
}

// RulesMsg is the wire form of the game's resolved ruleset. Every field is
// public information (it is printed in the in-game rules panel), so there is
// nothing to filter per viewer.
type RulesMsg struct {
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

// ContractMsg is the sets/runs/clean-run combination required to go down on
// the current deal.
type ContractMsg struct {
	Sets            int  `json:"sets"`
	Runs            int  `json:"runs"`
	RequireCleanRun bool `json:"requireCleanRun"`
}

func buildRulesMsg(cfg rules.RulesConfig) RulesMsg {
	cfg = rules.ResolveConfig(cfg)
	return RulesMsg{
		Profile:                cfg.Profile,
		DealSize:               cfg.DealSize,
		MinSetSize:             cfg.MinSetSize,
		MinRunSize:             cfg.MinRunSize,
		InitialMeldMinimum:     cfg.InitialMeldMinimum,
		DiscardDrawMinRound:    cfg.DiscardDrawMinRound,
		DiscardPickupMode:      string(cfg.DiscardPickupMode),
		JokerDiscardRestricted: cfg.JokerDiscardRestricted,
		FixedDealCount:         cfg.FixedDealCount,
		MatchEndMode:           string(cfg.MatchEndMode),
		TargetScore:            cfg.TargetScore,
	}
}

// ModuleDescriptorMsg is the wire form of rules.Descriptor() — what this game
// is, who may play it, which variations it ships, and what a lobby may
// configure. A client renders its whole new-game form from this, so adding a
// rule knob or a third variation becomes a server-only change.
type ModuleDescriptorMsg struct {
	ID         string             `json:"id"`
	Label      string             `json:"label"`
	MinPlayers int                `json:"minPlayers"`
	MaxPlayers int                `json:"maxPlayers"`
	Profiles   []ProfileSpecMsg   `json:"profiles"`
	Options    []rules.OptionSpec `json:"options"`
}

// ProfileSpecMsg carries each variation's fully-resolved ruleset alongside its
// name, so a lobby can describe a profile it has never heard of using the same
// summary renderer the in-game rules panel already uses.
type ProfileSpecMsg struct {
	ID    string   `json:"id"`
	Label string   `json:"label"`
	Rules RulesMsg `json:"rules"`
	// Contract is what this profile asks for on its first deal — enough for a
	// lobby to say "to go down: two sets" without owning the rotation table.
	Contract ContractMsg `json:"contract"`
}

// BuildModuleDescriptorMsg maps the module's self-description onto the wire.
func BuildModuleDescriptorMsg() ModuleDescriptorMsg {
	d := rules.Descriptor()
	profiles := make([]ProfileSpecMsg, 0, len(d.Profiles))
	for _, p := range d.Profiles {
		c := p.Contract()
		profiles = append(profiles, ProfileSpecMsg{
			ID: p.ID, Label: p.Label, Rules: buildRulesMsg(p.Rules),
			Contract: ContractMsg{Sets: c.Sets, Runs: c.Runs, RequireCleanRun: c.RequireCleanRun},
		})
	}
	return ModuleDescriptorMsg{
		ID:         d.ID,
		Label:      d.Label,
		MinPlayers: d.MinPlayers,
		MaxPlayers: d.MaxPlayers,
		Profiles:   profiles,
		Options:    d.Options,
	}
}

func BuildGameStateMsg(g models.Game, myPlayerID string) GameStateMsg {
	cardCounts := map[string]int{}
	for _, p := range g.Players {
		if p.ID == myPlayerID {
			continue
		}
		cardCounts[p.ID] = len(g.Hands[p.ID])
	}

	var myHand []string
	if g.Hands != nil {
		myHand = g.Hands[myPlayerID]
	}

	var pendingMeldCard string
	if g.CurrentTurn == myPlayerID {
		pendingMeldCard = g.DiscardDrawnCardPendingMeld
	}

	// The offer list is the single source for "what may this viewer do". The
	// four legacy canUndo* booleans below are *read back out of it* rather
	// than recomputed here — they were three separate patches for the missing
	// offer list, and deriving them keeps one implementation while a client
	// build predating the offers is still deployed.
	offers := rules.LegalActions(toRulesState(g), myPlayerID)
	offerEnabled := func(id string) bool {
		o := rules.FindOffer(offers, id)
		return o != nil && o.Enabled
	}
	offerWhyNot := func(id string) rules.RulesErrorCode {
		o := rules.FindOffer(offers, id)
		if o == nil || o.Enabled {
			return ""
		}
		return o.WhyNot
	}

	players := make([]PlayerMsg, 0, len(g.Players))
	for _, p := range g.Players {
		players = append(players, PlayerMsg{
			ID:           p.ID,
			Name:         p.Name,
			IsAI:         p.IsAI,
			AIDifficulty: p.AIDifficulty,
		})
	}

	meldMeta := map[string][]MeldMetaMsg{}
	for owner, metas := range g.MeldMeta {
		out := make([]MeldMetaMsg, 0, len(metas))
		for _, mi := range metas {
			out = append(out, MeldMetaMsg{MeldID: mi.MeldID, Type: mi.Type})
		}
		meldMeta[owner] = out
	}

	phaseStr := g.Phase
	// Rule values on the wire come from the game's resolved ruleset, not the
	// legacy scalar columns, so a document written before the ruleset was
	// persisted still reports the same numbers the engine is enforcing.
	cfg := GameRules(g)
	contract := cfg.ContractFor(g.GameNumber)
	return GameStateMsg{
		Type:                "game_state",
		Status:              g.Status,
		Game:                g.GameNumber,
		Round:               g.Round,
		Phase:               phaseStr,
		CurrentTurn:         g.CurrentTurn,
		MyHand:              myHand,
		DiscardPile:         g.DiscardPile,
		DeckCount:           len(g.DrawPile),
		ReshuffleCount:      g.ReshuffleCount,
		CardCounts:          cardCounts,
		Melds:               g.Melds,
		MeldMeta:            meldMeta,
		Players:             players,
		RoundReqMet:         g.RoundReqMet,
		TotalScores:         g.TotalScores,
		WinnerID:            g.WinnerID,
		IsDraw:              g.IsDraw,
		InitialMeldMinimum:  cfg.InitialMeldMinimum,
		DiscardDrawMinRound: cfg.DiscardDrawMinRound,
		// Read back out of the offer list rather than recomputed, exactly like
		// the canUndo* flags below: it is the same fact draw:discard already
		// carries, kept as a second spelling for clients that predate offers.
		DiscardLocked:               offerWhyNot(rules.OfferDrawDiscard) == rules.ErrDiscardLocked,
		DiscardDrawnCardPendingMeld: pendingMeldCard,
		RulesProfile:                g.RulesProfile,

		LegalActions: offers,
		Rules:        buildRulesMsg(cfg),
		Contract: ContractMsg{
			Sets:            contract.Sets,
			Runs:            contract.Runs,
			RequireCleanRun: contract.RequireCleanRun,
		},

		CanUndoDiscardDraw: offerEnabled(rules.OfferUndoDrawDiscard),
		CanUndoLayOff:      offerEnabled(rules.OfferUndoLayOff),
		CanUndoLayMeld:     offerEnabled(rules.OfferUndoLayMeld),
		CanUndoTurn:        offerEnabled(rules.OfferUndoTurn),
	}
}
