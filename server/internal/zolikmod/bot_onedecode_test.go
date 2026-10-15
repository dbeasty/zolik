package zolikmod

import (
	"fmt"
	"testing"

	"zolik/server/internal/ai"
	"zolik/server/internal/module"
)

// The bot decodes a state once, chooses on it and checks its choice on the
// same copy. Before, it decoded once to choose and again, inside Apply, to
// check. The move it makes must not change: this plays whole matches at every
// skill and compares the two at each position a bot is asked about.
func TestTheBotChoosesTheSameWithOneDecode(t *testing.T) {
	m := New()
	before := func(raw module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
		if a, ok := playAgent(raw, seat, ai.NewAgent(seat.Skill, seat.Seed)); ok {
			if _, _, err := m.Apply(raw, seat.PlayerID, a); err == nil {
				return a, true
			}
		}
		for _, o := range offers {
			if o.Undo {
				continue
			}
			if a, ok := module.SubmissionFor(o); ok {
				if _, _, err := m.Apply(raw, seat.PlayerID, a); err == nil {
					return a, true
				}
			}
		}
		return module.Action{}, false
	}
	players := []module.PlayerRef{{ID: "p1", Name: "p1"}, {ID: "p2", Name: "p2"}, {ID: "p3", Name: "p3"}}
	compared := 0
	for _, skill := range []module.Skill{module.SkillEasy, module.SkillMedium, module.SkillHard} {
		for seed := int64(1); seed <= 4; seed++ {
			raw, err := m.NewMatch(module.MatchConfig{}, players, seed)
			if err != nil {
				t.Fatal(err)
			}
			for step := 0; step < 600; step++ {
				if done, _, _ := m.Finished(raw); done {
					break
				}
				actor := module.ActiveSeat(m, raw, players[0].ID, players)
				if actor == "" {
					break
				}
				offers, _ := m.LegalActions(raw, actor)
				seat := module.BotSeat{PlayerID: actor, Skill: skill, Seed: seed}
				want, wantOK := before(raw, seat, offers)
				got, gotOK := heuristicBot{}.Act(raw, seat, offers)
				compared++
				if wantOK != gotOK || fmt.Sprint(want) != fmt.Sprint(got) {
					t.Fatalf("%s seed %d step %d: one decode chose %+v (%v), two chose %+v (%v)",
						skill, seed, step, got, gotOK, want, wantOK)
				}
				a := got
				if !gotOK {
					picks := module.ChooseActions(withoutUndos(offers), nil)
					if len(picks) == 0 {
						break
					}
					a = picks[0]
				}
				next, _, err := m.Apply(raw, actor, a)
				if err != nil {
					t.Fatalf("%s seed %d step %d: %+v refused: %v", skill, seed, step, a, err)
				}
				raw = next
			}
		}
	}
	if compared < 300 {
		t.Fatalf("only %d positions compared; the games ended too early to mean much", compared)
	}
}
