package ai

import "zolik/server/internal/rules"

// continentalNoFloor is ProfileContinental with the initial-meld point floor
// switched off, so a test can exercise the deal-by-deal sets/runs contract
// without every low-value meld also having to clear 35 points.
func continentalNoFloor() rules.RulesConfig {
	cfg := rules.ProfileContinental
	cfg.InitialMeldMinimum = 0
	return cfg
}

// openContinental is continentalNoFloor with the discard-pickup round gate
// also lifted, for tests where only the draw-source choice is under test.
func openContinental() rules.RulesConfig {
	cfg := continentalNoFloor()
	cfg.DiscardDrawMinRound = 0
	return cfg
}
