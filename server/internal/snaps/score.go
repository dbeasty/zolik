package snaps

// The scoring of a deal, as pure functions of the count. The engine and the
// bot's search both end deals through these, so a solved line is scored
// exactly as the table would score it.

// tally is one deal's count, by seat.
type tally struct {
	points    [2]int // card points taken
	tricks    [2]int
	marriages [2]int // announced, counted once a trick is taken
}

// total is a seat's count toward 66.
func (t tally) total(i int) int {
	if t.tricks[i] > 0 {
		return t.points[i] + t.marriages[i]
	}
	return t.points[i]
}

// closing is the talon's state for the scoring: who closed it, if anyone,
// and the other player's tricks and count at that moment.
type closing struct {
	by        int // seat, or -1
	oppTricks int
	oppPoints int
}

var notClosed = closing{by: -1}

// byLoser is what going out is worth against a loser with these tricks and
// this count: 3 if they took none (schwarz), 2 if they are under 33
// (schneider), otherwise 1.
func byLoser(tricks, total int) int {
	switch {
	case tricks == 0:
		return 3
	case total < schneider:
		return 2
	}
	return 1
}

// outPoints is what seat w scores for going out.
func (r ruleset) outPoints(t tally, c closing, w int) int {
	l := 1 - w
	switch {
	case c.by == -1:
		return byLoser(t.tricks[l], t.total(l))
	case c.by == w:
		if r.scoreAtClosing {
			return byLoser(c.oppTricks, c.oppPoints)
		}
		return byLoser(t.tricks[l], t.total(l))
	}
	// The closer's opponent went out first.
	if r.lateOutPays3 && c.oppTricks == 0 {
		return 3
	}
	return 2
}

// closerFailed is what the closer's opponent scores when the closer runs
// out of cards short of 66: 3 if they had no trick when the talon was
// closed, otherwise 2.
func closerFailed(c closing) int {
	if c.oppTricks == 0 {
		return 3
	}
	return 2
}

// runOut settles a deal whose cards ran out with the talon used up and
// nobody at 66: the winner's seat (-1 for a drawn deal) and their points.
func (r ruleset) runOut(t tally, lastWinner int) (winner, points int) {
	if r.lastTrickWins {
		return lastWinner, 1
	}
	a, b := t.total(0), t.total(1)
	switch {
	case a > b:
		return 0, byLoser(t.tricks[1], b)
	case b > a:
		return 1, byLoser(t.tricks[0], a)
	}
	return -1, 0
}
