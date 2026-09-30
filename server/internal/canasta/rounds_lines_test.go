package canasta

import (
	"encoding/json"
	"regexp"
	"testing"

	"zolik/server/internal/module"
)

// The account adds up, in every deal of real matches under every variation.
//
// Five seeds each, and Samba among them: the shapes that break an account —
// a group canasta still taking cards, a going-out meld of black threes, red
// threes short of the canastas they need — only turn up in some shuffles, and
// most of them only in Samba.
func TestEveryDealsAccountAddsUp(t *testing.T) {
	m := New()
	players := refs("p1", "p2", "p3", "p4")
	for _, variation := range []string{"classic", "modern_american", "samba"} {
		deals := 0
		for seed := int64(1); seed <= 5; seed++ {
			cfg := module.MatchConfig{Variation: variation, Options: module.Options{OptTargetScore: 3000}}
			state, err := m.NewMatch(cfg, players, seed)
			if err != nil {
				t.Fatal(err)
			}
			final, _, err := module.PlayWithOffers(m, state, players, module.DriverOptions{
				MaxActions: 8000, Prefer: driverPrefer,
			})
			if err != nil {
				t.Fatalf("%s seed %d: %v", variation, seed, err)
			}
			s, err := decode(final)
			if err != nil {
				t.Fatal(err)
			}
			log, err := m.Rounds(final)
			if err != nil {
				t.Fatal(err)
			}
			// The account is public and permanent, so it describes melds and
			// hands without naming a card: over fifteen matches rather than the
			// one the cross-module test plays.
			if blob, _ := json.Marshal(log); cardCode.Match(blob) {
				t.Errorf("%s seed %d: the round log names a card: %s", variation, seed, cardCode.Find(blob))
			}
			for i, r := range log.Rounds {
				deals++
				for _, rs := range r.Scores {
					if len(rs.Lines) == 0 && rs.Delta != 0 {
						t.Errorf("%s seed %d deal %d: %s moved %d with no account", variation, seed, r.Number, rs.PlayerID, rs.Delta)
					}
					if err := module.CheckLines(rs); err != nil {
						t.Errorf("%s seed %d deal %d: %v\n%s", variation, seed, r.Number, err, dump(rs.Lines))
					}
				}
				// The detail is all there, not dropped as a fallback.
				for _, tr := range s.Deals[i].Teams {
					if tr.MeldCards != 0 && len(tr.Melds) == 0 {
						t.Errorf("%s seed %d deal %d: team %d scored melds with no tally", variation, seed, r.Number, tr.TeamID)
					}
					if tr.InHand != 0 && len(tr.Hands) == 0 {
						t.Errorf("%s seed %d deal %d: team %d caught in hand with no tally", variation, seed, r.Number, tr.TeamID)
					}
				}
			}
		}
		if deals == 0 {
			t.Errorf("%s: no deal was scored, so nothing was checked", variation)
		}
		t.Logf("%s: %d deals checked", variation, deals)
	}
}

var cardCode = regexp.MustCompile(`"([2-9TJQKA][HDSC]|JOKER\d?)"`)

func dump(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func findLine(ls []module.ScoreLine, key string) *module.ScoreLine {
	for i := range ls {
		if ls[i].LabelKey == key {
			return &ls[i]
		}
	}
	return nil
}

func linesFor(t *testing.T, s *GameState, wentOut string, concealed bool) (DealResult, []module.ScoreLine, []module.ScoreLine) {
	t.Helper()
	res := scoreDeal(s, wentOut, concealed, wentOut == "")
	r := s.rules()
	a := teamLines(r, res, res.Teams[0])
	b := teamLines(r, res, res.Teams[1])
	for i, ls := range [][]module.ScoreLine{a, b} {
		if err := module.CheckLines(module.RoundScore{PlayerID: "team", Delta: res.Teams[i].Total, Lines: ls}); err != nil {
			t.Fatalf("team %d: %v\n%s", i, err, dump(ls))
		}
	}
	return res, a, b
}

func twoSides(r ruleset, a, b Team, hands map[string][]string) *GameState {
	a.ID, a.Players = 0, []string{"p1", "p3"}
	b.ID, b.Players = 1, []string{"p2", "p4"}
	return &GameState{
		Players:   []string{"p1", "p2", "p3", "p4"},
		TurnOrder: []string{"p1", "p2", "p3", "p4"},
		TeamOf:    map[string]int{"p1": 0, "p3": 0, "p2": 1, "p4": 1},
		Teams:     []Team{a, b},
		Hands:     hands,
		Rules:     &r,
	}
}

var naturalKings = Meld{Rank: "K", Cards: []string{"KH", "KD", "KS", "KC", "KH", "KD", "KS"}}
var mixedQueens = Meld{Rank: "Q", Cards: []string{"QH", "QD", "QS", "QC", "QH", "2D", "JOKER1"}}

// A partnership's account names each thing it was paid for, and the lines
// that are sums are sums of the lines under them.
func TestTeamLinesBreakTheDealDown(t *testing.T) {
	s := twoSides(classicRules(),
		Team{Melds: []Meld{naturalKings, mixedQueens, {Rank: "5", Cards: []string{"5H", "5D", "5S"}}},
			RedThrees: []string{"3H", "3D"}},
		Team{Melds: []Meld{{Rank: "8", Cards: []string{"8H", "8D", "8S"}}}, RedThrees: []string{"3H"}},
		map[string][]string{
			"p1": {}, "p3": {"AS", "4D"},
			"p2": {"3S", "3C", "2H", "JOKER2", "9D"}, "p4": {"KC"},
		})
	_, a, b := linesFor(t, s, "p1", false)

	if l := findLine(a, "canasta.line.meldCards"); l == nil || len(l.Sub) != 3 {
		t.Fatalf("melds should be listed one by one: %s", dump(a))
	}
	c := findLine(a, "canasta.line.canastas")
	if c == nil || len(c.Sub) != 2 ||
		c.Sub[0].LabelKey != "canasta.line.natural" || c.Sub[0].Points != 500 ||
		c.Sub[1].LabelKey != "canasta.line.mixed" || c.Sub[1].Points != 300 {
		t.Errorf("canastas should split natural and mixed: %s", dump(c))
	}
	if l := findLine(a, "canasta.line.redThrees"); l == nil || l.Points != 200 {
		t.Errorf("two red threes with a canasta are +200: %s", dump(a))
	}
	if l := findLine(a, "canasta.line.goingOut"); l == nil || l.Params["player"] != "p1" {
		t.Errorf("going out should name who went out: %s", dump(a))
	}
	// The partner who did not go out is caught; the one who did is not listed.
	if l := findLine(a, "canasta.line.inHand"); l == nil || len(l.Sub) != 1 || l.Sub[0].Params["player"] != "p3" {
		t.Errorf("only the partner still holding cards is listed: %s", dump(l))
	}

	// The other side: one red three and no canasta counts against them, with
	// the reason; the hand is split into black threes, wilds and the rest.
	if l := findLine(b, "canasta.line.redThreesShort"); l == nil || l.Points != -100 ||
		l.Params["need"] != 1 || l.Params["made"] != 0 {
		t.Errorf("a red three short of a canasta should say so: %s", dump(b))
	}
	in := findLine(b, "canasta.line.inHand")
	if in == nil || len(in.Sub) != 2 {
		t.Fatalf("both partners were caught: %s", dump(in))
	}
	p2 := in.Sub[0]
	if p2.Points != -(200 + 70 + 10) || len(p2.Sub) != 3 {
		t.Errorf("p2's hand: two black threes, two wilds, a nine: %s", dump(p2))
	}
	if p4 := in.Sub[1]; p4.Points != -10 || p4.Sub != nil {
		t.Errorf("a hand of one kind is not broken down again: %s", dump(p4))
	}
}

func TestTeamLinesRedThreesAllAndConcealed(t *testing.T) {
	s := twoSides(classicRules(),
		Team{Melds: []Meld{naturalKings}, RedThrees: []string{"3H", "3D", "3H", "3D"}},
		Team{},
		map[string][]string{"p1": {}, "p3": {}, "p2": {}, "p4": {}})
	_, a, _ := linesFor(t, s, "p1", true)
	if l := findLine(a, "canasta.line.redThreesAll"); l == nil || l.Points != 800 {
		t.Errorf("all four red threes pay 800: %s", dump(a))
	}
	if l := findLine(a, "canasta.line.goingOutConcealed"); l == nil || l.Points != classicRules().ConcealedBonus {
		t.Errorf("a concealed go-out is named as one: %s", dump(a))
	}
}

// Samba has no concealed bonus and pays the ordinary one; the line says what
// was paid. A samba is its own kind of canasta, and red threes short of two
// canastas cost a flat hundred each.
func TestTeamLinesSamba(t *testing.T) {
	r := sambaRules()
	samba := Meld{Kind: meldRun, Suit: "H", Cards: []string{"4H", "5H", "6H", "7H", "8H", "9H", "TH"}}
	s := twoSides(r,
		Team{Melds: []Meld{samba, naturalKings}},
		Team{Melds: []Meld{naturalKings}, RedThrees: []string{"3H", "3D", "3H", "3D", "3H", "3D"}},
		map[string][]string{"p1": {}, "p3": {}, "p2": {}, "p4": {}})
	_, a, b := linesFor(t, s, "p1", true)
	if findLine(a, "canasta.line.goingOutConcealed") != nil || findLine(a, "canasta.line.goingOut") == nil {
		t.Errorf("Samba pays a concealed hand as ordinary: %s", dump(a))
	}
	c := findLine(a, "canasta.line.canastas")
	if c == nil || findLine(c.Sub, "canasta.line.samba") == nil || findLine(c.Sub, "canasta.line.natural") == nil {
		t.Errorf("a samba and a natural are listed apart: %s", dump(c))
	}
	if l := findLine(b, "canasta.line.redThreesShort"); l == nil || l.Points != -600 {
		t.Errorf("six red threes, one canasta of two needed, flat: %s", dump(b))
	}
}

// A deal scored before the tallies were kept still has an account: its sums,
// one line each, which still add up.
func TestTeamLinesForADealKeptBeforeTallies(t *testing.T) {
	old := []byte(`{"teamId":0,"meldCards":210,"canastas":500,"redThrees":100,"goingOut":100,"inHand":35,"total":875,"running":875}`)
	var tr TeamResult
	if err := json.Unmarshal(old, &tr); err != nil {
		t.Fatal(err)
	}
	ls := teamLines(classicRules(), DealResult{WentOut: "p1"}, tr)
	if err := module.CheckLines(module.RoundScore{Delta: tr.Total, Lines: ls}); err != nil {
		t.Fatalf("%v\n%s", err, dump(ls))
	}
	if len(ls) != 5 {
		t.Errorf("want the five sums, got %s", dump(ls))
	}
	for _, l := range ls {
		if l.Sub != nil {
			t.Errorf("an old deal has no detail to break down: %s", dump(l))
		}
	}
	if findLine(ls, "canasta.line.redThreesSum") == nil {
		t.Errorf("red threes with no count are shown as their sum: %s", dump(ls))
	}
}
