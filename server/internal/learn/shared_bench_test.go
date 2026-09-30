package learn_test

import (
	"math/rand"
	"testing"
	"time"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
)

// What one bot move and one environment decision cost, per game, with a net
// the size the trainer builds (ml/configs: trunk 256x256, scorer 128, value
// 128). The weights are random: the cost of a forward pass does not depend on
// what the weights are.
//
//	go test ./internal/learn/ -run XXX -bench 'BotMove|EnvDecisions' -benchtime 3s

func BenchmarkBotMove(b *testing.B) {
	for _, tc := range sharedTables {
		b.Run(tc.name, func(b *testing.B) {
			g := mustGame(b, tc.game)
			ds := collectDecisions(b, g, tc.variation, tc.seats, 200)
			bot := benchBot(g, randomNet(g, []int{256, 256}, []int{128}, 1))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				d := ds[i%len(ds)]
				if _, ok := bot.Act(d.state, module.BotSeat{PlayerID: d.seat, Skill: module.SkillHard}, d.offers); !ok {
					b.Fatal("no move")
				}
			}
		})
	}
}

func BenchmarkEnvDecisions(b *testing.B) {
	for _, tc := range sharedTables {
		b.Run(tc.name, func(b *testing.B) {
			g := mustGame(b, tc.game)
			// Half the seats learn, half are the heuristic: every Apply asks
			// for a reward for several learner seats.
			plan := make([]string, tc.seats)
			for i := range plan {
				plan[i] = learn.Learner
				if i%2 == 1 {
					plan[i] = "hard"
				}
			}
			specs := make([]learn.TableSpec, 32)
			for i := range specs {
				specs[i] = learn.TableSpec{Plan: plan}
			}
			env, err := learn.NewEnv(g, tc.variation, specs, 1, 20000)
			if err != nil {
				b.Fatal(err)
			}
			obs, err := env.Observe()
			if err != nil {
				b.Fatal(err)
			}
			rnd := rand.New(rand.NewSource(1))
			b.ResetTimer()
			start := time.Now()
			for i := 0; i < b.N; i++ {
				choices := make([]int, len(obs))
				for j, o := range obs {
					choices[j] = rnd.Intn(len(o.Cands))
				}
				if obs, err = env.Step(choices); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(b.N*len(specs))/time.Since(start).Seconds(), "decisions/s")
		})
	}
}
