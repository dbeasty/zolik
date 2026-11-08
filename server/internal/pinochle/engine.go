package pinochle

import (
	"math/rand"
	"strconv"

	"zolik/server/internal/module"
	"zolik/server/internal/tricks"
)

var _ module.GameModule = (*Module)(nil)

// NewMatch seats the variation's table and deals the first hand.
func (m *Module) NewMatch(cfg module.MatchConfig, players []module.PlayerRef, seed int64) (module.State, error) {
	c := resolve(cfg)
	t := tableOf(c.variation)
	if len(players) != t.seats {
		return nil, module.Error{Code: ErrWrongPlayers, Message: "pinochle " + c.variation + " is played by " + strconv.Itoa(t.seats)}
	}
	s := &GameState{
		Status:         "active",
		Seed:           seed,
		Variation:      c.variation,
		Scoring:        c.scoring,
		MinBid:         c.minBid,
		Target:         c.target,
		PassCount:      c.pass,
		MeldNeeds:      c.meldNeeds,
		AllPass:        c.allPass,
		HideCardPoints: !c.showCardPoints,
		Pause:          cfg.PauseBetweenRounds(true),
		FirstDealer:    module.StartingSeat(seed, len(players)),
		Scores:         map[string]int{},
	}
	if !t.partners {
		s.PassCount = 0
	}
	for _, p := range players {
		s.Players = append(s.Players, p.ID)
	}
	for _, side := range s.sides() {
		s.Scores[side] = 0
	}
	startDeal(s)
	return encode(s)
}

// startDeal shuffles and deals the hand numbered s.Deal.
func startDeal(s *GameState) {
	t := s.table()
	deck := t.buildDeck()
	r := rand.New(rand.NewSource(s.Seed + int64(s.Deal)*7919 + 1))
	r.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })

	s.Hands = map[string][]string{}
	s.Bid, s.Bidder, s.Stuck, s.Trump = 0, "", false, ""
	s.Passed, s.LastBid = map[string]bool{}, map[string]int{}
	s.Widow, s.Buried, s.Given = nil, nil, map[string][]string{}
	s.Melds, s.Used, s.MeldLog = map[string]Meld{}, map[string]Used{}, nil
	s.Stock, s.TurnUp, s.Late = nil, "", false
	s.Trick, s.LastTrick, s.History = nil, nil, nil
	s.Taken, s.Tricks, s.Counters, s.MeldScore = map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}

	p := s.next(s.dealer())
	for i := 0; i < len(s.Players); i++ {
		s.Hands[p] = append([]string(nil), deck[i*t.handSize:(i+1)*t.handSize]...)
		p = s.next(p)
	}
	rest := deck[len(s.Players)*t.handSize:]

	switch {
	case s.Variation == VarTwoHanded:
		// The next card is turned for trumps and lies across the foot of
		// the stock; it is the last card drawn. A dix turned up scores ten
		// for the dealer at once.
		s.TurnUp = rest[0]
		s.Stock = append([]string(nil), rest[1:]...)
		s.Trump = string(tricks.Suit(s.TurnUp))
		if tricks.Rank(s.TurnUp) == '9' {
			s.MeldScore[s.side(s.dealer())] += pointsDix
		}
		for _, p := range s.Players {
			s.Used[p] = Used{}
		}
		s.Leader = s.next(s.dealer())
		s.Phase, s.Current = phasePlay, s.Leader
	default:
		s.Widow = append([]string(nil), rest...)
		s.Phase, s.Current = phaseAuction, s.next(s.dealer())
	}
	s.sortAll()
}

// Apply validates and applies one move. The state is decoded fresh on every
// call, so a refused action leaves the caller's bytes untouched — which is
// what lets the offer list probe it.
func (m *Module) Apply(raw module.State, playerID string, a module.Action) (module.State, []module.Event, error) {
	s, err := decode(raw)
	if err != nil {
		return raw, nil, err
	}
	if s.Status != "active" {
		return raw, nil, errCode(ErrGameNotActive)
	}
	if s.Intermission.Open {
		if a.Verb != module.VerbContinue {
			return raw, nil, errCode(ErrNotYourTurn)
		}
		if err := s.Intermission.Mark(s.Players, playerID); err != nil {
			return raw, nil, err
		}
		var events []module.Event
		if s.Intermission.Settled(s.Players) {
			s.Intermission.Close()
			startDeal(s)
			events = []module.Event{{Type: "deal_started", Data: map[string]any{"deal": s.Deal + 1}}}
		}
		out, err := encode(s)
		return out, events, err
	}
	if s.seat(playerID) < 0 || s.Current != playerID {
		return raw, nil, errCode(ErrNotYourTurn)
	}

	var events []module.Event
	switch a.Verb {
	case VerbBid:
		events, err = s.bid(playerID, a.Params[ParamAmount])
	case VerbPass:
		events, err = s.pass(playerID)
	case VerbTrump:
		events, err = s.nameTrump(playerID, a.Params[ParamSuit])
	case VerbBury:
		events, err = s.bury(playerID, a.Cards)
	case VerbGive:
		events, err = s.give(playerID, a.Cards)
	case VerbPlay:
		if len(a.Cards) != 1 {
			return raw, nil, errCode(ErrPickOneCard)
		}
		events, err = s.play(playerID, a.Cards[0])
	case VerbThrowIn:
		events, err = s.throwIn(playerID)
	case VerbMeld:
		events, err = s.meld(playerID, a.OfferID)
	case VerbDraw:
		events, err = s.drawTurn(playerID)
	default:
		return raw, nil, module.Error{Code: ErrUnknownAction, Message: a.Verb}
	}
	if err != nil {
		return raw, nil, err
	}
	out, err := encode(s)
	return out, events, err
}

// --- the auction -------------------------------------------------------------

// minNext is the least the next bid may be.
func (s *GameState) minNext() int {
	if s.Bid == 0 {
		return s.MinBid
	}
	return s.Bid + bidStep
}

func (s *GameState) bid(p, amount string) ([]module.Event, error) {
	if s.Phase != phaseAuction {
		return nil, errCode(ErrWrongPhase)
	}
	if s.Passed[p] {
		return nil, errCode(ErrAlreadyPassed)
	}
	n, err := strconv.Atoi(amount)
	if err != nil {
		return nil, errCode(ErrNotANumber)
	}
	switch {
	case n < s.minNext():
		return nil, errCode(ErrBidTooLow)
	case n%bidStep != 0:
		return nil, errCode(ErrBidStep)
	case n > maxBidTop:
		return nil, errCode(ErrBidTooHigh)
	}
	s.Bid, s.Bidder, s.LastBid[p] = n, p, n
	events := []module.Event{{Type: "bid", Data: map[string]any{"playerId": p, "amount": n}}}
	return append(events, s.advanceAuction(p)...), nil
}

func (s *GameState) pass(p string) ([]module.Event, error) {
	if s.Phase != phaseAuction {
		return nil, errCode(ErrWrongPhase)
	}
	if s.Passed[p] {
		return nil, errCode(ErrAlreadyPassed)
	}
	s.Passed[p], s.LastBid[p] = true, -1
	events := []module.Event{{Type: "passed", Data: map[string]any{"playerId": p}}}
	return append(events, s.advanceAuction(p)...), nil
}

// advanceAuction moves the auction on after p's call: to the next seat
// still in, or to its end.
func (s *GameState) advanceAuction(p string) []module.Event {
	var in []string
	for _, q := range s.Players {
		if !s.Passed[q] {
			in = append(in, q)
		}
	}
	switch {
	case s.Bidder != "" && len(in) == 1:
		return s.endAuction()
	case len(in) == 0:
		if s.AllPass == AllPassRedeal {
			return s.redeal()
		}
		// Nobody bid: the dealer is stuck with the hand at the minimum.
		s.Bidder, s.Bid, s.Stuck = s.dealer(), s.MinBid, true
		return append([]module.Event{{Type: "dealer_stuck", Data: map[string]any{"playerId": s.Bidder, "amount": s.Bid}}}, s.endAuction()...)
	}
	q := s.next(p)
	for s.Passed[q] {
		q = s.next(q)
	}
	s.Current = q
	return nil
}

func (s *GameState) endAuction() []module.Event {
	events := []module.Event{{Type: "auction_won", Data: map[string]any{"playerId": s.Bidder, "amount": s.Bid}}}
	if len(s.Widow) > 0 {
		s.Hands[s.Bidder] = append(s.Hands[s.Bidder], s.Widow...)
		s.sortAll()
	}
	s.Phase, s.Current = phaseTrump, s.Bidder
	return events
}

// redeal throws the hand in when everyone passed and the table plays that
// way: nobody scores, and the deal moves on.
func (s *GameState) redeal() []module.Event {
	rec := HandRecord{Number: s.Deal + 1, Redealt: true, Meld: map[string]int{}, Taken: map[string]int{}, Delta: map[string]int{}, Totals: map[string]int{}}
	for _, p := range s.Players {
		rec.Totals[p] = s.Scores[s.side(p)]
	}
	s.Rounds = append(s.Rounds, rec)
	s.Deal++
	startDeal(s)
	return []module.Event{{Type: "redeal", Data: map[string]any{"deal": s.Deal + 1}}}
}

// --- trumps, the widow and the pass ------------------------------------------

func (s *GameState) nameTrump(p, suit string) ([]module.Event, error) {
	if s.Phase != phaseTrump {
		return nil, errCode(ErrWrongPhase)
	}
	if len(suit) != 1 || !containsByte(suits, suit[0]) {
		return nil, errCode(ErrNoSuchSuit)
	}
	s.Trump = suit
	s.sortAll()
	events := []module.Event{{Type: "trump_named", Data: map[string]any{"playerId": p, "suit": suit}}}
	switch {
	case s.table().widow > 0:
		s.Phase = phaseBury
	case s.PassCount > 0:
		s.Phase, s.Current = phasePassIn, s.partner(s.Bidder)
	default:
		s.startPlay()
	}
	return events, nil
}

func containsByte(set string, b byte) bool {
	for i := 0; i < len(set); i++ {
		if set[i] == b {
			return true
		}
	}
	return false
}

func (s *GameState) bury(p string, cards []string) ([]module.Event, error) {
	if s.Phase != phaseBury {
		return nil, errCode(ErrWrongPhase)
	}
	if len(cards) != s.table().widow {
		return nil, errCode(ErrBuryCount)
	}
	if !hasCards(s.Hands[p], cards) {
		return nil, errCode(ErrCardNotInHand)
	}
	s.Hands[p] = removeCards(s.Hands[p], cards)
	s.Buried = append([]string(nil), cards...)
	s.startPlay()
	return []module.Event{{Type: "buried", Data: map[string]any{"playerId": p, "n": len(cards)}}}, nil
}

func (s *GameState) give(p string, cards []string) ([]module.Event, error) {
	if s.Phase != phasePassIn && s.Phase != phasePassBack {
		return nil, errCode(ErrWrongPhase)
	}
	if len(cards) != s.PassCount {
		return nil, errCode(ErrGiveCount)
	}
	if !hasCards(s.Hands[p], cards) {
		return nil, errCode(ErrCardNotInHand)
	}
	to := s.partner(p)
	s.Hands[p] = removeCards(s.Hands[p], cards)
	s.Hands[to] = append(s.Hands[to], cards...)
	s.Given[p] = append([]string(nil), cards...)
	s.sortAll()
	events := []module.Event{{Type: "cards_passed", Data: map[string]any{"playerId": p, "to": to, "n": len(cards)}}}
	if s.Phase == phasePassIn {
		s.Phase, s.Current = phasePassBack, s.Bidder
		return events, nil
	}
	s.startPlay()
	return events, nil
}

// startPlay lays everyone's meld on the table and gives the bidder the lead.
func (s *GameState) startPlay() {
	for _, p := range s.Players {
		s.Melds[p] = BestMeld(s.Hands[p], s.trump(), s.table())
	}
	s.Phase, s.Leader, s.Current = phasePlay, s.Bidder, s.Bidder
}

// throwIn is the bidder conceding before the first lead: the bid is lost
// and the other side scores its meld.
func (s *GameState) throwIn(p string) ([]module.Event, error) {
	if s.Phase != phasePlay || !s.table().auction {
		return nil, errCode(ErrWrongPhase)
	}
	if p != s.Bidder {
		return nil, errCode(ErrNotBidder)
	}
	if len(s.History) > 0 || len(s.Trick) > 0 {
		return nil, errCode(ErrTooLate)
	}
	events := []module.Event{{Type: "thrown_in", Data: map[string]any{"playerId": p, "amount": s.Bid}}}
	return append(events, s.endHand(true)...), nil
}

// --- play ----------------------------------------------------------------------

func (s *GameState) play(p, card string) ([]module.Event, error) {
	if s.Phase != phasePlay {
		return nil, errCode(ErrWrongPhase)
	}
	if !hasCards(s.Hands[p], []string{card}) {
		return nil, errCode(ErrCardNotInHand)
	}
	if code := s.whyIllegal(p, card); code != "" {
		return nil, errCode(code)
	}
	s.Hands[p] = removeCards(s.Hands[p], []string{card})
	if u := s.Used[p]; u != nil {
		u.release(s.Hands[p], card)
	}
	s.Trick = append(s.Trick, tricks.Play{Seat: s.seat(p), Card: card})
	events := []module.Event{{Type: "card_played", Data: map[string]any{"playerId": p, "card": card}}}
	if len(s.Trick) < len(s.Players) {
		s.Current = s.next(p)
		return events, nil
	}
	return append(events, s.endTrick()...), nil
}

// whyIllegal names the rule a card would break, or "" when it may be played.
func (s *GameState) whyIllegal(p, card string) string {
	pol, bound := s.policy()
	if !bound || len(s.Trick) == 0 {
		return ""
	}
	hand := s.Hands[p]
	legal := tricks.Legal(hand, tricks.Trick{Plays: s.Trick}, s.trump(), s.table().order(), pol)
	for _, c := range legal {
		if c == card {
			return ""
		}
	}
	led := tricks.Suit(s.Trick[0].Card)
	switch {
	case holdsSuit(hand, led) && tricks.Suit(card) != led:
		return ErrMustFollow
	case holdsSuit(hand, led):
		return ErrMustBeat
	case tricks.Suit(card) != s.trump():
		return ErrMustTrump
	}
	return ErrMustBeat
}

func holdsSuit(hand []string, suit byte) bool {
	for _, c := range hand {
		if tricks.Suit(c) == suit {
			return true
		}
	}
	return false
}

// winner is the seat taking a complete trick: the first of two identical
// cards beats the second, which tricks.Winning already does.
func (s *GameState) winner(plays []tricks.Play) int {
	best, _ := tricks.Trick{Plays: plays}.Winning(s.trump(), s.table().order())
	return best.Seat
}

func (s *GameState) endTrick() []module.Event {
	w := s.Players[s.winner(s.Trick)]
	side := s.side(w)
	s.Taken[side] += s.trickPoints(s.Trick)
	s.Tricks[side]++
	for _, pl := range s.Trick {
		if isCounter(pl.Card, s.Scoring) {
			s.Counters[side]++
		}
	}
	s.History = append(s.History, s.Trick)
	s.LastTrick, s.Trick = s.Trick, nil
	events := []module.Event{{Type: "trick_won", Data: map[string]any{"playerId": w}}}
	s.Leader, s.Current = w, w

	if s.Variation == VarTwoHanded && !s.Late {
		s.Phase = phaseMeld
		return events
	}
	if len(s.Hands[w]) == 0 {
		s.Taken[side] += s.table().lastTrick
		return append(events, s.endHand(false)...)
	}
	s.Phase = phasePlay
	return events
}

// --- two-handed: meld and draw ---------------------------------------------------

// meld lays one meld (or exchanges or shows the dix) and draws.
func (s *GameState) meld(p, offerID string) ([]module.Event, error) {
	if s.Phase != phaseMeld {
		return nil, errCode(ErrWrongPhase)
	}
	trump := s.trump()
	dix := "9" + s.Trump
	var item MeldItem
	switch offerID {
	case OfferExchange:
		if !s.canExchange(p) {
			return nil, errCode(ErrNoExchange)
		}
		s.Hands[p] = append(removeCards(s.Hands[p], []string{dix}), s.TurnUp)
		s.Used[p].release(s.Hands[p], dix)
		s.TurnUp = dix
		item = MeldItem{Kind: MeldDix, Suit: s.Trump, Times: 1, Points: pointsDix, Cards: []string{dix}}
	case OfferDix:
		if !s.canDeclareDix(p) {
			return nil, errCode(ErrNoSuchMeld)
		}
		s.Used[p].of(classSequence)[dix]++
		item = MeldItem{Kind: MeldDix, Suit: s.Trump, Times: 1, Points: pointsDix, Cards: []string{dix}}
	default:
		found := false
		for _, o := range twoHandOptions(s.Hands[p], s.Used[p], trump) {
			if o.ID == offerID {
				item, found = o.MeldItem, true
				break
			}
		}
		if !found {
			return nil, errCode(ErrNoSuchMeld)
		}
		u := s.Used[p].of(classOf(item.Kind))
		for c, n := range countOf(item.Cards) {
			u[c] += n
		}
	}
	s.MeldScore[s.side(p)] += item.Points
	s.MeldLog = append(s.MeldLog, LaidMeld{Player: p, Item: item})
	s.sortAll()
	events := []module.Event{{Type: "meld_laid", Data: map[string]any{"playerId": p, "kind": item.Kind, "points": item.Points}}}
	return append(events, s.draw()...), nil
}

// canExchange: the nine of trumps for the turned-up card, while it is still
// there and is not itself a nine.
func (s *GameState) canExchange(p string) bool {
	if s.TurnUp == "" || tricks.Rank(s.TurnUp) == '9' {
		return false
	}
	return countOf(s.Hands[p])["9"+s.Trump] > 0
}

// canDeclareDix: a nine of trumps not yet shown.
func (s *GameState) canDeclareDix(p string) bool {
	dix := "9" + s.Trump
	return countOf(s.Hands[p])[dix]-s.Used[p].of(classSequence)[dix] > 0
}

func (s *GameState) drawTurn(p string) ([]module.Event, error) {
	if s.Phase != phaseMeld {
		return nil, errCode(ErrWrongPhase)
	}
	return s.draw(), nil
}

// draw: the trick's winner takes the top of the stock, the other player the
// next card — the turned-up trump once the stock is down to it. When the
// stock is gone the melds go back into hand and the play tightens up.
func (s *GameState) draw() []module.Event {
	w := s.Current
	other := s.next(w)
	take := func(p string) {
		switch {
		case len(s.Stock) > 0:
			s.Hands[p] = append(s.Hands[p], s.Stock[0])
			s.Stock = s.Stock[1:]
		case s.TurnUp != "":
			s.Hands[p] = append(s.Hands[p], s.TurnUp)
			s.TurnUp = ""
		}
	}
	take(w)
	take(other)
	s.sortAll()
	s.Phase, s.Leader, s.Current = phasePlay, w, w
	if len(s.Stock) == 0 && s.TurnUp == "" {
		s.Late = true
		for _, p := range s.Players {
			s.Used[p] = Used{}
		}
		return []module.Event{{Type: "stock_exhausted"}}
	}
	return nil
}

// --- the end of a hand -------------------------------------------------------------

func (s *GameState) sideMeld(side string) int {
	n := 0
	for _, p := range s.members(side) {
		n += s.Melds[p].Total
	}
	return n
}

func (s *GameState) endHand(thrownIn bool) []module.Event {
	rec := HandRecord{
		Number: s.Deal + 1, Bidder: s.Bidder, Bid: s.Bid, Trump: s.Trump, Stuck: s.Stuck, ThrownIn: thrownIn,
		Meld: map[string]int{}, Taken: map[string]int{}, Delta: map[string]int{}, Totals: map[string]int{},
	}
	if s.table().auction {
		for _, c := range s.Buried {
			s.Taken[s.side(s.Bidder)] += cardPoints(c, s.Scoring)
		}
	}
	rec.Meld, rec.Delta, rec.Made = s.handDeltas(s.Taken, s.Tricks, s.Counters, thrownIn)
	for _, side := range s.sides() {
		rec.Taken[side] = s.Taken[side]
	}
	for side, d := range rec.Delta {
		s.Scores[side] += d
	}
	for _, p := range s.Players {
		rec.Totals[p] = s.Scores[s.side(p)]
	}
	s.Rounds = append(s.Rounds, rec)
	events := []module.Event{{Type: "deal_settled", Data: map[string]any{
		"deal": rec.Number, "bidder": rec.Bidder, "bid": rec.Bid, "made": rec.Made, "thrownIn": rec.ThrownIn,
	}}}
	s.Current = ""

	if side, ok := s.matchWinner(); ok {
		s.Status = "completed"
		s.Winners = s.members(side)
		return append(events, module.Event{Type: "match_completed", Data: map[string]any{"winners": s.Winners}})
	}
	s.Deal++
	if s.Pause {
		s.Intermission.Begin(s.Deal)
		return events
	}
	startDeal(s)
	return append(events, module.Event{Type: "deal_started", Data: map[string]any{"deal": s.Deal + 1}})
}

// matchWinner is the side that has won, if any: of the sides at or over the
// target, the bidding side goes out first; otherwise the highest, and a tie
// at the top plays another hand.
func (s *GameState) matchWinner() (string, bool) {
	var over []string
	for _, side := range s.sides() {
		if s.Scores[side] >= s.Target {
			over = append(over, side)
		}
	}
	if len(over) == 0 {
		return "", false
	}
	if s.Bidder != "" {
		bs := s.side(s.Bidder)
		for _, side := range over {
			if side == bs {
				return side, true
			}
		}
	}
	best, top := -1<<31, []string(nil)
	for _, side := range over {
		switch {
		case s.Scores[side] > best:
			best, top = s.Scores[side], []string{side}
		case s.Scores[side] == best:
			top = append(top, side)
		}
	}
	if len(top) != 1 {
		return "", false
	}
	return top[0], true
}

// handDeltas is what each side scores for a hand that took these tricks:
// the meld that counts, and the change to each side's score. Pure, so the
// bots can settle a hand they have only imagined the same way the table
// settles a real one. taken must already hold any buried cards.
func (s *GameState) handDeltas(taken, tricksWon, counters map[string]int, thrownIn bool) (meld, delta map[string]int, made bool) {
	meld, delta = map[string]int{}, map[string]int{}
	if !s.table().auction {
		for _, side := range s.sides() {
			meld[side] = s.MeldScore[side]
			delta[side] = s.MeldScore[side] + taken[side]
		}
		return meld, delta, false
	}
	bidSide := s.side(s.Bidder)
	counts := func(side string) bool {
		if s.MeldNeeds == MeldNeedsCounter {
			return counters[side] > 0
		}
		return tricksWon[side] > 0
	}
	for _, side := range s.sides() {
		m := s.sideMeld(side)
		if thrownIn && side == bidSide || !thrownIn && !counts(side) {
			m = 0
		}
		meld[side] = m
		switch {
		case side != bidSide && thrownIn:
			delta[side] = m
		case side != bidSide:
			delta[side] = m + taken[side]
		case thrownIn:
			delta[side] = -s.Bid
		case m+taken[side] >= s.Bid:
			made, delta[side] = true, m+taken[side]
		default:
			delta[side] = -s.Bid
		}
	}
	return meld, delta, made
}

// Finished reports whether the match is over, and who won it: a whole
// partnership at four.
func (m *Module) Finished(raw module.State) (bool, []string, error) {
	s, err := decode(raw)
	if err != nil {
		return false, nil, err
	}
	if s.Status != "completed" {
		return false, nil, nil
	}
	return true, s.Winners, nil
}
