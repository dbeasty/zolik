package rules

// DiscardPickupMode controls which cards a player may take from the discard
// pile on their draw.
type DiscardPickupMode string

const (
	// DiscardPickupTopOnly: only the top (last) card of the pile may be taken.
	DiscardPickupTopOnly DiscardPickupMode = "top_only"
	// DiscardPickupAnyFromPile: any card in the pile may be taken, along with
	// every card stacked above it (GameDesire "Žolík": "starting from the
	// lowest card").
	DiscardPickupAnyFromPile DiscardPickupMode = "any_from_pile"
)

// MatchEndMode controls when a match (as opposed to a single deal) ends.
type MatchEndMode string

const (
	// MatchEndAfterDeals: the match ends once deal number FixedDealCount
	// finishes (Continental Rummy: 7 fixed deals, each with its own
	// required contract).
	MatchEndAfterDeals MatchEndMode = "after_deals"
	// MatchEndAtScore: the match ends as soon as any player's TotalScore
	// reaches TargetScore after a deal finishes; play then continues,
	// re-dealing, until that happens.
	MatchEndAtScore MatchEndMode = "at_score"
)

// ContractRequirement describes what a player must lay down before they are
// considered "down" (off the initial-meld hook) for the current deal.
type ContractRequirement struct {
	Sets int
	Runs int
	// RequireCleanRun: at least one of the player's laid runs must contain
	// zero wild cards (jokers/flex-aces) before they count as down, on top
	// of any Sets/Runs count above.
	RequireCleanRun bool
}

// RulesConfig bundles every tunable rule so the engine never hardcodes a
// specific game's constants. A GameState always carries a fully-resolved
// RulesConfig (see ResolveProfile) — never re-derived from a profile name
// mid-game, so a future edit to a named profile can't change an in-flight
// game's behavior.
type RulesConfig struct {
	Profile string // "continental" | "zolik_classic" | "custom" — informational only.

	DealSize   int
	MinSetSize int
	MinRunSize int

	// InitialMeldMinimum: 0 disables the natural-point-value floor on a
	// player's initial meld entirely.
	InitialMeldMinimum int
	// DiscardDrawMinRound: 0 or 1 means no restriction; N>1 means the
	// discard pile can't be drawn from until table Round N.
	DiscardDrawMinRound int

	DiscardPickupMode DiscardPickupMode
	// JokerDiscardRestricted: a joker can never be discarded, except as the
	// exact card that empties the player's hand while they're already down
	// (i.e. the discard that ends the deal for them).
	JokerDiscardRestricted bool

	// FixedDealCount > 0 selects Continental's per-deal contract rotation
	// (ContractForDeal, deals 1..FixedDealCount); FixedDealCount == 0 means
	// every deal uses StaticContract instead (Žolík Classic: no rotation).
	FixedDealCount int
	StaticContract ContractRequirement

	MatchEndMode MatchEndMode
	// TargetScore is used when MatchEndMode == MatchEndAtScore.
	TargetScore int
}

// ContractFor returns the required combination for the given deal number.
func (c RulesConfig) ContractFor(dealNumber int) ContractRequirement {
	if c.FixedDealCount > 0 {
		return continentalContractFor(dealNumber)
	}
	return c.StaticContract
}

// IsFinalDeal reports whether dealNumber is the last deal of a fixed-length
// match (Continental). Always false under MatchEndAtScore, since that match
// structure keeps dealing until someone crosses TargetScore.
func (c RulesConfig) IsFinalDeal(dealNumber int) bool {
	return c.MatchEndMode == MatchEndAfterDeals && dealNumber >= c.FixedDealCount
}

// defaultConfig is applied whenever a GameState shows up with a zero-value
// Rules field (e.g. constructed directly by older tests/callers) so nothing
// silently runs with MinRunSize=0 etc. Equivalent to ProfileContinental.
func defaultConfig() RulesConfig {
	return ProfileContinental
}

// effectiveRules returns state.Rules, or ProfileContinental if state.Rules
// is the zero value (MinRunSize == 0 is the tell — no real config ever sets
// it to 0, since even a run-based deal needs a minimum run length).
func effectiveRules(state GameState) RulesConfig {
	return ResolveConfig(state.Rules)
}

// ResolveConfig returns cfg, or ProfileContinental if cfg is the zero value.
// Exported for callers outside this package (e.g. the AI agents) that hold
// a RulesConfig without a full GameState to check it against.
func ResolveConfig(cfg RulesConfig) RulesConfig {
	if cfg.MinRunSize == 0 {
		return defaultConfig()
	}
	return cfg
}
