package prsi

import (
	"math/rand"

	"zolik/server/internal/module"
)

// Module is the Prší game module.
type Module struct{}

// New returns the module. Stateless: every method takes the state it works on.
func New() *Module { return &Module{} }

var _ module.GameModule = (*Module)(nil)

// buildDeck returns the 32-card Czech deck.
func buildDeck() []string {
	out := make([]string, 0, len(ranks)*len(suits))
	for _, s := range suits {
		for _, r := range ranks {
			out = append(out, r+s)
		}
	}
	return out
}

func shuffle(cards []string, seed int64) []string {
	out := append([]string(nil), cards...)
	r := rand.New(rand.NewSource(seed))
	r.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

// NewMatch deals a fresh game.
func (m *Module) NewMatch(cfg module.MatchConfig, players []module.PlayerRef, seed int64) (module.State, error) {
	if len(players) < 2 {
		return nil, module.Error{Code: "TOO_FEW_PLAYERS", Message: "prsi needs at least two players"}
	}
	handSize := cfg.Opt(OptHandSize, defaultHandSize)

	deck := shuffle(buildDeck(), seed)
	s := &GameState{
		Status: "active",
		Hands:  map[string][]string{},
		Seed:   seed,
	}
	for _, p := range players {
		s.Players = append(s.Players, p.ID)
		s.TurnOrder = append(s.TurnOrder, p.ID)
	}

	for i := 0; i < handSize; i++ {
		for _, p := range s.TurnOrder {
			s.Hands[p] = append(s.Hands[p], deck[0])
			deck = deck[1:]
		}
	}

	// Turn the first card. A wild would leave nobody having named a suit, so
	// bury it and try the next — simpler and less surprising than asking the
	// dealer to declare one before anyone has looked at their hand.
	for len(deck) > 1 && rankOf(deck[0]) == rankWild {
		deck = append(deck[1:], deck[0])
	}
	s.DiscardPile = []string{deck[0]}
	s.DrawPile = deck[1:]
	s.Current = s.TurnOrder[0]

	// The opening card's effect applies to the first player, exactly as if it
	// had been played at them — that is how it is played at a table, and
	// exempting it would be a special case with no rule behind it.
	switch rankOf(s.top()) {
	case rankDrawTwo:
		s.PendingDraw = 2
	case rankSkip:
		s.SkipPending = true
	}
	return encode(s)
}

// Apply validates and applies one move.
func (m *Module) Apply(raw module.State, playerID string, a module.Action) (module.State, []module.Event, error) {
	s, err := decode(raw)
	if err != nil {
		return raw, nil, err
	}
	if s.Status != "active" {
		return raw, nil, module.Error{Code: ErrGameNotActive}
	}
	if s.Current != playerID {
		return raw, nil, module.Error{Code: ErrNotYourTurn}
	}

	switch a.Verb {
	case VerbPlay:
		return m.applyPlay(s, playerID, a)
	case VerbDraw:
		return m.applyDraw(s, playerID)
	case VerbPass:
		return m.applyPass(s, playerID)
	default:
		return raw, nil, module.Error{Code: ErrUnknownAction, Message: a.Verb}
	}
}

func (m *Module) applyPlay(s *GameState, playerID string, a module.Action) (module.State, []module.Event, error) {
	if len(a.Cards) != 1 {
		return nil, nil, module.Error{Code: ErrCardDoesNotFit, Message: "play exactly one card"}
	}
	card := a.Cards[0]
	if !hasCard(s.Hands[playerID], card) {
		return nil, nil, module.Error{Code: ErrCardNotInHand}
	}
	if !s.playable(card) {
		if s.PendingDraw > 0 {
			return nil, nil, module.Error{Code: ErrMustAnswerDraw}
		}
		return nil, nil, module.Error{Code: ErrCardDoesNotFit}
	}

	// A wild must name the suit that follows. Validated before anything is
	// mutated, so a missing or nonsense suit leaves the game untouched.
	declared := ""
	if rankOf(card) == rankWild {
		declared = a.Params["suit"]
		if declared == "" {
			return nil, nil, module.Error{Code: ErrSuitRequired}
		}
		if !isSuit(declared) {
			return nil, nil, module.Error{Code: ErrUnknownSuit, Message: declared}
		}
	}

	s.Hands[playerID] = removeCard(s.Hands[playerID], card)
	s.DiscardPile = append(s.DiscardPile, card)
	s.DeclaredSuit = declared

	events := []module.Event{{Type: "card_played", Data: map[string]any{
		"playerId": playerID, "card": card, "declaredSuit": declared,
	}}}

	// Shedding the last card ends it, before any effect is applied: a 7 that
	// empties your hand wins rather than obliging an opponent to draw.
	if len(s.Hands[playerID]) == 0 {
		s.Status = "completed"
		s.WinnerID = playerID
		s.Current = ""
		events = append(events, module.Event{Type: "game_ended", Data: map[string]any{"winnerId": playerID}})
		out, err := encode(s)
		return out, events, err
	}

	switch rankOf(card) {
	case rankDrawTwo:
		// Stacks: answering a 7 with a 7 passes on the whole debt plus two.
		s.PendingDraw += 2
		s.SkipPending = false
	case rankSkip:
		s.SkipPending = true
		s.PendingDraw = 0
	default:
		s.SkipPending = false
	}

	s.Current = s.nextPlayer(playerID)
	out, err := encode(s)
	return out, events, err
}

// applyDraw takes the card (or cards) the pile owes this player.
//
// One action covers both cases on purpose: an ordinary "I cannot play, so I
// draw one" and "I could not answer a 7, so I take the two". They are the
// same gesture at a table, and splitting them would make a client decide
// which one it was — a rule the client would then own.
func (m *Module) applyDraw(s *GameState, playerID string) (module.State, []module.Event, error) {
	count := 1
	if s.PendingDraw > 0 {
		count = s.PendingDraw
	}

	drawn := make([]string, 0, count)
	for i := 0; i < count; i++ {
		card, ok := s.drawOne()
		if !ok {
			break
		}
		drawn = append(drawn, card)
	}
	if len(drawn) == 0 {
		// Nothing anywhere. The turn still has to move, or play deadlocks.
		if s.PendingDraw == 0 {
			return nil, nil, module.Error{Code: ErrNothingToDraw}
		}
	}
	s.Hands[playerID] = append(s.Hands[playerID], drawn...)
	s.PendingDraw = 0
	s.SkipPending = false
	s.Current = s.nextPlayer(playerID)

	out, err := encode(s)
	return out, []module.Event{{Type: "cards_drawn", Data: map[string]any{
		"playerId": playerID, "count": len(drawn),
	}}}, err
}

// applyPass takes the turn an Ace cost you. Only legal while a skip is
// pending — otherwise passing would be a way to stall forever.
func (m *Module) applyPass(s *GameState, playerID string) (module.State, []module.Event, error) {
	if !s.SkipPending {
		return nil, nil, module.Error{Code: ErrCardDoesNotFit, Message: "nothing to pass on"}
	}
	s.SkipPending = false
	s.Current = s.nextPlayer(playerID)
	out, err := encode(s)
	return out, []module.Event{{Type: "turn_skipped", Data: map[string]any{"playerId": playerID}}}, err
}

// drawOne takes the top of the draw pile, recycling the discard pile when it
// runs out. The top card stays face up; everything under it is reshuffled.
func (s *GameState) drawOne() (string, bool) {
	if len(s.DrawPile) == 0 {
		if len(s.DiscardPile) <= 1 {
			return "", false
		}
		top := s.top()
		rest := s.DiscardPile[:len(s.DiscardPile)-1]
		s.Reshuffles++
		// Vary the seed per reshuffle, or the same recycled pile comes back
		// in exactly the same order every time.
		s.DrawPile = shuffle(rest, s.Seed+int64(s.Reshuffles)*7919)
		s.DiscardPile = []string{top}
	}
	if len(s.DrawPile) == 0 {
		return "", false
	}
	card := s.DrawPile[len(s.DrawPile)-1]
	s.DrawPile = s.DrawPile[:len(s.DrawPile)-1]
	return card, true
}

// Finished reports whether the match is over.
func (m *Module) Finished(raw module.State) (bool, string, error) {
	s, err := decode(raw)
	if err != nil {
		return false, "", err
	}
	return s.Status == "completed", s.WinnerID, nil
}

func isSuit(s string) bool {
	for _, x := range suits {
		if x == s {
			return true
		}
	}
	return false
}
