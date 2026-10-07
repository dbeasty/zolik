package lastcard

import (
	"math/rand"
	"strconv"

	"zolik/server/internal/module"
)

// Module is the Last Card game module.
type Module struct{}

// New returns the module. Stateless: every method takes the state it works on.
func New() *Module { return &Module{} }

var _ module.GameModule = (*Module)(nil)

// buildDeck returns the 108-card pack. Duplicates share a code, as Canasta's
// double deck does: two copies of the coral 7 are the same card.
func buildDeck() []string {
	out := make([]string, 0, 108)
	for _, c := range colours {
		out = append(out, c+"-0")
		for n := 1; n <= 9; n++ {
			face := c + "-" + strconv.Itoa(n)
			out = append(out, face, face)
		}
		for _, f := range []string{faceSkip, faceReverse, faceDrawTwo} {
			out = append(out, c+"-"+f, c+"-"+f)
		}
	}
	for i := 0; i < 4; i++ {
		out = append(out, cardWild, cardWildDrawFour)
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
		return nil, module.Error{Code: ErrTooFewPlayers, Message: "last card needs at least two players"}
	}
	handSize := cfg.Opt(OptHandSize, defaultHandSize)

	deck := shuffle(buildDeck(), seed)
	s := &GameState{
		Status:      "active",
		Direction:   1,
		Hands:       map[string][]string{},
		Seed:        seed,
		OpenDiscard: cfg.OpenDiscardPile(false),
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

	// Turn the first card. A wild would leave nobody having named a colour,
	// so it goes to the bottom and the next card is turned instead.
	for len(deck) > 1 && isWild(deck[0]) {
		deck = append(deck[1:], deck[0])
	}
	s.DiscardPile = []string{deck[0]}
	s.DrawPile = deck[1:]
	starter := s.TurnOrder[module.StartingSeat(seed, len(s.TurnOrder))]
	s.Current = starter

	// An opening action card takes effect as if the dealer — the player
	// before the starter — had just played it.
	switch faceOf(s.top()) {
	case faceSkip:
		s.Current = s.nextPlayer(starter)
	case faceDrawTwo:
		s.drawInto(starter, 2)
		s.Current = s.nextPlayer(starter)
	case faceReverse:
		if len(s.TurnOrder) == 2 {
			s.Current = s.nextPlayer(starter)
		} else {
			// Play turns back round, so the dealer, who played it, is first.
			s.Direction = -1
			s.Current = s.nextPlayer(starter)
		}
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
		return nil, nil, module.Error{Code: ErrCardDoesNotMatch, Message: "play exactly one card"}
	}
	card := a.Cards[0]
	hand := s.Hands[playerID]
	if !hasCard(hand, card) {
		return nil, nil, module.Error{Code: ErrCardNotInHand}
	}
	// Having drawn, the only card left to play this turn is the one drawn.
	if s.DrawnCard != "" && card != s.DrawnCard {
		return nil, nil, module.Error{Code: ErrOnlyDrawnCard}
	}
	if !s.matches(card) {
		return nil, nil, module.Error{Code: ErrCardDoesNotMatch}
	}
	if card == cardWildDrawFour && s.holdsColour(hand) {
		return nil, nil, module.Error{Code: ErrDrawFourHeld}
	}

	// A wild must name the colour that follows. Validated before anything is
	// mutated, so a missing or nonsense colour leaves the game untouched.
	declared := ""
	if isWild(card) {
		declared = a.Params["colour"]
		if declared == "" {
			return nil, nil, module.Error{Code: ErrColourRequired}
		}
		if !isColour(declared) {
			return nil, nil, module.Error{Code: ErrUnknownColour, Message: declared}
		}
	}

	s.Hands[playerID] = removeCard(hand, card)
	s.DiscardPile = append(s.DiscardPile, card)
	s.DeclaredColour = declared
	s.DrawnCard = ""
	s.BlankDraws = 0

	events := []module.Event{{Type: "card_played", Data: map[string]any{
		"playerId": playerID, "card": card, "declaredColour": declared,
	}}}

	if len(s.Hands[playerID]) == 0 {
		// The last card still does what it says: a Draw Two that goes out
		// makes the next player draw. Nothing hangs on it in a single deal,
		// but it is what the cards say, and a points game will count it.
		if victim := s.nextPlayer(playerID); faceOf(card) == faceDrawTwo || card == cardWildDrawFour {
			n := 2
			if card == cardWildDrawFour {
				n = 4
			}
			if got := s.drawInto(victim, n); got > 0 {
				events = append(events, drawnEvent(victim, got))
			}
		}
		s.Status = "completed"
		s.WinnerID = playerID
		s.Current = ""
		events = append(events, module.Event{Type: "game_ended", Data: map[string]any{"winnerId": playerID}})
		out, err := encode(s)
		return out, events, err
	}

	next := s.nextPlayer(playerID)
	switch {
	case faceOf(card) == faceSkip:
		events = append(events, skippedEvent(next))
		next = s.nextPlayer(next)
	case faceOf(card) == faceReverse:
		if len(s.TurnOrder) == 2 {
			// Between two players a Reverse brings it straight back.
			events = append(events, skippedEvent(next))
			next = playerID
		} else {
			s.Direction = -s.Direction
			if s.Direction == 0 {
				s.Direction = -1
			}
			next = s.nextPlayer(playerID)
		}
	case faceOf(card) == faceDrawTwo || card == cardWildDrawFour:
		n := 2
		if card == cardWildDrawFour {
			n = 4
		}
		got := s.drawInto(next, n)
		events = append(events, drawnEvent(next, got), skippedEvent(next))
		next = s.nextPlayer(next)
	}
	s.Current = next
	out, err := encode(s)
	return out, events, err
}

// applyDraw takes one card from the pile.
//
// If it can be played, the turn stays open for it: the player plays it or
// passes to keep it. If not, the turn is over. When there is nothing at all to
// draw — the whole pack is in hands — the turn passes, and once every player
// in turn has drawn nothing the deal ends: nobody can move, and waiting would
// strand the table.
func (m *Module) applyDraw(s *GameState, playerID string) (module.State, []module.Event, error) {
	if s.DrawnCard != "" {
		return nil, nil, module.Error{Code: ErrAlreadyDrew}
	}
	card, ok := s.drawOne()
	if !ok {
		s.BlankDraws++
		events := []module.Event{drawnEvent(playerID, 0)}
		if s.BlankDraws >= len(s.TurnOrder) {
			s.finishOnFewest()
			events = append(events, module.Event{Type: "game_ended", Data: map[string]any{"winnerId": s.WinnerID}})
		} else {
			s.Current = s.nextPlayer(playerID)
		}
		out, err := encode(s)
		return out, events, err
	}
	s.BlankDraws = 0
	s.Hands[playerID] = append(s.Hands[playerID], card)
	playable := s.matches(card) && (card != cardWildDrawFour || !s.holdsColour(s.Hands[playerID]))
	if playable {
		s.DrawnCard = card
	} else {
		s.Current = s.nextPlayer(playerID)
	}
	out, err := encode(s)
	return out, []module.Event{drawnEvent(playerID, 1)}, err
}

// applyPass keeps the card just drawn and ends the turn. Only legal after a
// draw turned up something playable — otherwise passing would be a way to
// stall forever.
func (m *Module) applyPass(s *GameState, playerID string) (module.State, []module.Event, error) {
	if s.DrawnCard == "" {
		return nil, nil, module.Error{Code: ErrNothingToKeep}
	}
	s.DrawnCard = ""
	s.Current = s.nextPlayer(playerID)
	out, err := encode(s)
	return out, []module.Event{{Type: "card_kept", Data: map[string]any{"playerId": playerID}}}, err
}

// drawInto deals up to n cards into a player's hand and says how many it
// managed.
func (s *GameState) drawInto(playerID string, n int) int {
	got := 0
	for i := 0; i < n; i++ {
		card, ok := s.drawOne()
		if !ok {
			break
		}
		s.Hands[playerID] = append(s.Hands[playerID], card)
		got++
	}
	return got
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

// finishOnFewest ends a deal nobody can move in: the fewest cards wins, the
// earliest seat taking a tie.
func (s *GameState) finishOnFewest() {
	best := ""
	for _, p := range s.TurnOrder {
		if best == "" || len(s.Hands[p]) < len(s.Hands[best]) {
			best = p
		}
	}
	s.Status = "completed"
	s.WinnerID = best
	s.Current = ""
	s.DrawnCard = ""
}

func drawnEvent(playerID string, n int) module.Event {
	return module.Event{Type: "cards_drawn", Data: map[string]any{"playerId": playerID, "count": n}}
}

func skippedEvent(playerID string) module.Event {
	return module.Event{Type: "turn_skipped", Data: map[string]any{"playerId": playerID}}
}

// Finished reports whether the match is over.
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
