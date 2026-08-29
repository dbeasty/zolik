package rules

import "time"

// Table/lobby constants — the single source of truth for values that used to
// be duplicated as literals across game/rest_handlers.go and game/manager.go.
const (
	MinPlayers = 2
	MaxPlayers = 8

	// AbandonWindow is how long a suspended game (e.g. every player
	// disconnected, or a claim was never made) stays claimable before it's
	// treated as abandoned.
	AbandonWindow = 24 * time.Hour
)

// The selectable values a lobby may set are declared by the module
// descriptor (see descriptor.go), which also carries their labels and is what
// clients render from. They deliberately do not exist as a second list here.

// IsDiscardLocked reports whether drawing from the discard pile is currently
// disallowed for the given table round under the game's configured
// discard-lock round (DiscardDrawMinRound: 0 or 1 means no restriction).
func IsDiscardLocked(round, discardDrawMinRound int) bool {
	return discardDrawMinRound > 1 && round < discardDrawMinRound
}

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

// DealStarterMode controls who leads the deal after the one just scored.
type DealStarterMode string

const (
	// DealStarterRotate: the seat after whoever led the last deal leads the
	// next one, all the way around the table regardless of who wins.
	DealStarterRotate DealStarterMode = "rotate"
	// DealStarterWinner: whoever went out leads the next deal — the
	// original behaviour, kept as a house-rule option rather than removed,
	// since a run of wins carrying the lead along with the score is exactly
	// what some tables want.
	DealStarterWinner DealStarterMode = "winner"
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
	// JokerReclaimMustPlay: a joker taken off the table this turn — swapped
	// out for the card it stands in for, or released by a lay-off that took
	// its exact place — must be played into a meld before the turn can end.
	// It cannot be kept in hand across the discard; the take can always be
	// undone instead (undo:lay_off right after a lay-off reclaim, undo:turn
	// always). The one exception mirrors JokerDiscardRestricted's: the
	// discard that empties an already-down hand ends the deal, and a deal
	// that is over holds no joker hostage.
	JokerReclaimMustPlay bool

	// FixedDealCount > 0 selects Continental's per-deal contract rotation
	// (ContractForDeal, deals 1..FixedDealCount); FixedDealCount == 0 means
	// every deal uses StaticContract instead (Žolík Classic: no rotation).
	FixedDealCount int
	StaticContract ContractRequirement

	// DealStarter decides who leads the deal after the one just scored.
	// Empty behaves as DealStarterRotate, so a RulesConfig built before this
	// field existed keeps meaning what it always meant to mean (whoever led
	// least recently, not whoever won).
	DealStarter DealStarterMode

	MatchEndMode MatchEndMode
	// PauseBetweenDeals stops the match after each deal is scored, instead of
	// dealing the next one in the same breath. False in every shipped profile:
	// it is set by the module adapter from a table option, so the engine's own
	// profiles — and every test built on them — behave exactly as before.
	PauseBetweenDeals bool
	// TargetScore is used when MatchEndMode == MatchEndAtScore.
	TargetScore int
}

// ContractFor returns the required combination for the given deal number.
//
// The clean-run rule is carried across from StaticContract even when a
// rotating profile supplies the sets/runs counts. It is a house rule about
// what "down" means, not part of any deal's combination, so it has to survive
// a profile whose contract changes from deal to deal — otherwise the knob
// that configures it (OptRequireCleanRun) would silently do nothing on
// Continental, which is worse than not offering it.
//
// StaticContract stays its home rather than a field of its own on
// RulesConfig: a resolved RulesConfig is persisted with every in-flight game,
// and moving the flag would have every stored Žolík Classic match come back
// with the zero value and quietly lose the rule mid-match.
func (c RulesConfig) ContractFor(dealNumber int) ContractRequirement {
	if c.FixedDealCount > 0 {
		req := continentalContractFor(dealNumber)
		req.RequireCleanRun = c.StaticContract.RequireCleanRun
		return req
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
