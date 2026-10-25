package sietemedio

import (
	"strconv"

	"zolik/server/internal/module"
)

// Module is the Siete y Medio game module.
type Module struct{}

// New returns the module. Stateless: every method takes the state it works on.
func New() *Module { return &Module{} }

var _ module.GameModule = (*Module)(nil)

// NewMatch seats the players, cuts for the bank and opens the staking.
func (m *Module) NewMatch(cfg module.MatchConfig, players []module.PlayerRef, seed int64) (module.State, error) {
	if len(players) < 2 || len(players) > 8 {
		return nil, module.Error{Code: ErrWrongPlayerCount, Message: "siete y medio seats two to eight"}
	}
	s := tableOf(cfg)
	s.Status = "active"
	s.Seed = seed
	s.Current = -1

	for _, p := range players {
		s.Seats = append(s.Seats, Seat{PlayerID: p.ID, Stack: s.StartingStack, Bot: p.IsAI})
	}
	s.Banker = cutForBank(s, seed)
	s.Deck = buildDeck(seed)

	startRound(s)
	return encode(s)
}

// cutForBank is the table cutting for the deal.
//
// Somebody has to bank first, and seat order is the wrong answer: the bank has
// a real edge at this game — it wins ties at most tables and collects from a
// seat that went over even when it goes over itself — so handing it to the
// host every match would hand them the edge. A cut is what a table does, and
// from the match seed it is reproducible, which is what a test needs.
//
// A pack of its own rather than the top of the one the round is dealt from, so
// cutting for the bank does not change what anybody is dealt.
func cutForBank(s *GameState, seed int64) int {
	cut := buildDeck(seed ^ 0x5157)
	best, at := -1, 0
	for i := range s.Seats {
		if v := cardHalves(cut[i]); v > best {
			best, at = v, i
		}
	}
	return at
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

	// Getting up is allowed at any point of a round — see applyLeave.
	if a.Verb == module.VerbLeave {
		events, err := applyLeave(s, playerID)
		if err != nil {
			return raw, nil, err
		}
		out, err := encode(s)
		return out, events, err
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
	case VerbStake:
		events, err = applyStake(s, seat, a)
	case VerbHit, VerbStand:
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

// startRound settles who can still afford a seat, makes sure somebody can hold
// the bank, shuffles, and opens the staking.
func startRound(s *GameState) []module.Event {
	if matchOverBeforeRound(s) {
		return endMatch(s)
	}
	// The bank was moved at settlement; all that is left is to check the seat
	// it moved to can still answer a stake, and to walk on round the table
	// until one can. A table where nobody can is a table with nothing to play
	// for, which is the match over.
	if !canBank(s, s.Banker) {
		s.Banker = nextBanker(s, s.Banker)
	}
	if !canBank(s, s.Banker) {
		return endMatch(s)
	}
	s.RoundNumber++

	for i := range s.Seats {
		st := &s.Seats[i]
		st.Stake, st.Cards = 0, nil
		st.Stood, st.Shown, st.NoRoom = false, false, false
		st.Outcome, st.Payout = outcomeUnfinished, 0
	}
	s.Current = -1
	s.Phase = phaseStakes
	// A fresh pack every round, which is how this game is dealt: it is a hand
	// of two or three cards each, and the pack goes back together between
	// deals. Nothing here is a shoe, and there is nothing to count.
	s.Deck = buildDeck(s.Seed + int64(s.RoundNumber)*7919)

	events := []module.Event{{Type: "round_started", Data: map[string]any{
		"round": s.RoundNumber, "banker": s.Seats[s.Banker].PlayerID,
	}}}
	events = append(events, closeNoRoom(s)...)
	if stakesIn(s) {
		events = append(events, deal(s)...)
	}
	return events
}

// matchOverBeforeRound marks the seats that can no longer meet the minimum and
// reports whether that, or the round limit, ends the match.
//
// Split out of startRound for holdem's reason: a table that pauses between
// rounds has to know the match is over *before* it stops to show a results
// screen, or the last round puts up an interstitial for a round that will never
// be dealt.
func matchOverBeforeRound(s *GameState) bool {
	live, people, left := 0, 0, false
	for i := range s.Seats {
		// A seat that asked to go mid-round gets up now the round is over.
		if s.Seats[i].Leaving {
			s.Seats[i].Leaving, s.Seats[i].Left = false, true
		}
		if s.Seats[i].Left {
			s.Seats[i].Out = true
			left = true
		}
		if s.Seats[i].Stack < s.MinStake {
			s.Seats[i].Out = true
		}
		if !s.Seats[i].Out {
			live++
			if !s.Seats[i].Bot {
				people++
			}
		}
	}
	// Two is the smallest table this game has: somebody to bank and somebody
	// to play against them. One seat with chips is not a short table, it is no
	// table at all.
	if live < 2 {
		return true
	}
	// Somebody got up and no person is still dealt in: the bots would only be
	// playing each other for nobody.
	if left && people == 0 {
		return true
	}
	return s.RoundLimit > 0 && s.RoundNumber >= s.RoundLimit
}

// closeNoRoom settles the seats the bank cannot answer a minimum stake for.
//
// It is la banca responde turned into a board state rather than a refusal:
// once the stakes already down have used up what the banker holds, a seat
// still waiting has no legal stake left, and showing it a control whose only
// value does not exist is a dead position. So it is marked as sitting the
// round out and the table stops waiting on it.
//
// Called after every stake as well as at the top of the round, because whether
// a seat has room depends on what the seats before it put up.
func closeNoRoom(s *GameState) []module.Event {
	var events []module.Event
	for i := range s.Seats {
		st := &s.Seats[i]
		if st.Out || st.NoRoom || st.inRound() || s.isBanker(i) {
			continue
		}
		if maxStakeFor(s, i) < s.MinStake {
			st.NoRoom = true
			events = append(events, module.Event{Type: "no_room", Data: map[string]any{
				"playerId": st.PlayerID,
			}})
		}
	}
	return events
}

// stakesIn reports every seat that can play having put a stake up.
func stakesIn(s *GameState) bool {
	for i := range s.Seats {
		st := &s.Seats[i]
		if st.Out || st.NoRoom || s.isBanker(i) {
			continue
		}
		if st.Stake == 0 {
			return false
		}
	}
	return true
}

func applyStake(s *GameState, seat *Seat, a module.Action) ([]module.Event, error) {
	if s.Phase != phaseStakes {
		return nil, errCode(ErrWrongPhase)
	}
	i := s.seatIndex(seat.PlayerID)
	if s.isBanker(i) {
		return nil, errCode(ErrBankerDoesNotStake)
	}
	if seat.Out || seat.NoRoom {
		return nil, errCode(ErrNotInRound)
	}
	if seat.Stake > 0 {
		return nil, errCode(ErrAlreadyStaked)
	}
	raw, ok := a.Params[ParamAmount]
	if !ok || raw == "" {
		return nil, errCode(ErrAmountRequired)
	}
	amount, err := strconv.Atoi(raw)
	if err != nil {
		return nil, errCode(ErrAmountNotNumber)
	}
	if amount < s.MinStake {
		return nil, errCode(ErrStakeTooSmall)
	}
	if amount > seat.Stack {
		return nil, errCode(ErrNotEnoughChips)
	}
	// What the seat holds comes before what the bank can answer for, because
	// "you have not got that many chips" is a different sentence to "the bank
	// cannot cover that", and a player handed the wrong one of the two goes
	// looking for the wrong thing.
	if amount > maxStakeFor(s, i) {
		return nil, errCode(ErrBankCannotCover)
	}

	seat.Stack -= amount
	seat.Stake = amount
	events := []module.Event{{Type: "stake_placed", Data: map[string]any{
		"playerId": seat.PlayerID, "amount": amount,
	}}}

	events = append(events, closeNoRoom(s)...)
	if stakesIn(s) {
		return append(events, deal(s)...), nil
	}
	return events, nil
}

// applyLeave is a player getting up from the table with their chips.
//
// Whenever they like. Before their stake is down, or between rounds, they go at
// once — a stake already up but not yet dealt to is handed back — and the table
// carries on as if their stake were in. With cards in front of them they play
// the round out, sat out if they have gone, and get up when it settles: the
// cards are dealt, and a hand walked away from is a stake nobody could settle.
//
// The banker is no exception. The bank is answered out of a stack that is still
// on the table until the round is paid, so a banker who asks to go banks the
// round out and the bank moves on at the top of the next one.
func applyLeave(s *GameState, playerID string) ([]module.Event, error) {
	seat := s.seat(playerID)
	if seat == nil {
		return nil, errCode(module.ErrNotSeated)
	}
	if seat.Left || seat.Leaving {
		return nil, errCode(ErrAlreadyLeft)
	}
	dealt := !s.Break.Open && s.Phase != phaseStakes
	if dealt && (seat.inRound() || s.isBanker(s.seatIndex(playerID))) {
		seat.Leaving = true
		return []module.Event{{Type: "leaving", Data: map[string]any{"playerId": playerID}}}, nil
	}
	if !s.Break.Open && seat.Stake > 0 {
		seat.Stack += seat.Stake
		seat.Stake = 0
	}
	seat.Left, seat.Out = true, true
	events := []module.Event{{Type: "left", Data: map[string]any{"playerId": playerID, "chips": seat.Stack}}}
	switch {
	case s.Break.Open:
		if s.Break.Settled(order(s)) {
			s.Break.Close()
			events = append(events, startRound(s)...)
		}
	case s.Phase == phaseStakes:
		// They may have been the stake the deal was waiting on — or the last
		// person at the table.
		if matchOverBeforeRound(s) {
			events = append(events, endMatch(s)...)
		} else if stakesIn(s) {
			events = append(events, deal(s)...)
		}
	}
	return events, nil
}

// deal puts one card in front of every stake and one in front of the bank.
//
// One card, face down, and then the table plays: that is the deal this game
// has, and it is why the first decision a player makes is taken on a single
// card. Everything after it is dealt face up, so the table can see what a seat
// is drawing to without knowing what it started from.
func deal(s *GameState) []module.Event {
	for _, i := range playOrder(s) {
		st := &s.Seats[i]
		if !st.inRound() && !s.isBanker(i) {
			continue
		}
		st.Cards = append(st.Cards, drawCard(s))
		if !s.FirstCardDown {
			st.Shown = true
		}
	}

	events := []module.Event{{Type: "cards_dealt", Data: map[string]any{"round": s.RoundNumber}}}
	s.Phase, s.Current = phasePlay, -1
	return append(events, advance(s)...)
}

// --- playing the hands --------------------------------------------------

func applyPlay(s *GameState, seat *Seat, verb string) ([]module.Event, error) {
	if s.Phase != phasePlay && s.Phase != phaseBank {
		return nil, errCode(ErrWrongPhase)
	}
	if s.Current < 0 || s.Seats[s.Current].PlayerID != seat.PlayerID {
		return nil, errCode(ErrNotYourTurn)
	}

	var events []module.Event
	switch verb {
	case VerbHit:
		card := drawCard(s)
		seat.Cards = append(seat.Cards, card)
		if IsBust(seat.Cards) {
			// Se pasó: the hand is over and goes face up, which is what a
			// player does with it at a real table.
			seat.Stood, seat.Shown = true, true
		} else if Total(seat.Cards) >= Target {
			// 7½ stands itself: there is no card in the pack that does not
			// bust it, so offering a draw would be offering a dead position.
			seat.Stood = true
		}
		events = []module.Event{{Type: "card_drawn", Data: map[string]any{
			"playerId": seat.PlayerID, "card": card,
		}}}
	case VerbStand:
		seat.Stood = true
		events = []module.Event{{Type: "stood", Data: map[string]any{"playerId": seat.PlayerID}}}
	}
	return append(events, advance(s)...), nil
}

// advance walks to the next seat with a decision in it, and turns the bank over
// when the table is done.
//
// It re-examines the seat already on turn first rather than stepping past it: a
// player who draws and is still under 7½ has another decision to make, and it
// is the same seat's.
func advance(s *GameState) []module.Event {
	if s.Phase == phaseBank {
		b := s.banker()
		if b == nil {
			return settle(s)
		}
		if b.Stood || Total(b.Cards) >= Target {
			b.Stood = true
			return settle(s)
		}
		return nil
	}

	ord := playOrder(s)
	from := 0
	if s.Current >= 0 {
		for k, i := range ord {
			if i == s.Current {
				from = k
				break
			}
		}
	}
	for k := from; k < len(ord); k++ {
		i := ord[k]
		st := &s.Seats[i]
		if s.isBanker(i) {
			return bankTurn(s)
		}
		if !st.inRound() || st.Stood {
			continue
		}
		if Total(st.Cards) >= Target {
			st.Stood = true
			if IsBust(st.Cards) {
				st.Shown = true
			}
			continue
		}
		s.Phase, s.Current = phasePlay, i
		return nil
	}
	return bankTurn(s)
}

// bankTurn is the banker turning their card over and playing last.
//
// The bank only draws when there is something to beat: with every hand gone
// over, the bank has already won them all and a table does not draw cards for
// the sake of it.
func bankTurn(s *GameState) []module.Event {
	b := s.banker()
	if b == nil {
		return settle(s)
	}
	s.Phase, s.Current = phaseBank, s.Banker
	b.Shown = true

	events := []module.Event{{Type: "bank_turn", Data: map[string]any{"playerId": b.PlayerID}}}
	if !contested(s) || Total(b.Cards) >= Target {
		b.Stood = true
		return append(events, settle(s)...)
	}
	return events
}

// contested reports at least one hand the bank still has to beat.
func contested(s *GameState) bool {
	for i := range s.Seats {
		if s.isBanker(i) {
			continue
		}
		if s.Seats[i].inRound() && !IsBust(s.Seats[i].Cards) {
			return true
		}
	}
	return false
}

// --- settlement ----------------------------------------------------------

// settle pays the round out of the banker's stack, records it, moves the bank,
// and goes on to the next round — or to the results screen, or to the end of
// the match.
func settle(s *GameState) []module.Event {
	b := s.banker()
	bankTotal := Total(b.Cards)
	bankBust := bankTotal > Target
	bankSeven := bankTotal == Target

	sum := RoundSummary{
		Number:    s.RoundNumber,
		Banker:    b.PlayerID,
		BankTotal: bankTotal,
		BankBust:  bankBust,
		BankSeven: bankSeven,
		Totals:    map[string]int{},
		Outcomes:  map[string]string{},
		Deltas:    map[string]int{},
		Stacks:    map[string]int{},
	}

	// The bank's own result is the sum of everybody else's, with the sign
	// turned round: every chip a seat wins comes out of the banker's stack and
	// every chip it loses goes into it. There is no house here to absorb the
	// difference, which is the whole reason this game is not blackjack.
	bankDelta := 0
	for _, i := range playOrder(s) {
		st := &s.Seats[i]
		if s.isBanker(i) || !st.inRound() {
			continue
		}
		st.Outcome, st.Payout = settleHand(s, st, bankTotal, bankBust, bankSeven)
		st.Stack += st.Payout
		st.Shown = true
		bankDelta += st.Stake - st.Payout
		sum.Totals[st.PlayerID] = Total(st.Cards)
		sum.Outcomes[st.PlayerID] = st.Outcome
		sum.Deltas[st.PlayerID] = st.Payout - st.Stake
	}
	b.Stack += bankDelta
	b.Shown = true
	sum.Totals[b.PlayerID] = bankTotal
	sum.Deltas[b.PlayerID] = bankDelta

	for i := range s.Seats {
		sum.Stacks[s.Seats[i].PlayerID] = s.Seats[i].Stack
		if sum.Deltas[s.Seats[i].PlayerID] > 0 {
			sum.Winners = append(sum.Winners, s.Seats[i].PlayerID)
		}
	}

	s.Rounds = append(s.Rounds, sum)
	s.Phase = ""
	s.Current = -1
	moveBank(s, sum)

	events := []module.Event{{Type: "round_settled", Data: map[string]any{
		"round": s.RoundNumber, "bankTotal": bankTotal,
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

// settleHand prices one hand against the bank's, returning what it was worth
// and what comes back to the stack — the stake included, so a push is a payout
// of the stake and a loss is a payout of nothing.
func settleHand(s *GameState, st *Seat, bankTotal int, bankBust, bankSeven bool) (string, int) {
	total := Total(st.Cards)
	if total > Target {
		// Gone over, and before the bank is looked at at all. A hand that
		// passed has lost even to a bank that then passes too, which is the
		// bank's whole edge in one line.
		return outcomeBust, 0
	}

	seven := total == Target
	switch {
	case seven && bankSeven:
		// Two sevens and a half. It is a tie like any other and goes wherever
		// this table sends a tie — the bonus is for beating the bank, not for
		// holding the hand.
		return s.tie(st)
	case seven:
		if s.SevenDoubles {
			return outcomeSeven, st.Stake * 3
		}
		return outcomeSeven, st.Stake * 2
	case bankSeven:
		return outcomeLose, 0
	case bankBust, total > bankTotal:
		return outcomeWin, st.Stake * 2
	case total == bankTotal:
		return s.tie(st)
	default:
		return outcomeLose, 0
	}
}

// tie settles a level hand the way this table settles one.
func (s *GameState) tie(st *Seat) (string, int) {
	if s.TiesToBank {
		return outcomeLose, 0
	}
	return outcomePush, st.Stake
}

// moveBank decides who holds the bank for the next round.
//
// Here rather than at the top of the next round, so that the board and the
// round log agree about who is about to deal while the table is still reading
// the result of the one just played.
func moveBank(s *GameState, sum RoundSummary) {
	switch s.BankPass {
	case bankRound:
		s.Banker = nextBanker(s, s.Banker)
	case bankOnSeven:
		// The classic rule: a player who makes exactly 7½ takes the bank. The
		// banker making it too keeps it — they were already holding it, and
		// nothing beat them.
		if sum.BankSeven {
			return
		}
		for _, i := range playOrder(s) {
			if s.isBanker(i) {
				continue
			}
			st := &s.Seats[i]
			if st.inRound() && Total(st.Cards) == Target && canBank(s, i) {
				s.Banker = i
				return
			}
		}
	}
}

// nextBanker is the seat after this one that can hold the bank, or the same
// seat when nobody else can.
func nextBanker(s *GameState, from int) int {
	n := len(s.Seats)
	for step := 1; step <= n; step++ {
		if i := (from + step) % n; canBank(s, i) {
			return i
		}
	}
	return from
}

// endMatch settles the match on chips. More than one winner is a real outcome:
// two seats can finish a fixed number of rounds level.
func endMatch(s *GameState) []module.Event {
	s.Status = "completed"
	s.Phase = ""
	s.Current = -1
	for i := range s.Seats {
		s.Seats[i].Shown = true
	}

	top := 0
	for i := range s.Seats {
		if s.Seats[i].Stack > top {
			top = s.Seats[i].Stack
		}
	}
	// Nobody is named when nobody has a chip left — a table that all went
	// broke was won by no one. It cannot happen at this game, where every chip
	// lost is a chip somebody else won, but endMatch is reached from four
	// places and a joint win for three busted seats would read as "You won."
	// to a player who had just lost everything.
	if top > 0 {
		for i := range s.Seats {
			if s.Seats[i].Stack == top {
				s.Winners = append(s.Winners, s.Seats[i].PlayerID)
			}
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

// drawCard takes the next card off the pack, replacing it if it runs out
// mid-round. Running out is not a state a hand can be left in.
//
// Forty cards and a hand of two or three each means it effectively never
// happens — the whole pack is 118 points and the table stops at 7½ a seat —
// but "effectively never" is not a guarantee, and a nil card is a hand worth
// nothing.
func drawCard(s *GameState) string {
	if len(s.Deck) == 0 {
		reshuffle(s)
	}
	card := s.Deck[len(s.Deck)-1]
	s.Deck = s.Deck[:len(s.Deck)-1]
	return card
}

// reshuffle replaces the pack, leaving out the cards already on the table — so
// a card in front of one player is never dealt to another a moment later.
func reshuffle(s *GameState) {
	inPlay := map[string]int{}
	for i := range s.Seats {
		for _, c := range s.Seats[i].Cards {
			inPlay[c]++
		}
	}
	fresh := buildDeck(s.Seed + int64(s.RoundNumber)*7919 + 1)
	out := make([]string, 0, len(fresh))
	for _, c := range fresh {
		if inPlay[c] > 0 {
			inPlay[c]--
			continue
		}
		out = append(out, c)
	}
	if len(out) == 0 {
		out = fresh
	}
	s.Deck = out
}
