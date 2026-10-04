package canasta

import (
	"encoding/json"
	"reflect"
	"testing"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
)

// TestSeenSurvivesEveryUndo: every capture, meld and lay-off a bot makes in
// real play is taken back on the spot, and the record is the one from before
// it, exactly — and so is the rest of the state. The undo tests in engine_test.go pin
// this on hand-built tables with nothing recorded; this is the same promise
// on tables with a deal's worth of captures, discards and passes behind them.
func TestSeenSurvivesEveryUndo(t *testing.T) {
	m := New()
	undoOf := map[string]string{
		VerbTakePile: VerbUndoTakePile,
		VerbLayMeld:  VerbUndoLayMeld,
		VerbLayOff:   VerbUndoLayOff,
	}
	checked := map[string]int{}
	withSeen := 0
	for _, tb := range []struct {
		v     string
		seats int
	}{{"classic", 2}, {"classic", 4}, {"samba", 4}} {
		cfg := module.MatchConfig{Variation: tb.v, Options: module.Options{OptTargetScore: 1500}}
		for seed := int64(1); seed <= 2; seed++ {
			_, st, err := learn.PlayOut(m, cfg, learn.Players(tb.seats), 9300+seed, 50_000, func(string) (module.Bot, module.Skill) {
				return botFunc(func(raw module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
					a, ok := bot{}.Act(raw, seat, offers)
					undo, undoable := undoOf[a.Verb]
					if !ok || !undoable {
						return a, ok
					}
					next, _, err := m.Apply(raw, seat.PlayerID, a)
					if err != nil {
						return a, ok
					}
					back, _, err := m.Apply(next, seat.PlayerID, module.Action{Verb: undo})
					if err != nil {
						return a, ok // not undoable here (a meld laid off onto, say)
					}
					before, after := mustDecode(t, raw), mustDecode(t, back)
					if !reflect.DeepEqual(before.Seen, after.Seen) {
						t.Fatalf("%s %d seats: %s then %s did not restore the record\nbefore: %+v\nafter:  %+v",
							tb.v, tb.seats, a.Verb, undo, before.Seen, after.Seen)
					}
					// The rest of the table comes back too, but for the
					// sequence counter, which never goes backwards so that a
					// sequence's id is never reused (Team.MeldSeq).
					for i := range after.Teams {
						after.Teams[i].MeldSeq = before.Teams[i].MeldSeq
					}
					if !reflect.DeepEqual(before, after) {
						t.Fatalf("%s %d seats: %s then %s did not restore the state", tb.v, tb.seats, a.Verb, undo)
					}
					checked[a.Verb]++
					if before.Seen != nil && len(before.Seen.Held) > 0 {
						withSeen++
					}
					return a, ok
				}), module.SkillHard
			})
			if err != nil || st.Illegal > 0 || st.Stalled {
				t.Fatalf("%s seed %d: %v illegal %d stalled %v", tb.v, seed, err, st.Illegal, st.Stalled)
			}
		}
	}
	t.Logf("undone exactly: %v (%d with captured cards on record)", checked, withSeen)
	for verb := range undoOf {
		if checked[verb] == 0 {
			t.Errorf("no %s was ever undone", verb)
		}
	}
	if withSeen == 0 {
		t.Error("no undo ran with captured cards on record")
	}
}

// TestStateWithoutSeenPlays: a match stored before the record existed has no
// "seen" key. It decodes, the inference reads it as a deal nobody remembers
// anything of, and it plays to the end — no migration.
func TestStateWithoutSeenPlays(t *testing.T) {
	m := New()
	for _, tb := range []struct {
		v     string
		seats int
	}{{"classic", 2}, {"classic", 4}, {"samba", 4}} {
		cfg := module.MatchConfig{Variation: tb.v, Options: module.Options{OptTargetScore: 1500}}
		players := learn.Players(tb.seats)
		// Play a while, so the stored state is mid-deal with a record.
		var stored module.State
		n := 0
		_, _, err := learn.PlayOut(m, cfg, players, 9400, 50_000, func(string) (module.Bot, module.Skill) {
			return botFunc(func(raw module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
				if n++; stored == nil && n > 60 {
					if s := mustDecode(t, raw); s.Seen != nil && len(s.Seen.Discards) > 0 {
						stored = raw
					}
				}
				return bot{}.Act(raw, seat, offers)
			}), module.SkillHard
		})
		if err != nil || stored == nil {
			t.Fatalf("%s: no mid-deal state to store (%v)", tb.v, err)
		}
		var doc map[string]any
		if err := json.Unmarshal(stored, &doc); err != nil {
			t.Fatal(err)
		}
		delete(doc, "seen")
		old, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		s := mustDecode(t, old)
		if s.Seen != nil {
			t.Fatalf("%s: the old state decoded with a record", tb.v)
		}
		if inf := infer(s, s.Current); len(inf.seats) != tb.seats-1 {
			t.Fatalf("%s: inference saw %d seats", tb.v, len(inf.seats))
		}
		state := module.State(old)
		for steps := 0; ; steps++ {
			if done, _, err := m.Finished(state); err != nil {
				t.Fatal(err)
			} else if done {
				break
			}
			if steps > 50_000 {
				t.Fatalf("%s: the old state never finished", tb.v)
			}
			actor := module.ActiveSeat(m, state, players[0].ID, players)
			offers, err := m.LegalActions(state, actor)
			if err != nil {
				t.Fatal(err)
			}
			a, ok := bot{}.Act(state, module.BotSeat{PlayerID: actor, Skill: module.SkillHard}, offers)
			if !ok {
				t.Fatalf("%s: no move for %s", tb.v, actor)
			}
			next, _, err := m.Apply(state, actor, a)
			if err != nil {
				t.Fatalf("%s: %s refused: %v", tb.v, a.Verb, err)
			}
			state = next
		}
	}
}
