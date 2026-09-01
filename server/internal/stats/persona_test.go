package stats

import (
	"testing"

	"zolik/server/internal/models"
)

// "Stable robots keep their scores" is the whole point of the persona, and
// these are the two halves of it: the record follows the opponent, and the
// difficulty-shaped questions still work on top of it.

func TestAPersonaCarriesItsOwnRecord(t *testing.T) {
	miroslav := models.Player{
		IsAI: true, Name: "Master Miroslav",
		AIDifficulty: "hard", AIPersona: "hard:miroslav",
	}
	ivan := models.Player{
		IsAI: true, Name: "Iron Ivan",
		AIDifficulty: "hard", AIPersona: "hard:ivan",
	}
	a, b := SubjectForPlayer(miroslav), SubjectForPlayer(ivan)
	if a.Key() == b.Key() {
		t.Fatalf("two hard bots share one record (%q): a persona has to be its own subject", a.Key())
	}
	if !a.Durable() {
		t.Error("a persona must accumulate a lifetime record")
	}
	// The same opponent, seated again in another lobby with a fresh player
	// id, is the same subject. That is the property a bot instance id could
	// never have, and the reason difficulty was the subject before.
	again := SubjectForPlayer(models.Player{
		IsAI: true, Name: "Master Miroslav",
		AIDifficulty: "hard", AIPersona: "hard:miroslav",
	})
	if again.Key() != a.Key() {
		t.Errorf("the same persona in a new lobby got a new key: %q vs %q", again.Key(), a.Key())
	}
}

// A persona key round-trips through the subject key, which is a BSON map key
// in the head-to-head record — so the extra colon must not confuse the parser.
func TestPersonaSurvivesTheSubjectKey(t *testing.T) {
	s := SubjectForPlayer(models.Player{IsAI: true, AIPersona: "hard:miroslav"})
	back, err := ParseSubjectKey(s.Key())
	if err != nil {
		t.Fatalf("ParseSubjectKey(%q): %v", s.Key(), err)
	}
	if back.Kind != SubjectAI || back.ID != "hard:miroslav" {
		t.Errorf("round-tripped to %+v", back)
	}
}

// "How do I do against hard?" is still a difficulty question, and a table of
// Miroslav and Ivan is one hard table rather than two.
func TestDifficultySplitsReadThroughThePersona(t *testing.T) {
	if got := SkillOfSubjectID("hard:miroslav"); got != "hard" {
		t.Errorf("SkillOfSubjectID = %q, want hard", got)
	}
	// And a record written before personas existed still answers.
	if got := SkillOfSubjectID("medium"); got != "medium" {
		t.Errorf("SkillOfSubjectID = %q, want medium", got)
	}

	opponents := []Standing{
		{Subject: Subject{Kind: SubjectAI, ID: "hard:miroslav"}},
		{Subject: Subject{Kind: SubjectAI, ID: "hard:ivan"}},
		{Subject: Subject{Kind: SubjectAI, ID: "easy:rita"}},
	}
	got := opponentAIDifficulties(opponents)
	if len(got) != 2 || got[0] != "easy" || got[1] != "hard" {
		t.Errorf("difficulties faced = %v, want [easy hard] — two hard bots are one hard table", got)
	}
}

// A bot seated before any of this existed keeps aggregating exactly where it
// always did.
func TestBotsWithoutAPersonaFallBackToDifficulty(t *testing.T) {
	s := SubjectForPlayer(models.Player{IsAI: true, AIDifficulty: "medium"})
	if s.ID != "medium" {
		t.Errorf("id = %q, want the bare difficulty", s.ID)
	}
	if s := SubjectForPlayer(models.Player{IsAI: true}); s.ID != "unspecified" {
		t.Errorf("id = %q, want unspecified rather than a silent easy", s.ID)
	}
}
