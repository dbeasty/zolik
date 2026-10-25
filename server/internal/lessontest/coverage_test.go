package lessontest_test

import (
	"testing"

	"zolik/server/internal/blackjack"
	"zolik/server/internal/canasta"
	"zolik/server/internal/ferbl"
	"zolik/server/internal/ginrummy"
	"zolik/server/internal/holdem"
	"zolik/server/internal/klondike"
	"zolik/server/internal/lastcard"
	"zolik/server/internal/lessontest"
	"zolik/server/internal/lora"
	"zolik/server/internal/marias"
	"zolik/server/internal/module"
	"zolik/server/internal/okobere"
	"zolik/server/internal/prsi"
	"zolik/server/internal/rummytiles"
	"zolik/server/internal/sedma"
	"zolik/server/internal/sietemedio"
	"zolik/server/internal/snaps"
	"zolik/server/internal/zolikmod"
)

// Every game the server hosts ships a guided lesson, at every table it can be
// configured for. A new module that arrives without one fails here, not on a
// player's screen — add it to this list and write its lessons.go.
func TestEveryGameShipsLessons(t *testing.T) {
	mods := []module.GameModule{
		zolikmod.New(), prsi.New(), canasta.New(), holdem.New(), ginrummy.New(), rummytiles.New(),
		blackjack.New(), marias.New(), klondike.New(), lastcard.New(), snaps.New(), sedma.New(),
		okobere.New(), ferbl.New(), lora.New(), sietemedio.New(),
	}
	for _, m := range mods {
		id := m.Descriptor().ID
		for _, cfg := range lessontest.Configs(m) {
			ls, ok, err := module.LessonsFor(m, cfg)
			if err != nil {
				t.Errorf("%s %s: %v", id, lessontest.Describe(cfg), err)
				continue
			}
			if !ok || len(ls) == 0 {
				t.Errorf("%s %s: no guided lesson", id, lessontest.Describe(cfg))
			}
		}
	}
}
