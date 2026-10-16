package rules_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"zolik/server/internal/ai"
	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// An offer probe runs against a scratch copy that is put back between probes
// rather than copied afresh (see probeScratch). Every probe must start from
// exactly what cloneState would have handed it — whatever the probe before it
// did to the copy — and the real state must never be touched.
//
// Played rather than built by hand, so the positions have melds on the table,
// joker debts, scores from earlier deals and everything else a probe can
// write to; and every probe the offer list makes is replayed against the
// scratch, plus a few the engine accepts, which write the most.
func TestProbeScratchStartsEveryProbeAsAFreshClone(t *testing.T) {
	for _, cfg := range []rules.RulesConfig{rules.ProfileZolikClassic, rules.ProfileContinental} {
		for seed := int64(1); seed <= 3; seed++ {
			order := []string{"p1", "p2", "p3"}
			gs, err := rules.StartMatch(cfg, order, seed, "p1")
			if err != nil {
				t.Fatal(err)
			}
			agent := ai.NewAgent(module.SkillMedium, seed)
			checked, withMelds := 0, 0
			for step := 0; step < 400 && gs.Status == rules.StatusActive; step++ {
				if len(gs.MeldMeta) > 0 {
					withMelds++
				}
				before, _ := json.Marshal(gs)
				want := rules.CloneStateForTest(gs)
				reset, release := rules.ScratchForTest(gs)
				for _, pid := range order {
					for _, a := range probesFor(gs, pid) {
						got := reset()
						if !reflect.DeepEqual(got, want) {
							t.Fatalf("seed %d step %d: the scratch is not a fresh clone before %v by %s", seed, step, a.Type, pid)
						}
						_, _ = rules.ApplyOwnedForTest(got, pid, a)
						checked++
					}
				}
				if !reflect.DeepEqual(reset(), want) {
					t.Fatalf("seed %d step %d: the scratch did not come back after the last probe", seed, step)
				}
				release()
				if after, _ := json.Marshal(gs); string(after) != string(before) {
					t.Fatalf("seed %d step %d: probing wrote to the real state", seed, step)
				}

				cur := gs.CurrentTurn
				hand := append([]string(nil), gs.Hands[cur]...)
				act := agent.ChooseAction(ai.VisibleFor(gs, ai.Ledger{}, cur), hand)
				out, err := rules.ApplyAction(gs, cur, act)
				if err != nil {
					// The agent's pick refused: play the first legal discard
					// or draw the offers describe, so the game goes on.
					out, err = fallback(gs, cur)
					if err != nil {
						break
					}
				}
				gs = out.State
				if gs.Phase == rules.PhaseIntermission {
					if gs, err = rules.ResumeAfterIntermission(gs); err != nil {
						t.Fatal(err)
					}
				}
			}
			if checked < 1000 || withMelds < 20 {
				t.Fatalf("seed %d: only %d probes checked, %d positions with melds down", seed, checked, withMelds)
			}
		}
	}
}

// probesFor is every action the offer list would probe for pid, and then
// some: every card in hand as a discard, a lay-off and a joker swap onto each
// meld, both draws and every undo.
func probesFor(gs rules.GameState, pid string) []rules.Action {
	out := []rules.Action{
		{Type: rules.ActionDrawCard, DrawFrom: rules.DrawFromDeck},
		{Type: rules.ActionDrawCard, DrawFrom: rules.DrawFromDiscard},
		{Type: rules.ActionLayMeld},
		{Type: rules.ActionUndoDrawDiscard}, {Type: rules.ActionUndoLayOff},
		{Type: rules.ActionUndoLayMeld}, {Type: rules.ActionUndoTurn},
	}
	hand := gs.Hands[pid]
	if len(hand) >= 3 {
		out = append(out, rules.Action{Type: rules.ActionLayMeld, Cards: hand[:3]})
	}
	for _, c := range hand {
		out = append(out, rules.Action{Type: rules.ActionDiscard, Card: c})
		for _, metas := range gs.MeldMeta {
			for _, m := range metas {
				out = append(out,
					rules.Action{Type: rules.ActionLayOff, MeldID: m.MeldID, Card: c},
					rules.Action{Type: rules.ActionSwapJoker, MeldID: m.MeldID, Card: c})
			}
		}
	}
	return out
}

func fallback(gs rules.GameState, pid string) (rules.ApplyOutcome, error) {
	for _, o := range rules.LegalActions(gs, pid) {
		if !o.Enabled || o.Verb == rules.VerbUndo || o.Verb == rules.VerbLayMeld {
			continue
		}
		a := rules.Action{}
		switch o.Verb {
		case rules.VerbDraw:
			a = rules.Action{Type: rules.ActionDrawCard, DrawFrom: rules.DrawFromDeck}
			if o.ID == rules.OfferDrawDiscard {
				a.DrawFrom = rules.DrawFromDiscard
			}
		case rules.VerbDiscard:
			if o.Source == nil || len(o.Source.Cards) == 0 {
				continue
			}
			a = rules.Action{Type: rules.ActionDiscard, Card: o.Source.Cards[0]}
		default:
			continue
		}
		if out, err := rules.ApplyAction(gs, pid, a); err == nil {
			return out, nil
		}
	}
	return rules.ApplyOutcome{}, rules.RulesError{Code: rules.ErrInvalidMeld}
}
