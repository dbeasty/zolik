package ferbl

import (
	"math/rand"
	"sort"

	"zolik/server/internal/module"
)

var _ module.GameModule = (*Module)(nil)

// NewMatch seats two to six players.
func (m *Module) NewMatch(cfg module.MatchConfig, players []module.PlayerRef, seed int64) (module.State, error) {
	if len(players) < 2 || len(players) > 6 {
		return nil, module.Error{Code: "TOO_FEW_PLAYERS", Message: "ferbl is played by two to six"}
	}
	c := resolve(cfg)
	s := &GameState{
		Status:        "active",
		Seed:          seed,
		StartingStack: c.startingStack,
		Vizi:          c.vizi,
		Hands:         c.hands,
		Pause:         cfg.PauseBetweenRounds(true),
		FirstDealer:   module.StartingSeat(seed, len(players)),
	}
	for _, p := range players {
		s.Seats = append(s.Seats, Seat{PlayerID: p.ID, Stack: c.startingStack})
	}
	s.startHand()
	return encode(s)
}

// startHand takes the antes and deals two cards to everyone who can pay
// one. A table where fewer than two can is over.
func (s *GameState) startHand() {
	can := 0
	for _, st := range s.Seats {
		if st.Stack >= s.Vizi {
			can++
		}
	}
	if can < 2 || s.Hand >= s.Hands {
		s.Status, s.Current = "completed", ""
		return
	}
	s.Deck = buildDeck()
	r := rand.New(rand.NewSource(s.Seed + int64(s.Hand)*7919))
	r.Shuffle(len(s.Deck), func(i, j int) { s.Deck[i], s.Deck[j] = s.Deck[j], s.Deck[i] })
	for i := range s.Seats {
		st := &s.Seats[i]
		*st = Seat{PlayerID: st.PlayerID, Stack: st.Stack}
	}
	for _, i := range s.order() {
		st := &s.Seats[i]
		if st.Stack < s.Vizi {
			continue
		}
		st.In = true
		st.Stack -= s.Vizi
		st.Total = s.Vizi
		st.Hand = s.draw(2)
	}
	s.openRound(phaseFirst)
}

func (s *GameState) draw(n int) []string {
	out := append([]string(nil), s.Deck[:n]...)
	s.Deck = s.Deck[n:]
	sortHand(out)
	return out
}

// openRound starts a betting round with the first live seat from the
// dealer's left — or, when nobody could bet anyway, goes straight on.
func (s *GameState) openRound(phase string) []module.Event {
	s.Phase, s.CurrentBet, s.Raises, s.Current = phase, 0, 0, ""
	for i := range s.Seats {
		s.Seats[i].Bet, s.Seats[i].Acted = 0, false
	}
	return s.advance()
}

// advance hands the turn to the next seat that owes the round an action,
// or closes the round.
func (s *GameState) advance() []module.Event {
	live, acting := 0, 0
	for _, st := range s.Seats {
		if st.live() {
			live++
			if st.canAct() {
				acting++
			}
		}
	}
	if live == 1 {
		return s.showdown()
	}
	start := 0
	if cur := s.index(s.Current); cur >= 0 {
		for k, i := range s.order() {
			if i == cur {
				start = k + 1
			}
		}
	}
	ord := s.order()
	for k := 0; k < len(ord); k++ {
		st := &s.Seats[ord[(start+k)%len(ord)]]
		if !st.canAct() {
			continue
		}
		if !st.Acted || st.Bet < s.CurrentBet {
			// Nobody left to bet against: the last seat with chips need
			// not act alone.
			if acting == 1 && st.Bet >= s.CurrentBet {
				break
			}
			s.Current = st.PlayerID
			return nil
		}
	}
	return s.closeRound()
}

// closeRound deals the second two cards, or goes to the showdown.
func (s *GameState) closeRound() []module.Event {
	s.Current = ""
	if s.Phase == phaseFirst {
		for _, i := range s.order() {
			if st := &s.Seats[i]; st.live() {
				st.Hand = append(st.Hand, s.draw(2)...)
				sortHand(st.Hand)
			}
		}
		return append([]module.Event{{Type: "cards_dealt"}}, s.openRound(phaseSecond)...)
	}
	return s.showdown()
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
			s.startHand()
			events = []module.Event{{Type: "hand_started", Data: map[string]any{"hand": s.Hand + 1}}}
		}
		out, err := encode(s)
		return out, events, err
	}
	st := s.seat(playerID)
	if st == nil || s.Current != playerID {
		return raw, nil, errCode(ErrNotYourTurn)
	}
	if err := s.refusal(st, a.Verb); err != nil {
		return raw, nil, err
	}
	events := s.act(st, a.Verb)
	out, err := encode(s)
	return out, events, err
}

// refusal is why this seat may not do verb now, or nil.
func (s *GameState) refusal(st *Seat, verb string) error {
	owe := s.toCall(st)
	switch verb {
	case VerbCheck:
		if owe > 0 {
			return errCode(ErrCannotCheck)
		}
	case VerbBet:
		if s.CurrentBet > 0 {
			return errCode(ErrCannotBet)
		}
	case VerbCall:
		if owe == 0 {
			return errCode(ErrNothingToCall)
		}
	case VerbRaise:
		switch {
		case s.CurrentBet == 0:
			return errCode(ErrCannotRaise)
		case s.Raises >= maxRaises:
			return errCode(ErrRaiseCap)
		case st.Stack <= owe:
			return errCode(ErrCannotRaise)
		}
	case VerbFold:
		if owe == 0 {
			return errCode(ErrNoNeedToFold)
		}
	default:
		return module.Error{Code: ErrUnknownAction, Message: verb}
	}
	return nil
}

// put moves chips from the seat's stack to the table, no more than it has.
func (st *Seat) put(n int) {
	n = min(n, st.Stack)
	st.Stack -= n
	st.Bet += n
	st.Total += n
}

func (s *GameState) act(st *Seat, verb string) []module.Event {
	st.Acted = true
	switch verb {
	case VerbBet:
		st.put(s.unit())
		s.CurrentBet = st.Bet
		st.Raised++
	case VerbCall:
		st.put(s.toCall(st))
	case VerbRaise:
		st.put(s.CurrentBet + s.unit() - st.Bet)
		s.CurrentBet = max(s.CurrentBet, st.Bet)
		s.Raises++
		st.Raised++
	case VerbFold:
		st.Folded = true
	}
	events := []module.Event{{Type: verb, Data: map[string]any{"playerId": st.PlayerID}}}
	return append(events, s.advance()...)
}

// showdown pays the pot out: side pot by side pot, each to the best hand
// among those who put in that much, ties to the first in turn order. A
// lone survivor takes it all unseen.
func (s *GameState) showdown() []module.Event {
	s.Current = ""
	rank := map[int]int{}
	for k, i := range s.order() {
		rank[i] = k
	}
	var live []int
	for i, st := range s.Seats {
		if st.live() {
			live = append(live, i)
		}
	}
	if len(live) > 1 {
		for _, i := range live {
			s.Seats[i].Shown = true
		}
	}
	// The levels at which the pot splits: each live seat's total.
	levels := map[int]bool{}
	for _, i := range live {
		levels[s.Seats[i].Total] = true
	}
	var ls []int
	for l := range levels {
		ls = append(ls, l)
	}
	sort.Ints(ls)
	won := map[int]int{}
	prev := 0
	for k, level := range ls {
		portion := 0
		for _, st := range s.Seats {
			top := level
			if k == len(ls)-1 {
				top = st.Total // what a folder put in above every level goes here
			}
			portion += max(0, min(st.Total, top)-prev)
		}
		best := -1
		for _, i := range live {
			if s.Seats[i].Total < level {
				continue
			}
			if best < 0 || better(s.Seats[i].Hand, s.Seats[best].Hand, rank[i], rank[best]) {
				best = i
			}
		}
		won[best] += portion
		prev = level
	}

	rec := HandRecord{
		Number: s.Hand + 1, Pot: s.pot(), Shown: map[string][]string{}, Scores: map[string]int{},
		Deltas: map[string]int{}, Stacks: map[string]int{},
	}
	for i := range s.Seats {
		st := &s.Seats[i]
		st.Stack += won[i]
		st.Delta = won[i] - st.Total
		if won[i] > 0 {
			rec.Winners = append(rec.Winners, st.PlayerID)
		}
		if st.Shown {
			rec.Shown[st.PlayerID] = append([]string(nil), st.Hand...)
			rec.Scores[st.PlayerID] = score(st.Hand)
		}
		rec.Deltas[st.PlayerID] = st.Delta
		rec.Stacks[st.PlayerID] = st.Stack
	}
	s.Rounds = append(s.Rounds, rec)
	s.Hand++
	events := []module.Event{{Type: "hand_settled", Data: map[string]any{"hand": rec.Number, "winners": rec.Winners}}}
	if s.Pause {
		s.Intermission.Begin(s.Hand)
		return events
	}
	s.startHand()
	if s.Status != "active" {
		return append(events, module.Event{Type: "match_completed"})
	}
	return events
}

// better is whether hand a beats hand b: a higher score, or the same score
// and a seat earlier in turn order.
func better(a, b []string, rankA, rankB int) bool {
	sa, sb := score(a), score(b)
	if sa != sb {
		return sa > sb
	}
	return rankA < rankB
}

// leaders is everyone level on the most chips.
func (s *GameState) leaders() []string {
	best := -1
	var out []string
	for _, st := range s.Seats {
		switch {
		case st.Stack > best:
			best, out = st.Stack, []string{st.PlayerID}
		case st.Stack == best:
			out = append(out, st.PlayerID)
		}
	}
	return out
}

// Finished reports whether the match is over, and who has the most chips.
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
