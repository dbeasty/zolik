package canasta

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"testing"

	"zolik/server/internal/module"
)

// No reachable position may leave the seat on turn with nothing to do but take
// moves back. The engine owns that property, not the bots: a capture it offers
// has to be a capture after which the turn can still end, and a deal whose
// stock is gone has to end the moment nobody can take the pile that way (see
// engine.go's turnCanFinish and advanceTurn).
//
// So the sweep plays every configuration with each bot skill and with seats
// that pick uniformly among the enabled offers, at the lobby's stock and with
// the stock cut short, and checks every position it passes through. A
// cut stock is what makes the empty-stock ending common rather than a once a
// thousand deals event: every deal runs out.
//
// The default run is a sample, since a seat choosing at random rarely goes out
// and a six-seat Samba deal then runs the whole stock down. The full sweep is
//
//	CANASTA_DEAD_END_SEEDS=1000 go test ./internal/canasta -run TestNoReachablePositionIsADeadEnd -timeout 0

type sweepConfig struct {
	variation string
	seats     int
}

var sweepConfigs = []sweepConfig{
	{"classic", 2}, {"classic", 3}, {"classic", 4},
	{"samba", 4}, {"samba", 6},
}

// sweepSeat is one way of choosing moves: a bot skill, or "" for random.
var sweepSeats = []module.Skill{module.SkillHard, module.SkillMedium, module.SkillEasy, ""}

// stockCuts is how many cards each deal's stock is left with; 0 leaves it.
var stockCuts = []int{0, 8}

func sweepSeedCount() int {
	if n, err := strconv.Atoi(os.Getenv("CANASTA_DEAD_END_SEEDS")); err == nil && n > 0 {
		return n
	}
	if testing.Short() {
		return 1
	}
	return 3
}

func TestNoReachablePositionIsADeadEnd(t *testing.T) {
	m := New()
	seeds := sweepSeedCount()
	for _, cfg := range sweepConfigs {
		for _, skill := range sweepSeats {
			for _, cut := range stockCuts {
				name := fmt.Sprintf("%s-%d-%s-cut%d", cfg.variation, cfg.seats, skillName(skill), cut)
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					fails := 0
					for seed := int64(1); seed <= int64(seeds); seed++ {
						if why := sweepMatch(m, cfg, skill, cut, seed, 2); why != "" {
							t.Errorf("seed %d: %s", seed, why)
							if fails++; fails >= 5 {
								t.Fatalf("stopping after %d failures", fails)
							}
						}
					}
				})
			}
		}
	}
}

func skillName(s module.Skill) string {
	if s == "" {
		return "random"
	}
	return string(s)
}

// sweepMatch plays one match for up to `deals` deals and reports the first
// position in which the seat on turn has nothing but undos, or the first move
// the engine refuses or no seat can make.
func sweepMatch(m *Module, cfg sweepConfig, skill module.Skill, cut int, seed int64, deals int) string {
	players := benchPlayers(cfg.seats)
	state, err := m.NewMatch(module.MatchConfig{Variation: cfg.variation}, players, seed)
	if err != nil {
		return "NewMatch: " + err.Error()
	}
	state = cutStock(state, cut)
	rng := rand.New(rand.NewSource(seed))
	started := 0
	for step := 0; step < 20000; step++ {
		if done, _, _ := m.Finished(state); done {
			return ""
		}
		actor := module.ActiveSeat(m, state, players[0].ID, players)
		if actor == "" {
			return "nobody on turn"
		}
		offers, err := m.LegalActions(state, actor)
		if err != nil {
			return "LegalActions: " + err.Error()
		}
		if why := deadEnd(state, actor, offers); why != "" {
			return fmt.Sprintf("step %d: %s", step, why)
		}

		var a module.Action
		ok := false
		if skill == "" {
			a, ok = randomMove(rng, offers)
		} else {
			seat := module.BotSeat{PlayerID: actor, Skill: skill, Seed: module.SeatSeed(seed, actor, "bot")}
			a, ok = m.Bot().Act(state, seat, offers)
		}
		if !ok {
			s, _ := decode(state)
			return fmt.Sprintf("step %d: no move for %s: hand %v, pile %v, stock %d, offers %v",
				step, actor, s.Hands[actor], s.DiscardPile, len(s.DrawPile), enabledVerbs(offers))
		}
		next, events, err := m.Apply(state, actor, a)
		if err != nil {
			return fmt.Sprintf("step %d: %s proposed an illegal %+v: %v", step, actor, a, err)
		}
		state = next
		if s, _ := decode(state); s != nil && s.Break.Open {
			// Nobody is on turn between deals; continue for everyone, and the
			// last of them is what deals the next hand.
			for _, p := range players {
				if next, more, err := m.Apply(state, p.ID, module.Action{Verb: module.VerbContinue}); err == nil {
					state = next
					events = append(events, more...)
				}
			}
		}
		for _, e := range events {
			if e.Type == "deal_started" {
				if started++; started >= deals {
					return ""
				}
				state = cutStock(state, cut)
			}
		}
	}
	return "action budget"
}

// deadEnd names the position if the seat on turn has no enabled offer other
// than taking a move back. Between deals the only offer is to continue, which
// is a move.
func deadEnd(state module.State, actor string, offers []module.ActionOffer) string {
	for _, o := range offers {
		if o.Enabled && !isUndo(o.Verb) {
			return ""
		}
	}
	s, _ := decode(state)
	t := s.team(actor)
	return fmt.Sprintf("dead end for %s (phase %s): hand %v, pile %v, stock %d, melded %v, canastas %d, laid %d, enabled %v",
		actor, s.Phase, s.Hands[actor], s.DiscardPile, len(s.DrawPile), t.HasMelded, t.canastas(), s.LaidThisTurn, enabledVerbs(offers))
}

func isUndo(verb string) bool {
	return verb == VerbUndoTakePile || verb == VerbUndoLayOff || verb == VerbUndoLayMeld
}

// randomMove picks uniformly among the enabled offers that are not undos, and
// among the cards an offer accepts when it takes one of several.
func randomMove(rng *rand.Rand, offers []module.ActionOffer) (module.Action, bool) {
	var live []module.ActionOffer
	for _, o := range offers {
		if o.Enabled && !isUndo(o.Verb) {
			live = append(live, o)
		}
	}
	rng.Shuffle(len(live), func(i, j int) { live[i], live[j] = live[j], live[i] })
	for _, o := range live {
		a, ok := module.SubmissionFor(o)
		if !ok {
			continue
		}
		if o.Source != nil && len(o.Source.Submit) == 0 && o.Source.MaxCards == 1 && len(o.Source.Cards) > 1 {
			a.Cards = []string{o.Source.Cards[rng.Intn(len(o.Source.Cards))]}
		}
		return a, true
	}
	return module.Action{}, false
}

// cutStock leaves the stock with its top n cards, so the deal runs out. The
// cards cut go nowhere: they are simply not in this deal.
func cutStock(state module.State, n int) module.State {
	if n <= 0 {
		return state
	}
	s, err := decode(state)
	if err != nil || len(s.DrawPile) <= n {
		return state
	}
	s.DrawPile = append([]string(nil), s.DrawPile[len(s.DrawPile)-n:]...)
	out, err := encode(s)
	if err != nil {
		return state
	}
	return out
}
