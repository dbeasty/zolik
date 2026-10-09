package snaps

import (
	"zolik/server/internal/tricks"
)

// The hard bot's search: the strict phase solved exactly.
//
// Once the talon is closed or used up nothing more is drawn, so the deal is
// the two hands played out under the strict rules — a small tree. With the
// talon used up every unseen card is in the other hand and the position is
// open; with it closed the other hand is sampled from the unseen cards and
// each sample solved. Deals end through score.go, as the engine ends them.

// position is a strict-phase deal, by seat.
type position struct {
	r      ruleset
	trump  byte
	hands  [2][]string
	t      tally
	close  closing
	closed bool // closed rather than used up: no last-trick bonus
	leader int
	led    string // the card led to the trick in progress, or ""
	marry  bool   // marriages may still be announced
	nodes  int
	limit  int
	cutoff bool
	// exhaustive turns alpha-beta off, for checking it against plain
	// minimax.
	exhaustive bool
}

// signed is a deal's worth in game points to seat 0: positive when seat 0
// wins it.
func signed(seat, points int) int {
	if seat == 0 {
		return points
	}
	return -points
}

// solve is the best value seat `me` can force from p, and the card that
// forces it when it is me to play. ok is false if the node limit ran out.
func (p *position) solve(me int) (best string, value int, ok bool) {
	toPlay := p.leader
	if p.led != "" {
		toPlay = 1 - p.leader
	}
	cards := p.legal(toPlay)
	v, card := p.search(cards, toPlay, -4, 4)
	if me == 1 {
		v = -v
	}
	return card, v, !p.cutoff
}

func (p *position) legal(seat int) []string {
	hand := p.hands[seat]
	if p.led == "" {
		return append([]string(nil), hand...)
	}
	return tricks.Legal(hand, tricks.Trick{Plays: []tricks.Play{{Seat: p.leader, Card: p.led}}}, p.trump, order, tricks.FollowBeatTrump)
}

// search is negamax-free minimax from seat 0's side, with alpha-beta: seat
// 0 maximises, seat 1 minimises.
func (p *position) search(cards []string, seat int, alpha, beta int) (int, string) {
	p.nodes++
	if p.limit > 0 && p.nodes > p.limit {
		p.cutoff = true
		return 0, cards[0]
	}
	bestCard := ""
	best := 4
	if seat == 0 {
		best = -4
	}
	for _, c := range cards {
		v := p.after(seat, c, alpha, beta)
		if seat == 0 {
			if v > best || bestCard == "" {
				best, bestCard = v, c
			}
			alpha = max(alpha, v)
		} else {
			if v < best || bestCard == "" {
				best, bestCard = v, c
			}
			beta = min(beta, v)
		}
		if alpha >= beta && !p.exhaustive {
			break
		}
	}
	return best, bestCard
}

// after is the value once seat has played card c.
func (p *position) after(seat int, c string, alpha, beta int) int {
	saveHand := p.hands[seat]
	p.hands[seat] = removeCard(p.hands[seat], c)
	defer func() { p.hands[seat] = saveHand }()

	if p.led == "" {
		saveT := p.t
		if p.marry {
			if o := partner(c); o != "" && hasCard(saveHand, o) {
				if string(tricks.Suit(c)) == string(p.trump) {
					p.t.marriages[seat] += marriageTrump
				} else {
					p.t.marriages[seat] += marriagePlain
				}
				if p.t.tricks[seat] > 0 && p.t.total(seat) >= goal {
					v := signed(seat, p.r.outPoints(p.t, p.close, seat))
					p.t = saveT
					return v
				}
			}
		}
		p.led, p.leader = c, seat
		v, _ := p.search(p.legal(1-seat), 1-seat, alpha, beta)
		p.led = ""
		p.t = saveT
		return v
	}

	// Second card: the trick is complete.
	lead := tricks.Play{Seat: p.leader, Card: p.led}
	trick := tricks.Trick{Plays: []tricks.Play{lead, {Seat: seat, Card: c}}}
	win, _ := trick.Winning(p.trump, order)
	w := win.Seat
	saveT, saveLed, saveLeader := p.t, p.led, p.leader
	defer func() { p.t, p.led, p.leader = saveT, saveLed, saveLeader }()

	p.t.points[w] += cardPoints(p.led) + cardPoints(c)
	p.t.tricks[w]++
	last := len(p.hands[0]) == 0 && len(p.hands[1]) == 0
	if last && !p.closed {
		p.t.points[w] += p.r.lastTrickBonus
	}
	switch {
	case p.t.total(w) >= goal:
		return signed(w, p.r.outPoints(p.t, p.close, w))
	case last && p.close.by >= 0:
		return signed(1-p.close.by, closerFailed(p.close))
	case last:
		seatWon, pts := p.r.runOut(p.t, w)
		if seatWon < 0 {
			return 0
		}
		return signed(seatWon, pts)
	}
	p.led, p.leader = "", w
	v, _ := p.search(p.legal(w), w, alpha, beta)
	return v
}
