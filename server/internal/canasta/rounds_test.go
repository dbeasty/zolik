package canasta

import (
	"reflect"
	"testing"
)

// The side that closes a deal is not the side that takes it. Going out is
// worth a hundred; a partnership caught holding cards can still out-score the
// closers, and the round log has to credit the points, not the exit.
func TestRoundsCreditsTheDealToThePointsNotTheCloser(t *testing.T) {
	deal := func(closer string, a, b int) DealResult {
		return DealResult{WentOut: closer, Teams: []TeamResult{
			{TeamID: 0, Total: a}, {TeamID: 1, Total: b},
		}}
	}
	cases := []struct {
		name     string
		deal     DealResult
		winners  []string
		headline map[string]any
	}{
		{"closer out-scored", deal("sarka", 180, 420), []string{"p2", "p4"},
			map[string]any{"player": "sarka", "diff": "-240"}},
		{"closer ahead", deal("p2", 90, 310), []string{"p2", "p4"},
			map[string]any{"player": "p2", "diff": "+220"}},
		{"level deal", deal("sarka", 200, 200), nil,
			map[string]any{"player": "sarka", "diff": "0"}},
		{"deck ran out", DealResult{Exhausted: true, Teams: []TeamResult{
			{TeamID: 0, Total: 50}, {TeamID: 1, Total: -30},
		}}, []string{"sarka", "p3"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := &GameState{
				TurnOrder: []string{"sarka", "p2", "p3", "p4"},
				TeamOf:    map[string]int{"sarka": 0, "p2": 1, "p3": 0, "p4": 1},
				Deals:     []DealResult{c.deal},
			}
			raw, err := encode(s)
			if err != nil {
				t.Fatal(err)
			}
			log, err := New().Rounds(raw)
			if err != nil {
				t.Fatal(err)
			}
			r := log.Rounds[0]
			if !reflect.DeepEqual(r.Winners, c.winners) {
				t.Errorf("winners = %v, want %v", r.Winners, c.winners)
			}
			switch {
			case c.headline == nil && r.Headline != nil:
				t.Errorf("headline = %+v, want none", r.Headline)
			case c.headline != nil && (r.Headline == nil || r.Headline.LabelKey != "canasta.round.closed" ||
				!reflect.DeepEqual(r.Headline.Params, c.headline)):
				t.Errorf("headline = %+v, want canasta.round.closed %v", r.Headline, c.headline)
			}
		})
	}
}
