package canasta

import (
	"fmt"
	"testing"

	"zolik/server/internal/module"
)

// The runtime gives up on a seat that takes more than this many actions in a
// row without the turn moving on: match/bots.go's botMaxStall, restated here
// because this package cannot import match. A bot turn longer than it freezes
// a live table on a bot nobody may act for.
const runtimeMaxStall = 30

// The match that found it: Samba at six seats, Hard in every seat, seed 823.
// p1 captured a big pile and laid off the twenty-six cards it now owed its own
// melds one at a time — thirty-two actions in one turn, two past the point
// where the runtime stops asking. See layOffTogether.
func TestHardSambaTurnFitsTheRuntimeStallLimit(t *testing.T) {
	m := New()
	run, who, stall := longestTurn(m, "samba", 6, 823, module.SkillHard, 20000)
	if stall != "" {
		t.Fatalf("samba, 6 seats, hard, seed 823: %s", stall)
	}
	if run > runtimeMaxStall {
		t.Errorf("samba, 6 seats, hard, seed 823: %s took %d actions in one turn; the runtime gives up after %d",
			who, run, runtimeMaxStall)
	}
}

// The turn itself, built: a side with four groups down and a hand that fits
// them twenty-four times over. Every natural of a rank goes on in one action,
// so the whole turn is a handful of actions rather than one per card, and the
// cards that go down are the ones that went down before.
func TestBotLaysOffARankInOneAction(t *testing.T) {
	m := New()
	raw := twoHanded(func(s *GameState) {
		s.Variation = "samba"
		s.Phase = phaseMeld
		s.Teams[0].HasMelded = true
		s.Teams[0].Melds = []Meld{
			{ID: "t0-A", Rank: "A", Cards: []string{"AC", "AD", "AH"}},
			{ID: "t0-4", Rank: "4", Cards: []string{"4C", "4D", "4H"}},
			{ID: "t0-5", Rank: "5", Cards: []string{"5C", "5D", "5H"}},
			{ID: "t0-9", Rank: "9", Cards: []string{"9C", "9D", "9H"}},
		}
		s.Hands["p1"] = []string{
			"AS", "AS", "AC", "AD", "AH", "AH",
			"4S", "4S", "4C", "4D", "4H", "4H",
			"5S", "5S", "5C", "5D", "5H", "5H",
			"9S", "9S", "9C", "9D", "9H", "9H",
			"6C", "7D", "8H",
		}
	})
	before, _ := decode(raw)
	var played []module.Action
	for step := 0; step < 40; step++ {
		s, err := decode(raw)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if s.Current != "p1" {
			break
		}
		offers, err := m.LegalActions(raw, "p1")
		if err != nil {
			t.Fatalf("LegalActions: %v", err)
		}
		a, ok := m.Bot().Act(raw, module.BotSeat{PlayerID: "p1", Skill: module.SkillHard}, offers)
		if !ok {
			t.Fatalf("after %v the bot has no move", played)
		}
		next, _, err := m.Apply(raw, "p1", a)
		if err != nil {
			t.Fatalf("after %v the bot proposed an illegal %+v: %v", played, a, err)
		}
		played = append(played, a)
		raw = next
	}
	after, _ := decode(raw)
	if after.Current == "p1" {
		t.Fatalf("the turn never ended: %v", played)
	}
	if len(played) > 8 {
		t.Errorf("the turn took %d actions, want one lay-off per meld and a discard: %v", len(played), played)
	}
	laid := 0
	for i := range after.Teams[0].Melds {
		laid += len(after.Teams[0].Melds[i].Cards)
	}
	for i := range before.Teams[0].Melds {
		laid -= len(before.Teams[0].Melds[i].Cards)
	}
	if laid != 24 {
		t.Errorf("laid off %d cards, want all 24 that fit: %v", laid, played)
	}
}

// Hard self-play over every table shape a lobby deals, to the end of the
// match: no seat is ever left without a move, and no turn is longer than the
// runtime will wait for. A handful of seeds per shape under -short; the full
// sweep is the bench's job (cmd/gamebench), and this is its floor.
func TestHardSelfPlayNeverStallsAtAnyTableSize(t *testing.T) {
	seeds := int64(25)
	if testing.Short() {
		seeds = 3
	}
	m := New()
	shapes := []struct {
		variation string
		seats     int
	}{
		{"classic", 2}, {"classic", 3}, {"classic", 4},
		{"samba", 4}, {"samba", 6},
	}
	for _, sh := range shapes {
		sh := sh
		t.Run(fmt.Sprintf("%s/%d", sh.variation, sh.seats), func(t *testing.T) {
			t.Parallel()
			for seed := int64(1); seed <= seeds; seed++ {
				run, who, stall := longestTurn(m, sh.variation, sh.seats, seed, module.SkillHard, 20000)
				if stall != "" {
					t.Errorf("seed %d: %s", seed, stall)
					continue
				}
				if run > runtimeMaxStall {
					t.Errorf("seed %d: %s took %d actions in one turn; the runtime gives up after %d",
						seed, who, run, runtimeMaxStall)
				}
			}
		})
	}
}

// longestTurn is selfPlay that also measures the longest run of actions one
// seat made without the turn moving on — the number the runtime's stall
// counter watches.
func longestTurn(m *Module, variation string, seats int, seed int64, skill module.Skill, budget int) (longest int, who, stall string) {
	players := benchPlayers(seats)
	state, err := m.NewMatch(module.MatchConfig{Variation: variation}, players, seed)
	if err != nil {
		return 0, "", "NewMatch: " + err.Error()
	}
	last, run := "", 0
	for step := 0; step < budget; step++ {
		if done, _, err := m.Finished(state); err != nil {
			return longest, who, "Finished: " + err.Error()
		} else if done {
			return longest, who, ""
		}
		actor := module.ActiveSeat(m, state, players[0].ID, players)
		if actor == "" {
			return longest, who, "nobody on turn"
		}
		if actor == last {
			run++
		} else {
			last, run = actor, 1
		}
		if run > longest {
			longest, who = run, actor
		}
		offers, err := m.LegalActions(state, actor)
		if err != nil {
			return longest, who, "LegalActions: " + err.Error()
		}
		seat := module.BotSeat{PlayerID: actor, Skill: skill, Seed: module.SeatSeed(seed, actor, "bot")}
		a, ok := m.Bot().Act(state, seat, offers)
		if !ok {
			s, _ := decode(state)
			return longest, who, fmt.Sprintf("step %d: no move for %s: hand %v, offers %v", step, actor, s.Hands[actor], enabledVerbs(offers))
		}
		if a.Verb == VerbUndoTakePile || a.Verb == VerbUndoLayOff || a.Verb == VerbUndoLayMeld {
			return longest, who, fmt.Sprintf("step %d: %s took a move back: %+v", step, actor, a)
		}
		next, _, err := m.Apply(state, actor, a)
		if err != nil {
			return longest, who, fmt.Sprintf("step %d: %s proposed an illegal %+v: %v", step, actor, a, err)
		}
		state = next
	}
	return longest, who, "action budget"
}
