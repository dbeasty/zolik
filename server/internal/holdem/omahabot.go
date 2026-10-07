package holdem

import (
	"math/rand"

	"zolik/server/internal/module"
)

// How a bot plays Pot-Limit Omaha.
//
// None of the Hold'em bot's hand reading carries over. A Chen score rates two
// cards; Omaha deals four, and a hand is made of exactly two of them with
// exactly three of the board, so "a pair with a card on the board" and "four
// to a flush" mean different things here. What does carry over is the
// arithmetic — equity against the price — which is betOnEquity, shared with
// Five-Card Draw. The equity is a rollout: opponents' four cards and the rest
// of the board dealt from the cards this seat cannot see, every hand scored by
// Omaha's own rule (omahaScore).
//
// The same disciplines as bot.go: it reads its own cards, the board and the
// public record and nothing else, and every move it makes is an offer it was
// handed, sized inside that offer's range — which at a pot-limit table is the
// pot, not the stack.
//
// Hard, as in Draw, reads a bet as a claim: half of a betting opponent's range
// is dealt as a hand that already holds at least two pair on the board as it
// stands. Omaha's hands run close and the nuts change street by street, so a
// bet is worth more as evidence here than in Hold'em, not less.

// omahaAct is one betting decision at an Omaha table.
func omahaAct(s *GameState, seat *Seat, mn menu, p profile, rnd *rand.Rand) (module.Action, bool) {
	opponents := opponentsOf(s)
	if opponents < 1 {
		return mn.action(choice{verb: VerbCheck})
	}
	eq := omahaEquity(s, seat, p, omahaTrials(p, opponents), rnd)
	bluffSpot := s.Street == streetTurn || s.Street == streetRiver
	return mn.action(betOnEquity(s, seat, mn, p, rnd, eq, opponents, bluffSpot))
}

// omahaTrials trades accuracy for time as the table fills: every opponent
// costs sixty five-card scores a trial.
func omahaTrials(p profile, opponents int) int {
	n := 300
	switch p.skill {
	case module.SkillHard:
		n = 400
	case module.SkillEasy:
		n = 150
	}
	if opponents > 3 {
		n = n * 2 / 3
	}
	return n
}

// omahaClaimTrust and omahaClaimFloor: how much of a betting opponent's range
// Hard deals as a made hand, and how good a hand. Measured — see
// TestOmahaBotLadderIsOrdered.
const (
	omahaClaimTrust = 0.5
	omahaClaimFloor = twoPair
)

// omahaEquity is how often this seat's four cards win, dealt out.
func omahaEquity(s *GameState, seat *Seat, p profile, trials int, rnd *rand.Rand) float64 {
	var seen uint64
	for _, cs := range [][]string{seat.Hole, s.Board} {
		for _, c := range cs {
			if r, su, ok := cardIndex(c); ok {
				seen |= 1 << (r*4 + su)
			}
		}
	}
	var poolBuf [52]code
	pool := poolBuf[:0]
	for _, c := range deckCodes {
		if seen&(1<<c) == 0 {
			pool = append(pool, c)
		}
	}

	var claims []bool
	for i := range s.Seats {
		o := &s.Seats[i]
		if o == seat || !o.inHand() {
			continue
		}
		claims = append(claims, p.skill == module.SkillHard && len(s.Board) >= 3 &&
			o.Bet > 0 && o.Bet == s.CurrentBet && o.Acted)
	}
	if len(claims) == 0 {
		return 1
	}

	mine := codesOf(seat.Hole)
	board := codesOf(s.Board)
	runout := 5 - len(board)
	won := 0.0
	for t := 0; t < trials; t++ {
		d := dealer{pool: pool, rnd: rnd}
		theirs := make([][4]code, len(claims))
		for i, claim := range claims {
			claim = claim && rnd.Float64() < omahaClaimTrust
			for tries := 0; ; tries++ {
				d.deal(theirs[i][:])
				if !claim || tries >= 24 || scoreCategory(omahaScore(theirs[i][:], board)) >= omahaClaimFloor {
					break
				}
				d.undeal(4)
			}
		}
		full := append(make([]code, 0, 5), board...)
		for j := 0; j < runout; j++ {
			full = append(full, d.next())
		}
		best := omahaScore(mine, full)
		ahead, split := true, 1
		for i := range theirs {
			sc := omahaScore(theirs[i][:], full)
			switch {
			case sc > best:
				ahead = false
			case sc == best:
				split++
			}
			if !ahead {
				break
			}
		}
		if ahead {
			won += 1 / float64(split)
		}
	}
	return won / float64(trials)
}

// omahaScore ranks the best hand of exactly two hole cards and three of the
// board, with the board as it stands (three to five cards) — the same order
// BestUsing gives, as a packed score (TestOmahaScoreAgreesWithBestUsing).
func omahaScore(hole, board []code) uint32 {
	var best uint32
	for a := 0; a < len(board); a++ {
		for b := a + 1; b < len(board); b++ {
			for c := b + 1; c < len(board); c++ {
				var tri hand
				tri.add(board[a])
				tri.add(board[b])
				tri.add(board[c])
				for i := 0; i < len(hole); i++ {
					for j := i + 1; j < len(hole); j++ {
						h := tri
						h.add(hole[i])
						h.add(hole[j])
						if sc := h.score(); sc > best {
							best = sc
						}
					}
				}
			}
		}
	}
	return best
}
