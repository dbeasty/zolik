package ginrummy

import (
	"reflect"
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

// TestBotDoesNotPeek.
//
// The bot is handed the whole state — the opponent's hand and the stock in the
// order it will be drawn — because that is what the runtime has. Nothing but
// this test stops it reading them, and the hint button shows a human whatever
// the bot would do, so a bot that peeked would leak the opponent's hand to
// them too.
//
// Every decision the bot makes before a knock is covered: the upcard, the
// draw, and the discard. Each is built against two opponent hands and two
// stocks of the same size, and has to come out the same against all four.
// (The lay-off is not: by then the knocker's hand is face up.)
func TestBotDoesNotPeek(t *testing.T) {
	// A set, a run, a pair of kings and loose cards: deadwood 35, so nothing
	// can knock and every skill has to choose a discard.
	eleven := []string{"5H", "5D", "5C", "7S", "8S", "9S", "AH", "KC", "KD", "QD", "4D"}
	ten := eleven[:10]
	theirs := [][]string{
		{"2H", "3H", "4H", "6H", "8H", "TH", "2S", "3S", "4S", "JD"},
		{"9H", "JH", "QH", "JC", "6D", "8D", "9D", "TD", "JD", "7D"},
	}
	stocks := [][]string{
		{"6C", "7C", "8C", "9C", "TC"},
		{"3D", "2D", "AC", "AS", "QS"},
	}
	type position struct {
		name  string
		phase string
		hand  []string
		top   string
	}
	var positions []position
	// The king turns p1's pair into a set and the two helps nothing: one of
	// each, so the same assertion covers taking the discard and leaving it.
	for _, top := range []string{"KH", "2C"} {
		positions = append(positions,
			position{"upcard", phaseUpcardNonDealer, ten, top},
			position{"draw", phaseDraw, ten, top},
		)
	}
	positions = append(positions, position{"discard", phaseDiscard, eleven, "2C"})

	m := New()
	for _, p := range positions {
		for _, skill := range module.Skills {
			var want module.Action
			for i, hand := range theirs {
				for j, stock := range stocks {
					raw := withState(t, func(s *GameState) {
						s.Phase = p.phase
						s.Hands["p1"] = append([]string(nil), p.hand...)
						s.Hands["p2"] = append([]string(nil), hand...)
						s.Stock = append([]string(nil), stock...)
						s.DiscardPile = []string{"3C", p.top}
						// What p2 has taken is public, and it is what the
						// danger score reads — so it is set, and set the same.
						s.Interest = map[string][]string{"p2": {"JC"}}
					})
					got := botAct(t, m, raw, module.BotSeat{PlayerID: "p1", Skill: skill})
					if i == 0 && j == 0 {
						want = got
						continue
					}
					if !reflect.DeepEqual(got, want) {
						t.Errorf("%s with %s up, %s: played %+v against hand %d and stock %d, but %+v against hand 0 and stock 0 — it is reading hidden cards",
							p.name, p.top, skill, got, i, j, want)
					}
				}
			}
		}
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
