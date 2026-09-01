package ginrummy

import (
	"testing"

	"zolik/server/internal/module"
)

// botAct asks the module's own bot what it would do, through exactly the path
// the runtime uses: the offer list from LegalActions and nothing else.
func botAct(t *testing.T, m *Module, raw module.State, seat module.BotSeat) module.Action {
	t.Helper()
	offers, err := m.LegalActions(raw, seat.PlayerID)
	if err != nil {
		t.Fatalf("LegalActions: %v", err)
	}
	a, ok := m.Bot().Act(raw, seat, offers)
	if !ok {
		t.Fatalf("bot had no move for %s among %d offers", seat.PlayerID, len(offers))
	}
	return a
}

func TestBot_TakesGinTheMomentItIsOffered(t *testing.T) {
	m := New()
	raw := withState(t, func(s *GameState) {
		s.Hands["p1"] = []string{
			"2H", "2D", "2C", "2S",
			"6H", "7H", "8H", "9H",
			"TH", "JH", "QH",
		}
	})
	a := botAct(t, m, raw, module.BotSeat{PlayerID: "p1", Skill: module.SkillEasy})
	if a.Verb != VerbKnock {
		t.Fatalf("expected the bot to knock, got %+v", a)
	}
	next, _, err := m.Apply(raw, "p1", a)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	s := stateOf(t, next)
	if len(s.Rounds) != 1 || s.Rounds[0].Kind != "gin" {
		t.Fatalf("expected the bot's move to gin, got %+v", s.Rounds)
	}
}

func TestBot_DiscardsTheirWorstCardWhenNoKnockIsAvailable(t *testing.T) {
	m := New()
	raw := withState(t, func(s *GameState) {
		s.Hands["p1"] = []string{
			"5H", "5D", "5C", // set — never discard from this
			"7S", "8S", "9S", // run — never discard from this
			"AH", "KC", "QD", "JS",
		}
		s.KnockLimit = 0 // deadwood here (1+10+10+10=31) can never knock
	})
	a := botAct(t, m, raw, module.BotSeat{PlayerID: "p1", Skill: module.SkillMedium})
	if a.Verb != VerbDiscard {
		t.Fatalf("expected a discard, got %+v", a)
	}
	if len(a.Cards) != 1 {
		t.Fatalf("expected exactly one discarded card, got %v", a.Cards)
	}
	// Discarding a meld card would be strictly worse than discarding an
	// isolated high card — the bot must never break a completed meld when
	// deadwood alone can be shed instead.
	for _, meldCard := range []string{"5H", "5D", "5C", "7S", "8S", "9S"} {
		if a.Cards[0] == meldCard {
			t.Errorf("bot discarded a meld card %s instead of loose deadwood", meldCard)
		}
	}
}

func TestBot_AlwaysLaysOffWhenItCan(t *testing.T) {
	m := New()
	raw := withState(t, func(s *GameState) {
		s.Phase = phaseLayoff
		s.Current = "p2"
		s.Knocker = "p1"
		s.KnockerMelds = []Meld{{ID: "m0", Kind: "set", Cards: []string{"5C", "5D", "5H"}}}
		s.Hands["p2"] = []string{"5S", "9C"}
	})
	a := botAct(t, m, raw, module.BotSeat{PlayerID: "p2", Skill: module.SkillMedium})
	if a.Verb != VerbLayOff {
		t.Fatalf("expected the bot to lay off, got %+v", a)
	}
}

// playSelfPlay plays a full match with both seats driven by the bot, at
// possibly different skills, and returns the winner.
func playSelfPlay(t *testing.T, seed int64, skillA, skillB module.Skill) string {
	t.Helper()
	m := New()
	cfg := module.MatchConfig{Options: module.Options{OptTargetScore: 100}}
	state, err := m.NewMatch(cfg, players(), seed)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}
	skills := map[string]module.Skill{"p1": skillA, "p2": skillB}

	for step := 0; step < 20000; step++ {
		done, winners, err := m.Finished(state)
		if err != nil {
			t.Fatalf("Finished: %v", err)
		}
		if done {
			if len(winners) != 1 {
				t.Fatalf("expected exactly one winner, got %v", winners)
			}
			return winners[0]
		}
		actor, offers, ok := activeOffers(t, m, state)
		if !ok {
			t.Fatalf("nobody has an offer, but the match is not finished")
		}
		seat := module.BotSeat{PlayerID: actor, Skill: skills[actor], Seed: module.SeatSeed(seed, actor, "bot")}
		a, ok := m.Bot().Act(state, seat, offers)
		if !ok {
			t.Fatalf("bot had no move for %s", actor)
		}
		next, _, err := m.Apply(state, actor, a)
		if err != nil {
			t.Fatalf("step %d: bot chose %+v but Apply refused: %v", step, a, err)
		}
		state = next
	}
	t.Fatal("self-play did not finish in time")
	return ""
}

// TestBot_StrengthRisesMonotonicallyAlongSkill is the strength gate: a harder
// skill must not win less often than an easier one, over enough seeded
// matches that a coincidence cannot pass as a result.
func TestBot_StrengthRisesMonotonicallyAlongSkill(t *testing.T) {
	const matches = 60
	wins := map[module.Skill]int{}
	for seed := int64(1); seed <= matches; seed++ {
		// Alternate which seat plays which skill so neither dealing order
		// nor going first is what wins it.
		var skillP1, skillP2 module.Skill
		if seed%2 == 0 {
			skillP1, skillP2 = module.SkillEasy, module.SkillHard
		} else {
			skillP1, skillP2 = module.SkillHard, module.SkillEasy
		}
		winner := playSelfPlay(t, seed, skillP1, skillP2)
		switch {
		case (winner == "p1" && skillP1 == module.SkillHard) || (winner == "p2" && skillP2 == module.SkillHard):
			wins[module.SkillHard]++
		default:
			wins[module.SkillEasy]++
		}
	}
	if wins[module.SkillHard] <= wins[module.SkillEasy] {
		t.Errorf("hard did not out-win easy over %d matches: hard=%d easy=%d",
			matches, wins[module.SkillHard], wins[module.SkillEasy])
	}
}
