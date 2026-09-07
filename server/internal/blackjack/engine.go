package blackjack

import (
	"strconv"

	"zolik/server/internal/module"
)

// Module is the Blackjack game module.
type Module struct{}

// New returns the module. Stateless: every method takes the state it works on.
func New() *Module { return &Module{} }

var _ module.GameModule = (*Module)(nil)

// NewMatch seats the players and opens the betting on the first round.
func (m *Module) NewMatch(cfg module.MatchConfig, players []module.PlayerRef, seed int64) (module.State, error) {
	if len(players) < 2 || len(players) > 7 {
		return nil, module.Error{Code: ErrWrongPlayerCount, Message: "blackjack seats two to seven"}
	}
	s := tableOf(cfg)
	s.Status = "active"
	s.Seed = seed
	s.Current, s.CurrentHand = -1, -1

	for _, p := range players {
		s.Seats = append(s.Seats, Seat{PlayerID: p.ID, Stack: s.StartingStack})
	}
	s.Shoe = buildShoe(s.Decks, seed)

	startRound(s)
	return encode(s)
}

// Apply validates and applies one action. State is decoded fresh every call,
// so a refused action returns the caller's own bytes untouched.
func (m *Module) Apply(raw module.State, playerID string, a module.Action) (module.State, []module.Event, error) {
	s, err := decode(raw)
	if err != nil {
		return raw, nil, err
	}
	if s.Status != "active" {
		return raw, nil, errCode(ErrGameNotActive)
	}

	// Between rounds nobody is on turn, so the intermission does the checking
	// turn order normally would.
	if s.Break.Open {
		if a.Verb != module.VerbContinue {
			return raw, nil, errCode(ErrGameNotActive)
		}
		if err := s.Break.Mark(order(s), playerID); err != nil {
			return raw, nil, err
		}
		var events []module.Event
		if s.Break.Settled(order(s)) {
			s.Break.Close()
			events = startRound(s)
		}
		out, err := encode(s)
		return out, events, err
	}

	seat := s.seat(playerID)
	if seat == nil {
		return raw, nil, errCode(module.ErrNotSeated)
	}

	var events []module.Event
	switch a.Verb {
	case VerbBet:
		events, err = applyBet(s, seat, a)
	case VerbInsure:
		events, err = applyInsurance(s, seat, true)
	case VerbDecline:
		events, err = applyInsurance(s, seat, false)
	case VerbHit, VerbStand, VerbDouble, VerbSplit, VerbSurrender:
		events, err = applyPlay(s, seat, a.Verb)
	default:
		err = module.Error{Code: ErrUnknownAction, Message: a.Verb}
	}
	if err != nil {
		return raw, nil, err
	}
	out, err := encode(s)
	return out, events, err
}

// --- the round ---------------------------------------------------------

// startRound settles who can still afford a seat, shuffles if the shoe is
// low, and opens the betting.
func startRound(s *GameState) []module.Event {
	if matchOverBeforeRound(s) {
		return endMatch(s)
	}
	s.RoundNumber++

	for i := range s.Seats {
		st := &s.Seats[i]
		st.Bet, st.Hands = 0, nil
		st.Insurance, st.InsuranceAnswered = 0, false
		st.Staked = 0
	}
	s.Dealer = nil
	s.HoleShown, s.Peeked = false, false
	s.Current, s.CurrentHand = -1, -1
	s.Phase = phaseBets

	// The cut card. A shoe is reshuffled between rounds rather than mid-deal,
	// which is both what a table does and what keeps a round dealt from one
	// known composition — mid-round shuffling is the thing card counters and
	// bug reports both notice.
	if len(s.Shoe) < s.Decks*52/4 {
		s.Shoe = buildShoe(s.Decks, s.Seed+int64(s.RoundNumber)*7919)
	}

	return []module.Event{{Type: "round_started", Data: map[string]any{"round": s.RoundNumber}}}
}

// matchOverBeforeRound marks the seats that can no longer meet the minimum
// and reports whether that, or the round limit, ends the match.
//
// Split out of startRound for holdem's reason: a table that pauses between
// rounds has to know the match is over *before* it stops to show a results
// screen, or the last round puts up an interstitial for a round that will
// never be dealt.
func matchOverBeforeRound(s *GameState) bool {
	live := 0
	for i := range s.Seats {
		if s.Seats[i].Stack < s.MinBet {
			s.Seats[i].Out = true
		}
		if !s.Seats[i].Out {
			live++
		}
	}
	if live == 0 {
		return true
	}
	return s.RoundLimit > 0 && s.RoundNumber >= s.RoundLimit
}

func applyBet(s *GameState, seat *Seat, a module.Action) ([]module.Event, error) {
	if s.Phase != phaseBets {
		return nil, errCode(ErrWrongPhase)
	}
	if seat.Out {
		return nil, errCode(ErrNotInRound)
	}
	if seat.Bet > 0 {
		return nil, errCode(ErrAlreadyBet)
	}
	raw, ok := a.Params[ParamAmount]
	if !ok || raw == "" {
		return nil, errCode(ErrAmountRequired)
	}
	amount, err := strconv.Atoi(raw)
	if err != nil {
		return nil, errCode(ErrAmountNotNumber)
	}
	if amount < s.MinBet {
		return nil, errCode(ErrBetTooSmall)
	}
	if amount > seat.Stack {
		return nil, errCode(ErrNotEnoughChips)
	}

	seat.Stack -= amount
	seat.Staked += amount
	seat.Bet = amount
	events := []module.Event{{Type: "bet_placed", Data: map[string]any{
		"playerId": seat.PlayerID, "amount": amount,
	}}}

	if betsIn(s) {
		return append(events, deal(s)...), nil
	}
	return events, nil
}

// betsIn reports every seat that can play having put a stake up.
func betsIn(s *GameState) bool {
	for i := range s.Seats {
		if !s.Seats[i].Out && s.Seats[i].Bet == 0 {
			return false
		}
	}
	return true
}

// deal puts two cards in front of every stake and two in front of the dealer,
// in the order a table deals them: one card round the table, then a second.
func deal(s *GameState) []module.Event {
	for pass := 0; pass < 2; pass++ {
		for i := range s.Seats {
			st := &s.Seats[i]
			if !st.inRound() {
				continue
			}
			if pass == 0 {
				st.Hands = []Hand{{ID: handID(st.PlayerID, 0), Bet: st.Bet}}
			}
			st.Hands[0].Cards = append(st.Hands[0].Cards, drawCard(s))
		}
		s.Dealer = append(s.Dealer, drawCard(s))
	}

	// A natural has no decisions left in it, so it is finished here rather
	// than offered a hit it could only lose by taking.
	for i := range s.Seats {
		st := &s.Seats[i]
		if st.inRound() && isNatural(st.Hands[0].Cards) {
			st.Hands[0].Done = true
		}
	}

	events := []module.Event{{Type: "cards_dealt", Data: map[string]any{
		"round": s.RoundNumber, "upCard": s.upCard(),
	}}}
	return append(events, openInsuranceOrPeek(s)...)
}

// openInsuranceOrPeek offers insurance against a dealer ace, or goes straight
// to the peek when there is nothing to insure against.
func openInsuranceOrPeek(s *GameState) []module.Event {
	if !s.AllowInsurance || rankOf(s.upCard()) != "A" {
		return peek(s)
	}

	asked := false
	for i := range s.Seats {
		st := &s.Seats[i]
		if !st.inRound() {
			continue
		}
		// A seat that cannot cover the premium has no decision to make, so it
		// is answered here rather than shown a control whose only live option
		// is "no". Never ending a turn in a dead position is the same rule
		// that finishes a natural above.
		if cost := insuranceCost(st); cost == 0 || cost > st.Stack {
			st.InsuranceAnswered = true
			continue
		}
		asked = true
	}
	if !asked {
		return peek(s)
	}

	s.Phase = phaseInsurance
	return []module.Event{{Type: "insurance_offered", Data: map[string]any{"round": s.RoundNumber}}}
}

// insuranceCost is half the seat's opening stake — the standard price, and
// the reason an odd stake insures for the lower half.
func insuranceCost(seat *Seat) int { return seat.Bet / 2 }

func applyInsurance(s *GameState, seat *Seat, take bool) ([]module.Event, error) {
	if s.Phase != phaseInsurance {
		return nil, errCode(ErrInsuranceClosed)
	}
	if !seat.inRound() {
		return nil, errCode(ErrNotInRound)
	}
	if seat.InsuranceAnswered {
		return nil, errCode(ErrInsuranceClosed)
	}
	cost := insuranceCost(seat)
	if take {
		if cost == 0 {
			return nil, errCode(ErrInsuranceClosed)
		}
		if cost > seat.Stack {
			return nil, errCode(ErrNotEnoughChips)
		}
		seat.Stack -= cost
		seat.Staked += cost
		seat.Insurance = cost
	}
	seat.InsuranceAnswered = true

	events := []module.Event{{Type: "insurance_answered", Data: map[string]any{
		"playerId": seat.PlayerID, "taken": take, "amount": seat.Insurance,
	}}}

	for i := range s.Seats {
		if s.Seats[i].inRound() && !s.Seats[i].InsuranceAnswered {
			return events, nil
		}
	}
	return append(events, peek(s)...), nil
}

// peek is the dealer looking at the hole card for blackjack.
//
// It is what makes surrender *late* and doubling safe: everything after this
// point is played against a dealer who is known not to have twenty-one
// already.
func peek(s *GameState) []module.Event {
	s.Peeked = true
	if isNatural(s.Dealer) {
		s.HoleShown = true
		return append([]module.Event{{Type: "dealer_blackjack"}}, settle(s)...)
	}

	s.Phase = phasePlay
	s.Current, s.CurrentHand = -1, -1
	return advance(s)
}

// --- playing the hands --------------------------------------------------

func applyPlay(s *GameState, seat *Seat, verb string) ([]module.Event, error) {
	if s.Phase != phasePlay {
		return nil, errCode(ErrWrongPhase)
	}
	if s.Current < 0 || s.Seats[s.Current].PlayerID != seat.PlayerID {
		return nil, errCode(ErrNotYourTurn)
	}
	hand := current(s)
	if hand == nil {
		return nil, errCode(ErrNotYourTurn)
	}

	var events []module.Event
	var err error
	switch verb {
	case VerbHit:
		events, err = applyHit(s, seat, hand)
	case VerbStand:
		hand.Done = true
		events = []module.Event{{Type: "stood", Data: map[string]any{"playerId": seat.PlayerID, "hand": hand.ID}}}
	case VerbDouble:
		events, err = applyDouble(s, seat, hand)
	case VerbSplit:
		events, err = applySplit(s, seat)
	case VerbSurrender:
		events, err = applySurrender(s, seat, hand)
	}
	if err != nil {
		return nil, err
	}
	return append(events, advance(s)...), nil
}

// current is the hand the module is waiting on, or nil.
func current(s *GameState) *Hand {
	if s.Current < 0 || s.Current >= len(s.Seats) {
		return nil
	}
	seat := &s.Seats[s.Current]
	if s.CurrentHand < 0 || s.CurrentHand >= len(seat.Hands) {
		return nil
	}
	return &seat.Hands[s.CurrentHand]
}

func applyHit(s *GameState, seat *Seat, hand *Hand) ([]module.Event, error) {
	card := drawCard(s)
	hand.Cards = append(hand.Cards, card)
	finishIfSettled(hand)
	return []module.Event{{Type: "card_drawn", Data: map[string]any{
		"playerId": seat.PlayerID, "hand": hand.ID, "card": card,
	}}}, nil
}

// finishIfSettled closes a hand that has nothing left to decide: bust, or
// twenty-one, where hitting again can only lose.
func finishIfSettled(hand *Hand) {
	t, _ := Total(hand.Cards)
	if t >= 21 {
		hand.Done = true
	}
}

func applyDouble(s *GameState, seat *Seat, hand *Hand) ([]module.Event, error) {
	if !canDouble(s, seat, hand) {
		if seat.Stack < hand.Bet {
			return nil, errCode(ErrNotEnoughChips)
		}
		return nil, errCode(ErrCannotDouble)
	}
	seat.Stack -= hand.Bet
	seat.Staked += hand.Bet
	hand.Bet *= 2
	hand.Doubled = true
	card := drawCard(s)
	hand.Cards = append(hand.Cards, card)
	// One card, and the hand is over whatever it made — that is the whole
	// bargain a double is.
	hand.Done = true
	return []module.Event{{Type: "doubled", Data: map[string]any{
		"playerId": seat.PlayerID, "hand": hand.ID, "bet": hand.Bet,
	}}}, nil
}

// canDouble states the rule once, so the offer list and Apply cannot hold
// different opinions about it.
func canDouble(s *GameState, seat *Seat, hand *Hand) bool {
	if hand.Done || len(hand.Cards) != 2 || hand.SplitAces {
		return false
	}
	if hand.Split && !s.DoubleAfterSplit {
		return false
	}
	return seat.Stack >= hand.Bet
}

func applySplit(s *GameState, seat *Seat) ([]module.Event, error) {
	hand := current(s)
	if !canSplit(s, seat, hand) {
		if hand != nil && seat.Stack < hand.Bet {
			return nil, errCode(ErrNotEnoughChips)
		}
		return nil, errCode(ErrCannotSplit)
	}

	moved := hand.Cards[1]
	aces := rankOf(hand.Cards[0]) == "A"
	seat.Stack -= hand.Bet
	seat.Staked += hand.Bet
	next := Hand{
		ID:        handID(seat.PlayerID, len(seat.Hands)),
		Cards:     []string{moved},
		Bet:       hand.Bet,
		Split:     true,
		SplitAces: aces,
	}

	// Grow the seat's hands first, then take the pointer again: appending can
	// move the slice, and a pointer taken before it would write the rest of
	// this move into an array nothing reads any more.
	//
	// The new hand goes right after the one it came from, so the seat plays
	// its boxes left to right the way a dealer works down them.
	seat.Hands = append(seat.Hands, Hand{})
	copy(seat.Hands[s.CurrentHand+2:], seat.Hands[s.CurrentHand+1:])
	seat.Hands[s.CurrentHand+1] = next

	cur := &seat.Hands[s.CurrentHand]
	cur.Cards = cur.Cards[:1]
	cur.Split, cur.SplitAces = true, aces
	// The hand in play is dealt its second card immediately; the new one waits
	// until play reaches it.
	cur.Cards = append(cur.Cards, drawCard(s))
	if aces {
		// Split aces take one card each and stand. Twenty-one made this way is
		// twenty-one, not a blackjack — which is what Hand.Split is for.
		cur.Done = true
	} else {
		finishIfSettled(cur)
	}

	return []module.Event{{Type: "split", Data: map[string]any{
		"playerId": seat.PlayerID, "hands": len(seat.Hands),
	}}}, nil
}

// canSplit states the rule once, for the offer list and for Apply.
func canSplit(s *GameState, seat *Seat, hand *Hand) bool {
	if hand == nil || hand.Done || len(hand.Cards) != 2 {
		return false
	}
	if !samePairValue(hand.Cards[0], hand.Cards[1]) {
		return false
	}
	if len(seat.Hands) > s.MaxSplits {
		return false
	}
	return seat.Stack >= hand.Bet
}

func applySurrender(s *GameState, seat *Seat, hand *Hand) ([]module.Event, error) {
	if !canSurrender(s, seat, hand) {
		return nil, errCode(ErrCannotSurrender)
	}
	hand.Surrendered = true
	hand.Done = true
	return []module.Event{{Type: "surrendered", Data: map[string]any{
		"playerId": seat.PlayerID, "hand": hand.ID,
	}}}, nil
}

// canSurrender is late surrender: the first two cards of an unsplit hand,
// after the dealer has peeked and before anything else has happened to it.
func canSurrender(s *GameState, seat *Seat, hand *Hand) bool {
	if !s.AllowSurrender || hand == nil || hand.Done {
		return false
	}
	return s.Peeked && !hand.Split && len(seat.Hands) == 1 && len(hand.Cards) == 2
}

// advance walks to the next hand with a decision in it, dealing second cards
// to hands created by a split on the way, and plays the dealer out when the
// table is done.
//
// A loop rather than a step because dealing a split hand its second card can
// finish it outright — twenty-one, or a split ace — and "keep going until
// there is a question to ask" costs one loop and removes a family of special
// cases. The same shape holdem's advance has, for the same reason.
func advance(s *GameState) []module.Event {
	for guard := 0; guard < 64; guard++ {
		seatIdx, handIdx := nextHand(s)
		if seatIdx < 0 {
			return dealerTurn(s)
		}
		s.Current, s.CurrentHand = seatIdx, handIdx

		hand := &s.Seats[seatIdx].Hands[handIdx]
		if len(hand.Cards) == 1 {
			hand.Cards = append(hand.Cards, drawCard(s))
			if hand.SplitAces {
				hand.Done = true
				continue
			}
			finishIfSettled(hand)
			if hand.Done {
				continue
			}
		}
		return nil
	}
	return dealerTurn(s)
}

// nextHand is the first unfinished hand at or after the one in play, in seat
// order. Returns -1, -1 when the table is done.
func nextHand(s *GameState) (int, int) {
	start, from := 0, 0
	if s.Current >= 0 {
		start, from = s.Current, s.CurrentHand
	}
	for i := start; i < len(s.Seats); i++ {
		st := &s.Seats[i]
		first := 0
		if i == start {
			first = from
			if first < 0 {
				first = 0
			}
		}
		for j := first; j < len(st.Hands); j++ {
			if !st.Hands[j].Done {
				return i, j
			}
		}
	}
	return -1, -1
}

// --- the dealer and the settlement --------------------------------------

// dealerTurn turns the hole card and draws to seventeen.
//
// The dealer only draws when there is something to beat: with every hand bust
// or surrendered the house has already won them, and a table does not draw
// cards for the sake of it.
func dealerTurn(s *GameState) []module.Event {
	s.HoleShown = true
	s.Current, s.CurrentHand = -1, -1

	if contested(s) {
		for {
			t, soft := Total(s.Dealer)
			if t > 17 || (t == 17 && !(soft && s.DealerHitsSoft17)) {
				break
			}
			s.Dealer = append(s.Dealer, drawCard(s))
		}
	}
	total, _ := Total(s.Dealer)
	events := []module.Event{{Type: "dealer_played", Data: map[string]any{
		"total": total, "bust": total > 21,
	}}}
	return append(events, settle(s)...)
}

// contested reports at least one hand the dealer still has to beat.
func contested(s *GameState) bool {
	for i := range s.Seats {
		for _, h := range s.Seats[i].Hands {
			if !h.Surrendered && !IsBust(h.Cards) {
				return true
			}
		}
	}
	return false
}

// settle pays the round out, records it, and moves on to the next one — or to
// the results screen, or to the end of the match.
func settle(s *GameState) []module.Event {
	dealerTotal, _ := Total(s.Dealer)
	dealerNatural := isNatural(s.Dealer)
	dealerBust := dealerTotal > 21

	sum := RoundSummary{
		Number:          s.RoundNumber,
		DealerTotal:     dealerTotal,
		DealerBust:      dealerBust,
		DealerBlackjack: dealerNatural,
		Outcomes:        map[string][]string{},
		Deltas:          map[string]int{},
		Stacks:          map[string]int{},
	}

	for i := range s.Seats {
		st := &s.Seats[i]
		won := 0
		for j := range st.Hands {
			h := &st.Hands[j]
			h.Outcome, h.Payout = settleHand(s, h, dealerTotal, dealerNatural, dealerBust)
			st.Stack += h.Payout
			won += h.Payout
			sum.Outcomes[st.PlayerID] = append(sum.Outcomes[st.PlayerID], h.Outcome)
		}
		if st.Insurance > 0 && dealerNatural {
			// 2:1, plus the premium itself back.
			won += st.Insurance * 3
			st.Stack += st.Insurance * 3
		}
		delta := won - st.Staked
		sum.Deltas[st.PlayerID] = delta
		sum.Stacks[st.PlayerID] = st.Stack
		if delta > 0 {
			sum.Winners = append(sum.Winners, st.PlayerID)
		}
	}

	s.Rounds = append(s.Rounds, sum)
	s.Phase = ""
	s.Current, s.CurrentHand = -1, -1

	events := []module.Event{{Type: "round_settled", Data: map[string]any{
		"round": s.RoundNumber, "dealerTotal": dealerTotal,
	}}}

	// The match end is checked before the pause, so the final round shows the
	// match's own result rather than an interstitial for a round nobody will
	// play.
	if matchOverBeforeRound(s) {
		return append(events, endMatch(s)...)
	}
	if s.Pause {
		s.Break.Begin(s.RoundNumber + 1)
		return events
	}
	return append(events, startRound(s)...)
}

// settleHand prices one hand against the dealer's, returning what it made of
// itself and what comes back to the stack — the stake included, so a push is
// a payout of the stake and a loss is a payout of nothing.
func settleHand(s *GameState, h *Hand, dealerTotal int, dealerNatural, dealerBust bool) (string, int) {
	switch {
	case h.Surrendered:
		// Half the stake back, and the odd chip with it: a player who folds
		// early should not also pay the rounding.
		return outcomeSurrender, h.Bet - h.Bet/2
	case IsBust(h.Cards):
		// Bust first, and before the dealer is looked at at all. A hand that
		// went over has lost even to a dealer who then went over too, which is
		// the house's whole edge in one line.
		return outcomeBust, 0
	}

	natural := isNatural(h.Cards) && !h.Split
	switch {
	case natural && dealerNatural:
		return outcomePush, h.Bet
	case natural:
		return outcomeBlackjack, h.Bet + h.Bet*s.BlackjackPayout/100
	case dealerNatural:
		return outcomeLose, 0
	case dealerBust:
		return outcomeWin, h.Bet * 2
	}

	total, _ := Total(h.Cards)
	switch {
	case total > dealerTotal:
		return outcomeWin, h.Bet * 2
	case total == dealerTotal:
		return outcomePush, h.Bet
	default:
		return outcomeLose, 0
	}
}

// endMatch settles the match on chips. More than one winner is a real
// outcome: two seats can finish a fixed number of rounds level.
func endMatch(s *GameState) []module.Event {
	s.Status = "completed"
	s.Phase = ""
	s.Current, s.CurrentHand = -1, -1

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

// drawCard takes the next card off the shoe, shuffling a fresh one if it runs
// out mid-round. That is the table's own "shuffle up": running out of cards is
// not a state a hand can be left in.
func drawCard(s *GameState) string {
	if len(s.Shoe) == 0 {
		s.Shoe = buildShoe(s.Decks, s.Seed+int64(s.RoundNumber)*7919+1)
	}
	card := s.Shoe[len(s.Shoe)-1]
	s.Shoe = s.Shoe[:len(s.Shoe)-1]
	return card
}

func handID(playerID string, n int) string {
	return "hand:" + playerID + ":" + strconv.Itoa(n)
}
