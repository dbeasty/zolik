package holdem

import (
	"encoding/json"
	"reflect"
	"testing"

	"zolik/server/internal/module"
)

func TestNarrateEventSaysEachPokerMove(t *testing.T) {
	m := &Module{}
	board := []string{"6H", "JD", "2H", "9C", "2S"}
	cases := []struct {
		name   string
		ev     module.Event
		key    string
		player string
		params map[string]any
	}{
		{"fold", module.Event{Type: "folded", Data: map[string]any{"playerId": "p"}}, "holdem.move.folded", "p", map[string]any{"player": "p"}},
		{"check", module.Event{Type: "checked", Data: map[string]any{"playerId": "p"}}, "holdem.move.checked", "p", map[string]any{"player": "p"}},
		{"call", module.Event{Type: "called", Data: map[string]any{"playerId": "p", "amount": 40, "allIn": false}}, "holdem.move.called", "p", map[string]any{"player": "p", "n": 40}},
		{"call all in", module.Event{Type: "called", Data: map[string]any{"playerId": "p", "amount": 40, "allIn": true}}, "holdem.move.calledAllIn", "p", map[string]any{"player": "p", "n": 40}},
		{"raise", module.Event{Type: "raised", Data: map[string]any{"playerId": "p", "to": 80, "allIn": false}}, "holdem.move.raised", "p", map[string]any{"player": "p", "n": 80}},
		{"raise all in", module.Event{Type: "raised", Data: map[string]any{"playerId": "p", "to": 80, "allIn": true}}, "holdem.move.raisedAllIn", "p", map[string]any{"player": "p", "n": 80}},
		{"stand pat", module.Event{Type: "stood_pat", Data: map[string]any{"playerId": "p"}}, "holdem.move.stoodPat", "p", map[string]any{"player": "p"}},
		{"draw one", module.Event{Type: "drew", Data: map[string]any{"playerId": "p", "count": 1}}, "holdem.move.drewOne", "p", map[string]any{"player": "p"}},
		{"draw three", module.Event{Type: "drew", Data: map[string]any{"playerId": "p", "count": 3}}, "holdem.move.drewMany", "p", map[string]any{"player": "p", "n": 3}},
		{"flop", module.Event{Type: "street", Data: map[string]any{"street": streetFlop, "board": board[:3]}}, "holdem.move.flop", "", map[string]any{"cards": board[:3]}},
		{"turn", module.Event{Type: "street", Data: map[string]any{"street": streetTurn, "board": board[:4]}}, "holdem.move.turn", "", map[string]any{"card": "9C"}},
		{"river", module.Event{Type: "street", Data: map[string]any{"street": streetRiver, "board": board}}, "holdem.move.river", "", map[string]any{"card": "2S"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mv, ok := m.NarrateEvent(nil, c.ev)
			if !ok {
				t.Fatalf("no line for %s", c.ev.Type)
			}
			if mv.Fact.LabelKey != c.key || mv.PlayerID != c.player {
				t.Errorf("got %q for %q, want %q for %q", mv.Fact.LabelKey, mv.PlayerID, c.key, c.player)
			}
			if !reflect.DeepEqual(mv.Fact.Params, c.params) {
				t.Errorf("params %v, want %v", mv.Fact.Params, c.params)
			}
		})
	}
}

// An event read back from a stored log has been through JSON, so its numbers
// are float64 and its board is []any. It has to read the same as a live one.
func TestNarrateEventReadsAnEventBackFromJSON(t *testing.T) {
	raw, err := json.Marshal(module.Event{Type: "street", Data: map[string]any{
		"street": streetFlop, "board": []string{"6H", "JD", "2H"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	var ev module.Event
	if err := json.Unmarshal(raw, &ev); err != nil {
		t.Fatal(err)
	}
	mv, ok := (&Module{}).NarrateEvent(nil, ev)
	if !ok || !reflect.DeepEqual(mv.Fact.Params["cards"], []string{"6H", "JD", "2H"}) {
		t.Errorf("flop from JSON: ok=%v params=%v", ok, mv.Fact.Params)
	}
	called := module.Event{Type: "called", Data: map[string]any{"playerId": "p", "amount": float64(40)}}
	if mv, _ := (&Module{}).NarrateEvent(nil, called); mv.Fact.Params["n"] != 40 {
		t.Errorf("call amount from JSON: %v", mv.Fact.Params["n"])
	}
}

// Draw's streets put nothing on the table, and neither does anything that is
// not a move, so neither gets a line.
func TestNarrateEventIsSilentWhereNothingWasPlayed(t *testing.T) {
	for _, ev := range []module.Event{
		{Type: "street", Data: map[string]any{"street": streetDraw, "board": []string{}}},
		{Type: "hand_started", Data: map[string]any{}},
		{Type: "hand_ended", Data: map[string]any{}},
	} {
		if mv, ok := (&Module{}).NarrateEvent(nil, ev); ok {
			t.Errorf("%s said %q", ev.Type, mv.Fact.LabelKey)
		}
	}
}
