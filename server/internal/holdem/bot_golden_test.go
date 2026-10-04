package holdem

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zolik/server/internal/module"
)

var updateGolden = flag.Bool("update-golden", false, "rewrite testdata/bot_golden.txt from the current bot")

// goldenPath holds one line per configuration: its name, how many decisions it
// played and a hash of every one of them.
var goldenPath = filepath.Join("testdata", "bot_golden.txt")

// TestBotMovesAreGolden pins every move the bot makes over a fixed sweep of
// seeds, at every skill, both variations and three table sizes — and, beside
// them, the exact bits equityBoth returns over a sweep of spots.
//
// It exists for changes that are meant to make the bot cheaper and nothing
// else: the rollouts behind every post-flop decision draw from the same coin
// as the bluffs and sizings do, so an optimisation that deals one card in a
// different order, or skips one draw, moves every decision after it. A
// difference here is a changed bot, whatever the change was for. When the bot
// is meant to change, regenerate with
//
//	go test ./internal/holdem/ -run TestBotMovesAreGolden -update-golden
func TestBotMovesAreGolden(t *testing.T) {
	got := map[string]string{}
	var order []string
	record := func(name string, n int, h []byte) {
		order = append(order, name)
		got[name] = fmt.Sprintf("%d %s", n, hex.EncodeToString(h)[:16])
	}

	m := New()
	for _, variation := range []string{"freezeout", "timed"} {
		for _, seats := range []int{2, 6, 9} {
			for _, skill := range module.Skills {
				name := fmt.Sprintf("moves/%s/%d/%s", variation, seats, skill)
				h := sha256.New()
				n := 0
				for seed := int64(1); seed <= 3; seed++ {
					n += goldenMatch(t, m, variation, seats, skill, seed, 250, func(line string) {
						fmt.Fprintln(h, line)
					})
				}
				record(name, n, h.Sum(nil))
			}
		}
	}

	// The equity sweep: random spots at every street, every floor and claim,
	// at the trial counts the bot actually asks for.
	h := sha256.New()
	rnd := rand.New(rand.NewSource(7))
	deck := buildDeck()
	const spots = 400
	for i := 0; i < spots; i++ {
		perm := rnd.Perm(52)
		hole := []string{deck[perm[0]], deck[perm[1]]}
		var board []string
		for j := 0; j < []int{0, 3, 4, 5}[i%4]; j++ {
			board = append(board, deck[perm[2+j]])
		}
		opponents := 1 + i%8
		cl := claim{floor: []int{highCard, pair, twoPair}[i%3], now: i%5 == 0}
		share := []float64{0, 0.3, 1, 0.55}[i%4]
		a, b := equityBoth(hole, board, opponents, rollouts(opponents), cl, share, rand.New(rand.NewSource(int64(i))))
		fmt.Fprintf(h, "%x %x\n", math.Float64bits(a), math.Float64bits(b))
	}
	record("equity", spots, h.Sum(nil))

	if *updateGolden {
		var sb strings.Builder
		for _, k := range order {
			fmt.Fprintf(&sb, "%s %s\n", k, got[k])
		}
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, []byte(sb.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}

	f, err := os.Open(goldenPath)
	if err != nil {
		t.Fatalf("%v (regenerate with -update-golden)", err)
	}
	defer f.Close()
	want := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if k, v, ok := strings.Cut(sc.Text(), " "); ok {
			want[k] = v
		}
	}
	for _, k := range order {
		if want[k] != got[k] {
			t.Errorf("%s: got %q, golden %q", k, got[k], want[k])
		}
	}
	if len(want) != len(order) {
		t.Errorf("golden has %d configurations, the sweep %d", len(want), len(order))
	}
}

// goldenMatch plays one match the way the server's bot loop and cmd/botcost
// do — a bot at every seat, seeded per seat — passing each decision to emit,
// and returns how many it made.
func goldenMatch(t *testing.T, m *Module, variation string, seats int, skill module.Skill, seed int64, maxDecisions int, emit func(string)) int {
	t.Helper()
	players := make([]module.PlayerRef, seats)
	for i := range players {
		id := fmt.Sprintf("bot:%d", i+1)
		players[i] = module.PlayerRef{ID: id, Name: id}
	}
	state, err := m.NewMatch(module.MatchConfig{Variation: variation}, players, seed)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}
	bot := module.BotFor(m)
	for n := 0; n < maxDecisions; n++ {
		done, _, err := m.Finished(state)
		if err != nil {
			t.Fatalf("Finished: %v", err)
		}
		if done {
			return n
		}
		awaited := module.AwaitedSeats(m, state, players[0].ID, players)
		if len(awaited) == 0 {
			t.Fatalf("seed %d: no seat could move", seed)
		}
		actor := awaited[0]
		offers, err := m.LegalActions(state, actor)
		if err != nil {
			t.Fatalf("LegalActions: %v", err)
		}
		a, ok := bot.Act(state, module.BotSeat{PlayerID: actor, Skill: skill, Seed: module.SeatSeed(seed, actor, "bot")}, offers)
		if !ok {
			t.Fatalf("seed %d: %s had no move", seed, actor)
		}
		emit(fmt.Sprintf("%s %s %v", actor, a.Verb, a.Params[ParamAmount]))
		next, _, err := m.Apply(state, actor, a)
		if err != nil {
			t.Fatalf("seed %d: %s proposed an illegal %v: %v", seed, actor, a, err)
		}
		state = next
	}
	return maxDecisions
}
