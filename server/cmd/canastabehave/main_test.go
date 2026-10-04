package main

import (
	"bytes"
	"strings"
	"testing"

	"zolik/server/internal/learn"
)

func contenders(t *testing.T, specs ...string) (learn.Game, []learn.Contender) {
	t.Helper()
	g, err := learn.Lookup("canasta")
	if err != nil {
		t.Fatal(err)
	}
	var out []learn.Contender
	for _, s := range specs {
		c, err := learn.ParseContender(g, s)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, c)
	}
	return g.(learn.Game), out
}

func sum(xs []int) int {
	n := 0
	for _, x := range xs {
		n += x
	}
	return n
}

// One whole heads-up match: every deal is counted once per seat and once per
// side, each deal ends exactly one way, the deal totals add up to the match
// score the bench measures, and the breakdown adds up to the totals.
func TestWatcherAccountsForEveryDeal(t *testing.T) {
	g, cs := contenders(t, "closer", "hard")
	bots := map[string]learn.Contender{labelA: cs[0], labelP: cs[0], labelB: cs[1]}
	rep, diff, st, err := play(g, 2, "classic", 11, 50_000, false, bots, false)
	if err != nil || st.Stalled || st.Illegal > 0 {
		t.Fatalf("%v %+v", err, st)
	}
	deals := rep.Table.Deals
	if deals < 1 || len(rep.Table.DealsMatch) != 1 || rep.Table.DealsMatch[0] != deals {
		t.Fatalf("%d deals, per match %v", deals, rep.Table.DealsMatch)
	}
	a, b := rep.Seat[labelA], rep.Seat[labelB]
	if a.SeatDeals != deals || b.SeatDeals != deals || rep.Seat[labelP].SeatDeals != 0 {
		t.Fatalf("seat-deals A %d, B %d, P %d over %d deals", a.SeatDeals, b.SeatDeals, rep.Seat[labelP].SeatDeals, deals)
	}
	for _, x := range []*seatTally{a, b} {
		if x.Out+x.Caught+x.Exhausted != x.SeatDeals || len(x.OutTurn) != x.Out || len(x.CaughtPoints) != x.Caught {
			t.Errorf("out %d + caught %d + exhausted %d != %d", x.Out, x.Caught, x.Exhausted, x.SeatDeals)
		}
		if x.Turns < x.SeatDeals || len(x.OpenTurn) > x.SeatDeals || len(x.QuotaTurn) > len(x.OpenTurn) {
			t.Errorf("turns %d, opened %d, quota %d over %d deals", x.Turns, len(x.OpenTurn), len(x.QuotaTurn), x.SeatDeals)
		}
	}
	if a.Out+b.Out+rep.Table.Exhausted != deals || a.Out != b.Caught || b.Out != a.Caught {
		t.Errorf("outs %d+%d, exhausted %d, over %d deals", a.Out, b.Out, rep.Table.Exhausted, deals)
	}
	sa, sb := rep.Side[labelA], rep.Side[labelB]
	if sa.Deals != deals || sb.Deals != deals {
		t.Fatalf("side deals %d %d", sa.Deals, sb.Deals)
	}
	if got := float64(sum(sa.Total) - sum(sb.Total)); got != diff {
		t.Errorf("deal totals differ by %v, match scores by %v", got, diff)
	}
	for _, s := range []*sideTally{sa, sb} {
		parts := sum(s.MeldCards) + sum(s.Canastas) + sum(s.RedThrees) + sum(s.GoingOut) - sum(s.InHand)
		if parts != sum(s.Total) {
			t.Errorf("breakdown adds to %d, totals to %d", parts, sum(s.Total))
		}
		n := 0
		for k := range s.NetBy {
			n += len(s.NetBy[k])
		}
		if n != deals || len(s.NetBy[endOwn]) == 0 && len(s.NetBy[endOther]) == 0 && rep.Table.Exhausted != deals {
			t.Errorf("deals by ending %d of %d", n, deals)
		}
	}
	if sum(sa.Net) != -sum(sb.Net) {
		t.Errorf("heads-up nets %d and %d do not cancel", sum(sa.Net), sum(sb.Net))
	}
	if len(sa.NetBy[endOwn]) != a.Out || len(sa.NetBy[endOther]) != b.Out {
		t.Errorf("side A's own-out deals %d, A went out %d", len(sa.NetBy[endOwn]), a.Out)
	}

	var out bytes.Buffer
	printReport(&out, rep, []float64{diff}, map[string]string{labelA: "closer", labelB: "hard"}, 2, "classic", 0, 0)
	for _, want := range []string{"per seat and deal", "score breakdown", "by how the deal ended", "out-goer's own turn"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("report lacks %q", want)
		}
	}
}

// At four seats a partner contender sits beside A, and the sides swap whole
// on the second seating.
func TestSeatingPutsThePartnerBesideA(t *testing.T) {
	players := learn.Players(4)
	sides := [][]string{{"p0", "p2"}, {"p1", "p3"}}
	label, side := seating(players, sides, false, true)
	want := map[string]string{"p0": labelA, "p2": labelP, "p1": labelB, "p3": labelB}
	for id, l := range want {
		if label[id] != l {
			t.Errorf("%s labelled %s, want %s", id, label[id], l)
		}
	}
	if side["p2"] != labelA || side["p3"] != labelB {
		t.Errorf("sides %v", side)
	}
	label, _ = seating(players, sides, true, false)
	if label["p1"] != labelA || label["p3"] != labelA || label["p0"] != labelB {
		t.Errorf("flipped labels %v", label)
	}
	label, _ = seating(learn.Players(2), nil, false, false)
	if label["p0"] != labelA || label["p1"] != labelB {
		t.Errorf("heads-up labels %v", label)
	}
}

// A partnership match with the partner set: the partner's seat-deals are
// counted apart from A's, and both belong to side A.
func TestPartnerSeatsAreCountedApart(t *testing.T) {
	if testing.Short() {
		t.Skip("a whole four-seat match")
	}
	g, cs := contenders(t, "closer", "hard", "medium")
	bots := map[string]learn.Contender{labelA: cs[0], labelP: cs[2], labelB: cs[1]}
	rep, _, st, err := play(g, 4, "classic", 5, 50_000, false, bots, true)
	if err != nil || st.Stalled || st.Illegal > 0 {
		t.Fatalf("%v %+v", err, st)
	}
	d := rep.Table.Deals
	if rep.Seat[labelA].SeatDeals != d || rep.Seat[labelP].SeatDeals != d || rep.Seat[labelB].SeatDeals != 2*d {
		t.Errorf("seat-deals A %d P %d B %d over %d deals", rep.Seat[labelA].SeatDeals, rep.Seat[labelP].SeatDeals, rep.Seat[labelB].SeatDeals, d)
	}
}

func TestIsWild(t *testing.T) {
	for c, want := range map[string]bool{"2S": true, "JOKER1": true, "10H": false, "KD": false, "3C": false} {
		if isWild(c) != want {
			t.Errorf("%s wild %v", c, !want)
		}
	}
}
