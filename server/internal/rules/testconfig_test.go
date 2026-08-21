package rules

// continentalNoFloor is ProfileContinental with the initial-meld point floor
// switched off, so a test can exercise the deal-by-deal sets/runs contract
// without every low-value meld also having to clear 35 points. Tests that
// care about the floor set Rules.InitialMeldMinimum explicitly.
func continentalNoFloor() RulesConfig {
	cfg := ProfileContinental
	cfg.InitialMeldMinimum = 0
	return cfg
}
