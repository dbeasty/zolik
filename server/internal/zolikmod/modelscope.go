package zolikmod

import (
	"encoding/json"

	"zolik/server/internal/module"
)

// The shipped Žolíky model plays Continental worse than the Hard heuristic,
// although Continental was one of the variations it trained on. On held-out
// seeds (gamebench, first seed 1,000,000) it lost 115 ± 16 penalty points a
// match to Hard at two seats, 67 ± 10 at four, and 51 ± 9 as the one model
// seat among three Hard. On the classic table it is ahead by 105-128.
//
// So at a Continental table an AI seat plays Hard (learn.Scoped). Lift this
// when a model passes the Continental bench.
func trainedTable(s module.State) bool {
	var t struct {
		Rules struct {
			Rules struct{ FixedDealCount int }
		} `json:"rules"`
	}
	if err := json.Unmarshal(s, &t); err != nil {
		return true
	}
	// FixedDealCount is what makes a table Continental (variationIndex).
	return t.Rules.Rules.FixedDealCount == 0
}
