// Package lastcard implements Last Card, the colour-matching shedding game of
// the Crazy Eights family, as a game module.
//
// Its nearest relative here is Prší, and the difference is the deck: Prší is
// dealt from the 32-card German pack and gives three of its ranks an effect;
// Last Card has a pack of its own, where the effects are printed on the cards
// as faces of their own and the four suits are colours.
//
// Rules implemented (the standard ruleset; house rules come later as options):
//   - 108 cards: in each of four colours one 0, two each of 1–9, and two each
//     of Skip, Reverse and Draw Two; plus four Wilds and four Wild Draw Fours.
//   - Deal 7 each, turn one card face up to start the pile.
//   - On your turn, play a card matching the pile's colour, number or symbol,
//     or a Wild. A Wild Draw Four may only be played when you hold no card of
//     the colour in play.
//   - Otherwise draw one. If it can be played you may play it straight away;
//     if not, or if you keep it, your turn ends.
//   - Skip: the next player loses their turn. Reverse: play changes
//     direction; between two players it is a Skip. Draw Two: the next player
//     draws two and loses their turn. Wild: name the colour that follows.
//     Wild Draw Four: name the colour; the next player draws four and loses
//     their turn.
//   - Empty your hand to win.
package lastcard

import (
	"encoding/json"
	"fmt"
	"strings"

	"zolik/server/internal/module"
)

// The four colours, in the order the deck is built and a hand is sorted.
//
// Coral, teal, violet and amber. Each is also drawn with a shape of its own,
// so a colour can be told apart by a player who does not see it.
var colours = []string{"C", "T", "V", "A"}

// Faces a coloured card can carry, besides the numbers 0–9.
const (
	faceSkip    = "S"
	faceReverse = "R"
	faceDrawTwo = "D"
)

// The two wilds. They have no colour, so their codes have no hyphen.
const (
	cardWild         = "W"
	cardWildDrawFour = "W4"
)

// GameState is the whole match. Opaque to the runtime — only this package
// reads it.
type GameState struct {
	Status    string   `json:"status"` // "active" | "completed"
	Players   []string `json:"players"`
	TurnOrder []string `json:"turnOrder"`
	Current   string   `json:"current"`

	// SittingOut are the players to be dealt out of deals from the next one
	// on, because they are away (module.DealsAround); DealtOut are those dealt
	// out of the deal in play. TurnOrder stays the whole table — standings,
	// seats and readying go by it — and inPlay is who holds cards this deal.
	SittingOut []string `json:"sittingOut,omitempty"`
	DealtOut   []string `json:"dealtOut,omitempty"`

	// Direction is +1 while play goes round the table in seating order and
	// -1 after an odd number of Reverses.
	Direction int `json:"direction"`

	DrawPile    []string            `json:"drawPile"`
	DiscardPile []string            `json:"discardPile"` // top = last
	Hands       map[string][]string `json:"hands"`

	// DeclaredColour is set while a wild is the top card: the colour its
	// player named, which is what the next card must match. Empty otherwise.
	DeclaredColour string `json:"declaredColour,omitempty"`

	// DrawnCard is the card the player to move has just drawn and may still
	// play this turn. Set only when that card is playable; while it is set,
	// that card is the only one they may play, and passing keeps it.
	DrawnCard string `json:"drawnCard,omitempty"`

	// BlankDraws counts consecutive turns on which there was nothing at all
	// to draw. When every player in turn has drawn nothing, nobody can move
	// and the deal ends on the fewest cards — see applyDraw.
	BlankDraws int `json:"blankDraws,omitempty"`

	// OpenDiscard is whether this table publishes the whole discard pile
	// rather than its top card. Resolved once at NewMatch.
	OpenDiscard bool `json:"openDiscard,omitempty"`

	// --- the call -----------------------------------------------------------

	// CallOn is whether this table plays the "Last card!" call. Resolved
	// once at NewMatch.
	CallOn bool `json:"callOn,omitempty"`
	// Called is the player on turn who has said "Last card!" this turn,
	// ahead of the play that leaves them one card. Cleared when the turn
	// passes.
	Called string `json:"called,omitempty"`
	// Unannounced is a player who went down to one card without calling.
	// Whoever moves next may catch them, until that player does anything
	// else.
	Unannounced string `json:"unannounced,omitempty"`

	// --- the Wild Draw Four challenge --------------------------------------

	// ChallengeOn is whether a Wild Draw Four may be challenged — and so
	// played as a bluff. Resolved once at NewMatch.
	ChallengeOn bool `json:"challengeOn,omitempty"`
	// DrawFour is a Wild Draw Four waiting on its victim's answer.
	DrawFour *DrawFourPending `json:"drawFour,omitempty"`
	// Reveal is a hand shown to a challenger, kept until that player next
	// acts.
	Reveal *Reveal `json:"reveal,omitempty"`

	// --- house rules ----------------------------------------------------------

	// Stacking is how draw cards may be passed on: stackOff, stackTwos (a
	// Draw Two on a Draw Two) or stackAny (any draw card on a Draw Two, a Wild
	// Draw Four on a Wild Draw Four too). Resolved once at NewMatch.
	Stacking int `json:"stacking,omitempty"`
	// PendingDraw is the stack waiting on the player to move: they add to it
	// or take it all. Only ever non-zero where Stacking is on.
	PendingDraw int `json:"pendingDraw,omitempty"`
	// DrawUntil keeps drawing until a playable card turns up.
	DrawUntil bool `json:"drawUntil,omitempty"`
	// SevenZero makes a 7 swap hands with the shortest hand and a 0 pass
	// every hand on.
	SevenZero bool `json:"sevenZero,omitempty"`

	// --- the match across deals ----------------------------------------------

	// TargetScore ends the match when a player's total reaches it. Zero is
	// a single deal.
	TargetScore int `json:"targetScore,omitempty"`
	// Scoring is how points are kept across deals: scoreWinnerTakes or
	// scoreLowest. Resolved once at NewMatch.
	Scoring int `json:"scoring,omitempty"`
	// HandSize is how many cards each deal gives each player.
	HandSize int `json:"handSize,omitempty"`
	// DealNumber counts deals from zero.
	DealNumber int            `json:"dealNumber,omitempty"`
	Scores     map[string]int `json:"scores,omitempty"`
	Deals      []DealResult   `json:"deals,omitempty"`
	// Pause stops between deals until every seat says go on.
	Pause        bool                `json:"pause,omitempty"`
	Intermission module.Intermission `json:"intermission,omitempty"`

	WinnerID string `json:"winnerId,omitempty"`
	Seed     int64  `json:"seed"`
	// Reshuffles counts how many times the pile has been recycled, which also
	// varies the reshuffle seed.
	Reshuffles int `json:"reshuffles"`
}

// DrawFourPending is a Wild Draw Four played and not yet answered.
type DrawFourPending struct {
	Player string `json:"player"`
	Victim string `json:"victim"`
	// Bluff records whether the player held a card of the colour in play
	// when they played it — what a challenge tests. Never shown to anyone.
	Bluff bool `json:"bluff,omitempty"`
}

// Reveal is a hand shown to one player: the hand a Wild Draw Four was
// challenged on, as it was when the card went down.
type Reveal struct {
	Viewer string   `json:"viewer"`
	Owner  string   `json:"owner"`
	Cards  []string `json:"cards"`
	Bluff  bool     `json:"bluff,omitempty"`
}

// DealResult is one deal, as the score sheet shows it.
type DealResult struct {
	Number int            `json:"number"`
	Winner string         `json:"winner"`
	Points int            `json:"points"`
	Totals map[string]int `json:"totals"`
	// What the points were made of: number cards, action cards and wilds
	// left in the other hands.
	Numbers int `json:"numbers,omitempty"`
	Actions int `json:"actions,omitempty"`
	Wilds   int `json:"wilds,omitempty"`
	// Charged is each player's own leftover cards, where the table scores
	// by penalty (scoreLowest): what that player was charged for the deal.
	Charged map[string]Breakdown `json:"charged,omitempty"`
}

// Breakdown is one hand's leftover points, by kind of card.
type Breakdown struct {
	Numbers int `json:"numbers,omitempty"`
	Actions int `json:"actions,omitempty"`
	Wilds   int `json:"wilds,omitempty"`
}

// Total is the breakdown's sum.
func (b Breakdown) Total() int { return b.Numbers + b.Actions + b.Wilds }

// breakdownOf is what a hand left over is worth, by kind of card.
func breakdownOf(hand []string) Breakdown {
	var b Breakdown
	for _, c := range hand {
		switch {
		case isWild(c):
			b.Wilds += cardPoints(c)
		case cardPoints(c) == pointsAction:
			b.Actions += cardPoints(c)
		default:
			b.Numbers += cardPoints(c)
		}
	}
	return b
}

// Ways of keeping score across deals.
const (
	// scoreWinnerTakes: the deal's winner scores everything left in the
	// other hands; first to the target wins.
	scoreWinnerTakes = 0
	// scoreLowest: every player is charged what is left in their own hand;
	// when a total reaches the target, the lowest total wins.
	scoreLowest = 1
)

// Error codes. Stable keys, rendered by the client's locale bundle.
const (
	ErrNotYourTurn      = "NOT_YOUR_TURN"
	ErrGameNotActive    = "GAME_NOT_ACTIVE"
	ErrCardNotInHand    = "CARD_NOT_IN_HAND"
	ErrCardDoesNotMatch = "LASTCARD_NO_MATCH"
	ErrDrawFourHeld     = "LASTCARD_DRAW_FOUR_HELD"
	ErrOnlyDrawnCard    = "LASTCARD_ONLY_DRAWN_CARD"
	ErrAlreadyDrew      = "LASTCARD_ALREADY_DREW"
	ErrNothingToKeep    = "LASTCARD_NOTHING_TO_KEEP"
	ErrColourRequired   = "LASTCARD_COLOUR_REQUIRED"
	ErrUnknownColour    = "LASTCARD_UNKNOWN_COLOUR"
	ErrAnswerDrawFour   = "LASTCARD_ANSWER_DRAW_FOUR"
	ErrNoDrawFour       = "LASTCARD_NO_DRAW_FOUR"
	ErrNothingToCatch   = "LASTCARD_NOTHING_TO_CATCH"
	ErrCallNotNow       = "LASTCARD_CALL_NOT_NOW"
	ErrAlreadyCalled    = "LASTCARD_ALREADY_CALLED"
	ErrMustAnswerDraw   = "LASTCARD_STACK_OR_TAKE"
	ErrUnknownAction    = "UNKNOWN_ACTION"
	ErrTooFewPlayers    = "TOO_FEW_PLAYERS"
)

// Verbs this module accepts.
const (
	VerbPlay = "play_card"
	VerbDraw = "draw"
	VerbPass = "pass" // keep the card just drawn and end the turn

	VerbCall      = "call"      // "Last card!", before the play that leaves one
	VerbCatch     = "catch"     // catch a player who went to one card silently
	VerbChallenge = "challenge" // dispute a Wild Draw Four
	VerbAccept    = "accept"    // take a Wild Draw Four's four cards
)

// Stacking settings.
const (
	stackOff  = 0
	stackTwos = 1
	stackAny  = 2
)

// stackable is whether card may answer the draw stack waiting on the pile:
// a Draw Two on a Draw Two, and where any draw card stacks, a Wild Draw Four
// on either. Never a Draw Two on a Wild Draw Four — four is not answered by
// two.
func (s *GameState) stackable(card string) bool {
	top := s.top()
	switch s.Stacking {
	case stackTwos:
		return faceOf(card) == faceDrawTwo && !isWild(card) && faceOf(top) == faceDrawTwo
	case stackAny:
		if card == cardWildDrawFour {
			return true
		}
		return faceOf(card) == faceDrawTwo && !isWild(card) && faceOf(top) == faceDrawTwo && !isWild(top)
	}
	return false
}

// Card values at the end of a deal: what the winner scores for each card
// left in another hand.
const (
	pointsAction = 20
	pointsWild   = 50
)

// cardPoints is what one card left in hand is worth to the deal's winner.
func cardPoints(card string) int {
	if isWild(card) {
		return pointsWild
	}
	f := faceOf(card)
	if len(f) == 1 && f[0] >= '0' && f[0] <= '9' {
		return int(f[0] - '0')
	}
	return pointsAction
}

// colourOf is a card's colour, or "" for a wild.
func colourOf(card string) string {
	if i := strings.IndexByte(card, '-'); i > 0 {
		return card[:i]
	}
	return ""
}

// faceOf is what is printed on a card: "0".."9", a symbol, or the wild's own
// code.
func faceOf(card string) string {
	if i := strings.IndexByte(card, '-'); i > 0 {
		return card[i+1:]
	}
	return card
}

func isWild(card string) bool { return card == cardWild || card == cardWildDrawFour }

func (s *GameState) top() string {
	if len(s.DiscardPile) == 0 {
		return ""
	}
	return s.DiscardPile[len(s.DiscardPile)-1]
}

// colourInPlay is the colour a card must match: the colour named by a wild if
// one is on top, otherwise the top card's own.
func (s *GameState) colourInPlay() string {
	if s.DeclaredColour != "" {
		return s.DeclaredColour
	}
	return colourOf(s.top())
}

// matches reports whether card may go on the pile by colour, number or
// symbol — the plain test, before the Wild Draw Four's own condition.
//
// A wild on top is answered by its named colour only. Matching the wild's
// own face would let any Wild answer any Wild whatever colour was just named,
// and the naming would mean nothing — but a wild goes on anything, so that
// case is the wild's own rule, not this one.
func (s *GameState) matches(card string) bool {
	if isWild(card) {
		return true
	}
	top := s.top()
	if colourOf(card) == s.colourInPlay() {
		return true
	}
	return !isWild(top) && faceOf(card) == faceOf(top)
}

// holdsColour reports whether hand has a coloured card of the colour in play,
// which is what forbids a Wild Draw Four.
func (s *GameState) holdsColour(hand []string) bool {
	want := s.colourInPlay()
	for _, c := range hand {
		if colourOf(c) == want {
			return true
		}
	}
	return false
}

// step is the player n seats on from `from` in the direction of play, among
// those dealt into this deal.
func (s *GameState) step(from string, n int) string {
	count := len(s.TurnOrder)
	if count == 0 {
		return ""
	}
	dir := s.Direction
	if dir == 0 {
		dir = 1
	}
	at := -1
	for i, p := range s.TurnOrder {
		if p == from {
			at = i
		}
	}
	if at < 0 {
		return s.inPlay()[0]
	}
	for moved := 0; moved < n; {
		at = ((at+dir)%count + count) % count
		if !s.dealtOut(s.TurnOrder[at]) {
			moved++
		}
	}
	return s.TurnOrder[at]
}

// inPlay is who holds cards this deal: the table less anybody dealt out.
func (s *GameState) inPlay() []string {
	if len(s.DealtOut) == 0 {
		return s.TurnOrder
	}
	out := make([]string, 0, len(s.TurnOrder))
	for _, p := range s.TurnOrder {
		if !s.dealtOut(p) {
			out = append(out, p)
		}
	}
	return out
}

func (s *GameState) dealtOut(p string) bool {
	for _, q := range s.DealtOut {
		if q == p {
			return true
		}
	}
	return false
}

func (s *GameState) nextPlayer(from string) string { return s.step(from, 1) }

func decode(raw module.State) (*GameState, error) {
	var s GameState
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("lastcard: decode state: %w", err)
	}
	return &s, nil
}

func encode(s *GameState) (module.State, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("lastcard: encode state: %w", err)
	}
	return raw, nil
}

func removeCard(hand []string, card string) []string {
	out := make([]string, 0, len(hand))
	dropped := false
	for _, c := range hand {
		if !dropped && c == card {
			dropped = true
			continue
		}
		out = append(out, c)
	}
	return out
}

func hasCard(hand []string, card string) bool {
	for _, c := range hand {
		if c == card {
			return true
		}
	}
	return false
}

func isColour(c string) bool {
	for _, x := range colours {
		if x == c {
			return true
		}
	}
	return false
}

// colourKey is the message key naming a colour. Written out as literals so
// the key manifest (cmd/dump-keys) can find every one of them.
func colourKey(c string) string {
	switch c {
	case "C":
		return "lastcard.colour.C"
	case "T":
		return "lastcard.colour.T"
	case "V":
		return "lastcard.colour.V"
	}
	return "lastcard.colour.A"
}

// colourNameKey is the colour as a word inside a sentence ("Play a coral
// card"), where colourKey is the colour as a label of its own, with its shape
// ("● Coral"). Written out for the same reason colourKey is.
func colourNameKey(c string) string {
	switch c {
	case "C":
		return "lastcard.colourName.C"
	case "T":
		return "lastcard.colourName.T"
	case "V":
		return "lastcard.colourName.V"
	}
	return "lastcard.colourName.A"
}
