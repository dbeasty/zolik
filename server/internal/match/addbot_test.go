package match

import (
	"testing"

	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// Seating a bot is where the host's choice becomes an opponent, and it is the
// step this feature already got wrong once: every piece downstream of it —
// the difficulty on the seat, the agent's strength parameter, the statistics
// bucket keyed on it — existed and was correct while the adapter passed a
// hardcoded "medium", so easy and hard were unreachable from the product for
// as long as they had existed. These pin the decision itself.
//
// No database: personaFor reads the match it is handed and nothing else, which
// is what makes it testable at all.

func handlers() *Handlers { return NewHandlers(&Manager{registry: registry()}, false) }

func matchWith(options map[string]int, players ...models.Player) models.Match {
	return models.Match{ModuleID: "zolik", Seed: 4242, Options: options, Players: players}
}

func TestAddBotHonoursTheTablesSetting(t *testing.T) {
	h := handlers()
	m := matchWith(map[string]int{module.OptBotSkill: module.SkillOpt(module.SkillHard)})
	for i := 0; i < 4; i++ {
		p := h.personaFor(m, "")
		if p.Skill != module.SkillHard {
			t.Fatalf("seat %d drew %q at a hard table", i, p.Skill)
		}
		m.Players = append(m.Players, models.Player{IsAI: true, AIPersona: p.Key()})
	}
}

// An explicit skill on the request beats the table, which is what builds a
// deliberately mixed table one bot at a time from the table screen.
func TestAddBotHonoursAPerSeatOverride(t *testing.T) {
	h := handlers()
	m := matchWith(map[string]int{module.OptBotSkill: module.SkillOpt(module.SkillEasy)})
	if got := h.personaFor(m, "hard").Skill; got != module.SkillHard {
		t.Errorf("asked for hard at an easy table, got %q", got)
	}
}

// Mixed is the addition that makes a table interesting: the strength is drawn
// per seat, so the opponents differ from each other rather than from the last
// table.
func TestMixedTableSeatsDifferentStrengths(t *testing.T) {
	h := handlers()
	m := matchWith(map[string]int{module.OptBotSkill: module.BotSkillAuto})
	seen := map[module.Skill]bool{}
	names := map[string]bool{}
	for i := 0; i < 9; i++ {
		p := h.personaFor(m, "")
		seen[p.Skill] = true
		if names[p.Key()] {
			t.Fatalf("%s was seated twice", p.Name)
		}
		names[p.Key()] = true
		m.Players = append(m.Players, models.Player{IsAI: true, AIPersona: p.Key()})
	}
	if len(seen) < 2 {
		t.Errorf("nine seats all drew %v — that is not a mixed table", seen)
	}
}

// The same match seeded the same way seats the same opponents. Not a
// cosmetic property: a bot loop that restarts after a reconnect re-derives
// every seat's strength and seed, and a table that drew differently the second
// time would swap the opponent out mid-deal.
func TestSeatingIsReproducible(t *testing.T) {
	h := handlers()
	m := matchWith(map[string]int{module.OptBotSkill: module.BotSkillAuto})
	first := h.personaFor(m, "")
	if again := h.personaFor(m, ""); again != first {
		t.Errorf("the same table seated %v then %v", first, again)
	}
}

// A table created before the option existed carries no botSkill at all, and
// must go on playing the opponent it always did.
func TestATableWithNoSettingPlaysAtMedium(t *testing.T) {
	h := handlers()
	if got := h.personaFor(matchWith(nil), "").Skill; got != module.SkillMedium {
		t.Errorf("a table with no setting drew %q, want medium", got)
	}
}

// A stale client naming a strength this build has never heard of must not seat
// something arbitrary — it falls back to the table's own setting.
func TestAnUnknownSkillFallsBackToTheTable(t *testing.T) {
	h := handlers()
	m := matchWith(map[string]int{module.OptBotSkill: module.SkillOpt(module.SkillEasy)})
	if got := h.personaFor(m, "grandmaster").Skill; got != module.SkillEasy {
		t.Errorf("an unknown skill drew %q; want the table's own easy", got)
	}
}

// And the loop that drives the seat has to read back what seating wrote.
func TestBotSeatCarriesTheSeatedStrength(t *testing.T) {
	m := matchWith(nil,
		models.Player{ID: "bot:1", IsAI: true, AIDifficulty: "hard", AIPersona: "hard:miroslav"},
		models.Player{ID: "bot:2", IsAI: true}, // seated before strengths existed
	)
	if got := botSeatFor(m, "bot:1").Skill; got != module.SkillHard {
		t.Errorf("seat skill = %q, want hard", got)
	}
	// Empty means "the module's own default", which the agent resolves to
	// Medium — never to the weakest setting, which would silently weaken every
	// table already in the database.
	if got := botSeatFor(m, "bot:2").Skill; got != "" {
		t.Errorf("an unrecorded strength became %q; it must stay empty for the bot to default", got)
	}
	// Same seat, same seed, every time.
	if botSeatFor(m, "bot:1").Seed != botSeatFor(m, "bot:1").Seed {
		t.Error("a seat's seed is not stable")
	}
	if botSeatFor(m, "bot:1").Seed == botSeatFor(m, "bot:2").Seed {
		t.Error("two seats share one seed — their mistakes would be identical")
	}
}

// Every module offers the control, because a client renders it off the
// descriptor and would otherwise show nothing for a game that has bots.
func TestEveryModuleOffersTheOption(t *testing.T) {
	for _, d := range registry().Descriptors() {
		spec := d.Option(module.OptBotSkill)
		if spec == nil {
			t.Errorf("%s does not offer %s", d.ID, module.OptBotSkill)
			continue
		}
		if !spec.Allows(module.BotSkillAuto) {
			t.Errorf("%s does not offer a mixed table", d.ID)
		}
		for _, s := range module.Skills {
			if !spec.Allows(module.SkillOpt(s)) {
				t.Errorf("%s does not offer %s", d.ID, s)
			}
		}
	}
}
