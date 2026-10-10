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

// NewMatch seats the table, resolves its options and deals the first hand.
func (m *Module) NewMatch(cfg module.MatchConfig, players []module.PlayerRef, seed int64) (module.State, error) {
	if len(players) < 2 {
		return nil, module.Error{Code: ErrTooFewPlayers, Message: "last card needs at least two players"}
	}
	s := &GameState{
		Status:      "active",
		Seed:        seed,
		OpenDiscard: cfg.OpenDiscardPile(false),
		HandSize:    cfg.Opt(OptHandSize, defaultHandSize),
		TargetScore: cfg.Opt(OptTargetScore, defaultTargetScore),
		CallOn:      cfg.Opt(OptLastCardCall, module.OptOn) == module.OptOn,
		ChallengeOn: cfg.Opt(OptDrawFourChallenge, module.OptOn) == module.OptOn,
		Pause:       cfg.PauseBetweenRounds(true),
		Stacking:    cfg.Opt(OptStacking, stackOff),
		DrawUntil:   cfg.Opt(OptDrawUntilPlayable, module.OptOff) == module.OptOn,
		SevenZero:   cfg.Opt(OptSevenZero, module.OptOff) == module.OptOn,
		Scoring:     cfg.Opt(OptScoring, scoreWinnerTakes),
		Scores:      map[string]int{},
	}
	for _, p := range players {
		s.Players = append(s.Players, p.ID)
		s.TurnOrder = append(s.TurnOrder, p.ID)
		s.Scores[p.ID] = 0
	}
	s.deal()
	return encode(s)
}

// deal shuffles and deals the next hand, turns the first card and seats the
// first player. The first player moves one seat round with each deal.
func (s *GameState) deal() {
	handSize := s.HandSize
	if handSize <= 0 {
		handSize = defaultHandSize
	}
	dealSeed := s.Seed + int64(s.DealNumber)*104729
	deck := shuffle(buildDeck(), dealSeed)

	s.Hands = map[string][]string{}
	s.Direction = 1
	s.DeclaredColour, s.DrawnCard, s.Called, s.Unannounced = "", "", "", ""
	s.DrawFour, s.Reveal = nil, nil
	s.BlankDraws, s.Reshuffles, s.PendingDraw = 0, 0, 0

	// Whoever asked to sit out is dealt out of this deal — unless that would
	// leave fewer than two to play, when everybody is dealt in.
	s.DealtOut = nil
	if len(s.TurnOrder)-len(s.SittingOut) >= 2 {
		for _, p := range s.TurnOrder {
			for _, q := range s.SittingOut {
				if p == q {
					s.DealtOut = append(s.DealtOut, p)
				}
			}
		}
	}
	in := s.inPlay()

	for i := 0; i < handSize; i++ {
		for _, p := range in {
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
	n := len(in)
	starter := in[(module.StartingSeat(s.Seed, n)+s.DealNumber)%n]
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
		if n == 2 {
			s.Current = s.nextPlayer(starter)
		} else {
			// Play turns back round, so the dealer, who played it, is first.
			s.Direction = -1
			s.Current = s.nextPlayer(starter)
		}
	}
}

// Apply validates and applies one move. The state is decoded fresh on every
// call, so a refused move leaves the caller's state untouched.
func (m *Module) Apply(raw module.State, playerID string, a module.Action) (module.State, []module.Event, error) {
	s, err := decode(raw)
	if err != nil {
		return raw, nil, err
	}
	if s.Status != "active" {
		return raw, nil, module.Error{Code: ErrGameNotActive}
	}

	if s.Intermission.Open {
		if a.Verb != module.VerbContinue {
			return raw, nil, module.Error{Code: module.ErrNotPaused}
		}
		if err := s.Intermission.Mark(s.TurnOrder, playerID); err != nil {
			return raw, nil, err
		}
		var events []module.Event
		if s.Intermission.Settled(s.TurnOrder) {
			s.Intermission.Close()
			s.deal()
			events = []module.Event{{Type: "deal_started", Data: map[string]any{"deal": s.DealNumber + 1}}}
		}
		out, err := encode(s)
		return out, events, err
	}
	if a.Verb == module.VerbContinue {
		return raw, nil, module.Error{Code: module.ErrNotPaused}
	}
	if s.Current != playerID {
		return raw, nil, module.Error{Code: ErrNotYourTurn}
	}

	// A hand shown to a challenger stays on their screen until they move.
	if s.Reveal != nil && s.Reveal.Viewer == playerID {
		s.Reveal = nil
	}

	var events []module.Event
	switch a.Verb {
	case VerbCatch:
		events, err = s.applyCatch(playerID)
	case VerbCall:
		events, err = s.applyCall(playerID)
	case VerbChallenge:
		s.Unannounced = ""
		events, err = s.applyChallenge(playerID)
	case VerbAccept:
		s.Unannounced = ""
		events, err = s.applyAccept(playerID)
	case VerbPlay, VerbDraw, VerbPass:
		// A Wild Draw Four waiting on an answer takes a challenge, an
		// acceptance — or, where any draw card stacks, another Wild Draw
		// Four on top.
		if s.DrawFour != nil && !(a.Verb == VerbPlay && s.Stacking == stackAny) {
			return nil, nil, module.Error{Code: ErrAnswerDrawFour}
		}
		// Anything but a catch closes the window on a player who went to one
		// card in silence: the moment has passed.
		s.Unannounced = ""
		switch a.Verb {
		case VerbPlay:
			events, err = s.applyPlay(playerID, a)
		case VerbDraw:
			events, err = s.applyDraw(playerID)
		default:
			events, err = s.applyPass(playerID)
		}
	default:
		return raw, nil, module.Error{Code: ErrUnknownAction, Message: a.Verb}
	}
	if err != nil {
		return nil, nil, err
	}
	out, err := encode(s)
	return out, events, err
}

// passTurn hands the turn on, clearing what belonged to the turn just ended.
func (s *GameState) passTurn(to string) {
	s.Current = to
	s.Called = ""
	s.DrawnCard = ""
}

func (s *GameState) applyPlay(playerID string, a module.Action) ([]module.Event, error) {
	if len(a.Cards) != 1 {
		return nil, module.Error{Code: ErrCardDoesNotMatch, Message: "play exactly one card"}
	}
	card := a.Cards[0]
	hand := s.Hands[playerID]
	if !hasCard(hand, card) {
		return nil, module.Error{Code: ErrCardNotInHand}
	}
	// Having drawn, the only card left to play this turn is the one drawn.
	if s.DrawnCard != "" && card != s.DrawnCard {
		return nil, module.Error{Code: ErrOnlyDrawnCard}
	}
	// A stack waiting on this player is answered by a draw card of their
	// own, or taken; nothing else may go down on it.
	stacked := s.PendingDraw > 0
	bluff := false
	if stacked {
		if !s.stackable(card) {
			if s.DrawFour != nil {
				return nil, module.Error{Code: ErrAnswerDrawFour}
			}
			return nil, module.Error{Code: ErrMustAnswerDraw}
		}
	} else {
		if !s.matches(card) {
			return nil, module.Error{Code: ErrCardDoesNotMatch}
		}
		// Without the challenge, holding the colour simply forbids the card.
		// With it, the card may go down anyway — and that is the bluff a
		// challenge exists to test.
		bluff = card == cardWildDrawFour && s.holdsColour(hand)
		if bluff && !s.ChallengeOn {
			return nil, module.Error{Code: ErrDrawFourHeld}
		}
	}

	declared := ""
	if isWild(card) {
		declared = a.Params["colour"]
		if declared == "" {
			return nil, module.Error{Code: ErrColourRequired}
		}
		if !isColour(declared) {
			return nil, module.Error{Code: ErrUnknownColour, Message: declared}
		}
	}

	// The hand as it was when the card went down, for a challenger to see.
	before := append([]string(nil), hand...)
	s.Hands[playerID] = removeCard(hand, card)
	s.DiscardPile = append(s.DiscardPile, card)
	s.DeclaredColour = declared
	s.DrawnCard = ""
	s.BlankDraws = 0

	// What the card did, filled in below as it happens, so the line that
	// narrates the play can say it in one sentence — see events.go.
	played := map[string]any{"playerId": playerID, "card": card, "declaredColour": declared}
	events := []module.Event{{Type: "card_played", Data: played}}

	next := s.nextPlayer(playerID)
	if len(s.Hands[playerID]) == 0 {
		// The last card still does what it says: a Draw card that goes out
		// makes the next player draw — the whole stack, where there is one —
		// which a points game counts.
		if n := drawCount(card) + s.PendingDraw; n > 0 {
			s.PendingDraw = 0
			if got := s.drawInto(next, n); got > 0 {
				played["effect"], played["victim"], played["n"] = effectDraw, next, got
				events = append(events, effectEvent(drawnEvent(next, got)))
			}
		}
		return append(events, s.endDeal(playerID)...), nil
	}

	// Sevens and zeros move hands — before the call is judged, because what
	// matters is the hand the player is left holding.
	if s.SevenZero {
		events = append(events, s.sevenZero(playerID, card)...)
	}
	if len(s.Hands[playerID]) == 1 && s.CallOn {
		if s.Called == playerID {
			events = append(events, module.Event{Type: "last_card", Data: map[string]any{"playerId": playerID}})
		} else {
			s.Unannounced = playerID
		}
	}
	s.Called = ""

	if s.Stacking != stackOff && drawCount(card) > 0 {
		// The draw goes on the stack and the next player answers it. A Wild
		// Draw Four that starts a stack can still be challenged; one stacked
		// on another is an answer, not a bluff.
		s.PendingDraw += drawCount(card)
		s.DrawFour, s.Reveal = nil, nil
		played["effect"], played["victim"], played["n"] = effectStack, next, s.PendingDraw
		if card == cardWildDrawFour && s.ChallengeOn && !stacked {
			s.DrawFour = &DrawFourPending{Player: playerID, Victim: next, Bluff: bluff}
			s.Reveal = &Reveal{Owner: playerID, Cards: before, Bluff: bluff}
			played["effect"] = effectChallengeable
		}
		s.passTurn(next)
		return events, nil
	}

	switch {
	case faceOf(card) == faceSkip:
		played["effect"], played["victim"] = effectSkip, next
		events = append(events, effectEvent(skippedEvent(next)))
		s.passTurn(s.nextPlayer(next))
	case faceOf(card) == faceReverse:
		if len(s.inPlay()) == 2 {
			// Between two players a Reverse brings it straight back.
			played["effect"], played["victim"] = effectSkip, next
			events = append(events, effectEvent(skippedEvent(next)))
			s.passTurn(playerID)
		} else {
			s.Direction = -s.direction()
			played["effect"] = effectReverse
			played["clockwise"] = s.Direction > 0
			s.passTurn(s.nextPlayer(playerID))
		}
	case card == cardWildDrawFour && s.ChallengeOn:
		// The victim answers before anything is drawn.
		s.DrawFour = &DrawFourPending{Player: playerID, Victim: next, Bluff: bluff}
		s.Reveal = &Reveal{Owner: playerID, Cards: before, Bluff: bluff}
		played["effect"], played["victim"] = effectChallengeable, next
		s.passTurn(next)
	case faceOf(card) == faceDrawTwo || card == cardWildDrawFour:
		got := s.drawInto(next, drawCount(card))
		played["effect"], played["victim"], played["n"] = effectDraw, next, got
		events = append(events, effectEvent(drawnEvent(next, got)), effectEvent(skippedEvent(next)))
		s.passTurn(s.nextPlayer(next))
	default:
		s.passTurn(next)
	}
	return events, nil
}

func drawCount(card string) int {
	switch {
	case card == cardWildDrawFour:
		return 4
	case faceOf(card) == faceDrawTwo:
		return 2
	}
	return 0
}

func (s *GameState) direction() int {
	if s.Direction == 0 {
		return 1
	}
	return s.Direction
}

// applyDraw takes one card from the pile.
//
// If it can be played, the turn stays open for it: the player plays it or
// passes to keep it. If not, the turn is over. When there is nothing at all to
// draw — the whole pack is in hands — the turn passes, and once every player
// in turn has drawn nothing the deal ends: nobody can move, and waiting would
// strand the table.
func (s *GameState) applyDraw(playerID string) ([]module.Event, error) {
	if s.DrawnCard != "" {
		return nil, module.Error{Code: ErrAlreadyDrew}
	}
	if s.PendingDraw > 0 {
		// Taking the stack: every card on it, and the turn.
		got := s.drawInto(playerID, s.PendingDraw)
		s.PendingDraw = 0
		s.DrawFour, s.Reveal = nil, nil
		s.passTurn(s.nextPlayer(playerID))
		return []module.Event{
			{Type: "stack_taken", Data: map[string]any{"playerId": playerID, "count": got}},
			effectEvent(skippedEvent(playerID)),
		}, nil
	}
	if s.DrawUntil {
		return s.drawUntilPlayable(playerID)
	}
	card, ok := s.drawOne()
	if !ok {
		s.BlankDraws++
		events := []module.Event{drawnEvent(playerID, 0)}
		if s.BlankDraws >= len(s.inPlay()) {
			return append(events, s.endDeal(s.fewestCards())...), nil
		}
		s.passTurn(s.nextPlayer(playerID))
		return events, nil
	}
	s.BlankDraws = 0
	s.Hands[playerID] = append(s.Hands[playerID], card)
	if s.playableDrawn(playerID, card) {
		s.DrawnCard = card
	} else {
		s.passTurn(s.nextPlayer(playerID))
	}
	return []module.Event{drawnEvent(playerID, 1)}, nil
}

// playableDrawn is whether a card just drawn may be played this turn: it
// matches, and if it is a Wild Draw Four the table either allows a bluff or
// the hand holds nothing of the colour.
func (s *GameState) playableDrawn(playerID, card string) bool {
	if !s.matches(card) {
		return false
	}
	return card != cardWildDrawFour || s.ChallengeOn || !s.holdsColour(s.Hands[playerID])
}

// applyPass keeps the card just drawn and ends the turn. Only legal after a
// draw turned up something playable — otherwise passing would be a way to
// stall forever.
func (s *GameState) applyPass(playerID string) ([]module.Event, error) {
	if s.DrawnCard == "" {
		return nil, module.Error{Code: ErrNothingToKeep}
	}
	s.passTurn(s.nextPlayer(playerID))
	return []module.Event{{Type: "card_kept", Data: map[string]any{"playerId": playerID}}}, nil
}

// applyCall says "Last card!" — on your turn, holding two cards, before the
// play that leaves you one. It does not end the turn.
func (s *GameState) applyCall(playerID string) ([]module.Event, error) {
	if !s.CallOn || s.DrawFour != nil || len(s.Hands[playerID]) != 2 {
		return nil, module.Error{Code: ErrCallNotNow}
	}
	if s.Called == playerID {
		return nil, module.Error{Code: ErrAlreadyCalled}
	}
	s.Called = playerID
	return []module.Event{{Type: "last_card_called", Data: map[string]any{"playerId": playerID}}}, nil
}

// applyCatch catches a player who went down to one card without calling:
// they draw two. The catcher's own turn carries on.
func (s *GameState) applyCatch(playerID string) ([]module.Event, error) {
	target := s.Unannounced
	if !s.CallOn || target == "" || target == playerID {
		return nil, module.Error{Code: ErrNothingToCatch}
	}
	got := s.drawInto(target, 2)
	s.Unannounced = ""
	return []module.Event{
		{Type: "caught", Data: map[string]any{"playerId": target, "by": playerID, "count": got}},
		effectEvent(drawnEvent(target, got)),
	}, nil
}

// applyChallenge disputes a Wild Draw Four. If the player who played it held
// a card of the colour in play, they draw the four instead and the
// challenger plays on; if not, the challenger draws six and loses the turn.
// Either way the challenger sees the hand it was played from.
func (s *GameState) applyChallenge(playerID string) ([]module.Event, error) {
	d := s.DrawFour
	if d == nil || d.Victim != playerID {
		return nil, module.Error{Code: ErrNoDrawFour}
	}
	s.DrawFour = nil
	if s.Reveal != nil {
		s.Reveal.Viewer = playerID
	}
	owed := s.owed(4)
	s.PendingDraw = 0
	if d.Bluff {
		got := s.drawInto(d.Player, 4)
		return []module.Event{
			{Type: "challenge_won", Data: map[string]any{"playerId": playerID, "against": d.Player, "count": got}},
			effectEvent(drawnEvent(d.Player, got)),
		}, nil
	}
	got := s.drawInto(playerID, owed+2)
	s.passTurn(s.nextPlayer(playerID))
	return []module.Event{
		{Type: "challenge_lost", Data: map[string]any{"playerId": playerID, "against": d.Player, "count": got}},
		effectEvent(drawnEvent(playerID, got)),
		effectEvent(skippedEvent(playerID)),
	}, nil
}

// applyAccept takes a Wild Draw Four as played: four cards, and the turn.
func (s *GameState) applyAccept(playerID string) ([]module.Event, error) {
	d := s.DrawFour
	if d == nil || d.Victim != playerID {
		return nil, module.Error{Code: ErrNoDrawFour}
	}
	s.DrawFour = nil
	s.Reveal = nil
	got := s.drawInto(playerID, s.owed(4))
	s.PendingDraw = 0
	s.passTurn(s.nextPlayer(playerID))
	return []module.Event{
		{Type: "draw_four_accepted", Data: map[string]any{"playerId": playerID, "count": got}},
		effectEvent(skippedEvent(playerID)),
	}, nil
}

// endDeal scores the deal just won and either ends the match, pauses for the
// table to read the score, or deals the next hand.
func (s *GameState) endDeal(winner string) []module.Event {
	res := DealResult{Number: s.DealNumber + 1, Winner: winner}
	if s.Scores == nil {
		s.Scores = map[string]int{}
	}
	for _, p := range s.TurnOrder {
		if p == winner {
			continue
		}
		b := breakdownOf(s.Hands[p])
		res.Numbers += b.Numbers
		res.Actions += b.Actions
		res.Wilds += b.Wilds
		if s.Scoring == scoreLowest {
			// Each player pays for their own hand.
			if res.Charged == nil {
				res.Charged = map[string]Breakdown{}
			}
			res.Charged[p] = b
			s.Scores[p] += b.Total()
		}
	}
	res.Points = res.Numbers + res.Actions + res.Wilds
	if s.Scoring != scoreLowest {
		s.Scores[winner] += res.Points
	}
	res.Totals = map[string]int{}
	for _, p := range s.TurnOrder {
		res.Totals[p] = s.Scores[p]
	}
	s.Deals = append(s.Deals, res)
	s.DrawFour, s.Unannounced, s.Called, s.DrawnCard = nil, "", "", ""
	s.PendingDraw = 0

	events := []module.Event{{Type: "deal_ended", Data: map[string]any{
		"deal": res.Number, "winnerId": winner, "points": res.Points, "lowest": s.Scoring == scoreLowest,
	}}}
	if over, champion := s.matchOver(winner); over {
		s.Status = "completed"
		s.WinnerID = champion
		s.Current = ""
		return append(events, module.Event{Type: "game_ended", Data: map[string]any{"winnerId": champion}})
	}
	s.DealNumber++
	if s.Pause {
		s.Intermission.Begin(s.DealNumber)
		s.Current = ""
		return events
	}
	s.deal()
	return append(events, module.Event{Type: "deal_started", Data: map[string]any{"deal": s.DealNumber + 1}})
}

// fewestCards is who wins a deal nobody can move in: the fewest cards, the
// earliest seat taking a tie.
func (s *GameState) fewestCards() string {
	best := ""
	for _, p := range s.inPlay() {
		if best == "" || len(s.Hands[p]) < len(s.Hands[best]) {
			best = p
		}
	}
	return best
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
		s.DrawPile = shuffle(rest, s.Seed+int64(s.DealNumber)*104729+int64(s.Reshuffles)*7919)
		s.DiscardPile = []string{top}
	}
	if len(s.DrawPile) == 0 {
		return "", false
	}
	card := s.DrawPile[len(s.DrawPile)-1]
	s.DrawPile = s.DrawPile[:len(s.DrawPile)-1]
	return card, true
}

func drawnEvent(playerID string, n int) module.Event {
	return module.Event{Type: "cards_drawn", Data: map[string]any{"playerId": playerID, "count": n}}
}

// What a card did, as card_played records it for the narration.
const (
	effectSkip          = "skip"
	effectReverse       = "reverse"
	effectDraw          = "draw"
	effectStack         = "stack"
	effectChallengeable = "challengeable"
)

// effectEvent marks an event as the consequence of a card just played, which
// that card's own line already says — so the strip does not say it twice.
func effectEvent(ev module.Event) module.Event {
	ev.Data["effect"] = true
	return ev
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

// owed is what a Wild Draw Four costs its victim: the stack it sits on where
// stacking is played, otherwise the card's own count.
func (s *GameState) owed(dflt int) int {
	if s.PendingDraw > 0 {
		return s.PendingDraw
	}
	return dflt
}

// drawUntilPlayable draws until a card that can be played turns up, which is
// then the player's to play or keep. If the pack runs out first the turn
// passes with whatever was drawn.
func (s *GameState) drawUntilPlayable(playerID string) ([]module.Event, error) {
	got := 0
	for {
		card, ok := s.drawOne()
		if !ok {
			break
		}
		got++
		s.Hands[playerID] = append(s.Hands[playerID], card)
		if s.playableDrawn(playerID, card) {
			s.BlankDraws = 0
			s.DrawnCard = card
			return []module.Event{drawnEvent(playerID, got)}, nil
		}
	}
	events := []module.Event{drawnEvent(playerID, got)}
	if got == 0 {
		s.BlankDraws++
		if s.BlankDraws >= len(s.inPlay()) {
			return append(events, s.endDeal(s.fewestCards())...), nil
		}
	} else {
		s.BlankDraws = 0
	}
	s.passTurn(s.nextPlayer(playerID))
	return events, nil
}

// sevenZero applies the house rule's two cards. A 7 swaps the player's hand
// with the shortest other hand, the nearest in the direction of play taking a
// tie — the swap a player would choose, made for them, so the rule needs no
// way to name another seat. A 0 passes every hand to the next player in the
// direction of play.
func (s *GameState) sevenZero(playerID, card string) []module.Event {
	switch faceOf(card) {
	case "7":
		if isWild(card) {
			return nil
		}
		target := ""
		for i, p := 1, s.nextPlayer(playerID); i < len(s.inPlay()); i, p = i+1, s.nextPlayer(p) {
			if target == "" || len(s.Hands[p]) < len(s.Hands[target]) {
				target = p
			}
		}
		if target == "" {
			return nil
		}
		s.Hands[playerID], s.Hands[target] = s.Hands[target], s.Hands[playerID]
		return []module.Event{{Type: "hands_swapped", Data: map[string]any{"playerId": playerID, "with": target}}}
	case "0":
		if isWild(card) {
			return nil
		}
		moved := map[string][]string{}
		for _, p := range s.inPlay() {
			moved[s.nextPlayer(p)] = s.Hands[p]
		}
		s.Hands = moved
		return []module.Event{{Type: "hands_passed", Data: map[string]any{"playerId": playerID}}}
	}
	return nil
}

// matchOver says whether the deal just won ends the match, and who won it.
//
// Winner-takes: the deal's winner, once their total reaches the target.
// Lowest: once anyone's total reaches the target, the lowest total — the
// deal's winner taking a tie, since they just went out, then the earliest
// seat. A single deal (no target) ends with the deal's winner either way.
func (s *GameState) matchOver(dealWinner string) (bool, string) {
	if s.TargetScore <= 0 {
		return true, dealWinner
	}
	if s.Scoring != scoreLowest {
		return s.Scores[dealWinner] >= s.TargetScore, dealWinner
	}
	reached := false
	for _, p := range s.TurnOrder {
		if s.Scores[p] >= s.TargetScore {
			reached = true
		}
	}
	if !reached {
		return false, ""
	}
	best := dealWinner
	for _, p := range s.TurnOrder {
		if s.Scores[p] < s.Scores[best] {
			best = p
		}
	}
	return true, best
}

var _ module.DealsAround = (*Module)(nil)

// DealAround puts a player down to sit out the deals from the next one on, or
// back in. The deal in play is not touched: a bot finishes it for them.
func (m *Module) DealAround(raw module.State, playerID string, out bool) (module.State, error) {
	s, err := decode(raw)
	if err != nil {
		return raw, err
	}
	keep := s.SittingOut[:0:0]
	for _, p := range s.SittingOut {
		if p != playerID {
			keep = append(keep, p)
		}
	}
	if out {
		keep = append(keep, playerID)
	}
	s.SittingOut = keep
	return encode(s)
}
