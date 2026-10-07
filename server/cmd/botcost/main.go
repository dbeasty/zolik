// Command botcost measures what one bot decision costs, for every game, table
// size and skill the server can seat.
//
// aibench asks how *strong* a bot is. This asks how *expensive* it is, which
// is the number the deep-agent capacity plan divides by
// (docs/bot-compute-gating-plan.md §3.2): CPU per decision and memory churned
// per decision. It plays whole matches with every seat a bot of one skill,
// through the same Bot.Act, LegalActions and Apply the server's bot loop uses,
// and times each Act on its own.
//
// It runs on one core by default (-procs 1). A decision is single-threaded,
// and letting the collector borrow other cores would hide its share of the
// cost — on a one-vCPU box there are no other cores to borrow.
//
// Two clocks per decision. Wall time is what a seat waits; CPU time (this
// process's, which on one core with one goroutine is the decision plus its
// share of the collector) is what it costs. On a busy machine the first
// inflates and the second does not, so read CPU when comparing two runs.
//
//	go run ./cmd/botcost                      # every game, variation, seat count and skill
//	go run ./cmd/botcost -game holdem -matches 20
//	go run ./cmd/botcost -csv > costs.csv
//	go run ./cmd/botcost -learned            # add AI rows: the shipped models
//
// -learned throws the same switch the admin console's Bots card does
// (learn.SetHardModel) for every game that ships a model, and adds an AI row
// for each such game, which measures the trained model exactly as live play
// runs it (skill column "ai/net"). A game with no model has no AI row: there
// an AI seat plays Hard, which already has one.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"zolik/server/internal/blackjack"
	"zolik/server/internal/botstats"
	"zolik/server/internal/canasta"
	"zolik/server/internal/ginrummy"
	"zolik/server/internal/holdem"
	"zolik/server/internal/lastcard"
	"zolik/server/internal/learn"
	"zolik/server/internal/marias"
	"zolik/server/internal/module"
	"zolik/server/internal/prsi"
	"zolik/server/internal/rummytiles"
	"zolik/server/internal/zolikmod"
)

func main() {
	game := flag.String("game", "", "only games whose id contains this")
	matches := flag.Int("matches", 5, "matches per configuration")
	decisions := flag.Int("decisions", 1500, "bot decisions per match at most (long games are cut, not skipped)")
	procs := flag.Int("procs", 1, "GOMAXPROCS for the run")
	seed := flag.Int64("seed", 1, "first seed")
	asCSV := flag.Bool("csv", false, "print CSV instead of a table")
	cpuProfile := flag.String("cpuprofile", "", "write a CPU profile of the whole run to this file")
	learned := flag.Bool("learned", false, "switch every shipped model on and add AI rows for the games that ship one")
	flag.Parse()
	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "botcost:", err)
			os.Exit(2)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			fmt.Fprintln(os.Stderr, "botcost:", err)
			os.Exit(2)
		}
		defer pprof.StopCPUProfile()
	}

	runtime.GOMAXPROCS(*procs)
	if *learned {
		for _, st := range learn.HardModels() {
			if _, err := learn.SetHardModel(st.Game, true); err != nil {
				fmt.Fprintf(os.Stderr, "%s: model not switched on: %v\n", st.Game, err)
				continue
			}
			fmt.Fprintf(os.Stderr, "%s: AI seats play %s\n", st.Game, st.Model.Title)
		}
	}
	reg := module.NewRegistry(zolikmod.New(), prsi.New(), canasta.New(), holdem.New(), ginrummy.New(), rummytiles.New(), blackjack.New(), marias.New(), lastcard.New())

	var rows []row
	failed := false
	for _, id := range reg.IDs() {
		if *game != "" && !strings.Contains(id, *game) {
			continue
		}
		mod := reg.Get(id)
		for _, c := range configs(mod.Descriptor()) {
			_, hasNet := module.BotFor(mod).(interface{ Heuristic() module.Bot })
			skills := module.Skills
			if hasNet {
				skills = module.AllSkills
			}
			for _, skill := range skills {
				r := row{module: id, variation: c.variation, seats: c.seats, skill: string(skill)}
				if skill == module.SkillAI {
					r.skill += "/net"
				}
				for i := 0; i < *matches; i++ {
					if err := play(mod, c, skill, *seed+int64(i), *decisions, &r); err != nil {
						fmt.Fprintf(os.Stderr, "%s/%s/%d/%s seed %d: %v\n", id, c.variation, c.seats, skill, *seed+int64(i), err)
						failed = true
					}
				}
				rows = append(rows, r)
			}
		}
	}

	if *asCSV {
		printCSV(rows)
	} else {
		printTable(rows)
	}
	if failed {
		pprof.StopCPUProfile()
		os.Exit(1)
	}
}

type config struct {
	variation string
	seats     int
}

// configs is every variation at its smallest and largest table. Seat count
// matters to cost: Hold'em's equity rollouts shrink as opponents are added,
// but each rollout evaluates every opponent's hand.
func configs(d module.ModuleDescriptor) []config {
	vars := d.Variations
	if len(vars) == 0 {
		vars = []module.VariationSpec{{}}
	}
	var out []config
	for _, v := range vars {
		lo, hi := d.MinPlayers, d.MaxPlayers
		if v.MinPlayers > 0 {
			lo = v.MinPlayers
		}
		if v.MaxPlayers > 0 {
			hi = v.MaxPlayers
		}
		if lo < 1 {
			lo = 1
		}
		if hi < lo {
			hi = lo
		}
		out = append(out, config{v.ID, lo})
		if hi != lo {
			out = append(out, config{v.ID, hi})
		}
	}
	return out
}

// row accumulates one configuration across its matches.
type row struct {
	module, variation, skill string
	seats                    int

	durs      []time.Duration
	cpus      []time.Duration
	allocs    uint64
	bytes     uint64
	declined  int // Act returned no move
	refused   int // Act's move was refused by Apply
	matches   int
	finished  int
	truncated int
}

var errStuck = errors.New("no seat could move")

func play(mod module.GameModule, c config, skill module.Skill, seed int64, maxDecisions int, r *row) error {
	players := make([]module.PlayerRef, c.seats)
	for i := range players {
		id := fmt.Sprintf("bot:%d", i+1)
		players[i] = module.PlayerRef{ID: id, Name: id}
	}
	state, err := mod.NewMatch(module.MatchConfig{Variation: c.variation}, players, seed)
	if err != nil {
		return fmt.Errorf("NewMatch: %w", err)
	}
	r.matches++
	bot := module.BotFor(mod)
	viewer := players[0].ID

	var before, after runtime.MemStats
	for n := 0; n < maxDecisions; n++ {
		done, _, err := mod.Finished(state)
		if err != nil {
			return fmt.Errorf("Finished: %w", err)
		}
		if done {
			r.finished++
			return nil
		}
		awaited := module.AwaitedSeats(mod, state, viewer, players)
		if len(awaited) == 0 {
			return errStuck
		}
		actor := awaited[0]
		offers, err := mod.LegalActions(state, actor)
		if err != nil {
			return fmt.Errorf("LegalActions: %w", err)
		}
		seat := module.BotSeat{PlayerID: actor, Skill: skill, Seed: module.SeatSeed(seed, actor, "bot")}

		runtime.ReadMemStats(&before)
		c0 := botstats.ProcessCPU()
		t0 := time.Now()
		a, ok := bot.Act(state, seat, offers)
		d := time.Since(t0)
		c := botstats.ProcessCPU() - c0
		runtime.ReadMemStats(&after)

		r.durs = append(r.durs, d)
		r.cpus = append(r.cpus, c)
		r.allocs += after.Mallocs - before.Mallocs
		r.bytes += after.TotalAlloc - before.TotalAlloc

		// Past this point it is the runtime's fallback, not the bot, and
		// none of it is timed: the offer list the loop falls back to is
		// what a bot that declined would cost the server anyway.
		var candidates []module.Action
		if ok {
			candidates = append(candidates, a)
		} else {
			r.declined++
		}
		candidates = append(candidates, module.ChooseActions(offers, nil)...)
		moved := false
		for i, cand := range candidates {
			next, _, err := mod.Apply(state, actor, cand)
			if err != nil {
				if i == 0 && ok {
					r.refused++
				}
				continue
			}
			state, moved = next, true
			break
		}
		if !moved {
			return fmt.Errorf("%s: %w", actor, errStuck)
		}
	}
	r.truncated++
	return nil
}

func quantile(sorted []time.Duration, q float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	i := int(q * float64(len(sorted)-1))
	return sorted[i]
}

type summary struct {
	n                        int
	mean, p50, p95, p99, max time.Duration
	cpuMean, cpuP95, cpuP99  time.Duration
	allocs, kb               float64
}

func (r row) summary() summary {
	s := summary{n: len(r.durs)}
	if s.n == 0 {
		return s
	}
	d := append([]time.Duration(nil), r.durs...)
	sort.Slice(d, func(i, j int) bool { return d[i] < d[j] })
	var sum time.Duration
	for _, x := range d {
		sum += x
	}
	s.mean = sum / time.Duration(s.n)
	s.p50, s.p95, s.p99, s.max = quantile(d, .50), quantile(d, .95), quantile(d, .99), d[len(d)-1]
	c := append([]time.Duration(nil), r.cpus...)
	sort.Slice(c, func(i, j int) bool { return c[i] < c[j] })
	var csum time.Duration
	for _, x := range c {
		csum += x
	}
	s.cpuMean, s.cpuP95, s.cpuP99 = csum/time.Duration(len(c)), quantile(c, .95), quantile(c, .99)
	s.allocs = float64(r.allocs) / float64(s.n)
	s.kb = float64(r.bytes) / float64(s.n) / 1024
	return s
}

func us(d time.Duration) string { return fmt.Sprintf("%.0f", float64(d)/float64(time.Microsecond)) }

func printTable(rows []row) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(w, "game\tvariation\tseats\tskill\tdecisions\tmean µs\tp50 µs\tp95 µs\tp99 µs\tmax µs\tcpu mean\tcpu p95\tcpu p99\tallocs/dec\tKB/dec\tdeclined\trefused\tfinished\t")
	for _, r := range rows {
		s := r.summary()
		v := r.variation
		if v == "" {
			v = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%.0f\t%.1f\t%d\t%d\t%d/%d\t\n",
			r.module, v, r.seats, r.skill, s.n,
			us(s.mean), us(s.p50), us(s.p95), us(s.p99), us(s.max),
			us(s.cpuMean), us(s.cpuP95), us(s.cpuP99),
			s.allocs, s.kb, r.declined, r.refused, r.finished, r.matches)
	}
	_ = w.Flush()
}

func printCSV(rows []row) {
	fmt.Println("game,variation,seats,skill,decisions,mean_us,p50_us,p95_us,p99_us,max_us,cpu_mean_us,cpu_p95_us,cpu_p99_us,allocs_per_decision,kb_per_decision,declined,refused,finished,matches")
	for _, r := range rows {
		s := r.summary()
		fmt.Printf("%s,%s,%d,%s,%d,%s,%s,%s,%s,%s,%s,%s,%s,%.0f,%.1f,%d,%d,%d,%d\n",
			r.module, r.variation, r.seats, r.skill, s.n,
			us(s.mean), us(s.p50), us(s.p95), us(s.p99), us(s.max),
			us(s.cpuMean), us(s.cpuP95), us(s.cpuP99),
			s.allocs, s.kb, r.declined, r.refused, r.finished, r.matches)
	}
}
