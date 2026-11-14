package blackpeter

import (
	"math/rand"
	"strconv"

	"zolik/server/internal/module"
)

var _ module.GameModule = (*Module)(nil)

// NewMatch seats two to six players and deals the first round.
func (m *Module) NewMatch(cfg module.MatchConfig, players []module.PlayerRef, seed int64) (module.State, error) {
	if len(players) < minPlayers || len(players) > maxPlayers {
		return nil, module.Error{Code: ErrPlayerCount, Message: "black peter is played by two to six"}
	}
	s := tableOf(cfg)
	s.Status = "active"
	s.Seed = seed
	s.Dealer = module.StartingSeat(seed, len(players))
	for _, p := range players {
		s.Seats = append(s.Seats, Seat{PlayerID: p.ID})
	}
	s.startRound()
	return encode(s)
}

// rng is this round's randomness at this point in it, so a replay of the same
// moves takes the same cards.
func (s *GameState) rng(salt int64) *rand.Rand {
	return rand.New(rand.NewSource(s.Seed + int64(s.Round)*7919 + int64(s.Draws)*104729 + salt))
}

// startRound shuffles, deals every card from the dealer's left, and puts down
// every pair that came in the deal. The first to draw is the dealer's left.
func (s *GameState) startRound() []module.Event {
	deck := buildDeck()
	r := rand.New(rand.NewSource(s.Seed + int64(s.Round)*7919))
	r.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
	n := len(s.Seats)
	for i := range s.Seats {
		s.Seats[i].Hand, s.Seats[i].Out = nil, false
	}
	for k, c := range deck {
		st := &s.Seats[(s.Dealer+1+k)%n]
		st.Hand = append(st.Hand, c)
	}
	s.Pairs, s.OutOrder, s.Draws = nil, nil, 0

	var events []module.Event
	for i := range s.Seats {
		st := &s.Seats[i]
		rest, found := takePairs(st.Hand, s.ByColour)
		st.Hand = rest
		s.Pairs = append(s.Pairs, found...)
		if len(found) > 0 {
			events = append(events, module.Event{Type: "pairs_laid", Data: map[string]any{"playerId": st.PlayerID, "count": len(found)}})
		}
	}
	for i := range s.Seats {
		events = append(events, s.goOutIfEmpty(i)...)
	}
	if s.holders() <= 1 {
		return append(events, s.settle()...)
	}
	s.Current = s.Seats[s.leftOf(s.Dealer)].PlayerID
	return events
}

// goOutIfEmpty marks seat i out of the round if it has just run out of cards.
func (s *GameState) goOutIfEmpty(i int) []module.Event {
	st := &s.Seats[i]
	if len(st.Hand) > 0 || st.Out {
		return nil
	}
	st.Out = true
	s.OutOrder = append(s.OutOrder, st.PlayerID)
	return []module.Event{{Type: "went_out", Data: map[string]any{"playerId": st.PlayerID}}}
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
		if err := s.Intermission.Mark(s.players(), playerID); err != nil {
			return raw, nil, err
		}
		var events []module.Event
		if s.Intermission.Settled(s.players()) {
			s.Intermission.Close()
			events = append([]module.Event{{Type: "round_started", Data: map[string]any{"round": s.Round + 1}}}, s.startRound()...)
		}
		out, err := encode(s)
		return out, events, err
	}
	if a.Verb != VerbDraw {
		return raw, nil, module.Error{Code: ErrUnknownAction, Message: a.Verb}
	}
	events, err := s.draw(playerID, a.Params[ParamPick])
	if err != nil {
		return raw, nil, err
	}
	out, err := encode(s)
	return out, events, err
}

// drawRefusal is why this player may not take a card now.
func (s *GameState) drawRefusal(playerID string) error {
	st := s.seat(playerID)
	switch {
	case st == nil:
		return errCode(ErrNotYourTurn)
	case len(st.Hand) == 0:
		return errCode(ErrOut)
	case s.Current != playerID:
		return errCode(ErrNotYourTurn)
	}
	return nil
}

// draw takes one card from the nearest seat on the player's left still
// holding cards: the one at pick, or one at random where none was chosen.
// The card lands somewhere in the taker's hand, and goes down with its
// partner if it has one there.
func (s *GameState) draw(playerID, pick string) ([]module.Event, error) {
	if err := s.drawRefusal(playerID); err != nil {
		return nil, err
	}
	me := s.index(playerID)
	from := s.leftOf(me)
	src := &s.Seats[from]
	at := 0
	if pick == "" {
		at = s.rng(1).Intn(len(src.Hand))
	} else {
		n, err := strconv.Atoi(pick)
		if err != nil || n < 0 || n >= len(src.Hand) {
			return nil, errCode(ErrNoCard)
		}
		at = n
	}
	card := src.Hand[at]
	src.Hand = append(append([]string(nil), src.Hand[:at]...), src.Hand[at+1:]...)

	st := &s.Seats[me]
	events := []module.Event{{Type: "card_taken", Data: map[string]any{"playerId": playerID, "from": src.PlayerID}}}
	partner := -1
	for i, c := range st.Hand {
		if pairs(c, card, s.ByColour) {
			partner = i
			break
		}
	}
	if partner >= 0 {
		pair := []string{st.Hand[partner], card}
		st.Hand = append(append([]string(nil), st.Hand[:partner]...), st.Hand[partner+1:]...)
		s.Pairs = append(s.Pairs, pair)
		events = append(events, module.Event{Type: "pair_laid", Data: map[string]any{"playerId": playerID, "cards": pair}})
	} else {
		// Where it lands is nobody's business but the holder's: a random
		// place, so the seat that gave it cannot point at it later.
		pos := s.rng(2).Intn(len(st.Hand) + 1)
		st.Hand = append(st.Hand[:pos], append([]string{card}, st.Hand[pos:]...)...)
	}
	s.Draws++
	events = append(events, s.goOutIfEmpty(from)...)
	events = append(events, s.goOutIfEmpty(me)...)

	if s.holders() <= 1 {
		return append(events, s.settle()...), nil
	}
	// The turn passes left: to the seat just drawn from, or past it if that
	// seat has just gone out.
	s.Current = s.Seats[s.leftOf(me)].PlayerID
	return events, nil
}

// loser is the one seat still holding a card — Black Peter.
func (s *GameState) loser() int {
	for i, st := range s.Seats {
		if len(st.Hand) > 0 {
			return i
		}
	}
	return -1
}

// settle marks the round's loser and moves on: to the next round, a pause
// before it, or the end of the match.
func (s *GameState) settle() []module.Event {
	l := s.loser()
	rec := RoundRecord{Number: s.Round + 1, OutOrder: append([]string(nil), s.OutOrder...), Marks: map[string]int{}}
	if l >= 0 {
		s.Seats[l].Marks++
		rec.Loser = s.Seats[l].PlayerID
		// The loser deals the next round.
		s.Dealer = l
	} else {
		s.Dealer = (s.Dealer + 1) % len(s.Seats)
	}
	for _, st := range s.Seats {
		rec.Marks[st.PlayerID] = st.Marks
	}
	s.Rounds = append(s.Rounds, rec)
	s.Round++
	s.Current = ""
	events := []module.Event{{Type: "round_lost", Data: map[string]any{"playerId": rec.Loser, "round": rec.Number}}}
	if s.Round >= s.RoundLimit {
		s.Status = "completed"
		return append(events, module.Event{Type: "match_completed", Data: map[string]any{"winners": s.leaders()}})
	}
	if s.Pause {
		s.Intermission.Begin(s.Round)
		return events
	}
	events = append(events, module.Event{Type: "round_started", Data: map[string]any{"round": s.Round + 1}})
	return append(events, s.startRound()...)
}

// leaders is everyone level on the fewest marks.
func (s *GameState) leaders() []string {
	best := -1
	var out []string
	for _, st := range s.Seats {
		switch {
		case best < 0 || st.Marks < best:
			best, out = st.Marks, []string{st.PlayerID}
		case st.Marks == best:
			out = append(out, st.PlayerID)
		}
	}
	return out
}

// Finished reports whether the match is over, and who has the fewest marks.
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
