package lora

import (
	"math/rand"

	"zolik/server/internal/module"
	"zolik/server/internal/tricks"
)

var _ module.GameModule = (*Module)(nil)

// NewMatch seats four players and deals the first game.
func (m *Module) NewMatch(cfg module.MatchConfig, players []module.PlayerRef, seed int64) (module.State, error) {
	if len(players) != seats {
		return nil, module.Error{Code: "TOO_FEW_PLAYERS", Message: "lóra is played by four"}
	}
	c := resolve(cfg)
	s := &GameState{
		Status:      "active",
		Seed:        seed,
		Talies:      c.talies,
		Maturita:    c.maturita,
		Pause:       cfg.PauseBetweenRounds(true),
		FirstLeader: module.StartingSeat(seed, len(players)),
		Scores:      map[string]int{},
	}
	for _, p := range players {
		s.Players = append(s.Players, p.ID)
		s.Scores[p.ID] = 0
	}
	s.startDeal()
	return encode(s)
}

// startDeal shuffles and deals eight cards each, from the leader round, and
// opens the game: the next in the tálie, or the maturant's choice.
func (s *GameState) startDeal() {
	deck := buildDeck()
	r := rand.New(rand.NewSource(s.Seed + int64(s.Deal)*7919))
	r.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })

	s.Hands = map[string][]string{}
	p := s.talieLeader()
	for i := 0; i < len(s.Players); i++ {
		s.Hands[p] = append([]string(nil), deck[i*handSize:(i+1)*handSize]...)
		sortHand(s.Hands[p])
		p = s.next(p)
	}
	s.Trick, s.LastTrick, s.Tricks = nil, nil, 0
	s.Laid, s.Quart, s.Turned = nil, nil, false
	s.Won, s.Tuks, s.Points = map[string]int{}, map[string]int{}, map[string]int{}
	s.Voids = map[string]string{}
	s.Leader = s.talieLeader()
	if s.maturita() {
		s.Contract, s.Phase, s.Current = "", phaseChoose, s.Leader
		return
	}
	s.begin(sequence[s.Game])
}

// begin starts game g, the tálie's leader to lead.
func (s *GameState) begin(g string) {
	s.Contract = g
	s.Leader, s.Current = s.talieLeader(), s.talieLeader()
	switch g {
	case gameKvarty:
		s.Phase = phaseQuart
	case gameDesitky:
		s.Phase = phaseTens
	default:
		s.Phase = phasePlay
	}
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
			s.startDeal()
			events = []module.Event{{Type: "deal_started", Data: map[string]any{"deal": s.Deal + 1}}}
		}
		out, err := encode(s)
		return out, events, err
	}
	if s.seat(playerID) < 0 || s.Current != playerID {
		return raw, nil, errCode(ErrNotYourTurn)
	}
	events, err := s.act(playerID, a)
	if err != nil {
		return raw, nil, err
	}
	out, err := encode(s)
	return out, events, err
}

func (s *GameState) act(p string, a module.Action) ([]module.Event, error) {
	switch a.Verb {
	case VerbChoose:
		if s.Phase != phaseChoose {
			return nil, errCode(ErrNotChoosing)
		}
		g := a.Params[ParamGame]
		if g == gamePrPo && !s.choosable(g) {
			return nil, errCode(ErrPrPoThirdOnly)
		}
		if !s.choosable(g) {
			return nil, errCode(ErrNoSuchGame)
		}
		s.begin(g)
		return []module.Event{{Type: "game_chosen", Data: map[string]any{"playerId": p, "game": g}}}, nil
	case VerbPlay:
		if len(a.Cards) != 1 {
			return nil, errCode(ErrPickOneCard)
		}
		switch s.Phase {
		case phasePlay:
			return s.playTrick(p, a.Cards[0])
		case phaseQuart:
			return s.layQuart(p, a.Cards[0])
		case phaseTens:
			return s.layTen(p, a.Cards[0])
		}
		return nil, errCode(ErrNotYourTurn)
	case VerbDone:
		if s.Phase != phaseTens || !s.Turned {
			return nil, errCode(ErrNothingLaid)
		}
		s.passTurn()
		return []module.Event{{Type: "turn_ended", Data: map[string]any{"playerId": p}}}, nil
	case VerbTuk:
		if s.Phase != phaseTens {
			return nil, errCode(ErrNotYourTurn)
		}
		if s.Turned || len(s.legal(p)) > 0 {
			return nil, errCode(ErrMustLay)
		}
		return s.tuk(p), nil
	}
	return nil, module.Error{Code: ErrUnknownAction, Message: a.Verb}
}

// legal is the cards p may play now.
func (s *GameState) legal(p string) []string {
	hand := s.Hands[p]
	switch s.Phase {
	case phasePlay:
		if len(s.Trick) == 0 && redLeadRule(s.Contract) {
			var other []string
			for _, c := range hand {
				if tricks.Suit(c) != red {
					other = append(other, c)
				}
			}
			if len(other) > 0 {
				return other
			}
		}
		return tricks.Legal(hand, tricks.Trick{Plays: s.Trick}, tricks.NoTrump, order, tricks.FollowOnly)
	case phaseQuart:
		return append([]string(nil), hand...)
	case phaseTens:
		var out []string
		for _, c := range hand {
			if s.fitsRow(c) {
				out = append(out, c)
			}
		}
		return out
	}
	return nil
}

// --- the trick games --------------------------------------------------------

func (s *GameState) playTrick(p, card string) ([]module.Event, error) {
	if !hasCard(s.Hands[p], card) {
		return nil, errCode(ErrCardNotInHand)
	}
	if !hasCard(s.legal(p), card) {
		if len(s.Trick) == 0 {
			return nil, errCode(ErrNoRedLead)
		}
		return nil, errCode(ErrMustFollow)
	}
	s.noteVoids(p, card)
	s.Hands[p] = removeCard(s.Hands[p], card)
	s.Trick = append(s.Trick, tricks.Play{Seat: s.seat(p), Card: card})
	events := []module.Event{{Type: "card_played", Data: map[string]any{"playerId": p, "card": card}}}
	if len(s.Trick) < len(s.Players) {
		s.Current = s.next(p)
		return events, nil
	}

	best, _ := tricks.Trick{Plays: s.Trick}.Winning(tricks.NoTrump, order)
	w := s.Players[best.Seat]
	pen := trickPenalty(s.Contract, s.Trick, s.Tricks)
	s.Points[w] += pen
	s.Won[w]++
	s.Tricks++
	s.LastTrick, s.Trick = s.Trick, nil
	s.Leader, s.Current = w, w
	events = append(events, module.Event{Type: "trick_won", Data: map[string]any{"playerId": w, "penalty": pen}})

	if s.Tricks == numTrick || !s.penaltyLeft() || s.maturantHit() {
		return append(events, s.endDeal()...), nil
	}
	return events, nil
}

// noteVoids records what playing card shows about p's hand.
func (s *GameState) noteVoids(p, card string) {
	void := func(suit byte) {
		if indexOf(s.Voids[p], suit) < 0 {
			s.Voids[p] += string(suit)
		}
	}
	switch {
	case len(s.Trick) > 0 && tricks.Suit(card) != tricks.Suit(s.Trick[0].Card):
		void(tricks.Suit(s.Trick[0].Card))
	case len(s.Trick) == 0 && redLeadRule(s.Contract) && tricks.Suit(card) == red:
		for i := 0; i < len(suits); i++ {
			if suits[i] != red {
				void(suits[i])
			}
		}
	}
}

// penaltyLeft is whether anything can still cost a point in this trick
// game. Červené, filky and červený král stop once their cards are taken.
func (s *GameState) penaltyLeft() bool {
	held := func(f func(string) bool) bool {
		for _, p := range s.Players {
			for _, c := range s.Hands[p] {
				if f(c) {
					return true
				}
			}
		}
		return false
	}
	switch s.Contract {
	case gameCervene:
		return held(func(c string) bool { return tricks.Suit(c) == red })
	case gameFilky:
		return held(func(c string) bool { return tricks.Rank(c) == 'Q' })
	case gameKral:
		return held(func(c string) bool { return c == redKing })
	}
	return s.Tricks < numTrick
}

// maturantHit is whether the maturant has taken a penalty: the maturita has
// failed and the deal stops there.
func (s *GameState) maturantHit() bool {
	m := s.maturant()
	return m != "" && s.Points[m] > 0
}

// --- kvarty -----------------------------------------------------------------

// layQuart is the leader starting a quart with card. The quart is the four
// ranks from it upwards (fewer at the top of the suit), and every card of it
// is laid at once by whoever holds it, as the rules oblige; it stops short at
// a card already laid. Whoever laid its highest card leads the next.
func (s *GameState) layQuart(p, card string) ([]module.Event, error) {
	if !hasCard(s.Hands[p], card) {
		return nil, errCode(ErrCardNotInHand)
	}
	suit, from := tricks.Suit(card), rankIndex(card)
	top := min(from+3, len(ascending)-1)
	s.Quart = nil
	for k := from; k <= top; k++ {
		c := cardAt(k, suit)
		h := s.holder(c)
		if h == "" {
			break
		}
		s.Hands[h] = removeCard(s.Hands[h], c)
		s.Laid = append(s.Laid, c)
		s.Quart = append(s.Quart, tricks.Play{Seat: s.seat(h), Card: c})
	}
	last := s.Players[s.Quart[len(s.Quart)-1].Seat]
	s.Leader, s.Current = last, last
	events := []module.Event{{Type: "quart_laid", Data: map[string]any{"playerId": p, "card": card, "leader": last}}}
	for _, q := range s.Players {
		if len(s.Hands[q]) == 0 {
			return append(events, s.endDeal()...), nil
		}
	}
	return events, nil
}

// --- desítky ----------------------------------------------------------------

// row is the span of a suit's row as rank indexes, ok false while no ten of
// it has been laid.
func (s *GameState) row(suit byte) (lo, hi int, ok bool) {
	lo, hi = len(ascending), -1
	for _, c := range s.Laid {
		if tricks.Suit(c) == suit {
			lo, hi = min(lo, rankIndex(c)), max(hi, rankIndex(c))
		}
	}
	return lo, hi, hi >= 0
}

// fitsRow is whether card may be laid: a ten opening its suit's row, or the
// next card above or below a row.
func (s *GameState) fitsRow(card string) bool {
	lo, hi, ok := s.row(tricks.Suit(card))
	i := rankIndex(card)
	if !ok {
		return tricks.Rank(card) == 'T'
	}
	return i == lo-1 || i == hi+1
}

func (s *GameState) layTen(p, card string) ([]module.Event, error) {
	if !hasCard(s.Hands[p], card) {
		return nil, errCode(ErrCardNotInHand)
	}
	if !s.fitsRow(card) {
		return nil, errCode(ErrCannotLay)
	}
	s.Hands[p] = removeCard(s.Hands[p], card)
	s.Laid = append(s.Laid, card)
	s.Turned = true
	events := []module.Event{{Type: "card_laid", Data: map[string]any{"playerId": p, "card": card}}}
	if len(s.Hands[p]) == 0 {
		return append(events, s.endDeal()...), nil
	}
	// Nothing more to lay: the turn passes without asking.
	if len(s.legal(p)) == 0 {
		s.passTurn()
	}
	return events, nil
}

func (s *GameState) passTurn() {
	s.Turned = false
	s.Current = s.next(s.Current)
}

// tuk is a player who can lay nothing knocking: a point, and the turn
// passes. A maturant who knocks has failed.
func (s *GameState) tuk(p string) []module.Event {
	s.Tuks[p]++
	s.Points[p]++
	events := []module.Event{{Type: "tuk", Data: map[string]any{"playerId": p}}}
	if s.maturantHit() {
		return append(events, s.endDeal()...)
	}
	s.passTurn()
	return events
}

// --- settling ---------------------------------------------------------------

// endDeal scores the deal and moves the match on.
func (s *GameState) endDeal() []module.Event {
	if !trickGame(s.Contract) {
		for _, p := range s.Players {
			s.Points[p] += len(s.Hands[p])
		}
	}
	rec := DealRecord{
		Number: s.Deal + 1, Talie: s.Talie + 1, Contract: s.Contract,
		Deltas: map[string]int{}, Totals: map[string]int{},
	}
	if m := s.maturant(); m != "" {
		rec.Maturant = m
		rec.Passed = s.Points[m] == 0
		if !rec.Passed {
			rec.Deltas[m] = maturitaPenalty
		}
	} else {
		for _, p := range s.Players {
			rec.Deltas[p] = s.Points[p]
		}
	}
	for _, p := range s.Players {
		s.Scores[p] += rec.Deltas[p]
		rec.Totals[p] = s.Scores[p]
	}
	s.Rounds = append(s.Rounds, rec)
	s.Current = ""
	events := []module.Event{{Type: "deal_settled", Data: map[string]any{
		"deal": rec.Number, "game": rec.Contract, "deltas": rec.Deltas,
	}}}

	switch {
	case rec.Maturant != "" && !rec.Passed:
		s.Attempt++
		if s.Attempt == maxAttempts {
			s.Failed = rec.Maturant
			s.Status = "completed"
		}
	case rec.Maturant != "":
		s.nextTalie()
	default:
		s.Game++
		if s.Game == len(sequence) && !s.Maturita {
			s.nextTalie()
		}
	}
	if s.Status == "completed" {
		return append(events, module.Event{Type: "match_completed", Data: map[string]any{"winners": s.winners()}})
	}
	s.Deal++
	if s.Pause {
		s.Intermission.Begin(s.Deal)
		return events
	}
	s.startDeal()
	return append(events, module.Event{Type: "deal_started", Data: map[string]any{"deal": s.Deal + 1}})
}

func (s *GameState) nextTalie() {
	s.Talie++
	s.Game, s.Attempt = 0, 0
	if s.Talie >= s.Talies {
		s.Status = "completed"
	}
}

// winners is everyone level on the fewest points, the failed maturant
// excepted.
func (s *GameState) winners() []string {
	best := -1
	var out []string
	for _, p := range s.Players {
		if p == s.Failed {
			continue
		}
		switch {
		case best < 0 || s.Scores[p] < best:
			best, out = s.Scores[p], []string{p}
		case s.Scores[p] == best:
			out = append(out, p)
		}
	}
	return out
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
	return true, s.winners(), nil
}
