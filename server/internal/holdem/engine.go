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

	// startHand rotates the button before the first hand, so it is seeded one
	// seat behind wherever the match should actually open — a random seat
	// from the match seed, rather than always seat 0.
	opening := module.StartingSeat(seed, len(players))

	s := &GameState{
		Status:        "active",
		Variation:     cfg.Variation,
		Seed:          seed,
		BigBlind:      cfg.Opt(OptBigBlind, v.bigBlind),
		StartingStack: cfg.Opt(OptStartingStack, v.startingStack),
		HandLimit:     cfg.Opt(OptHandLimit, v.handLimit),
		Reveal:        cfg.Opt(OptShowdownReveal, RevealEveryone),
		Button:        (opening - 1 + len(players)) % len(players),
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
	if matchOverAfterHand(s) {
		return endMatch(s)
	}
	return dealHand(s)
}

// matchOverAfterHand settles who is still in and reports whether that ends the
// match.
//
// Split out of startHand because a table that pauses between hands has to know
// the match is finished *before* it stops to show a results screen, or the last
// hand would put up an interstitial for a hand that will never be dealt.
func matchOverAfterHand(s *GameState) bool {
	// Anybody who ran out of chips between hands is out of the match. Done
	// here rather than when they lose the pot, so a player who is all-in and
	// wins stays seated.
	for i := range s.Seats {
		if s.Seats[i].Stack <= 0 {
			s.Seats[i].Out = true
		}
	}
	if len(s.liveSeats()) < 2 {
		return true
	}
	return s.HandLimit > 0 && s.HandNumber >= s.HandLimit
}

// dealHand resets the seats, moves the button, deals and posts the blinds.
func dealHand(s *GameState) []module.Event {
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

	// At a showdown nobody is on turn, so the intermission does the checking
	// the turn order normally would.
	if s.Break.Open {
		if a.Verb == VerbShow {
			events, err := applyShow(s, playerID)
			if err != nil {
				return raw, nil, err
			}
			out, err := encode(s)
			return out, events, err
		}
		if a.Verb != module.VerbContinue {
			return raw, nil, errCode(ErrGameNotActive)
		}
		if err := s.Break.Mark(order(s), playerID); err != nil {
			return raw, nil, err
		}
		var events []module.Event
		if s.Break.Settled(order(s)) {
			closing := s.Closing
			s.Break.Close()
			s.Closing = false
			// The last showdown goes on to nothing, so agreeing to go on ends
			// the match instead of dealing. Which of the two this is was
			// decided when the hand ended — see endHand.
			if closing {
				events = endMatch(s)
			} else {
				events = dealHand(s)
			}
		}
		out, err := encode(s)
		return out, events, err
	}
	if a.Verb == VerbShow {
		// Not at a showdown: whatever is on the table is being played, and the
		// hand this would have shown was dealt over.
		return raw, nil, module.Error{Code: module.ErrNotPaused}
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
		reveal(s, &res, contenders)
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
	s.Hands = append(s.Hands, summarise(s, res))

	events := []module.Event{{Type: "hand_ended", Data: map[string]any{
		"handNumber": res.HandNumber, "pots": len(res.Pots),
	}}}

	// The table stops, always.
	//
	// Every other game here asks first. Poker does not, because poker is the
	// one whose round ends in something to look at, and it used to deal
	// straight over the top of it — see GameState.Break.
	//
	// Including the hand that ends the match, which is the change of mind
	// worth naming. This used to settle the match here and skip the stop, so
	// that a finished match put up one results screen rather than two. But the
	// deciding hand is the one hand people actually talk about afterwards, and
	// skipping its showdown meant the best hand of the match was the one
	// nobody was shown and nobody could show into. So the last showdown
	// happens like every other one; agreeing to go on ends the match instead
	// of dealing the next hand.
	s.Closing = matchOverAfterHand(s)
	s.Break.Begin(s.HandNumber + 1)
	return events
}

// Reveal settings: which hands a showdown turns face up on its own.
const (
	// RevealWinners is the table rule — only a player who is pushed chips has
	// to prove they earned them; everyone else may keep what they paid for.
	RevealWinners = 1
	// RevealEveryone shows every hand that reached the showdown, which is what
	// this module has always done and what an online table usually offers.
	RevealEveryone = 2
)

// reveal turns the contenders' hands face up, as far as the table's setting
// says, and names the rest as mucked.
//
// The filter lives here rather than in View for one reason that is not
// negotiable: `HandResult.Shown` is what goes onto the wire. View reads it to
// build sentences that carry the hole cards in their params, so a hand left in
// here is public whatever a later renderer chooses to draw. Filtering at the
// point of drawing would be a setting that hides cards from the screen and
// ships them anyway.
func reveal(s *GameState, res *HandResult, contenders []int) {
	winners := map[string]bool{}
	for _, p := range res.Pots {
		for _, w := range p.Winners {
			winners[w] = true
		}
	}
	for _, idx := range contenders {
		st := &s.Seats[idx]
		if s.Reveal == RevealWinners && !winners[st.PlayerID] {
			res.Mucked = append(res.Mucked, st.PlayerID)
			continue
		}
		best := Best(append(append([]string(nil), st.Hole...), s.Board...))
		res.Shown = append(res.Shown, ShownHand{
			PlayerID: st.PlayerID, Hole: append([]string(nil), st.Hole...),
			Best: best.Cards, LabelKey: categoryKey(best.Category),
		})
	}
}

// applyShow turns one player's hand face up, by their own choice.
//
// It works off the seats rather than off anything stored with the result,
// because it does not have to store anything: `dealHand` is what clears
// `Seat.Hole`, and the showdown stop happens before it runs. So for as long as
// there is a window to show in, every seat is still physically holding the
// hand it just played.
//
// A hand that never reached a showdown is turned over with no name attached.
// There may be no board to make five cards against, and `Best` answers "high
// card" when it is given fewer — which would print a claim the player never
// made out of a hand they were only ever showing for the look of it.
func applyShow(s *GameState, playerID string) ([]module.Event, error) {
	if s.LastHand == nil || !s.Break.Open {
		// No hand on the table to show. Whatever cards the seats hold belong
		// to a hand still being played.
		return nil, module.Error{Code: module.ErrNotPaused}
	}
	idx := s.seatIndex(playerID)
	if idx < 0 {
		return nil, module.Error{Code: module.ErrNotSeated, Message: playerID}
	}
	// Having said "go on" is having mucked. A seat that already readied has
	// released the hand, and the table may deal the moment the last one does.
	if s.Break.Ready[playerID] {
		return nil, module.Error{Code: module.ErrAlreadyReady, Message: playerID}
	}
	if s.LastHand.shows(playerID) {
		return nil, errCode(ErrAlreadyShown)
	}
	st := &s.Seats[idx]
	if len(st.Hole) == 0 {
		return nil, errCode(ErrNothingToShow)
	}

	shown := ShownHand{
		PlayerID:  playerID,
		Hole:      append([]string(nil), st.Hole...),
		Voluntary: true,
	}
	if len(st.Hole)+len(s.Board) >= 5 && !s.LastHand.Uncontested && st.inHand() {
		best := Best(append(append([]string(nil), st.Hole...), s.Board...))
		shown.Best, shown.LabelKey = best.Cards, categoryKey(best.Category)
	}
	s.LastHand.Shown = append(s.LastHand.Shown, shown)
	s.LastHand.Mucked = without(s.LastHand.Mucked, playerID)

	// The cards go out with the event because they are now public — this
	// player just made them so. Every other field a client needs is already
	// in the state it will be sent alongside.
	return []module.Event{{Type: "shown", Data: map[string]any{
		"playerId": playerID, "hole": append([]string(nil), st.Hole...),
	}}}, nil
}

// without returns ids with one removed, keeping the rest in order.
func without(ids []string, drop string) []string {
	out := ids[:0]
	for _, id := range ids {
		if id != drop {
			out = append(out, id)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
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
		pot := PotResult{
			Amount: amount, Winners: ids,
			LabelKey: categoryKey(best[winners[0]].Category),
		}
		// The five cards only when one player took the pot. Split winners
		// tie on rank but can hold different suits, so any one set of five
		// would be the wrong hand for somebody; each of theirs is on its own
		// "showed" line instead.
		if len(winners) == 1 {
			pot.Cards = append([]string(nil), best[winners[0]].Cards...)
		}
		out = append(out, pot)
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

// summarise reduces a finished hand to what outlives it. Stacks is read after
// the pot has been distributed, so it is what each seat is actually sitting on
// going into the next hand.
func summarise(s *GameState, res HandResult) HandSummary {
	sum := HandSummary{
		Number:      res.HandNumber,
		Uncontested: res.Uncontested,
		Deltas:      map[string]int{},
		Stacks:      map[string]int{},
	}
	seen := map[string]bool{}
	for _, p := range res.Pots {
		sum.Pot += p.Amount
		for _, w := range p.Winners {
			if !seen[w] {
				seen[w] = true
				sum.Winners = append(sum.Winners, w)
			}
		}
	}
	// The net move, which is not HandResult.Deltas.
	//
	// That field is what the pot *paid* a seat: it is measured against the
	// stack after the street's bets were already swept away, so a player who
	// called 40 and lost shows a delta of zero. What a round table needs is how
	// the stack actually moved, which is the winnings less everything put in —
	// and Committed still holds that here, because startHand has not run yet
	// and is where it is cleared.
	for i := range s.Seats {
		pid := s.Seats[i].PlayerID
		sum.Stacks[pid] = s.Seats[i].Stack
		if net := res.Deltas[pid] - s.Seats[i].Committed; net != 0 {
			sum.Deltas[pid] = net
		}
	}
	return sum
}

// order is the seat order an intermission readies through. Every seat, whether
// or not they are still in the match: a player who has been busted out is still
// at the table and still reading the results.
func order(s *GameState) []string {
	out := make([]string, 0, len(s.Seats))
	for i := range s.Seats {
		out = append(out, s.Seats[i].PlayerID)
	}
	return out
}
