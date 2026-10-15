package zolikmod_test

import (
	"encoding/json"
	"testing"

	"zolik/server/internal/module"
	"zolik/server/internal/zolikmod"
)

// One decoded state, read for every seat in any order, must answer exactly as
// the module does for each seat on its own - at every position of a played
// match, not just the deal. A projection that let a read for one viewer change
// the next viewer's answer would be a hidden-card leak.
func TestProjectionAnswersLikeTheModuleForEveryViewer(t *testing.T) {
	mod := zolikmod.New()
	players := []module.PlayerRef{{ID: "p1", Name: "p1"}, {ID: "p2", Name: "p2"}, {ID: "p3", Name: "p3"}}
	state, err := mod.NewMatch(module.MatchConfig{}, players, 42)
	if err != nil {
		t.Fatal(err)
	}
	bot := module.BotFor(mod)
	viewers := []string{"p2", "p3", "p1", "", "p1", "stranger"}

	check := func(step int) {
		proj, ok := module.ProjectionFor(mod, state)
		if !ok {
			t.Fatal("no projection")
		}
		for _, v := range viewers {
			gotV, errV := proj.View(v)
			wantV, wantErrV := mod.View(state, v)
			gotO, errO := proj.LegalActions(v)
			wantO, wantErrO := mod.LegalActions(state, v)
			if (errV == nil) != (wantErrV == nil) || (errO == nil) != (wantErrO == nil) {
				t.Fatalf("step %d viewer %q: errors differ", step, v)
			}
			if !sameJSON(gotV, wantV) {
				t.Fatalf("step %d viewer %q: view differs from the module's", step, v)
			}
			if !sameJSON(gotO, wantO) {
				t.Fatalf("step %d viewer %q: offers differ from the module's", step, v)
			}
		}
		if !sameJSON(proj.Standings(), module.StandingsFor(mod, state)) {
			t.Fatalf("step %d: standings differ", step)
		}
		if !sameJSON(proj.Rounds(), module.RoundsFor(mod, state)) {
			t.Fatalf("step %d: round log differs", step)
		}
		done, winners, err := mod.Finished(state)
		pdone, pwinners := proj.Finished()
		if err != nil || done != pdone || !sameJSON(winners, pwinners) {
			t.Fatalf("step %d: finished differs", step)
		}
	}

	for step := 0; step < 400; step++ {
		check(step)
		done, _, err := mod.Finished(state)
		if err != nil || done {
			return
		}
		actor := module.ActiveSeat(mod, state, players[0].ID, players)
		if actor == "" {
			return
		}
		offers, err := mod.LegalActions(state, actor)
		if err != nil {
			return
		}
		action, ok := bot.Act(state, module.BotSeat{PlayerID: actor}, offers)
		if !ok {
			if action, ok = module.ChooseAction(offers, nil); !ok {
				return
			}
		}
		next, _, err := mod.Apply(state, actor, action)
		if err != nil {
			fallback, ok := module.ChooseAction(offers, nil)
			if !ok {
				return
			}
			if next, _, err = mod.Apply(state, actor, fallback); err != nil {
				return
			}
		}
		state = next
	}
}

func sameJSON(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}
