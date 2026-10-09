package okobere

import (
	"math/rand"
	"strconv"

	"zolik/server/internal/module"
)

var _ module.GameModule = (*Module)(nil)

// NewMatch seats two to six players: one banks, the rest punt.
func (m *Module) NewMatch(cfg module.MatchConfig, players []module.PlayerRef, seed int64) (module.State, error) {
	if len(players) < 2 || len(players) > 6 {
		return nil, module.Error{Code: "TOO_FEW_PLAYERS", Message: "oko bere is played by two to six"}
	}
	c := resolve(cfg)
	s := &GameState{
		Status:        "active",
		Seed:          seed,
		StartingStack: c.startingStack,
		MinBet:        c.minBet,
		BankSize:      c.bank,
		RoundsPerBank: c.roundsPerBank,
		BankTurns:     c.bankTurns,
		OkoPays:       c.okoPays,
		Pause:         cfg.PauseBetweenRounds(true),
		FirstBanker:   module.StartingSeat(seed, len(players)),
	}
	for _, p := range players {
		s.Seats = append(s.Seats, Seat{PlayerID: p.ID, Stack: c.startingStack})
	}
	if s.beginBankTurn() {
		s.startRound()
	}
	return encode(s)
}

// beginBankTurn hands the bank to the seat whose turn it is, skipping any
// who cannot stake it. false means the match is over.
func (s *GameState) beginBankTurn() bool {
	for ; s.Turn < len(s.Seats)*s.BankTurns; s.Turn++ {
		b := s.banker()
		if b.Stack < s.MinBet || s.punterCount() == 0 {
			continue
		}
		s.Bank = min(s.BankSize, b.Stack)
		b.Stack -= s.Bank
		s.RoundsInBank = 0
		return true
	}
	s.Status, s.Current = "completed", ""
	return false
}

// punterCount is how many seats besides the banker can afford to bet.
func (s *GameState) punterCount() int {
	n := 0
	for i, st := range s.Seats {
		if i != s.bankerIndex() && st.Stack >= s.MinBet {
			n++
		}
	}
	return n
}

// endBankTurn gives the banker back whatever is left in the bank and passes
// it on. false means the match is over.
func (s *GameState) endBankTurn() bool {
	s.banker().Stack += s.Bank
	s.Bank = 0
	s.Turn++
	return s.beginBankTurn()
}

// startRound shuffles and deals everyone who plays one card face down.
func (s *GameState) startRound() {
	s.Deck = buildDeck()
	r := rand.New(rand.NewSource(s.Seed + int64(s.Round)*7919))
	r.Shuffle(len(s.Deck), func(i, j int) { s.Deck[i], s.Deck[j] = s.Deck[j], s.Deck[i] })
	b := s.bankerIndex()
	for i := range s.Seats {
		st := &s.Seats[i]
		st.Hand, st.Bet, st.Done, st.Result, st.Delta = nil, 0, false, "", 0
		if i != b && st.Stack < s.MinBet {
			st.Done, st.Result = true, resultSatOut
			continue
		}
		st.Hand = []string{s.draw()}
	}
	s.Phase = phasePunters
	s.Current = s.Seats[b].PlayerID
	s.advance()
}

func (s *GameState) draw() string {
	c := s.Deck[0]
	s.Deck = s.Deck[1:]
	return c
}

// advance moves to the next punter still to play, clockwise from the
// banker, sitting out anyone the bank can no longer cover; then to the
// banker; then, if nobody has a stake left to play for, to the settlement.
func (s *GameState) advance() []module.Event {
	n := len(s.Seats)
	b := s.bankerIndex()
	for k := 1; k < n; k++ {
		st := &s.Seats[(b+k)%n]
		if st.Done {
			continue
		}
		if st.Bet == 0 && s.maxBet(st) < s.MinBet {
			st.Done, st.Result = true, resultSatOut
			continue
		}
		s.Phase, s.Current = phasePunters, st.PlayerID
		return nil
	}
	if s.committed() == 0 {
		return s.settle(false)
	}
	s.Phase, s.Current = phaseBank, s.Seats[b].PlayerID
	return nil
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
			s.startRound()
			events = []module.Event{{Type: "round_started", Data: map[string]any{"round": s.Round + 1}}}
		}
		out, err := encode(s)
		return out, events, err
	}
	st := s.seat(playerID)
	if st == nil || s.Current != playerID {
		return raw, nil, errCode(ErrNotYourTurn)
	}

	var events []module.Event
	switch a.Verb {
	case VerbBet:
		err = s.bet(st, a.Params[ParamAmount])
	case VerbCard:
		events, err = s.card(st)
	case VerbStand:
		events, err = s.stand(st)
	default:
		return raw, nil, module.Error{Code: ErrUnknownAction, Message: a.Verb}
	}
	if err != nil {
		return raw, nil, err
	}
	out, err := encode(s)
	return out, events, err
}

// betRefusal is why amount is not a stake this seat may put down now.
func (s *GameState) betRefusal(st *Seat, amount string) (int, error) {
	if s.Phase != phasePunters {
		return 0, errCode(ErrNotYourTurn)
	}
	if st.Bet > 0 {
		return 0, errCode(ErrAlreadyBet)
	}
	n, err := strconv.Atoi(amount)
	if err != nil {
		return 0, errCode(ErrBetAmount)
	}
	switch {
	case n < s.MinBet:
		return 0, errCode(ErrBetTooSmall)
	case n > st.Stack:
		return 0, errCode(ErrBetTooLarge)
	case n > s.bankFree():
		return 0, errCode(ErrBankTooSmall)
	}
	return n, nil
}

func (s *GameState) bet(st *Seat, amount string) error {
	n, err := s.betRefusal(st, amount)
	if err != nil {
		return err
	}
	st.Stack -= n
	st.Bet = n
	return nil
}

// card is "ještě": one more card, for a punter who has staked or for the
// banker.
func (s *GameState) card(st *Seat) ([]module.Event, error) {
	banker := st == s.banker()
	if banker && s.Phase != phaseBank {
		return nil, errCode(ErrNotYourTurn)
	}
	if !banker && st.Bet == 0 {
		return nil, errCode(ErrBetFirst)
	}
	if len(s.Deck) == 0 {
		return nil, errCode(ErrDeckEmpty)
	}
	st.Hand = append(st.Hand, s.draw())
	events := []module.Event{{Type: "card_taken", Data: map[string]any{"playerId": st.PlayerID}}}
	if banker {
		switch {
		case oko(st.Hand):
			return append(events, s.settle(true)...), nil
		case total(st.Hand) >= goal:
			return append(events, s.settle(false)...), nil
		}
		return events, nil
	}
	switch {
	case oko(st.Hand):
		// Two aces win at once, whatever the bank turns up.
		pay := min(s.OkoPays*st.Bet, s.Bank)
		s.Bank -= pay
		st.Stack += st.Bet + pay
		st.Delta = pay
		st.Done, st.Result = true, resultOko
		events = append(events, module.Event{Type: "oko", Data: map[string]any{"playerId": st.PlayerID}})
	case total(st.Hand) > goal:
		s.Bank += st.Bet
		st.Delta = -st.Bet
		st.Done, st.Result = true, resultBust
		events = append(events, module.Event{Type: "bust", Data: map[string]any{"playerId": st.PlayerID}})
	case total(st.Hand) == goal:
		st.Done = true
	default:
		return events, nil
	}
	return append(events, s.advance()...), nil
}

// stand is "stačí".
func (s *GameState) stand(st *Seat) ([]module.Event, error) {
	if st == s.banker() {
		if s.Phase != phaseBank {
			return nil, errCode(ErrNotYourTurn)
		}
		return s.settle(false), nil
	}
	if st.Bet == 0 {
		return nil, errCode(ErrBetFirst)
	}
	st.Done = true
	return s.advance(), nil
}

// settle pays the round out against the banker's hand and moves on.
func (s *GameState) settle(bankerOko bool) []module.Event {
	b := s.banker()
	bt := total(b.Hand)
	bust := bt > goal && !bankerOko
	for i := range s.Seats {
		st := &s.Seats[i]
		if st == b || st.Result != "" || st.Bet == 0 {
			continue
		}
		if !bankerOko && (bust || total(st.Hand) > bt) {
			s.Bank -= st.Bet
			st.Stack += 2 * st.Bet
			st.Delta, st.Result = st.Bet, resultWon
		} else {
			s.Bank += st.Bet
			st.Delta, st.Result = -st.Bet, resultLost
		}
	}
	b.Result = resultBanker
	rec := RoundRecord{
		Number: s.Round + 1, Banker: b.PlayerID, BankerTotal: bt, BankerOko: bankerOko,
		Results: map[string]string{}, Deltas: map[string]int{}, Stacks: map[string]int{},
	}
	bankDelta := 0
	for i := range s.Seats {
		st := &s.Seats[i]
		if st != b && st.Result != resultSatOut {
			bankDelta -= st.Delta
		}
	}
	b.Delta = bankDelta
	for _, st := range s.Seats {
		rec.Results[st.PlayerID] = st.Result
		rec.Deltas[st.PlayerID] = st.Delta
	}
	s.Round++
	s.RoundsInBank++
	events := []module.Event{{Type: "round_settled", Data: map[string]any{"round": rec.Number}}}

	more := true
	switch {
	case s.Bank < s.MinBet:
		rec.BankBroken = true
		more = s.endBankTurn()
	case s.RoundsInBank >= s.RoundsPerBank, s.punterCount() == 0:
		more = s.endBankTurn()
	}
	for _, st := range s.Seats {
		rec.Stacks[st.PlayerID] = s.worth(st.PlayerID)
	}
	s.Rounds = append(s.Rounds, rec)
	s.Current = ""
	if !more {
		return append(events, module.Event{Type: "match_completed", Data: map[string]any{"winners": s.leaders()}})
	}
	if s.Pause {
		s.Intermission.Begin(s.Round)
		return events
	}
	s.startRound()
	return append(events, module.Event{Type: "round_started", Data: map[string]any{"round": s.Round + 1}})
}

// worth is a player's chips, with the bank counted for whoever holds it.
func (s *GameState) worth(id string) int {
	st := s.seat(id)
	if s.Status == "active" && s.index(id) == s.bankerIndex() {
		return st.Stack + s.Bank
	}
	return st.Stack
}

// leaders is everyone level on the most chips.
func (s *GameState) leaders() []string {
	best := -1
	var out []string
	for _, st := range s.Seats {
		w := s.worth(st.PlayerID)
		switch {
		case w > best:
			best, out = w, []string{st.PlayerID}
		case w == best:
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
