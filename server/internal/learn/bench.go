package learn

import (
	"fmt"
	"math"
	"runtime"
	"sync"

	"zolik/server/internal/module"
)

// The duplicate bench: whether one bot is better than another, with the cards
// taken out of the answer.
//
// A seed fixes the deal, so a seat is dealt the same cards whoever sits in it.
// Measuring one seating per seed measures the cards as much as the player —
// internal/ai/sim's Duel once inverted a whole ruleset's strength ladder that
// way — so every seed is played twice, with the contenders swapped between the
// seats, and the two results are one sample. What is left is the players.
//
// It drives the module through GameModule alone, the way the runtime does:
// ActiveSeat to find who is on turn, LegalActions for their offers, Apply for
// their choice. A number here is a number about the bot people play against.

// Contender is one side of a comparison.
type Contender struct {
	Name  string
	Bot   module.Bot
	Skill module.Skill
}

// BenchResult is A's advantage over B, per match, in the game's outcome unit.
type BenchResult struct {
	Seeds   int
	Mean    float64 // A minus B, averaged over seeds and both seatings
	StdErr  float64
	Illegal int // actions the engine refused; a bug at any strength
	Stalls  int // seeds with a match that ran out of budget or found nobody on turn
	// StalledSeeds and StallWhy say which, and what stopped them, so a stall
	// can be replayed rather than guessed at.
	StalledSeeds []int64
	StallWhy     map[string]int
}

// Significant reports whether A is ahead by more than two standard errors.
func (r BenchResult) Significant() bool { return r.StdErr > 0 && r.Mean > 2*r.StdErr }

func (r BenchResult) String() string {
	out := fmt.Sprintf("%+.2f ± %.2f per match over %d seeds (illegal %d, stalls %d)",
		r.Mean, r.StdErr, r.Seeds, r.Illegal, r.Stalls)
	if r.Stalls > 0 {
		out += fmt.Sprintf(" stalled seeds %v %v", r.StalledSeeds, r.StallWhy)
	}
	return out
}

// Bench plays a against b on seeds [first, first+n), a seed per worker.
//
// Seats alternate between the contenders — by partnership where the game has
// them, so partners are always the same contender — and the second seating of
// each seed flips every seat. Bots must be safe to call from several
// goroutines, which every bot here is: they are functions of the position.
func Bench(g Benchable, seats int, variation string, a, b Contender, first int64, n, budget int) (BenchResult, error) {
	type outcome struct {
		sample  float64
		stalled bool
		illegal int
		why     []string
		err     error
	}
	results := make([]outcome, n)
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < runtime.GOMAXPROCS(0); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				o := &results[i]
				o.sample, o.illegal, o.why, o.err = benchSeed(g, seats, variation, a, b, first+int64(i), budget)
				o.stalled = len(o.why) > 0
			}
		}()
	}
	for i := 0; i < n; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	res := BenchResult{StallWhy: map[string]int{}}
	samples := make([]float64, 0, n)
	for i, o := range results {
		if o.err != nil {
			return res, fmt.Errorf("seed %d: %w", first+int64(i), o.err)
		}
		res.Illegal += o.illegal
		if o.stalled {
			res.Stalls++
			res.StalledSeeds = append(res.StalledSeeds, first+int64(i))
			for _, w := range o.why {
				res.StallWhy[w]++
			}
			continue
		}
		samples = append(samples, o.sample)
	}
	res.Seeds = len(samples)
	res.Mean, res.StdErr = meanErr(samples)
	return res, nil
}

// benchSeed plays one seed in both seatings and returns A's advantage, or the
// reasons it stalled.
func benchSeed(g Benchable, seats int, variation string, a, b Contender, seed int64, budget int) (float64, int, []string, error) {
	sample, illegal := 0.0, 0
	var why []string
	for _, flip := range []bool{false, true} {
		players := Players(seats)
		cfg := g.Config(seats, variation)
		sideOf := sidesOf(g.Module(), cfg, players)
		isA := map[string]bool{}
		for _, p := range players {
			isA[p.ID] = (sideOf[p.ID]%2 == 0) != flip
		}
		final, st, err := PlayOut(g.Module(), cfg, players, seed, budget, func(id string) (module.Bot, module.Skill) {
			if isA[id] {
				return a.Bot, a.Skill
			}
			return b.Bot, b.Skill
		})
		illegal += st.Illegal
		if err != nil {
			return 0, illegal, nil, err
		}
		if st.Stalled {
			why = append(why, st.Why)
			continue
		}
		var sumA, sumB float64
		var nA, nB int
		for _, p := range players {
			o, err := g.Outcome(final, p.ID)
			if err != nil {
				return 0, illegal, nil, err
			}
			if isA[p.ID] {
				sumA, nA = sumA+o, nA+1
			} else {
				sumB, nB = sumB+o, nB+1
			}
		}
		if nA > 0 && nB > 0 {
			sample += (sumA/float64(nA) - sumB/float64(nB)) / 2
		}
	}
	return sample, illegal, why, nil
}

// Players are the ids a bench or environment seats: p0, p1, ...
func Players(n int) []module.PlayerRef {
	out := make([]module.PlayerRef, n)
	for i := range out {
		id := fmt.Sprintf("p%d", i)
		out[i] = module.PlayerRef{ID: id, Name: id, IsAI: true}
	}
	return out
}

// sidesOf numbers each seat's partnership, or gives every seat its own number
// in a game without partnerships.
func sidesOf(m module.GameModule, cfg module.MatchConfig, players []module.PlayerRef) map[string]int {
	out := make(map[string]int, len(players))
	if sides := module.SidesOf(m, cfg, players); len(sides) > 0 {
		for i, side := range sides {
			for _, id := range side {
				out[id] = i
			}
		}
		return out
	}
	for i, p := range players {
		out[p.ID] = i
	}
	return out
}

// PlayStats is what went wrong in a match, if anything.
type PlayStats struct {
	Actions int
	Illegal int
	Stalled bool
	// Why is what stopped a stalled match: the action budget, nobody on turn,
	// or a seat that nothing in its offer list could move.
	Why string
}

// PlayOut plays one match to the end with the bots seatBot names.
//
// A refused action is counted and the seat is played by its offer list for
// that one move, so a single bad choice costs a number in a column rather than
// the whole sample; a seat whose offer list is refused too has nothing left to
// try, and the match is a stall.
func PlayOut(m module.GameModule, cfg module.MatchConfig, players []module.PlayerRef, seed int64, budget int,
	seatBot func(id string) (module.Bot, module.Skill)) (module.State, PlayStats, error) {
	var st PlayStats
	state, err := m.NewMatch(cfg, players, seed)
	if err != nil {
		return nil, st, fmt.Errorf("learn: NewMatch: %w", err)
	}
	for ; st.Actions < budget; st.Actions++ {
		if done, _, err := m.Finished(state); err != nil {
			return nil, st, err
		} else if done {
			return state, st, nil
		}
		actor := module.ActiveSeat(m, state, players[0].ID, players)
		if actor == "" {
			st.Stalled, st.Why = true, "nobody on turn"
			return state, st, nil
		}
		offers, err := m.LegalActions(state, actor)
		if err != nil {
			return nil, st, fmt.Errorf("learn: LegalActions(%s): %w", actor, err)
		}
		bot, skill := seatBot(actor)
		seat := module.BotSeat{PlayerID: actor, Skill: skill, Seed: module.SeatSeed(seed, actor, "bot")}
		next, ok := step(m, state, actor, bot, seat, offers, &st)
		if !ok {
			st.Stalled, st.Why = true, "no move for "+actor
			return state, st, nil
		}
		state = next
	}
	st.Stalled, st.Why = true, "action budget"
	return state, st, nil
}

// step applies one bot move, falling back to the offer list once if the engine
// refuses it.
func step(m module.GameModule, s module.State, actor string, bot module.Bot, seat module.BotSeat,
	offers []module.ActionOffer, st *PlayStats) (module.State, bool) {
	if a, ok := bot.Act(s, seat, offers); ok {
		if next, _, err := m.Apply(s, actor, a); err == nil {
			return next, true
		}
		st.Illegal++
	}
	for _, a := range module.ChooseActions(offers, nil) {
		if next, _, err := m.Apply(s, actor, a); err == nil {
			return next, true
		}
	}
	return nil, false
}

func meanErr(xs []float64) (mean, stderr float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	for _, x := range xs {
		mean += x
	}
	mean /= float64(len(xs))
	if len(xs) < 2 {
		return mean, 0
	}
	v := 0.0
	for _, x := range xs {
		v += (x - mean) * (x - mean)
	}
	v /= float64(len(xs) - 1)
	return mean, math.Sqrt(v / float64(len(xs)))
}
