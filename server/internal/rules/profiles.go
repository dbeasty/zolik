package rules

// ProfileContinental reproduces the game's original ruleset exactly:
// 12-card deal, 4+ card runs, a rotating 7-deal contract (RoundRequirementFor
// below), a 35-point initial-meld floor, discard-pickup locked until table
// round 3, and a joker can never be discarded except to go out.
var ProfileContinental = RulesConfig{
	Profile:                "continental",
	DealSize:               12,
	MinSetSize:             3,
	MinRunSize:             4,
	InitialMeldMinimum:     35,
	DiscardDrawMinRound:    3,
	DiscardPickupMode:      DiscardPickupTopOnly,
	JokerDiscardRestricted: true,
	FixedDealCount:         7,
	DealStarter:            DealStarterRotate,
	MatchEndMode:           MatchEndAfterDeals,
}

// ProfileZolikClassic follows the GameDesire "Žolík HD" ruleset: 13-card
// deal, 3+ card runs, no fixed per-deal contract or point-value floor, free
// pickup of any card from the discard pile, house rules requiring at least
// one joker-free ("clean") run before a player is down, a joker can never be
// discarded except to go out, and the match keeps re-dealing until someone
// crosses the target score.
var ProfileZolikClassic = RulesConfig{
	Profile:                "zolik_classic",
	DealSize:               13,
	MinSetSize:             3,
	MinRunSize:             3,
	InitialMeldMinimum:     0,
	DiscardDrawMinRound:    0,
	DiscardPickupMode:      DiscardPickupAnyFromPile,
	JokerDiscardRestricted: true,
	FixedDealCount:         0,
	StaticContract:         ContractRequirement{Sets: 0, Runs: 0, RequireCleanRun: true},
	DealStarter:            DealStarterRotate,
	MatchEndMode:           MatchEndAtScore,
	TargetScore:            200,
}

// ResolveProfile returns the named base profile, or ProfileZolikClassic for
// an unknown/empty name (the default ruleset).
func ResolveProfile(name string) RulesConfig {
	switch name {
	case "continental":
		return ProfileContinental
	case "zolik_classic", "":
		return ProfileZolikClassic
	default:
		return ProfileZolikClassic
	}
}

// continentalContractFor is the original RoundRequirementFor table, kept
// exactly as-is and only reachable through RulesConfig.ContractFor now.
func continentalContractFor(dealNumber int) ContractRequirement {
	switch dealNumber {
	case 1:
		return ContractRequirement{Sets: 2, Runs: 0}
	case 2:
		return ContractRequirement{Sets: 1, Runs: 1}
	case 3:
		return ContractRequirement{Sets: 0, Runs: 2}
	case 4:
		return ContractRequirement{Sets: 3, Runs: 0}
	case 5:
		return ContractRequirement{Sets: 2, Runs: 1}
	case 6:
		return ContractRequirement{Sets: 1, Runs: 2}
	case 7:
		return ContractRequirement{Sets: 0, Runs: 3}
	default:
		return ContractRequirement{}
	}
}
