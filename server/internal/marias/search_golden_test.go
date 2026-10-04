package marias

import (
	"bufio"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"zolik/server/internal/module"
)

// The hard bot's search is bounded (searchLimits), and the bound is the only
// thing allowed to change its moves. The golden file holds the card and the
// node count the search chose at every card-play position of a seed sweep,
// recorded with goldenLimits by the search as it was before its solver was
// rewritten (and checked against that code). Under the same limits every
// position must be answered with the same card in the same number of
// nodes; under a smaller budget, every position that took no more than it.
//
// The positions come from matches between medium seats, whose play does not
// touch the search, so the sweep is the same however the search changes.
//
//	go test ./internal/marias -run SearchGolden -update-search-golden

var updateSearchGolden = flag.Bool("update-search-golden", false, "rewrite testdata/search_golden.txt")

const searchGoldenFile = "testdata/search_golden.txt"

// goldenLimits is the search the golden file was recorded with.
var goldenLimits = searchLimits{total: 2_000_000, perSample: 400_000}

type searchPosition struct {
	id    string // variation/seed/step
	state *GameState
	me    string
	legal []string
	seed  int64
}

// searchPositions are the card plays of medium-seat matches with more than
// one legal card.
func searchPositions(t *testing.T, seeds int64) []searchPosition {
	t.Helper()
	m := New()
	var out []searchPosition
	for _, variation := range []string{variationVoleny, variationLicit} {
		for seed := int64(1); seed <= seeds; seed++ {
			cfg := module.MatchConfig{Variation: variation, Options: module.Options{OptDeals: 9, module.OptPauseBetweenRounds: module.OptOff}}
			state, err := m.NewMatch(cfg, players, seed)
			if err != nil {
				t.Fatal(err)
			}
			for step := 0; ; step++ {
				s, err := decode(state)
				if err != nil {
					t.Fatal(err)
				}
				if s.Status != "active" {
					break
				}
				if step > 4000 {
					t.Fatalf("%s seed %d did not finish", variation, seed)
				}
				actor := s.Current
				offers, err := m.LegalActions(state, actor)
				if err != nil {
					t.Fatal(err)
				}
				if s.Phase == phasePlay {
					for _, o := range offers {
						if o.ID == OfferPlay && o.Enabled && len(o.Source.Cards) > 1 {
							out = append(out, searchPosition{
								id:    fmt.Sprintf("%s/%d/%d", variation, seed, step),
								state: s, me: actor, legal: o.Source.Cards,
								seed: seed*7919 + int64(step),
							})
						}
					}
				}
				a, ok := m.Bot().Act(state, module.BotSeat{PlayerID: actor, Skill: module.SkillMedium, Seed: seed}, offers)
				if !ok {
					t.Fatalf("%s seed %d: %s had no move", variation, seed, actor)
				}
				if state, _, err = m.Apply(state, actor, a); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	return out
}

// search is the card the search chose, or "-" where no sample could be
// solved and the bot falls back on its rules of thumb.
func (p searchPosition) search(lim searchLimits) (string, int) {
	c, nodes, ok := p.state.searchPlay(p.me, p.legal, rand.New(rand.NewSource(p.seed)), lim)
	if !ok {
		c = "-"
	}
	return c, nodes
}

func TestSearchGolden(t *testing.T) {
	if testing.Short() && !*updateSearchGolden {
		t.Skip("solves every position of a seed sweep")
	}
	positions := searchPositions(t, 4)

	if *updateSearchGolden {
		var b strings.Builder
		fmt.Fprintln(&b, "# position card nodes — recorded with goldenLimits; see search_golden_test.go")
		for _, p := range positions {
			c, nodes := p.search(goldenLimits)
			fmt.Fprintf(&b, "%s %s %d\n", p.id, c, nodes)
		}
		if err := os.WriteFile(searchGoldenFile, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}

	type want struct {
		card  string
		nodes int
	}
	golden := map[string]want{}
	f, err := os.Open(searchGoldenFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) != 3 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		n, err := strconv.Atoi(fields[2])
		if err != nil {
			t.Fatal(err)
		}
		golden[fields[0]] = want{fields[1], n}
	}
	if len(golden) != len(positions) {
		t.Fatalf("golden file has %d positions, the sweep %d: regenerate it from the commit that recorded it", len(golden), len(positions))
	}

	var nodes []int
	checked, over := 0, 0
	for _, p := range positions {
		g, ok := golden[p.id]
		if !ok {
			t.Fatalf("position %s is not in the golden file", p.id)
		}
		nodes = append(nodes, g.nodes)
		if defaultLimits != goldenLimits && g.nodes > defaultLimits.total {
			over++
			continue
		}
		c, n := p.search(defaultLimits)
		if c != g.card || n != g.nodes {
			t.Errorf("%s: plays %s in %d nodes, was %s in %d", p.id, c, n, g.card, g.nodes)
		}
		checked++
	}
	sort.Ints(nodes)
	q := func(f float64) int { return nodes[int(f*float64(len(nodes)-1))] }
	t.Logf("%d positions unchanged under %+v; %d over its budget. nodes p50 %d p90 %d p95 %d p99 %d max %d",
		checked, defaultLimits, over, q(.5), q(.9), q(.95), q(.99), nodes[len(nodes)-1])
}
