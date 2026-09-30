package learn_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"hash"
	"sync"
	"testing"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
)

// One agent serves every seat: a single NetBot value on one Policy plays all
// the seats of a table, switching between players by nothing more than the
// arguments of each call, and chooses exactly what a table of separate bots
// would — one per seat, each with its own Policy parsed from its own copy of
// the model.
func TestOneBotPlaysEverySeat(t *testing.T) {
	moves := 400
	if testing.Short() {
		moves = 120
	}
	for _, tc := range sharedTables {
		t.Run(tc.name, func(t *testing.T) {
			g := mustGame(t, tc.game)
			model, err := randomNet(g, []int{32, 32}, []int{16}, 7).Marshal()
			if err != nil {
				t.Fatal(err)
			}
			for _, temp := range []float64{0, 1} {
				shared := learn.NetBot{Game: g, Policy: loadPolicy(t, model), Fallback: g.Heuristic(), Temperature: temp}
				own := map[string]module.Bot{}
				for _, p := range learn.Players(tc.seats) {
					own[p.ID] = learn.NetBot{Game: g, Policy: loadPolicy(t, model), Fallback: g.Heuristic(), Temperature: temp}
				}
				seatsSeen := map[string]int{}
				n := 0
				for seed := int64(1); n < moves && seed < 50; seed++ {
					n += compareSeats(t, g, tc.variation, tc.seats, seed, moves-n, shared, own, seatsSeen)
				}
				if len(seatsSeen) != tc.seats {
					t.Fatalf("temperature %v: the shared bot played %d of %d seats: %v", temp, len(seatsSeen), tc.seats, seatsSeen)
				}
				t.Logf("temperature %v: %d moves compared across seats %v", temp, n, seatsSeen)
			}
		})
	}
}

// loadPolicy parses its own copy of the model, so no two calls share a weight.
func loadPolicy(t *testing.T, model []byte) *learn.Policy {
	t.Helper()
	n, err := learn.LoadNet(append([]byte(nil), model...))
	if err != nil {
		t.Fatal(err)
	}
	return learn.NewPolicy(n)
}

// compareSeats plays one match with the shared bot at every seat, asking the
// seat's own bot at every move as well, and fails on the first disagreement.
func compareSeats(t *testing.T, g learn.Game, variation string, seats int, seed int64, max int,
	shared module.Bot, own map[string]module.Bot, seen map[string]int) int {
	t.Helper()
	m := g.Module()
	players := learn.Players(seats)
	state, err := m.NewMatch(g.Config(seats, variation), players, seed)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for ; n < max; n++ {
		if done, _, _ := m.Finished(state); done {
			break
		}
		actor := module.ActiveSeat(m, state, players[0].ID, players)
		if actor == "" {
			break
		}
		offers, err := m.LegalActions(state, actor)
		if err != nil {
			t.Fatal(err)
		}
		seat := module.BotSeat{PlayerID: actor, Skill: module.SkillHard, Seed: module.SeatSeed(seed, actor, "bot")}
		a, ok := shared.Act(state, seat, offers)
		b, okB := own[actor].Act(state, seat, offers)
		ja, _ := json.Marshal(a)
		jb, _ := json.Marshal(b)
		if ok != okB || string(ja) != string(jb) {
			t.Fatalf("seed %d move %d seat %s: shared bot chose %s (%v), the seat's own bot %s (%v)", seed, n, actor, ja, ok, jb, okB)
		}
		seen[actor]++
		next, _, err := m.Apply(state, actor, a)
		if !ok || err != nil {
			// The same fallback either way; step past a refusal as the bench does.
			a, ok = module.ChooseAction(offers, nil)
			if !ok {
				break
			}
			if next, _, err = m.Apply(state, actor, a); err != nil {
				break
			}
		}
		state = next
	}
	return n
}

// Many matches at once on one Policy, every seat of every match a NetBot on
// the same weights. Under -race this is the check that a forward pass shares
// nothing it writes; without it, that where each match ends is the same
// whether it was played alone or among a crowd of others.
func TestSharedPolicyUnderConcurrency(t *testing.T) {
	seeds, budget := 16, 300
	if testing.Short() {
		seeds, budget = 8, 120
	}
	for _, tc := range sharedTables {
		t.Run(tc.name, func(t *testing.T) {
			g := mustGame(t, tc.game)
			p := learn.NewPolicy(randomNet(g, []int{32, 32}, []int{16}, 3))
			cold := learn.NetBot{Game: g, Policy: p, Fallback: g.Heuristic()}
			hot := learn.NetBot{Game: g, Policy: p, Fallback: g.Heuristic(), Temperature: 1}
			if tc.game == "zolik" {
				// A sampling NetBot seeds its coin from the state's bytes, and a
				// Žolíky deal carries the wall-clock time it was dealt at
				// (rules.GameState.Created): the same seed samples differently
				// from one run to the next. Not this test's subject — the
				// forward pass is the same at any temperature.
				hot.Temperature = 0
			}
			// A match's record is every action its bots chose, hashed: the final
			// state will not do, since Žolíky stamps its deal with the wall clock.
			play := func(seed int64) string {
				rec := &recorder{h: sha256.New()}
				players := learn.Players(tc.seats)
				_, st, err := learn.PlayOut(g.Module(), g.Config(tc.seats, tc.variation), players, seed, budget,
					func(id string) (module.Bot, module.Skill) {
						if id[len(id)-1]%2 == 0 {
							return rec.wrap(cold), module.SkillHard
						}
						return rec.wrap(hot), module.SkillHard
					})
				if err != nil {
					return "error: " + err.Error()
				}
				return fmt.Sprintf("%x %d %d", rec.h.Sum(nil)[:8], st.Actions, st.Illegal)
			}
			alone := make([]string, seeds)
			for i := range alone {
				alone[i] = play(int64(i + 1))
			}
			crowd := make([]string, 3*seeds)
			var wg sync.WaitGroup
			for i := range crowd {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					crowd[i] = play(int64(i%seeds + 1))
				}(i)
			}
			wg.Wait()
			for i, got := range crowd {
				if want := alone[i%seeds]; got != want {
					t.Fatalf("seed %d: alone %s, among others %s", i%seeds+1, want, got)
				}
			}
		})
	}
}

// recorder hashes every action the bots it wraps choose, in order. One match
// is played on one goroutine, so it needs no lock.
type recorder struct{ h hash.Hash }

func (r *recorder) wrap(b module.Bot) module.Bot { return recorded{b, r} }

type recorded struct {
	bot module.Bot
	rec *recorder
}

func (r recorded) Act(s module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	a, ok := r.bot.Act(s, seat, offers)
	j, _ := json.Marshal(a)
	fmt.Fprintf(r.rec.h, "%s %v %s\n", seat.PlayerID, ok, j)
	return a, ok
}
