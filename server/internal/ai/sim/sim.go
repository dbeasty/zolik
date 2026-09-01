// Package sim plays whole deals of Žolíky between agents, through the real
// engine and without a database.
//
// It was a helper inside internal/ai's own test file, and it moved here for
// one reason: a claim that a bot is *stronger* is only worth anything if
// something measures it, and a measurement that lives in a _test.go file
// cannot be run by a command, over a seed sweep, across every ruleset. The
// same code now backs the package's own regression tests and cmd/aibench, so
// the numbers in a benchmark and the numbers in CI come from one simulator.
//
// It is deliberately thin. It does not model a table, a lobby or a socket: it
// applies actions to a rules.GameState exactly as the module does, feeds the
// same ai.Ledger the module persists, and hands the agent the same
// ai.VisibleFor snapshot the module hands it. Anywhere that stopped being true
// the benchmark would be measuring a bot nobody plays against.
package sim

import (
	"fmt"

	"zolik/server/internal/ai"
	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// Result summarises one simulated deal.
type Result struct {
	Melds   int
	LayOffs int
	Turns   int
	// Deals is how many deals actually finished — somebody went out.
	Deals     int
	DownCount int
	Ended     bool
	Stalled   bool
	LastErr   error
	// Rejections is how many actions the engine refused. It is a correctness
	// number, not a strength one: a legal player proposes legal moves, and any
	// value above zero at any strength is a bug rather than a weakness.
	Rejections int
	// StrandedLays counts turns that ended with cards laid but the player
	// still short of the table's initial-meld point floor — the shape of the
	// "Karel laid A-2-3 against a 35-point floor" report.
	StrandedLays  int
	FirstStranded error
	// Scores is each seat's total at the end, lower being better: this is the
	// number a head-to-head is decided on.
	Scores map[string]int
	// Out is the seat that went out last, if any.
	Out string
}

// Seat is one player in a simulated deal: who they are and how well they play.
type Seat struct {
	ID    string
	Skill module.Skill
	// Profile overrides the strength ladder for this seat. Tuning only: it is
	// how one knob at a time gets priced, by playing the same seeds with it
	// on and off.
	Profile *ai.Profile
}

// Options tunes a run.
type Options struct {
	Rules      rules.RulesConfig
	Seed       int64
	Seats      []Seat
	MaxActions int
}

// Play runs a deal to completion, or until the action budget runs out.
//
// Every agent is driven through the same three steps the server drives it
// through — build the visible state, ask for an action, apply it — including
// the server's own fallback when the engine refuses one, so a rejection is
// counted rather than being allowed to wedge the run.
func Play(o Options) Result {
	cfg := o.Rules
	ids := make([]string, 0, len(o.Seats))
	for _, s := range o.Seats {
		ids = append(ids, s.ID)
	}
	res := Result{Scores: map[string]int{}}
	st, err := rules.StartMatch(cfg, ids, o.Seed, "")
	if err != nil {
		res.LastErr = err
		res.Stalled = true
		return res
	}

	agents := map[string]*ai.HeuristicAgent{}
	for i, s := range o.Seats {
		// A per-seat seed, derived the same way the runtime derives one, so
		// two easy bots at one table blunder independently instead of in
		// lockstep.
		seed := module.SeatSeed(o.Seed, s.ID, fmt.Sprint("sim", i))
		if s.Profile != nil {
			agents[s.ID] = ai.NewAgentWithProfile(*s.Profile, seed)
		} else {
			agents[s.ID] = ai.NewAgent(s.Skill, seed)
		}
	}

	var ledger ai.Ledger
	laidWhileShort := map[string]bool{}
	prevTurn := ""
	budget := o.MaxActions
	if budget <= 0 {
		budget = 4000
	}

	for i := 0; i < budget; i++ {
		if st.Status != rules.StatusActive {
			res.Ended = true
			break
		}
		actor := st.CurrentTurn
		if actor != prevTurn {
			res.Turns++
			prevTurn = actor
		}
		agent := agents[actor]
		if agent == nil {
			break
		}
		visible := ai.VisibleFor(st, ledger, actor)
		action := agent.ChooseAction(visible, append([]string(nil), st.Hands[actor]...))

		before := ai.Before(st)
		outcome, err := rules.ApplyAction(st, actor, action)
		if err != nil {
			res.Rejections++
			res.LastErr = fmt.Errorf("%s(%s) %+v (hand %v): %w",
				actor, agent.Difficulty(), action, st.Hands[actor], err)
			// Mirror the server's fallback so one rejection doesn't wedge the
			// sim — and so a rejection costs a bad move rather than the run.
			fallback := rules.Action{
				Type: rules.ActionDiscard,
				Card: ai.PickWorstDiscard(st.Hands[actor], cfg,
					len(st.Hands[actor]) == 1 && st.RoundReqMet[actor], st.DiscardTakenCard),
			}
			before = ai.Before(st)
			outcome, err = rules.ApplyAction(st, actor, fallback)
			if err != nil {
				res.Stalled = true
				break
			}
			action = fallback
		}
		ledger.Observe(before, outcome.State, actor, action)

		// A deal that ends inside this outcome has already re-dealt: the
		// table is cleared and everyone's down status reset, so nothing in
		// outcome.State describes the deal the action was taken in.
		dealEnded := false
		for _, e := range outcome.Events {
			if e.Type == "deal_ended" {
				dealEnded = true
			}
		}
		switch action.Type {
		case rules.ActionLayMeld:
			res.Melds++
			// Cards went down while the player was still short of the
			// table's point floor. Legitimate mid-plan (the floor is summed
			// across melds, so 27 + 21 clears 35), but only if the rest of
			// the plan lands this turn — see the discard case below.
			if !st.RoundReqMet[actor] && cfg.InitialMeldMinimum > 0 {
				laidWhileShort[actor] = true
			}
		case rules.ActionLayOff:
			res.LayOffs++
		case rules.ActionDiscard:
			// The discard ends the turn. Anything laid this turn that did
			// not bring the player down is stranded on the table: it cannot
			// make them down, it spent cards a qualifying combination might
			// have wanted, and it is a lay-off target for every opponent.
			if laidWhileShort[actor] && !dealEnded && !outcome.State.RoundReqMet[actor] {
				res.StrandedLays++
				if res.FirstStranded == nil {
					res.FirstStranded = fmt.Errorf(
						"%s finished a turn having laid melds %v while still short of the %d-point floor",
						actor, outcome.State.Melds[actor], cfg.InitialMeldMinimum)
				}
			}
			delete(laidWhileShort, actor)
		}
		if dealEnded {
			res.Deals++
			res.Out = actor
			laidWhileShort = map[string]bool{}
		}
		st = outcome.State
	}
	for _, p := range ids {
		if st.RoundReqMet[p] {
			res.DownCount++
		}
		res.Scores[p] = st.TotalScores[p]
	}
	return res
}

// Tally is a running head-to-head record for one skill.
type Tally struct {
	Skill  module.Skill
	Wins   int
	Deals  int
	Points int
	// Faults are the numbers that must be zero at every strength: a weak bot
	// is one that plays badly, never one that plays illegally or stops the
	// table.
	Rejections   int
	Stalls       int
	StrandedLays int
}

// WinRate is wins per finished deal, or zero when nothing finished.
func (t Tally) WinRate() float64 {
	if t.Deals == 0 {
		return 0
	}
	return float64(t.Wins) / float64(t.Deals)
}

// MeanPoints is the average end-of-run penalty, lower being better.
func (t Tally) MeanPoints() float64 {
	if t.Deals == 0 {
		return 0
	}
	return float64(t.Points) / float64(t.Deals)
}

// HeadToHead plays one skill against another over a seed sweep and returns a
// tally for each, in the order given.
//
// Seats alternate who is dealt first across seeds, because Žolíky's dealer
// advantage is real and a sweep that always seats the same skill first would
// measure the seat as much as the strength.
func HeadToHead(cfg rules.RulesConfig, a, b module.Skill, seeds int, maxActions int) (Tally, Tally) {
	return Duel(cfg, Contender{Skill: a}, Contender{Skill: b}, seeds, maxActions)
}

// Contender is one side of a duel: a named strength, or a hand-built profile
// being priced against one.
type Contender struct {
	Skill   module.Skill
	Profile *ai.Profile
}

func (c Contender) label() module.Skill {
	if c.Profile != nil {
		return c.Profile.Skill
	}
	return c.Skill
}

// Duel plays two contenders against each other over a seed sweep.
//
// Seats alternate who is dealt first across seeds, because Žolíky's dealer
// advantage is real and a sweep that always seated the same contender first
// would measure the seat as much as the strength.
func Duel(cfg rules.RulesConfig, a, b Contender, seeds int, maxActions int) (Tally, Tally) {
	ta, tb := Tally{Skill: a.label()}, Tally{Skill: b.label()}
	for seed := int64(1); seed <= int64(seeds); seed++ {
		first, second := a, b
		ids := []string{"p1", "p2"}
		flipped := seed%2 == 0
		if flipped {
			first, second = b, a
		}
		r := Play(Options{
			Rules:      cfg,
			Seed:       seed,
			MaxActions: maxActions,
			Seats: []Seat{
				{ID: ids[0], Skill: first.Skill, Profile: first.Profile},
				{ID: ids[1], Skill: second.Skill, Profile: second.Profile},
			},
		})
		seatA, seatB := ids[0], ids[1]
		if flipped {
			seatA, seatB = ids[1], ids[0]
		}
		record(&ta, r, seatA)
		record(&tb, r, seatB)
	}
	return ta, tb
}

// Table plays a mixed table over a seed sweep and tallies every contender.
//
// Duel is the sharper instrument and Table is the more honest one. A duel
// isolates a knob; a real Žolíky table has three or four seats, and several of
// the things that separate a strong player from a weak one — noticing that
// somebody else is one card from out, choosing which of four tables' melds to
// feed — do not exist at all with one opponent. A knob that measures dead
// head-to-head can still be worth its code here.
//
// Seats rotate by seed so that no contender keeps the dealer's advantage.
func Table(cfg rules.RulesConfig, cs []Contender, seeds, maxActions int) []Tally {
	tallies := make([]Tally, len(cs))
	for i, c := range cs {
		tallies[i] = Tally{Skill: c.label()}
	}
	ids := make([]string, len(cs))
	for i := range cs {
		ids[i] = fmt.Sprintf("p%d", i+1)
	}
	for seed := int64(1); seed <= int64(seeds); seed++ {
		shift := int(seed) % len(cs)
		seats := make([]Seat, len(cs))
		// seatOf[i] is where contender i is sitting this deal.
		seatOf := make([]string, len(cs))
		for i, c := range cs {
			pos := (i + shift) % len(cs)
			seats[pos] = Seat{ID: ids[pos], Skill: c.Skill, Profile: c.Profile}
			seatOf[i] = ids[pos]
		}
		r := Play(Options{Rules: cfg, Seed: seed, MaxActions: maxActions, Seats: seats})
		for i := range cs {
			record(&tallies[i], r, seatOf[i])
		}
	}
	return tallies
}

// record folds one run into a tally from the point of view of one seat.
//
// The winner is the lower total, which is how Žolíky scores: a deal is won by
// going out and leaving everybody else holding cards.
func record(t *Tally, r Result, seat string) {
	t.Rejections += r.Rejections
	t.StrandedLays += r.StrandedLays
	if r.Stalled {
		t.Stalls++
	}
	if r.Deals == 0 {
		return // nothing was decided; scoring it would score the budget
	}
	t.Deals++
	t.Points += r.Scores[seat]
	best, tied := 1<<30, false
	for _, s := range r.Scores {
		if s < best {
			best, tied = s, false
		} else if s == best {
			tied = true
		}
	}
	if r.Scores[seat] == best && !tied {
		t.Wins++
	}
}

// ProfileForSkill exposes the strength table to the tests that police it. The
// ladder is a claim about behaviour, so something has to be able to read it
// back.
func ProfileForSkill(s module.Skill) ai.Profile { return ai.ProfileFor(s) }
