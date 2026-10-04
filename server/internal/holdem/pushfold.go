package holdem

import "math"

// Short stacks: jam or fold, from the chart (pushfold_table.go).
//
// Once the effective stack is short, a raise that is not all in commits most
// of it anyway, and a flop seen with a few big blinds behind is a flop with no
// play left in it. The jam-or-fold game is near-equilibrium there — Miltersen
// & Sørensen (AAMAS 2007) solved the heads-up tournament restricted to it and
// found the restriction gives away little at those depths — so at or below
// profile.pushFoldBB Hard stops scoring hands with Chen's formula and plays
// the heads-up equilibrium chart instead, the small blind's jams for opening
// and the big blind's calls for answering.
//
// The chart is heads-up. With more players in, it is extended conservatively
// rather than solved — nothing peer-reviewed covers more than three, and
// Ganzfried & Sandholm's (AAMAS 2008) three-way solve is a tournament one:
//
//	opening into k > 1 opponents   jam the hands the big blind *calls* with
//	                               at (k-1) times the stack: the calling range
//	                               is the tighter one, and every extra player
//	                               left to wake up with a hand deepens it again
//	answering with k > 1 in        the calling range at k times the stack
//
// so a hand that jams into a full table is one that would call a jam from
// much deeper, heads-up.

// effectiveBB is the effective stack at the start of this betting round, in
// big blinds: the seat's own chips, or the deepest opponent's if that is less.
func effectiveBB(s *GameState, seat *Seat) float64 {
	deepest := 0
	for i := range s.Seats {
		o := &s.Seats[i]
		if o == seat || !o.inHand() {
			continue
		}
		deepest = max(deepest, o.Stack+o.Bet)
	}
	eff := min(seat.Stack+seat.Bet, deepest)
	return float64(eff) / float64(max(s.BigBlind, 1))
}

// inChart reports whether a class plays at this many big blinds by a row of
// the chart: entries are the deepest stack it plays at, in half big blinds.
func inChart(row *[169]uint8, class int, bb float64) bool {
	up := row[class]
	if up == pushFoldBeyond {
		return true
	}
	return 2*math.Max(bb, 1) <= float64(up)
}

// pushFold is the short-stack preflop decision, or false when the stack is not
// short enough for the chart to be the game (or the opponent is one it
// is not played against; see profile.nonFolder).
func pushFold(s *GameState, seat *Seat, mn menu, p profile) (choice, bool) {
	if p.pushFoldBB <= 0 {
		return choice{}, false
	}
	bb := effectiveBB(s, seat)
	if bb > p.pushFoldBB || bb <= 0 {
		return choice{}, false
	}
	k := opponentsOf(s)
	if k < 1 {
		return choice{}, false
	}
	class := classOf(seat.Hole)
	owed := s.toCall(seat)

	// Nobody has raised: this seat opens, or not. The big blind with a limp in
	// front of it opens too — the limper's hand is not the range a jam is
	// answering. Heads-up against a player who does not fold to raises, a jam
	// has no fold equity to collect, and the chart gives way (nonFolder).
	if s.CurrentBet <= s.BigBlind {
		if k == 1 && !foldsToRaises(opponentReads(s, seat), p.nonFolder) {
			return choice{}, false
		}
		jam := inChart(&pushFoldJam, class, bb)
		if k > 1 {
			jam = inChart(&pushFoldCall, class, bb*float64(k-1))
		}
		switch {
		case jam:
			return choice{verb: VerbRaise, to: shove(mn)}, true
		case owed == 0:
			return choice{verb: VerbCheck}, true
		default:
			return choice{verb: VerbFold}, true
		}
	}

	// Somebody has raised. All in or not, at these depths it is a jam to
	// answer: call it all in, or re-jam over a raise that left chips behind,
	// with the hands that call a jam.
	if !inChart(&pushFoldCall, class, bb*float64(k)) {
		return choice{verb: VerbFold}, true
	}
	if owed >= seat.Stack || !mn.can(VerbRaise) {
		return choice{verb: VerbCall}, true
	}
	return choice{verb: VerbRaise, to: shove(mn)}, true
}

// foldsToRaises reports whether a heads-up opponent has shown the fold equity
// an open jam is for (profile.nonFolder): at least one fold to a raise before
// the flop, and enough of them once there are enough to judge by.
func foldsToRaises(r *SeatReads, share float64) bool {
	if share <= 0 {
		return true
	}
	if r == nil || r.FoldedToRaise == 0 {
		return false
	}
	return folds(r.FoldedToRaise, r.FacedRaise, share)
}
