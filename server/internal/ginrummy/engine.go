package ginrummy

import (
	"zolik/server/internal/module"
)

// Module is the Gin Rummy game module.
type Module struct{}

// New returns the module. Stateless: every method takes the state it works on.
func New() *Module { return &Module{} }

var _ module.GameModule = (*Module)(nil)

// NewMatch deals the first hand of a fresh match.
func (m *Module) NewMatch(cfg module.MatchConfig, players []module.PlayerRef, seed int64) (module.State, error) {
	if len(players) != 2 {
		return nil, module.Error{Code: ErrWrongPlayerCount, Message: "gin rummy seats exactly two"}
	}

	s := &GameState{
		Status:    "active",
		Variation: cfg.Variation,
		Hands:     map[string][]string{},
		Scores:    map[string]int{},
		HandsWon:  map[string]int{},
		Seed:      seed,
	}
	for _, p := range players {
		s.Players = append(s.Players, p.ID)
	}

	v := resolveVariation(cfg)
	s.TargetScore = cfg.Opt(OptTargetScore, v.targetScore)
	s.KnockLimitOpt = cfg.Opt(OptKnockLimit, v.knockLimit)
	s.BigGin = cfg.Opt(OptBigGin, module.BoolOpt(v.bigGin)) == module.OptOn
	s.LineBonuses = cfg.Opt(OptLineBonuses, module.BoolOpt(v.lineBonuses)) == module.OptOn
	s.Pause = cfg.PauseBetweenRounds(true)

	s.Dealer = s.Players[module.StartingSeat(seed, len(s.Players))]
	dealHand(s)
	return encode(s)
}

// dealHand shuffles and deals one hand, leaving the non-dealer to decide on
// the upcard. The seed is varied per hand, the same trick prsi and canasta
// use, so a match is reproducible yet does not deal the same cards twice.
func dealHand(s *GameState) {
	deck := shuffle(buildDeck(), s.Seed+int64(s.HandNumber)*7919+1)
	nonDealer := other(s.Players, s.Dealer)

	s.Hands = map[string][]string{s.Dealer: nil, nonDealer: nil}
	for i := 0; i < 10; i++ {
		for _, p := range []string{nonDealer, s.Dealer} {
			s.Hands[p] = append(s.Hands[p], deck[len(deck)-1])
			deck = deck[:len(deck)-1]
		}
	}

	up := deck[len(deck)-1]
	deck = deck[:len(deck)-1]
	s.DiscardPile = []string{up}
	s.Stock = deck
	s.KnockDiscard = ""

	if s.KnockLimitOpt == oklahomaSentinel {
		s.KnockLimit = cardValue(up)
	} else {
		s.KnockLimit = s.KnockLimitOpt
	}

	s.Current = nonDealer
	s.Phase = phaseUpcardNonDealer
	s.ForcedStockDraw = false
	s.Interest = map[string][]string{}
	s.Knocker = ""
	s.KnockGin = false
	s.KnockerDeadwood = 0
	s.KnockerMelds = nil
}

// Apply validates and applies one move. The state is decoded fresh on every
// call, so a refused action returns the caller's own bytes untouched.
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
			return raw, nil, errCode(ErrGameNotActive)
		}
		if err := s.Intermission.Mark(s.Players, playerID); err != nil {
			return raw, nil, err
		}
		var events []module.Event
		if s.Intermission.Settled(s.Players) {
			s.Intermission.Close()
			dealHand(s)
			events = []module.Event{{Type: "hand_started", Data: map[string]any{"handNumber": s.HandNumber}}}
		}
		out, err := encode(s)
		return out, events, err
	}

	if s.Current != playerID {
		return raw, nil, errCode(ErrNotYourTurn)
	}

	var events []module.Event
	switch a.Verb {
	case VerbDraw:
		events, err = applyDraw(s, playerID, a)
	case VerbPass:
		events, err = applyPass(s, playerID)
	case VerbDiscard:
		events, err = applyDiscard(s, playerID, a)
	case VerbKnock:
		events, err = applyKnock(s, playerID, a)
	case VerbLayOff:
		events, err = applyLayOff(s, playerID, a)
	case VerbFinishLayoff:
		events, err = applyFinishLayoff(s, playerID)
	default:
		err = module.Error{Code: ErrUnknownAction, Message: a.Verb}
	}
	if err != nil {
		return raw, nil, err
	}
	out, err := encode(s)
	return out, events, err
}

// --- the upcard dance --------------------------------------------------

func applyDraw(s *GameState, playerID string, a module.Action) ([]module.Event, error) {
	fromDiscard := a.OfferID == OfferDrawDiscard

	switch s.Phase {
	case phaseUpcardNonDealer, phaseUpcardDealer:
		if !fromDiscard {
			return nil, errCode(ErrWrongPhase)
		}
		return takeUpcard(s, playerID)
	case phaseDraw:
		if fromDiscard {
			if s.ForcedStockDraw {
				return nil, errCode(ErrUpcardDeclined)
			}
			if len(s.DiscardPile) == 0 {
				return nil, errCode(ErrNothingToDraw)
			}
			card := s.DiscardPile[len(s.DiscardPile)-1]
			s.DiscardPile = s.DiscardPile[:len(s.DiscardPile)-1]
			s.Hands[playerID] = append(s.Hands[playerID], card)
			s.Interest[playerID] = append(s.Interest[playerID], card)
		} else {
			if len(s.Stock) == 0 {
				return nil, errCode(ErrNothingToDraw)
			}
			card := s.Stock[len(s.Stock)-1]
			s.Stock = s.Stock[:len(s.Stock)-1]
			s.Hands[playerID] = append(s.Hands[playerID], card)
			s.ForcedStockDraw = false
		}
		s.Phase = phaseDiscard
		return []module.Event{{Type: "card_drawn", Data: map[string]any{
			"playerId": playerID, "fromDiscard": fromDiscard,
		}}}, nil
	default:
		return nil, errCode(ErrWrongPhase)
	}
}

func takeUpcard(s *GameState, playerID string) ([]module.Event, error) {
	if len(s.DiscardPile) == 0 {
		return nil, errCode(ErrNothingToDraw)
	}
	card := s.DiscardPile[0]
	s.DiscardPile = nil
	s.Hands[playerID] = append(s.Hands[playerID], card)
	s.Interest[playerID] = append(s.Interest[playerID], card)
	s.Phase = phaseDiscard
	return []module.Event{{Type: "upcard_taken", Data: map[string]any{"playerId": playerID, "card": card}}}, nil
}

// applyPass declines the upcard. Non-dealer first; if the dealer also
// declines, the turn passes to non-dealer with a mandatory stock draw — taking
// the discard at that point would just be accepting the card both players
// just refused, one decision late.
func applyPass(s *GameState, playerID string) ([]module.Event, error) {
	switch s.Phase {
	case phaseUpcardNonDealer:
		s.Phase = phaseUpcardDealer
		s.Current = s.Dealer
		return []module.Event{{Type: "upcard_passed", Data: map[string]any{"playerId": playerID}}}, nil
	case phaseUpcardDealer:
		s.Phase = phaseDraw
		s.Current = other(s.Players, s.Dealer)
		s.ForcedStockDraw = true
		return []module.Event{{Type: "upcard_passed", Data: map[string]any{"playerId": playerID}}}, nil
	default:
		return nil, errCode(ErrWrongPhase)
	}
}

// --- the ordinary turn --------------------------------------------------

// applyDiscard is the plain "no" to knocking: keep the turn open rather than
// end the hand. A stock faced at two cards without a knock here is the dead
// hand the rules describe — nobody scores, and the same dealer redeals.
func applyDiscard(s *GameState, playerID string, a module.Action) ([]module.Event, error) {
	if s.Phase != phaseDiscard {
		return nil, errCode(ErrWrongPhase)
	}
	if len(a.Cards) != 1 {
		return nil, errCode(ErrCardDoesNotFit)
	}
	card := a.Cards[0]
	if !hasCard(s.Hands[playerID], card) {
		return nil, errCode(ErrCardNotInHand)
	}
	s.Hands[playerID] = removeCard(s.Hands[playerID], card)
	s.DiscardPile = append(s.DiscardPile, card)
	events := []module.Event{{Type: "card_discarded", Data: map[string]any{"playerId": playerID, "card": card}}}

	if len(s.Stock) <= 1 {
		return append(events, deadHand(s)...), nil
	}
	s.Current = other(s.Players, playerID)
	s.Phase = phaseDraw
	return events, nil
}

// applyKnock is both knock and gin: which one a player got is arithmetic, not
// a declaration, so one verb covers both. An empty card list is the eleven-
// card big gin declaration, only meaningful when the option is on.
func applyKnock(s *GameState, playerID string, a module.Action) ([]module.Event, error) {
	if s.Phase != phaseDiscard {
		return nil, errCode(ErrWrongPhase)
	}
	hand := s.Hands[playerID]

	if len(a.Cards) == 0 {
		if !s.BigGin || len(hand) != 11 {
			return nil, errCode(ErrCardDoesNotFit)
		}
		deadwood, melds := Deadwood(hand)
		if deadwood != 0 {
			return nil, errCode(ErrDeadwoodTooHigh)
		}
		return knockOut(s, playerID, melds, 0, true), nil
	}

	if len(a.Cards) != 1 {
		return nil, errCode(ErrCardDoesNotFit)
	}
	card := a.Cards[0]
	if !hasCard(hand, card) {
		return nil, errCode(ErrCardNotInHand)
	}
	rest := removeCard(hand, card)
	deadwood, melds := Deadwood(rest)
	if deadwood > s.KnockLimit {
		return nil, errCode(ErrDeadwoodTooHigh)
	}
	s.Hands[playerID] = rest
	s.KnockDiscard = card
	return knockOut(s, playerID, melds, deadwood, deadwood == 0), nil
}

func knockOut(s *GameState, playerID string, melds []Meld, deadwood int, gin bool) []module.Event {
	s.Knocker = playerID
	s.KnockGin = gin
	s.KnockerDeadwood = deadwood
	s.KnockerMelds = melds

	events := []module.Event{{Type: "knocked", Data: map[string]any{
		"playerId": playerID, "gin": gin, "deadwood": deadwood,
	}}}
	if gin {
		return append(events, endHand(s)...)
	}
	s.Phase = phaseLayoff
	s.Current = other(s.Players, playerID)
	return events
}

// --- the lay-off sub-phase ----------------------------------------------

func applyLayOff(s *GameState, playerID string, a module.Action) ([]module.Event, error) {
	if s.Phase != phaseLayoff {
		return nil, errCode(ErrWrongPhase)
	}
	if playerID != other(s.Players, s.Knocker) {
		return nil, errCode(ErrNotYourTurn)
	}
	if len(a.Cards) != 1 {
		return nil, errCode(ErrCardDoesNotFit)
	}
	card := a.Cards[0]
	if !hasCard(s.Hands[playerID], card) {
		return nil, errCode(ErrCardNotInHand)
	}
	idx := meldIndex(s.KnockerMelds, a.Target)
	if idx < 0 {
		return nil, errCode(ErrNoSuchMeld)
	}
	if !extendsMeld(s.KnockerMelds[idx], card) {
		return nil, errCode(ErrCardDoesNotExtendMeld)
	}
	s.Hands[playerID] = removeCard(s.Hands[playerID], card)
	s.KnockerMelds[idx].Cards = insertIntoMeld(s.KnockerMelds[idx], card)
	return []module.Event{{Type: "laid_off", Data: map[string]any{
		"playerId": playerID, "card": card, "meldId": a.Target,
	}}}, nil
}

func applyFinishLayoff(s *GameState, playerID string) ([]module.Event, error) {
	if s.Phase != phaseLayoff {
		return nil, errCode(ErrWrongPhase)
	}
	if playerID != other(s.Players, s.Knocker) {
		return nil, errCode(ErrNotYourTurn)
	}
	return endHand(s), nil
}

// Finished reports whether the match is over and who won.
func (m *Module) Finished(raw module.State) (bool, []string, error) {
	s, err := decode(raw)
	if err != nil {
		return false, nil, err
	}
	if s.Status != "completed" || s.WinnerID == "" {
		return s.Status == "completed", nil, nil
	}
	return true, []string{s.WinnerID}, nil
}
