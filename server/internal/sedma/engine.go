package sedma

import (
	"math/rand"

	"zolik/server/internal/module"
	"zolik/server/internal/tricks"
)

var _ module.GameModule = (*Module)(nil)

// NewMatch seats two, three or four players and deals the first hand.
func (m *Module) NewMatch(cfg module.MatchConfig, players []module.PlayerRef, seed int64) (module.State, error) {
	if len(players) < 2 || len(players) > 4 {
		return nil, module.Error{Code: "TOO_FEW_PLAYERS", Message: "sedma is played by two, three or four"}
	}
	c := resolve(cfg)
	s := &GameState{
		Status:         "active",
		Seed:           seed,
		Target:         c.target,
		ShowCardPoints: c.showCardPoints,
		Pause:          cfg.PauseBetweenRounds(true),
		FirstDealer:    module.StartingSeat(seed, len(players)),
		Scores:         map[string]int{},
	}
	for _, p := range players {
		s.Players = append(s.Players, p.ID)
		s.Scores[p.ID] = 0
	}
	startDeal(s)
	return encode(s)
}

// startDeal shuffles and deals the hand numbered s.Deal: four cards each and
// the rest face down as the stock. The player to the dealer's left leads.
func startDeal(s *GameState) {
	deck := buildDeck(len(s.Players))
	r := rand.New(rand.NewSource(s.Seed + int64(s.Deal)*7919))
	r.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })

	s.Hands = map[string][]string{}
	p := s.next(s.dealer())
	for i := 0; i < len(s.Players); i++ {
		s.Hands[p] = append([]string(nil), deck[i*handSize:(i+1)*handSize]...)
		sortHand(s.Hands[p])
		p = s.next(p)
	}
	s.Stock = append([]string(nil), deck[len(s.Players)*handSize:]...)
	s.Leader = s.next(s.dealer())
	s.Phase, s.Current = phasePlay, s.Leader
	s.Trick, s.LastTrick, s.History = nil, nil, nil
	s.Points, s.Cards = map[string]int{}, map[string]int{}
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
		if len(a.Cards) != 1 {
			return raw, nil, errCode(ErrPickOneCard)
		}
		events, err = s.play(playerID, a.Cards[0])
	case VerbStop:
		events, err = s.stop()
	default:
		return raw, nil, module.Error{Code: ErrUnknownAction, Message: a.Verb}
	}
	if err != nil {
		return raw, nil, err
	}
	out, err := encode(s)
	return out, events, err
}

// legal is the cards p may play now: any card, except that the leader
// continuing a trick must play the led rank or a seven.
func (s *GameState) legal(p string) []string {
	if s.Phase == phaseDecide {
		if p != s.Leader {
			return nil
		}
		return s.continuers()
	}
	return append([]string(nil), s.Hands[p]...)
}

// play puts one card on the trick.
func (s *GameState) play(p, card string) ([]module.Event, error) {
	if !hasCard(s.Hands[p], card) {
		return nil, errCode(ErrCardNotInHand)
	}
	if s.Phase == phaseDecide && !captures(card, s.led()) {
		return nil, errCode(ErrMustCapture)
	}
	s.Phase = phasePlay
	s.Hands[p] = removeCard(s.Hands[p], card)
	s.Trick = append(s.Trick, tricks.Play{Seat: s.seat(p), Card: card})
	events := []module.Event{{Type: "card_played", Data: map[string]any{"playerId": p, "card": card}}}

	if len(s.Trick)%len(s.Players) != 0 {
		s.Current = s.next(p)
		return events, nil
	}
	// A round is complete. The leader may go on with the led rank or a
	// seven while they hold one; otherwise the trick is over.
	if len(s.continuers()) > 0 {
		s.Phase, s.Current = phaseDecide, s.Leader
		return events, nil
	}
	return append(events, s.endTrick()...), nil
}

// stop is the leader letting the trick go.
func (s *GameState) stop() ([]module.Event, error) {
	if s.Phase != phaseDecide {
		return nil, errCode(ErrNotDeciding)
	}
	return s.endTrick(), nil
}

// endTrick gives the trick to its winner, draws everyone back to four, and
// ends the hand when the cards have run out.
func (s *GameState) endTrick() []module.Event {
	w := s.Players[winning(s.Trick)]
	side := s.side(w)
	for _, pl := range s.Trick {
		s.Points[side] += cardPoints(pl.Card)
	}
	s.Cards[side] += len(s.Trick)
	s.History = append(s.History, s.Trick)
	s.LastTrick, s.Trick = s.Trick, nil
	events := []module.Event{{Type: "trick_won", Data: map[string]any{"playerId": w}}}

	// Everyone draws one at a time, the winner first, until each holds four
	// or the stock is gone. Hands are always the same size, so a short stock
	// still comes out even.
	for p := w; len(s.Stock) > 0 && len(s.Hands[p]) < handSize; p = s.next(p) {
		s.Hands[p] = append(s.Hands[p], s.Stock[0])
		s.Stock = s.Stock[1:]
	}
	for _, p := range s.Players {
		sortHand(s.Hands[p])
	}

	s.Leader, s.Current, s.Phase = w, w, phasePlay
	if len(s.Hands[w]) == 0 {
		s.Points[side] += pointsLastTrick
		return append(events, s.endHand()...)
	}
	return events
}

// endHand scores the hand and moves the match on.
func (s *GameState) endHand() []module.Event {
	rec := HandRecord{Number: s.Deal + 1, Points: map[string]int{}, Totals: map[string]int{}}
	best := -1
	var top []string
	for _, side := range s.sides() {
		switch {
		case s.Points[side] > best:
			best, top = s.Points[side], []string{side}
		case s.Points[side] == best:
			top = append(top, side)
		}
	}
	total := len(buildDeck(len(s.Players)))
	switch {
	case len(top) == len(s.sides()):
		rec.Kind = kindNone // three players, thirty each
	case len(top) > 1:
		rec.Kind, rec.Stakes = kindTie, 1
	case s.Cards[top[0]] == total:
		rec.Kind, rec.Stakes = kindAllCards, 3
	case best == pointsTotal:
		rec.Kind, rec.Stakes = kindAllPoints, 2
	default:
		rec.Kind, rec.Stakes = kindWon, 1
	}
	won := map[string]bool{}
	if rec.Kind != kindNone {
		for _, side := range top {
			won[side] = true
		}
	}
	for _, p := range s.Players {
		if won[s.side(p)] {
			s.Scores[p] += rec.Stakes
			rec.Winners = append(rec.Winners, p)
		}
	}
	for _, p := range s.Players {
		rec.Points[p] = s.Points[s.side(p)]
		rec.Totals[p] = s.Scores[p]
	}
	s.Rounds = append(s.Rounds, rec)
	events := []module.Event{{Type: "deal_settled", Data: map[string]any{
		"deal": rec.Number, "winners": rec.Winners, "stakes": rec.Stakes, "kind": rec.Kind,
	}}}

	s.Current = ""
	for _, p := range s.Players {
		if s.Scores[p] >= s.Target {
			s.Status = "completed"
			return append(events, module.Event{Type: "match_completed", Data: map[string]any{"winners": s.leaders()}})
		}
	}
	s.Deal++
	if s.Pause {
		s.Intermission.Begin(s.Deal)
		return events
	}
	startDeal(s)
	return append(events, module.Event{Type: "deal_started", Data: map[string]any{"deal": s.Deal + 1}})
}

// leaders is everyone level on the most stakes.
func (s *GameState) leaders() []string {
	best := -1
	var out []string
	for _, p := range s.Players {
		switch {
		case s.Scores[p] > best:
			best, out = s.Scores[p], []string{p}
		case s.Scores[p] == best:
			out = append(out, p)
		}
	}
	return out
}

// Finished reports whether the match is over, and who won it: a whole
// partnership, at four.
func (m *Module) Finished(raw module.State) (bool, []string, error) {
	s, err := decode(raw)
	if err != nil {
		return false, nil, err
	}
	if s.Status != "completed" {
		return false, nil, nil
	}
	return true, s.leaders(), nil
}
