package snaps

import (
	"math/rand"

	"zolik/server/internal/module"
	"zolik/server/internal/tricks"
)

var _ module.GameModule = (*Module)(nil)

// buildDeck is the variation's cards, in the server's codes.
func buildDeck(ranks string) []string {
	out := make([]string, 0, 4*len(ranks))
	for _, s := range "HDCS" {
		for _, r := range ranks {
			out = append(out, string(r)+string(s))
		}
	}
	return out
}

// NewMatch seats two players and deals the first hand.
func (m *Module) NewMatch(cfg module.MatchConfig, players []module.PlayerRef, seed int64) (module.State, error) {
	if len(players) != 2 {
		return nil, module.Error{Code: "TOO_FEW_PLAYERS", Message: "šnaps is played by two"}
	}
	c := resolve(cfg)
	s := &GameState{
		Status:         "active",
		Variation:      c.variation,
		Seed:           seed,
		Target:         c.target,
		ShowCardPoints: c.showCardPoints,
		Pause:          cfg.PauseBetweenRounds(true),
		FirstDealer:    module.StartingSeat(seed, 2),
		Scores:         map[string]int{},
	}
	for _, p := range players {
		s.Players = append(s.Players, p.ID)
		s.Scores[p.ID] = 0
	}
	startDeal(s)
	return encode(s)
}

// startDeal shuffles and deals the deal numbered s.Deal: a hand to each,
// the next card turned up for trumps, and the rest face down on top of it.
// The packets a real dealer gives make no difference to a shuffled pack, so
// only the counts are kept.
func startDeal(s *GameState) {
	r := s.rules()
	deck := buildDeck(r.ranks)
	rng := rand.New(rand.NewSource(s.Seed + int64(s.Deal)*7919))
	rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })

	dealer := s.dealer()
	leader := s.other(dealer)
	s.Hands = map[string][]string{
		leader: append([]string(nil), deck[:r.handSize]...),
		dealer: append([]string(nil), deck[r.handSize:2*r.handSize]...),
	}
	sortHand(s.Hands[leader])
	sortHand(s.Hands[dealer])
	up := deck[2*r.handSize]
	s.Talon = append(append([]string(nil), deck[2*r.handSize+1:]...), up)
	s.Trump = string(tricks.Suit(up))

	s.ClosedBy, s.ClosedOppTricks, s.ClosedOppPoints = "", 0, 0
	s.StrictFrom = -1
	s.Current = leader
	s.Trick, s.LastTrick, s.History = nil, nil, nil
	s.TricksWon, s.Points = map[string]int{}, map[string]int{}
	s.Marriages, s.Known = map[string][]string{}, map[string][]string{}
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
	case VerbPlay:
		events, err = s.play(playerID, a.Cards)
	case VerbExchange:
		err = s.exchange(playerID)
	case VerbClose:
		err = s.close(playerID)
	default:
		return raw, nil, module.Error{Code: ErrUnknownAction, Message: a.Verb}
	}
	if err != nil {
		return raw, nil, err
	}
	out, err := encode(s)
	return out, events, err
}

// exchangeRefusal is why p may not exchange now, or nil.
func (s *GameState) exchangeRefusal(p string) error {
	if len(s.Trick) > 0 {
		return errCode(ErrNotOnLead)
	}
	if !s.open() {
		return errCode(ErrTalonClosed)
	}
	r := s.rules()
	if !hasCard(s.Hands[p], string(r.exchange)+s.Trump) {
		return errCode(ErrNoExchangeCard)
	}
	if r.exchangeAfterTrick && s.TricksWon[p] == 0 {
		return errCode(ErrExchangeNoTrick)
	}
	return nil
}

// exchange swaps the trump spodek (or nine) for the turned-up trump.
func (s *GameState) exchange(p string) error {
	if err := s.exchangeRefusal(p); err != nil {
		return err
	}
	low := string(s.rules().exchange) + s.Trump
	up := s.Talon[len(s.Talon)-1]
	s.Hands[p] = append(removeCard(s.Hands[p], low), up)
	sortHand(s.Hands[p])
	s.Talon[len(s.Talon)-1] = low
	s.Known[p] = append(s.Known[p], up) // everyone saw it taken
	return nil
}

// closeRefusal is why p may not close now, or nil.
func (s *GameState) closeRefusal() error {
	if len(s.Trick) > 0 {
		return errCode(ErrNotOnLead)
	}
	if !s.open() {
		return errCode(ErrTalonClosed)
	}
	return nil
}

// close turns the trump face down on the talon: no more drawing, the strict
// rules from now on, and the closer must reach 66.
func (s *GameState) close(p string) error {
	if err := s.closeRefusal(); err != nil {
		return err
	}
	o := s.other(p)
	s.ClosedBy = p
	s.ClosedOppTricks, s.ClosedOppPoints = s.TricksWon[o], s.total(o)
	s.StrictFrom = len(s.History)
	return nil
}

// playRefusal says which obligation a card breaks.
func (s *GameState) playRefusal(p, card string) error {
	if !hasCard(s.Hands[p], card) {
		return errCode(ErrCardNotInHand)
	}
	if hasCard(s.legal(p), card) {
		return nil
	}
	led := tricks.Trick{Plays: s.Trick}.Led()
	for _, c := range s.Hands[p] {
		if tricks.Suit(c) == led {
			if tricks.Suit(card) != led {
				return errCode(ErrMustFollow)
			}
			return errCode(ErrMustBeat)
		}
	}
	return errCode(ErrMustTrump)
}

// play puts one card on the trick, and settles the trick and the deal when
// they are over.
func (s *GameState) play(p string, cards []string) ([]module.Event, error) {
	if len(cards) != 1 {
		return nil, errCode(ErrPickOneCard)
	}
	card := cards[0]
	if err := s.playRefusal(p, card); err != nil {
		return nil, err
	}

	events := []module.Event{{Type: "card_played", Data: map[string]any{"playerId": p, "card": card}}}
	lead := len(s.Trick) == 0
	// A král or svršek led while holding the other is a marriage, and the
	// other card is shown (docs/snaps-rules.md, simplification 2).
	if lead && s.marriagesAllowed() {
		if other := partner(card); other != "" && hasCard(s.Hands[p], other) {
			suit := string(tricks.Suit(card))
			s.Marriages[p] = append(s.Marriages[p], suit)
			s.Known[p] = append(s.Known[p], other)
			events = append(events, module.Event{Type: "marriage", Data: map[string]any{"playerId": p, "suit": suit}})
		}
	}
	s.Hands[p] = removeCard(s.Hands[p], card)
	s.Known[p] = removeCard(s.Known[p], card)
	s.Trick = append(s.Trick, tricks.Play{Seat: s.seat(p), Card: card})

	if lead {
		// A marriage can carry a player who has a trick to 66 on the lead.
		if s.TricksWon[p] > 0 && s.total(p) >= goal {
			return append(events, s.endOut(p)...), nil
		}
		s.Current = s.other(p)
		return events, nil
	}
	return append(events, s.finishTrick()...), nil
}

// finishTrick takes the trick, draws from the talon, and ends the deal if
// someone is out or the cards have run out.
func (s *GameState) finishTrick() []module.Event {
	win, _ := tricks.Trick{Plays: s.Trick}.Winning(s.Trump[0], order)
	w := s.Players[win.Seat]
	l := s.other(w)
	for _, pl := range s.Trick {
		s.Points[w] += cardPoints(pl.Card)
	}
	s.TricksWon[w]++
	s.History = append(s.History, s.Trick)
	s.LastTrick, s.Trick = s.Trick, nil
	events := []module.Event{{Type: "trick_won", Data: map[string]any{"playerId": w}}}

	if s.open() {
		// The winner draws first; the loser's card is the turned-up trump
		// when it is the last one, and everyone saw it.
		s.Hands[w] = append(s.Hands[w], s.Talon[0])
		s.Hands[l] = append(s.Hands[l], s.Talon[1])
		if len(s.Talon) == 2 {
			s.Known[l] = append(s.Known[l], s.Talon[1])
			s.StrictFrom = len(s.History)
		}
		s.Talon = s.Talon[2:]
		sortHand(s.Hands[w])
		sortHand(s.Hands[l])
	}
	s.Current = w

	last := len(s.Hands[w]) == 0
	r := s.rules()
	if last && s.ClosedBy == "" {
		s.Points[w] += r.lastTrickBonus
	}
	switch {
	case s.total(w) >= goal:
		return append(events, s.endOut(w)...)
	case !last:
		return events
	case s.ClosedBy != "":
		winner := s.other(s.ClosedBy)
		return append(events, s.endDeal(winner, closerFailed(s.closing()), reasonCloserFailed)...)
	}
	seat, points := r.runOut(s.tally(), s.seat(w))
	if seat < 0 {
		return append(events, s.endDeal("", 0, reasonDraw)...)
	}
	reason := reasonHigher
	if r.lastTrickWins {
		reason = reasonLastTrick
	}
	return append(events, s.endDeal(s.Players[seat], points, reason)...)
}

// tally is the count by seat, for the scoring.
func (s *GameState) tally() tally {
	var t tally
	for i, p := range s.Players {
		t.points[i] = s.Points[p]
		t.tricks[i] = s.TricksWon[p]
		for _, suit := range s.Marriages[p] {
			t.marriages[i] += s.marriageValue(suit)
		}
	}
	return t
}

func (s *GameState) closing() closing {
	if s.ClosedBy == "" {
		return notClosed
	}
	return closing{by: s.seat(s.ClosedBy), oppTricks: s.ClosedOppTricks, oppPoints: s.ClosedOppPoints}
}

// endOut ends the deal with p going out.
func (s *GameState) endOut(p string) []module.Event {
	return s.endDeal(p, s.rules().outPoints(s.tally(), s.closing(), s.seat(p)), reasonOut)
}

// endDeal records the deal and moves the match on: the next deal, a pause
// before it, or the end.
func (s *GameState) endDeal(winner string, points int, reason string) []module.Event {
	rec := DealRecord{
		Number: s.Deal + 1, Winner: winner, GamePoints: points, Reason: reason, ClosedBy: s.ClosedBy,
		Points: map[string]int{}, Totals: map[string]int{},
	}
	if winner != "" {
		s.Scores[winner] += points
	}
	for _, p := range s.Players {
		rec.Points[p] = s.total(p)
		rec.Totals[p] = s.Scores[p]
	}
	s.Rounds = append(s.Rounds, rec)
	events := []module.Event{{Type: "deal_settled", Data: map[string]any{
		"deal": rec.Number, "winner": winner, "gamePoints": points, "reason": reason,
	}}}

	s.Current = ""
	if winner != "" && s.Scores[winner] >= s.Target {
		s.Status = "completed"
		return append(events, module.Event{Type: "match_completed", Data: map[string]any{"winner": winner}})
	}
	s.Deal++
	if s.Pause {
		s.Intermission.Begin(s.Deal)
		return events
	}
	startDeal(s)
	return append(events, module.Event{Type: "deal_started", Data: map[string]any{"deal": s.Deal + 1}})
}

// Finished reports whether the match is over, and who won it.
func (m *Module) Finished(raw module.State) (bool, []string, error) {
	s, err := decode(raw)
	if err != nil {
		return false, nil, err
	}
	if s.Status != "completed" {
		return false, nil, nil
	}
	best := -1
	var winners []string
	for _, p := range s.Players {
		switch {
		case s.Scores[p] > best:
			best, winners = s.Scores[p], []string{p}
		case s.Scores[p] == best:
			winners = append(winners, p)
		}
	}
	return true, winners, nil
}
