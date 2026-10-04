package learn

import (
	"errors"
	"os"
	"testing"

	"zolik/server/internal/learn/models"
	"zolik/server/internal/module"
)

// shipForTest puts a model in the switch table as if the binary had shipped
// it, for the length of one test.
func shipForTest(t *testing.T, n *Net) {
	t.Helper()
	b, err := n.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	prev, had := hardModels["nim"]
	hardModels["nim"] = newHardModel("nim", b, models.Info{Game: "nim"})
	t.Cleanup(func() {
		if had {
			hardModels["nim"] = prev
		} else {
			delete(hardModels, "nim")
		}
	})
}

// nimMoves is what each skill plays at pile 13: the heuristic (perfect) takes
// one, the shipped net (preferThird) takes three.
func nimMoves(t *testing.T, bot module.Bot) map[module.Skill]string {
	t.Helper()
	s, _ := nim{}.NewMatch(module.MatchConfig{}, Players(2), 3)
	offers, _ := nim{}.LegalActions(s, "p0")
	out := map[module.Skill]string{}
	for _, skill := range []module.Skill{module.SkillEasy, module.SkillMedium, module.SkillHard} {
		a, _ := bot.Act(s, module.BotSeat{PlayerID: "p0", Skill: skill}, offers)
		out[skill] = a.OfferID
	}
	return out
}

func TestHardModelSwitch(t *testing.T) {
	t.Setenv("ZOLIK_LEARNED_MODEL_NIM", "")
	shipForTest(t, preferThird())

	// Off, the default: the heuristic, unchanged.
	if _, ok := HardModel(nimGame{}, perfect{}).(perfect); !ok {
		t.Fatal("with the switch off, HardModel changed the bot")
	}

	was, err := SetHardModel("nim", true)
	if err != nil || was {
		t.Fatalf("enabling: was=%v err=%v", was, err)
	}
	got := nimMoves(t, HardModel(nimGame{}, perfect{}))
	want := map[module.Skill]string{module.SkillEasy: "take1", module.SkillMedium: "take1", module.SkillHard: "take3"}
	for skill := range want {
		if got[skill] != want[skill] {
			t.Errorf("on: %s seat played %s, want %s", skill, got[skill], want[skill])
		}
	}

	// Hints (match.Manager.Hint) unwrap to the heuristic through this seam,
	// so a player's hint does not change with the switch.
	layered, ok := HardModel(nimGame{}, perfect{}).(interface{ Heuristic() module.Bot })
	if !ok {
		t.Fatal("the model bot does not expose its heuristic for hints")
	}
	if _, ok := layered.Heuristic().(perfect); !ok {
		t.Errorf("Heuristic() is %T, want the module's heuristic", layered.Heuristic())
	}

	// Per move, with the switch on: nothing parsed, nothing allocated.
	if n := testing.AllocsPerRun(100, func() { _ = HardModel(nimGame{}, perfect{}) }); n != 0 {
		t.Errorf("HardModel allocates %v times per call with the switch on", n)
	}

	// And off again takes effect at the very next call.
	if was, _ := SetHardModel("nim", false); !was {
		t.Error("disabling reported the switch was already off")
	}
	if _, ok := HardModel(nimGame{}, perfect{}).(perfect); !ok {
		t.Error("with the switch turned back off, HardModel still seats the model")
	}
}

func TestHardModelEnvironmentWins(t *testing.T) {
	shipForTest(t, preferThird())
	b, _ := preferThird().Marshal()
	fits := t.TempDir() + "/nim.bin"
	if err := os.WriteFile(fits, b, 0o644); err != nil {
		t.Fatal(err)
	}

	// Switch off, environment names a model: the environment's model plays.
	t.Setenv("ZOLIK_LEARNED_MODEL_NIM", fits)
	if got := nimMoves(t, HardModel(nimGame{}, perfect{})); got[module.SkillHard] != "take3" {
		t.Errorf("with the environment naming a model and the switch off, Hard played %s", got[module.SkillHard])
	}
	if s := statusOf(t, "nim"); s.EnvOverride != fits {
		t.Errorf("status envOverride = %q, want %q", s.EnvOverride, fits)
	}

	// Switch on, environment names a file that is not a model: still the
	// environment's say — which is the heuristic — not the shipped model.
	if _, err := SetHardModel("nim", true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = SetHardModel("nim", false) })
	t.Setenv("ZOLIK_LEARNED_MODEL_NIM", t.TempDir()+"/missing.bin")
	if got := nimMoves(t, HardModel(nimGame{}, perfect{})); got[module.SkillHard] != "take1" {
		t.Errorf("the switch overrode the environment: Hard played %s", got[module.SkillHard])
	}
}

func TestHardModelRefusesAModelThatDoesNotFit(t *testing.T) {
	t.Setenv("ZOLIK_LEARNED_MODEL_NIM", "")
	wrong := preferThird()
	wrong.CandDim = 4
	wrong.Scorer = []Dense{dense(5, 1, []float32{0, 0, 0, 1, 0}, []float32{0})}
	shipForTest(t, wrong)

	if _, err := SetHardModel("nim", true); !errors.Is(err, ErrModelDoesNotFit) {
		t.Fatalf("enabling a model for another encoder: %v", err)
	}
	s := statusOf(t, "nim")
	if s.Enabled || s.Fits || s.Problem == "" || !s.Embedded {
		t.Errorf("status after refusal: %+v", s)
	}
	if _, ok := HardModel(nimGame{}, perfect{}).(perfect); !ok {
		t.Error("a refused model is playing")
	}
	// Turning it off is always allowed.
	if _, err := SetHardModel("nim", false); err != nil {
		t.Errorf("disabling: %v", err)
	}
}

func TestHardModelUnknownGame(t *testing.T) {
	if _, err := SetHardModel("no-such-game", true); !errors.Is(err, ErrUnknownGame) {
		t.Errorf("got %v, want ErrUnknownGame", err)
	}
}

// Every game this binary ships a model for is off until an operator says so.
func TestShippedModelsDefaultOff(t *testing.T) {
	for _, name := range models.Games() {
		m := hardModels[name]
		if m == nil {
			t.Errorf("%s: shipped but not in the switch table", name)
			continue
		}
		if m.enabled.Load() {
			t.Errorf("%s: on by default", name)
		}
	}
}

func statusOf(t *testing.T, game string) HardModelStatus {
	t.Helper()
	for _, s := range HardModels() {
		if s.Game == game {
			return s
		}
	}
	t.Fatalf("no status for %s", game)
	return HardModelStatus{}
}
