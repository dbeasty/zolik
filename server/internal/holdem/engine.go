package holdem

import (
	"sort"
	"strconv"

	"zolik/server/internal/module"
)

// Module is the Hold'em game module.
type Module struct{}

// New returns the module. Stateless: every method takes the state it works on.
func New() *Module { return &Module{} }

var _ module.GameModule = (*Module)(nil)

// NewMatch seats the players and deals the first hand.
func (m *Module) NewMatch(cfg module.MatchConfig, players []module.PlayerRef, seed int64) (module.State, error) {
	if len(players) < 2 || len(players) > 9 {
		return nil, module.Error{Code: "WRONG_PLAYER_COUNT", Message: "hold'em seats two to nine"}
	}
	v := resolveVariation(cfg)

	s := &GameState{
		Status:        "active",
		Variation:     cfg.Variation,
		Seed:          seed,
		BigBlind:      cfg.Opt(OptBigBlind, v.bigBlind),
		StartingStack: cfg.Opt(OptStartingStack, v.startingStack),
		HandLimit:     cfg.Opt(OptHandLimit, v.handLimit),
		Button:        -1, // startHand rotates first, so the first button is seat 0
	}
	s.SmallBlind = s.BigBlind / 2
	if s.SmallBlind < 1 {
		s.SmallBlind = 1
	}
	for _, p := range players {
		s.Seats = append(s.Seats, Seat{PlayerID: p.ID, Stack: s.StartingStack})
	}

	startHand(s)
	return encode(s)
}

// startHand resets the seats, moves the button, deals and posts the blinds.
func startHand(s *GameState) []module.Event {
	// Anybody who ran out of chips between hands is out of the match. Done
	// here rather than when they lose the pot, so a player who is all-in and
	// wins stays seated.
	for i := range s.Seats {
		if s.Seats[i].Stack <= 0 {
			s.Seats[i].Out = true
		}
	}
	if len(s.liveSeats()) < 2 {
		return endMatch(s)
	}
	if s.HandLimit > 0 && s.HandNumber >= s.HandLimit {
		return endMatch(s)
	}

	s.HandNumber++
	for i := range s.Seats {
		st := &s.Seats[i]
		st.Bet, st.Committed = 0, 0
		st.Folded, st.AllIn, st.Acted = false, false, false
		st.Hole = nil
	}
	s.Board = nil
	s.Pot = 0
	s.CurrentBet = 0
	s.MinRaise = s.BigBlind
	s.Street = streetPreflop

	// Vary the deal per hand so a match is reproducible from its seed without
	// dealing the same cards every hand.
	s.Deck = shuffle(buildDeck(), s.Seed+int64(s.HandNumber)*7919)

	s.Button = s.nextSeat(s.Button, func(st *Seat) bool { return !st.Out })

	live := s.liveSeats()
	for i := 0; i < 2; i++ {
		for _, idx := range live {
			s.Seats[idx].Hole = append(s.Seats[idx].Hole, s.draw())
		}
	}

	// Heads-up reverses the blinds: the button posts the small blind and acts
	// first before the flop, then last on every street after it. It is the one
	// positional rule that is not "clockwise from the button", and getting it
	// wrong is invisible until a two-handed game plays out oddly.
	sb, bb := s.blindSeats()
	post(s, sb, s.SmallBlind)
	post(s, bb, s.BigBlind)
	s.CurrentBet = s.BigBlind
	s.MinRaise = s.BigBlind

	if len(live) == 2 {
		s.Current = sb
	} else {
		s.Current = s.nextSeat(bb, func(st *Seat) bool { return st.canAct() })
	}
	events := []module.Event{{Type: "hand_started", Data: map[string]any{
		"handNumber": s.HandNumber, "button": s.Seats[s.Button].PlayerID,
	}}}

	// Blinds can put somebody all in before they ever act, which can mean the
	// hand is already past the point of anyone having a decision.
	return append(events, advance(s)...)
}

// blindSeats returns the small and big blind seat indices.
func (s *GameState) blindSeats() (int, int) {
	live := s.liveSeats()
	notOut := func(st *Seat) bool { return !st.Out }
	if len(live) == 2 {
		return s.Button, s.nextSeat(s.Button, notOut)
	}
	sb := s.nextSeat(s.Button, notOut)
	return sb, s.nextSeat(sb, notOut)
}

// post puts a blind in, all-in for less when the stack cannot cover it.
func post(s *GameState, idx, amount int) {
	st := &s.Seats[idx]
	paid := min(amount, st.Stack)
	st.Stack -= paid
	st.Bet += paid
	st.Committed += paid
	if st.Stack == 0 {
		st.AllIn = true
	}
	if st.Bet > s.CurrentBet {
		s.CurrentBet = st.Bet
	}
}

func (s *GameState) draw() string {
	if len(s.Deck) == 0 {
		return ""
	}
	card := s.Deck[len(s.Deck)-1]
	s.Deck = s.Deck[:len(s.Deck)-1]
	return card
}

// Apply validates and applies one action.
func (m *Module) Apply(raw module.State, playerID string, a module.Action) (module.State, []module.Event, error) {
	s, err := decode(raw)
	if err != nil {
		return raw, nil, err
	}
	if s.Status != "active" {
		return raw, nil, errCode(ErrGameNotActive)
	}
	if s.Current < 0 || s.Seats[s.Current].PlayerID != playerID {
		return raw, nil, errCode(ErrNotYourTurn)
	}
	seat := &s.Seats[s.Current]
	if !seat.canAct() {
		return raw, nil, errCode(ErrSeatNotInHand)
	}

	var events []module.Event
	switch a.Verb {
	case VerbFold:
		events, err = applyFold(s, seat)
	case VerbCheck:
		events, err = applyCheck(s, seat)
	case VerbCall:
		events, err = applyCall(s, seat)
	case VerbRaise:
		events, err = applyRaise(s, seat, a)
	default:
		err = module.Error{Code: ErrUnknownAction, Message: a.Verb}
	}
	if err != nil {
		return raw, nil, err
	}

	events = append(events, advance(s)...)
	out, err := encode(s)
	return out, events, err
}

func applyFold(s *GameState, seat *Seat) ([]module.Event, error) {
	seat.Folded = true
	seat.Acted = true
	return []module.Event{{Type: "folded", Data: map[string]any{"playerId": seat.PlayerID}}}, nil
}

func applyCheck(s *GameState, seat *Seat) ([]module.Event, error) {
	if seat.Bet != s.CurrentBet {
		return nil, errCode(ErrCannotCheck)
	}
	seat.Acted = true
	return []module.Event{{Type: "checked", Data: map[string]any{"playerId": seat.PlayerID}}}, nil
}

func applyCall(s *GameState, seat *Seat) ([]module.Event, error) {
	owed := s.toCall(seat)
	if owed == 0 {
		// Refused rather than silently treated as a check: a client that meant
		// to check should say so, and one that thought there was a bet to call
		// is looking at stale state and needs to know.
		return nil, errCode(ErrNothingToCall)
	}
	seat.Stack -= owed
	seat.Bet += owed
	seat.Committed += owed
	seat.Acted = true
	if seat.Stack == 0 {
		seat.AllIn = true
	}
	return []module.Event{{Type: "called", Data: map[string]any{
		"playerId": seat.PlayerID, "amount": owed,
	}}}, nil
}

func applyRaise(s *GameState, seat *Seat, a module.Action) ([]module.Event, error) {
	raw, ok := a.Params[ParamAmount]
	if !ok || raw == "" {
		return nil, errCode(ErrAmountRequired)
	}
	amount, err := strconv.Atoi(raw)
	if err != nil {
		return nil, errCode(ErrAmountNotNumber)
	}

	maxTo := seat.Bet + seat.Stack
	if maxTo <= s.CurrentBet {
		// Cannot get above the bet even with everything: this seat's only
		// options are calling all-in or folding.
		return nil, errCode(ErrCannotRaise)
	}
	if amount > maxTo {
		return nil, errCode(ErrNotEnoughChips)
	}
	if amount <= s.CurrentBet {
		return nil, errCode(ErrRaiseTooSmall)
	}
	// A raise must be at least the size of the last one — except when a player
	// puts their whole stack in, which is always allowed and does not reopen
	// the betting for anyone who has already acted.
	full := amount >= s.CurrentBet+s.MinRaise
	if !full && amount != maxTo {
		return nil, errCode(ErrRaiseTooSmall)
	}

	put := amount - seat.Bet
	seat.Stack -= put
	seat.Bet = amount
	seat.Committed += put
	seat.Acted = true
	if seat.Stack == 0 {
		seat.AllIn = true
	}

	if full {
		s.MinRaise = amount - s.CurrentBet
		// A full raise puts the decision back to everyone still able to make
		// one. An all-in for less does not.
		for i := range s.Seats {
			if i != s.seatIndex(seat.PlayerID) && s.Seats[i].canAct() {
				s.Seats[i].Acted = false
			}
		}
	}
	s.CurrentBet = amount

	return []module.Event{{Type: "raised", Data: map[string]any{
		"playerId": seat.PlayerID, "to": amount, "allIn": seat.AllIn,
	}}}, nil
}

// --- moving the hand along ---------------------------------------------------

// advance runs the hand forward until somebody has a decision to make, or the
// hand is over.
//
// A loop rather than a single step because a street can close with nobody able
// to act — everyone is all in — and then the board simply runs out. Handling
// that as "keep going until there is a question to ask" costs one loop and
// removes a whole family of special cases.
func advance(s *GameState) []module.Event {
	var events []module.Event
	for i := 0; i < 16; i++ {
		if s.Status != "active" {
			return events
		}
		if len(s.contenders()) <= 1 {
			return append(events, endHand(s)...)
		}
		if !bettingClosed(s) {
			// Stay put when the seat already on turn still owes a decision.
			//
			// advance is called from two places with different expectations:
			// after an action, where the actor is done and the turn must move
			// on, and after a deal or a new street, where Current has just been
			// set to the correct first player. Deciding by "has this seat acted
			// yet" serves both without either caller having to say which it is
			// — and without it, every street silently skipped its first player.
			if s.Current >= 0 && s.Seats[s.Current].canAct() && !s.Seats[s.Current].Acted {
				return events
			}
			next := s.nextSeat(s.Current, func(st *Seat) bool { return st.canAct() && !st.Acted })
			if next < 0 {
				next = s.nextSeat(s.Current, func(st *Seat) bool { return st.canAct() })
			}
			if next < 0 {
				return append(events, endHand(s)...)
			}
			s.Current = next
			return events
		}
		if s.Street == streetRiver {
			return append(events, endHand(s)...)
		}
		events = append(events, nextStreet(s)...)
	}
	return events
}

// bettingClosed reports whether this street's action is complete: everyone who
// could act has acted, and everyone still in has matched the bet.
func bettingClosed(s *GameState) bool {
	for i := range s.Seats {
		st := &s.Seats[i]
		if !st.canAct() {
			continue
		}
		if !st.Acted || st.Bet != s.CurrentBet {
			return false
		}
	}
	return true
}

// nextStreet sweeps the bets into the pot and turns the next cards.
func nextStreet(s *GameState) []module.Event {
	for i := range s.Seats {
		s.Pot += s.Seats[i].Bet
		s.Seats[i].Bet = 0
		s.Seats[i].Acted = false
	}
	s.CurrentBet = 0
	s.MinRaise = s.BigBlind

	switch s.Street {
	case streetPreflop:
		s.Street = streetFlop
		s.Board = append(s.Board, s.draw(), s.draw(), s.draw())
	case streetFlop:
		s.Street = streetTurn
		s.Board = append(s.Board, s.draw())
	case streetTurn:
		s.Street = streetRiver
		s.Board = append(s.Board, s.draw())
	}

	// After the flop the action starts left of the button, in every game size
	// including heads-up — which is what makes the button act last.
	first := s.nextSeat(s.Button, func(st *Seat) bool { return st.canAct() })
	if first >= 0 {
		s.Current = first
	}
	return []module.Event{{Type: "street", Data: map[string]any{
		"street": s.Street, "board": append([]string(nil), s.Board...),
	}}}
}

// --- ending a hand -----------------------------------------------------------

// endHand awards the pot (or pots) and starts the next hand.
func endHand(s *GameState) []module.Event {
	for i := range s.Seats {
		s.Pot += s.Seats[i].Bet
		s.Seats[i].Bet = 0
	}

	before := map[string]int{}
	for i := range s.Seats {
		before[s.Seats[i].PlayerID] = s.Seats[i].Stack
	}

	res := HandResult{HandNumber: s.HandNumber, Board: append([]string(nil), s.Board...)}
	contenders := s.contenders()

	if len(contenders) == 1 {
		// Everyone folded. No hand is shown, which matters: showing it would
		// leak information the winner is entitled to keep.
		w := &s.Seats[contenders[0]]
		w.Stack += s.Pot
		res.Uncontested = true
		res.Pots = []PotResult{{Amount: s.Pot, Winners: []string{w.PlayerID}}}
	} else {
		res.Pots = distributePots(s, contenders)
		for _, idx := range contenders {
			st := &s.Seats[idx]
			best := Best(append(append([]string(nil), st.Hole...), s.Board...))
			res.Shown = append(res.Shown, ShownHand{
				PlayerID: st.PlayerID, Hole: append([]string(nil), st.Hole...),
				Best: best.Cards, LabelKey: categoryKey(best.Category),
			})
		}
	}

	res.Deltas = map[string]int{}
	for i := range s.Seats {
		if d := s.Seats[i].Stack - before[s.Seats[i].PlayerID]; d != 0 {
			res.Deltas[s.Seats[i].PlayerID] = d
		}
	}
	s.Pot = 0
	s.LastHand = &res
	s.Current = -1

	events := []module.Event{{Type: "hand_ended", Data: map[string]any{
		"handNumber": res.HandNumber, "pots": len(res.Pots),
	}}}
	return append(events, startHand(s)...)
}

// distributePots builds the side pots and awards each to the best hand among
// the players eligible for it.
//
// Side pots exist because a player can only win what they could lose: someone
// all in for 50 cannot take a fourth player's 200. Building them by
// contribution level is the only construction that gets that right without a
// special case per all-in.
func distributePots(s *GameState, contenders []int) []PotResult {
	levels := map[int]bool{}
	for i := range s.Seats {
		if s.Seats[i].Committed > 0 {
			levels[s.Seats[i].Committed] = true
		}
	}
	sorted := make([]int, 0, len(levels))
	for lvl := range levels {
		sorted = append(sorted, lvl)
	}
	sort.Ints(sorted)

	best := map[int]HandRank{}
	for _, idx := range contenders {
		best[idx] = Best(append(append([]string(nil), s.Seats[idx].Hole...), s.Board...))
	}

	var out []PotResult
	prev := 0
	for _, lvl := range sorted {
		amount := 0
		var eligible []int
		for i := range s.Seats {
			c := min(s.Seats[i].Committed, lvl)
			if c > prev {
				amount += c - prev
			}
			if s.Seats[i].Committed >= lvl && s.Seats[i].inHand() {
				eligible = append(eligible, i)
			}
		}
		prev = lvl
		if amount == 0 || len(eligible) == 0 {
			continue
		}

		winners := []int{eligible[0]}
		for _, idx := range eligible[1:] {
			cmp := best[idx].Compare(best[winners[0]])
			switch {
			case cmp > 0:
				winners = []int{idx}
			case cmp == 0:
				winners = append(winners, idx)
			}
		}

		share := amount / len(winners)
		remainder := amount - share*len(winners)
		ids := make([]string, 0, len(winners))
		for n, idx := range winners {
			s.Seats[idx].Stack += share
			// Odd chips go to the first winner clockwise from the button,
			// which is what a real table does and what keeps the arithmetic
			// exact rather than losing a chip.
			if n < remainder {
				s.Seats[idx].Stack++
			}
			ids = append(ids, s.Seats[idx].PlayerID)
		}
		out = append(out, PotResult{
			Amount: amount, Winners: ids,
			LabelKey: categoryKey(best[winners[0]].Category),
		})
	}
	return out
}

// endMatch settles the match on chips.
//
// More than one winner is a real outcome here, not a defensive nil check: a
// fixed-length match can end with two equal stacks, and a split pot means two
// players genuinely won the same chips. This is the case that made
// `Finished` return a list.
func endMatch(s *GameState) []module.Event {
	s.Status = "completed"
	s.Current = -1

	top := 0
	for i := range s.Seats {
		if s.Seats[i].Stack > top {
			top = s.Seats[i].Stack
		}
	}
	for i := range s.Seats {
		if s.Seats[i].Stack == top {
			s.Winners = append(s.Winners, s.Seats[i].PlayerID)
		}
	}
	return []module.Event{{Type: "match_ended", Data: map[string]any{
		"winners": append([]string(nil), s.Winners...), "chips": top,
	}}}
}

// Finished reports whether the match is over and who won.
func (m *Module) Finished(raw module.State) (bool, []string, error) {
	s, err := decode(raw)
	if err != nil {
		return false, nil, err
	}
	if s.Status != "completed" {
		return false, nil, nil
	}
	return true, append([]string(nil), s.Winners...), nil
}
