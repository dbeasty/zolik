package holdem

// The public record: what everyone at the table saw, kept in the state so
// something other than a person's memory can use it.
//
// Two scales, for two questions. HandLog is this hand, action by action — the
// betting sequence, which is most of what a hand means before anyone's cards
// are shown. Seat.Reads is the whole match, counted rather than listed — how
// loose, how aggressive, how often a big bet turned out to be nothing — which
// is what a regular means by "a read" on somebody.
//
// Neither is anything a seat could not have written down for itself. The
// counters are fed from the actions as they are made and from hands as they
// are turned face up, and from nothing else: a hand that was never shown
// contributes its betting and not its cards.
//
// All of it is a pure function of the actions, updated inside Apply, so a
// replay (match/replay.go) rebuilds it byte for byte along with the rest of
// the state.

// decisionAt is the table as one seat decided, captured before its action
// moved any chips.
type decisionAt struct {
	seat int
	owed int // to call
	bet  int // the street's bet to match
	pot  int // every chip on the table
	put  int // this seat's chips in the hand so far
}

// reads is the seat's counters, made on first use so a state stored before
// they existed simply starts counting.
func (st *Seat) reads() *SeatReads {
	if st.Reads == nil {
		st.Reads = &SeatReads{}
	}
	return st.Reads
}

// record writes one accepted action into the hand's log and the seat's reads,
// and notes a raise as the street's aggressor.
//
// Called after the action applied and before advance can move to the next
// street, so s.Street is still the street the action was made on.
func record(s *GameState, verb string, d decisionAt) {
	st := &s.Seats[d.seat]
	act := PublicAct{Seat: d.seat, Street: s.Street, Verb: verb, Amount: st.Committed - d.put, Pot: d.pot}
	if verb == VerbRaise {
		act.Raise = s.CurrentBet - d.bet
		s.Aggressor = d.seat
	}

	r := st.reads()
	if s.opening() {
		// Once per hand each: a limp and a later call of a raise are one hand
		// in which this seat chose to play, not two.
		if (verb == VerbCall || verb == VerbRaise) && !s.didPreflop(d.seat, VerbCall, VerbRaise) {
			r.VPIP++
		}
		if verb == VerbRaise && !s.didPreflop(d.seat, VerbRaise) {
			r.PFR++
		}
		if d.owed > 0 && d.bet > s.BigBlind {
			r.FacedRaise++
			if verb == VerbFold {
				r.FoldedToRaise++
			}
		}
	} else {
		// Facing a bet after the flop, which is the situation "folds too
		// much" is about. Before the flop every hand faces the big blind, and
		// how often a seat folds there is what VPIP already says.
		if d.owed > 0 {
			r.FacedBet++
			if verb == VerbFold {
				r.FoldedToBet++
			}
		}
		switch verb {
		case VerbRaise:
			r.PostBets++
		case VerbCall:
			r.PostCalls++
		}
	}
	s.HandLog = append(s.HandLog, act)
}

// didPreflop reports whether this seat already made one of these moves before
// the flop in this hand.
func (s *GameState) didPreflop(seat int, verbs ...string) bool {
	for _, a := range s.HandLog {
		if a.Seat != seat || (a.Street != streetPreflop && a.Street != streetPredraw) {
			continue
		}
		for _, v := range verbs {
			if a.Verb == v {
				return true
			}
		}
	}
	return false
}

// bigBet is the size from which a bet is a claim worth checking at the
// showdown: three quarters of the pot. Smaller bets are made with too wide a
// range for "was it a bluff" to mean much.
const bigBet = 0.75

// readShown settles the bluff counters for a hand that has just gone face up.
//
// The one place a hole card reaches Reads, and it is only ever reached from
// reveal and applyShow — the two moments the card became public. Each big bet
// this seat made after the flop is judged against the board as it stood when
// the bet went in, because that is what the bet was a claim about: a river
// card that happened to rescue a bluff does not make it a value bet.
//
// Before the flop is not judged. Every hand there is high card or a pair, and
// a raise with ace-king is not a bluff by any definition worth counting.
func readShown(s *GameState, idx int) {
	st := &s.Seats[idx]
	// The bluff test reads a bet against the board it was made on with two
	// hole cards, which is a Hold'em question. A Draw hand changed between
	// its bets, and an Omaha hand's pair and draws are made by other rules.
	if len(st.Hole) < 2 || !s.rules().holdem() {
		return
	}
	for _, a := range s.HandLog {
		if a.Seat != idx || a.Verb != VerbRaise || a.Street == streetPreflop || a.sizeOfPot() < bigBet {
			continue
		}
		n := boardAt(a.Street)
		if n > len(s.Board) {
			continue
		}
		r := st.reads()
		r.BigBetsShown++
		if weakAt(st.Hole, s.Board[:n], a.Street) {
			r.BluffsShown++
		}
	}
}

// boardAt is how many board cards are showing on a street.
func boardAt(street string) int {
	switch street {
	case streetFlop:
		return 3
	case streetTurn:
		return 4
	case streetRiver, streetShowdown:
		return 5
	default:
		return 0
	}
}

// weakAt is the bluff test: no pair of the seat's own and nothing better, and
// — with a card still to come — no real draw either.
//
// "Of its own" because a pair on the board is everybody's pair: betting it big
// with two unrelated cards is betting nothing. A draw of eight or more outs is
// excused, since a flush or open-ended straight draw is a semi-bluff with a
// real hand behind it, not air; a gutshot is not.
func weakAt(hole, board []string, street string) bool {
	if rankOf(hole[0]) == rankOf(hole[1]) {
		return false
	}
	for _, h := range hole {
		for _, b := range board {
			if rankOf(h) == rankOf(b) {
				return false
			}
		}
	}
	if len(hole)+len(board) >= 5 && Best(append(append([]string(nil), hole...), board...)).Category >= straight {
		return false
	}
	if street != streetRiver && drawOuts(hole, board) >= 8 {
		return false
	}
	return true
}
