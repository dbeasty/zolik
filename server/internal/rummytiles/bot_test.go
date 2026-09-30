package rummytiles

import (
	"reflect"
	"testing"

	"zolik/server/internal/module"
)

// TestBotDoesNotPeek.
//
// The bot is handed the whole state — every rack at the table and the pool in
// the order it will be drawn — because that is what the runtime has. Nothing
// but this test stops it reading them, and the hint button shows a human
// whatever the bot would do, so a bot that peeked would leak the other racks
// to them too.
//
// Three players, so there are two racks to hide. Each position is built with
// two sets of other racks and two pools of the same sizes, and has to come out
// the same against all four, at every skill. The offers are asked for per
// variant, the way the runtime asks, so a leak through the offer list would
// fail here as well.
func TestBotDoesNotPeek(t *testing.T) {
	others := []map[string][]string{
		{
			"p2": {"1-K", "2-K", "3-K", "11-R", "11-B"},
			"p3": {"9-B", "9-O", "13-K", "JOKER1", "6-O"},
		},
		{
			"p2": {"12-R", "12-B", "12-K", "5-O", "6-K"},
			"p3": {"2-R", "2-B", "8-O", "13-B", "JOKER2"},
		},
	}
	pools := [][]string{
		{"4-K", "5-K", "10-O", "11-O"},
		{"8-B", "9-R", "11-K", "1-O"},
	}
	run := []Set{{ID: "s0", Kind: "run", Cards: []string{"5-R", "6-R", "7-R"}}}

	for _, p := range []struct {
		name   string
		hand   []string
		melded bool
		table  []Set
		// placed is a set p1 has already laid this turn, so the position is
		// one where committing is on offer.
		placed []string
		verb   string // what the position is for; a leak-free draw everywhere proves nothing
	}{
		{name: "a set to lay", hand: []string{"3-R", "3-B", "3-O", "9-K", "12-O", "1-B"}, verb: VerbPlace},
		{name: "nothing to lay", hand: []string{"1-R", "4-B", "7-O", "10-K", "13-R", "2-O"}, verb: VerbDraw},
		{name: "a tile to add", hand: []string{"8-R", "1-B", "4-O"}, melded: true, table: run, verb: VerbAdd},
		{name: "a turn to commit", hand: []string{"10-B", "11-B", "12-B", "1-R", "4-O"}, melded: true, table: run,
			placed: []string{"10-B", "11-B", "12-B"}, verb: VerbCommit},
	} {
		for _, skill := range module.Skills {
			var want module.Action
			for i, racks := range others {
				for j, pool := range pools {
					raw := withState(t, func(s *GameState) {
						s.Players = []string{"p1", "p2", "p3"}
						s.Scores = map[string]int{"p1": 0, "p2": 0, "p3": 0}
						s.Hands = map[string][]string{
							"p1": append([]string(nil), p.hand...),
							"p2": append([]string(nil), racks["p2"]...),
							"p3": append([]string(nil), racks["p3"]...),
						}
						s.Pool = append([]string(nil), pool...)
						s.InitialMeld["p1"] = p.melded
						s.Sets = cloneSets(p.table)
						s.Workspace = &Workspace{Sets: cloneSets(p.table)}
					})
					if p.placed != nil {
						var err error
						if raw, err = apply(t, raw, "p1", moduleAction(VerbPlace, "", p.placed, nil)); err != nil {
							t.Fatalf("place: %v", err)
						}
					}
					m := New()
					offers, err := m.LegalActions(raw, "p1")
					if err != nil {
						t.Fatalf("LegalActions: %v", err)
					}
					got, ok := m.Bot().Act(raw, module.BotSeat{PlayerID: "p1", Skill: skill}, offers)
					if !ok {
						t.Fatalf("%s, %s: bot had no move", p.name, skill)
					}
					if i == 0 && j == 0 {
						if got.Verb != p.verb {
							t.Fatalf("%s, %s: played %+v, want a %s", p.name, skill, got, p.verb)
						}
						want = got
						continue
					}
					if !reflect.DeepEqual(got, want) {
						t.Errorf("%s, %s: played %+v against racks %d and pool %d, but %+v against racks 0 and pool 0 — it is reading hidden tiles",
							p.name, skill, got, i, j, want)
					}
				}
			}
		}
	}
}
