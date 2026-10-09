package lastcard

import (
	"testing"

	"zolik/server/internal/module"
)

// Lowest wins: everyone but the deal's winner is charged their own hand, the
// winner nothing, and the score sheet says what each was charged for.
func TestLowestChargesEachHandToItsOwner(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Scoring = scoreLowest
		s.TargetScore = 500
		s.Scores = map[string]int{"p1": 0, "p2": 0, "p3": 0}
		s.Hands["p1"] = []string{"T-9"}
		s.Hands["p2"] = []string{"C-8", "V-S"}            // 8 + 20
		s.Hands["p3"] = []string{cardWildDrawFour, "A-3"} // 50 + 3
	})
	raw = apply(t, raw, "p1", playCard("T-9"))
	s := stateOf(t, raw)
	if s.Scores["p1"] != 0 || s.Scores["p2"] != 28 || s.Scores["p3"] != 53 {
		t.Errorf("scores %v, want p1 0, p2 28, p3 53", s.Scores)
	}
	log, _ := New().Rounds(raw)
	for _, rs := range log.Rounds[0].Scores {
		if err := module.CheckLines(rs); err != nil {
			t.Error(err)
		}
		if rs.PlayerID == "p3" && rs.Delta != 53 {
			t.Errorf("p3's delta %d, want 53", rs.Delta)
		}
	}
}

// When a total reaches the target the match ends — and the lowest total
// wins it, not the player who went out.
func TestLowestTotalWinsWhenSomeoneReachesTheTarget(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Scoring = scoreLowest
		s.TargetScore = 200
		s.Scores = map[string]int{"p1": 150, "p2": 190, "p3": 40}
		s.Hands["p1"] = []string{"T-9"}
		s.Hands["p2"] = []string{"C-8", "V-S"}
		s.Hands["p3"] = []string{"A-3"}
	})
	raw = apply(t, raw, "p1", playCard("T-9"))
	done, winners, _ := New().Finished(raw)
	if !done || len(winners) != 1 || winners[0] != "p3" {
		t.Fatalf("finished %v winners %v, want p3 with the lowest total", done, winners)
	}
	st, _ := New().Standings(raw)
	if st[0].PlayerID != "p3" || st[len(st)-1].PlayerID != "p2" || *st[0].Shown != 43 {
		t.Errorf("standings %v, want p3 (43) first and p2 last", st)
	}
}

func TestTheRulesSayWhichScoringTheTablePlays(t *testing.T) {
	lowest := module.StatedRuleIDs(New(), module.MatchConfig{Options: module.Options{OptScoring: scoreLowest}})
	if !lowest["lastcard.rules.scoring.lowest"] || lowest["lastcard.rules.scoring"] {
		t.Errorf("a lowest-wins table states %v", lowest)
	}
	winner := module.StatedRuleIDs(New(), module.MatchConfig{})
	if !winner["lastcard.rules.scoring"] || winner["lastcard.rules.scoring.lowest"] {
		t.Error("the default table does not state winner-takes scoring")
	}
}

func TestBotsPlayAWholeLowestWinsMatch(t *testing.T) {
	m := New()
	ps := players("p1", "p2", "p3")
	cfg := module.MatchConfig{Options: module.Options{OptScoring: scoreLowest, OptTargetScore: 200}}
	skills := map[string]module.Skill{"p1": module.SkillEasy, "p2": module.SkillMedium, "p3": module.SkillHard}
	for seed := int64(1); seed <= 5; seed++ {
		state, _ := m.NewMatch(cfg, ps, seed)
		for step := 0; ; step++ {
			if step > 40000 {
				t.Fatalf("seed %d: no finish", seed)
			}
			if done, _, _ := m.Finished(state); done {
				break
			}
			s, _ := decode(state)
			actor := s.Current
			if s.Intermission.Open {
				actor = s.Intermission.Waiting(s.TurnOrder)[0]
			}
			offers, _ := m.LegalActions(state, actor)
			a, _ := m.Bot().Act(state, module.BotSeat{PlayerID: actor, Skill: skills[actor]}, offers)
			next, _, err := m.Apply(state, actor, a)
			if err != nil {
				t.Fatalf("seed %d: %v", seed, err)
			}
			state = next
		}
		s, _ := decode(state)
		for _, p := range s.TurnOrder {
			if s.Scores[p] < s.Scores[s.WinnerID] {
				t.Errorf("seed %d: %s won on %d but %s had %d", seed, s.WinnerID, s.Scores[s.WinnerID], p, s.Scores[p])
			}
		}
	}
}
