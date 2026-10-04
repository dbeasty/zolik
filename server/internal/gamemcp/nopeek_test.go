package gamemcp

import (
	"encoding/json"
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"zolik/server/internal/module"
)

// hidden names where each game keeps what a seat must not see: the other
// seats' hands and the stock. A scramble deals those cards out again among
// themselves, each hand keeping its size — two states a seat cannot tell
// apart, so everything it is shown must be identical in both.
var hidden = map[string]struct {
	hands func(st map[string]any) map[string][]any // by player id
	stock []string                                 // path to the stock
}{
	"zolik": {
		hands: func(st map[string]any) map[string][]any {
			return anyHands(dig(st, "rules", "Hands").(map[string]any))
		},
		stock: []string{"rules", "DrawPile"},
	},
	"canasta": {
		hands: func(st map[string]any) map[string][]any { return anyHands(st["hands"].(map[string]any)) },
		stock: []string{"drawPile"},
	},
	"holdem": {
		hands: func(st map[string]any) map[string][]any {
			out := map[string][]any{}
			for _, s := range st["seats"].([]any) {
				seat := s.(map[string]any)
				hole, _ := seat["hole"].([]any)
				out[seat["playerId"].(string)] = hole
			}
			return out
		},
		stock: []string{"deck"},
	},
}

func anyHands(m map[string]any) map[string][]any {
	out := map[string][]any{}
	for id, h := range m {
		hand, _ := h.([]any)
		out[id] = hand
	}
	return out
}

func dig(m map[string]any, path ...string) any {
	var v any = m
	for _, p := range path {
		v = v.(map[string]any)[p]
	}
	return v
}

// scramble redeals every card seat cannot see.
func scramble(t *testing.T, gameID string, raw module.State, seat string, rnd *rand.Rand) module.State {
	t.Helper()
	var st map[string]any
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatal(err)
	}
	h := hidden[gameID]
	hands := h.hands(st)
	parent := dig(st, h.stock[:len(h.stock)-1]...)
	if len(h.stock) == 1 {
		parent = st
	}
	stock, _ := parent.(map[string]any)[h.stock[len(h.stock)-1]].([]any)
	var pool []any
	var ids []string
	for id, hand := range hands {
		if id != seat {
			ids = append(ids, id)
			pool = append(pool, hand...)
		}
	}
	pool = append(pool, stock...)
	rnd.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	for _, id := range ids {
		n := len(hands[id])
		copy(hands[id], pool[:n])
		pool = pool[n:]
	}
	copy(stock, pool)
	out, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// seesOthers reports whether seat's board shows any other seat's hand face
// up — a showdown, a deal's end — where a scramble would rightly show.
func seesOthers(vm module.ViewModel, seat string) bool {
	for _, z := range vm.Zones {
		if z.Kind != module.ZoneHand || z.OwnerID == seat {
			continue
		}
		for _, c := range z.Cards {
			if !c.FaceDown && c.Card != "" {
				return true
			}
		}
	}
	return false
}

// An observation never depends on a card its seat cannot see: redealing the
// other hands and the stock changes nothing in the text, the moves or the
// structured view.
func TestObserveDoesNotPeek(t *testing.T) {
	for _, tc := range tableCases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewService()
			out := mustCall[NewTableOut](t, svc, "new_table", tc.in)
			rnd := rand.New(rand.NewSource(2))
			checked, plays := 0, 0
			for plays < peekPlays() {
				tb := svc.tables[out.Table]
				if done, _ := tb.finished(); done {
					break
				}
				var actor *SeatInfo
				for i := range tb.Seats {
					if tb.Seats[i].Claude && tb.isAwaited(tb.Seats[i].Player) {
						actor = &tb.Seats[i]
						break
					}
				}
				if actor == nil {
					t.Fatalf("no claude seat to move")
				}
				// Every claude seat, on turn or not.
				for _, s := range tb.Seats {
					if !s.Claude {
						continue
					}
					before, err := tb.observe(s.Index)
					if err != nil {
						t.Fatal(err)
					}
					if seesOthers(before.View, s.Player) {
						continue
					}
					real := tb.State
					for k := 0; k < 2; k++ {
						tb.State = scramble(t, tb.Game, real, s.Player, rnd)
						after, err := tb.observe(s.Index)
						tb.State = real
						if err != nil {
							t.Fatal(err)
						}
						if before.Text != after.Text {
							t.Fatalf("seat %d's text changed with only hidden cards moved:\n%s\n---\n%s", s.Index, before.Text, after.Text)
						}
						if !reflect.DeepEqual(before.View, after.View) {
							t.Fatalf("seat %d's view changed with only hidden cards moved", s.Index)
						}
						if !reflect.DeepEqual(movesText(before.Moves), movesText(after.Moves)) {
							t.Fatalf("seat %d's moves changed with only hidden cards moved:\n%v\n%v", s.Index, movesText(before.Moves), movesText(after.Moves))
						}
						checked++
					}
				}
				obs := mustCall[Observation](t, svc, "observe", ObserveIn{Table: out.Table, Seat: actor.Index})
				i := rnd.Intn(len(obs.Moves))
				mustCall[PlayOut](t, svc, "play", PlayIn{Table: out.Table, Seat: actor.Index, Move: &i})
				plays++
			}
			if checked < 20 {
				t.Fatalf("only %d positions checked", checked)
			}
		})
	}
}

func movesText(ms []Move) []string {
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.Text
	}
	return out
}

// The log a claude seat reads between its moves names only cards it could
// see: every card code in a line is face up on its board before or after
// the action — never the card a bot drew blind.
func TestLogDoesNotPeek(t *testing.T) {
	for _, tc := range tableCases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewService()
			out := mustCall[NewTableOut](t, svc, "new_table", tc.in)
			tb := svc.tables[out.Table]
			viewer := tb.Seats[tc.in.ClaudeSeats[0]].Player
			rnd := rand.New(rand.NewSource(3))
			checkedLines := 0
			for plays := 0; plays < peekPlays(); plays++ {
				if done, _ := tb.finished(); done {
					break
				}
				var actor int = -1
				for _, s := range tb.Seats {
					if s.Claude && tb.isAwaited(s.Player) {
						actor = s.Index
						break
					}
				}
				obs := mustCall[Observation](t, svc, "observe", ObserveIn{Table: out.Table, Seat: actor})
				i := rnd.Intn(len(obs.Moves))
				mustCall[PlayOut](t, svc, "play", PlayIn{Table: out.Table, Seat: actor, Move: &i})
			}
			// Replay the match from the log to rebuild the board around each
			// entry, then check the entry's public lines against it.
			m := tb.g.m
			st, err := m.NewMatch(tb.Config, tb.players(), tb.Seed)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range tb.Log {
				before := st
				st, _, err = m.Apply(st, tb.Seats[e.Seat].Player, e.Action)
				if err != nil {
					t.Fatalf("replaying entry %d: %v", e.N, err)
				}
				if e.Seat == seatIndex(tb, viewer) {
					continue // its own moves may name its own cards
				}
				visible := faceUp(t, m, viewer, before, st)
				for _, line := range e.Public {
					for _, c := range cardsIn(line) {
						if !visible[c] {
							t.Fatalf("entry %d %q names %s, which %s could not see", e.N, line, c, viewer)
						}
					}
					checkedLines++
				}
			}
			if checkedLines < 10 {
				t.Fatalf("only %d lines checked", checkedLines)
			}
		})
	}
}

func seatIndex(tb *Table, player string) int { return tb.seatOf(player).Index }

func faceUp(t *testing.T, m module.GameModule, viewer string, states ...module.State) map[string]bool {
	out := map[string]bool{}
	for _, s := range states {
		vm, err := m.View(s, viewer)
		if err != nil {
			t.Fatal(err)
		}
		for _, z := range vm.Zones {
			for _, c := range z.Cards {
				if !c.FaceDown && c.Card != "" {
					out[cardText(c.Card)] = true
				}
			}
			for _, g := range z.Groups {
				for _, c := range g.Cards {
					out[cardText(c)] = true
				}
			}
		}
	}
	return out
}

// cardsIn finds the cards a rendered line names ("7♠", "10♦", "JK").
func cardsIn(line string) []string {
	var out []string
	for _, f := range strings.FieldsFunc(line, func(r rune) bool { return r == ' ' || r == ',' || r == '(' || r == ')' || r == ':' }) {
		if f == "JK" {
			out = append(out, f)
			continue
		}
		for _, g := range suitGlyph {
			if strings.HasSuffix(f, g) && len(f) > len(g) {
				out = append(out, f)
			}
		}
	}
	return out
}

// peekPlays is how many claude moves a no-peek walk makes.
func peekPlays() int {
	if testing.Short() {
		return 150
	}
	return 400
}
