package ui

import (
	"fmt"
	"strings"

	"zolik/client-tui/api"
)

// Lookups over the server's legal-action list, plus wording for the facts it
// sends. The invariant that makes this file worth having: it contains no
// rule knowledge — no phase name, no profile name, no "am I allowed to"
// expression. Every question is answered by reading what the server already
// decided (server/internal/rules/offers.go).
//
// If you find yourself adding `state.Phase == "meld"` here, the thing you
// want is a new field on the offer, not a condition in this file — otherwise
// the drift this module exists to end starts over.

// Offer IDs the server guarantees are present. Mirrors rules/offers.go.
const (
	OfferDrawDeck        = "draw:deck"
	OfferDrawDiscard     = "draw:discard"
	OfferLayMeld         = "lay_meld"
	OfferDiscard         = "discard"
	OfferUndoDrawDiscard = "undo:draw_discard"
	OfferUndoLayOff      = "undo:lay_off"
	OfferUndoLayMeld     = "undo:lay_meld"
	OfferUndoTurn        = "undo:turn"
)

func layOffOfferID(meldID string) string    { return "lay_off:" + meldID }
func swapJokerOfferID(meldID string) string { return "swap_joker:" + meldID }

func findOffer(state api.GameState, id string) *api.ActionOffer {
	for i := range state.LegalActions {
		if state.LegalActions[i].ID == id {
			return &state.LegalActions[i]
		}
	}
	return nil
}

// can reports whether the server is currently offering this action.
//
// Absent offer means "no": that only happens against a server predating
// Phase 1, and an inert control is a better failure mode than one that sends
// an action the server rejects.
func can(state api.GameState, id string) bool {
	o := findOffer(state, id)
	return o != nil && o.Enabled
}

// whyNot returns the engine's own reason an action is unavailable, or "" if
// it is available.
func whyNot(state api.GameState, id string) string {
	o := findOffer(state, id)
	if o == nil || o.Enabled {
		return ""
	}
	return o.WhyNot
}

// canLayOffAnywhere reports whether any meld on the table is accepting a
// lay-off right now.
func canLayOffAnywhere(state api.GameState) bool {
	for _, o := range state.LegalActions {
		if o.Verb == "lay_off" && o.Enabled {
			return true
		}
	}
	return false
}

func canLayOffOnto(state api.GameState, meldID string) bool {
	return can(state, layOffOfferID(meldID))
}

func canSwapJokerOn(state api.GameState, meldID string) bool {
	return can(state, swapJokerOfferID(meldID))
}

// eligibleCards lists the cards in hand this offer will accept.
func eligibleCards(state api.GameState, id string) []string {
	o := findOffer(state, id)
	if o == nil || o.Source == nil {
		return nil
	}
	return o.Source.Cards
}

// positionsForCard reports which end(s) of a run this card may extend, as
// the server resolved it. Empty means "send no position hint".
func positionsForCard(state api.GameState, meldID, card string) []string {
	o := findOffer(state, layOffOfferID(meldID))
	if o == nil || o.Source == nil {
		return nil
	}
	for _, p := range o.Source.Placements {
		if p.Card == card {
			return p.Positions
		}
	}
	return nil
}

// reasonMessages gives the server's stable error-code vocabulary its
// wording. The codes are owned by the engine; only the phrasing lives here,
// which is the seam a locale bundle replaces in Phase 2.
var reasonMessages = map[string]string{
	"NOT_YOUR_TURN":           "not your turn",
	"WRONG_PHASE":             "not available right now",
	"MUST_DRAW_FIRST":         "draw a card before melding",
	"GAME_SUSPENDED":          "the game is paused",
	"GAME_NOT_ACTIVE":         "the game is not running",
	"DISCARD_LOCKED":          "discard pickup is locked for now",
	"DISCARD_PILE_EMPTY":      "the discard pile is empty",
	"NO_CARDS_LEFT":           "no cards left to draw",
	"ROUND_REQ_NOT_MET":       "lay your own initial meld first",
	"INCOMPLETE_INITIAL_MELD": "finish your initial meld first",
	"DISCARD_CARD_NOT_MELDED": "the card you picked up must go into your meld",
	"JOKER_DISCARD_FORBIDDEN": "a joker can't be discarded",
	"NOTHING_TO_UNDO":         "nothing to undo",
	"NO_JOKER_IN_MELD":        "no joker in this meld",
	"JOKER_SWAP_MISMATCH":     "that card doesn't take the joker's place",
	"BREAKS_CLEAN_RUN":        "that run has to stay joker-free",
	"WRONG_RUN_END":           "that card extends the other end of the run",
	"INVALID_MELD":            "no card in your hand fits here",
	"CARD_NOT_IN_HAND":        "that card is not in your hand",
}

// reasonText renders an engine error code for a player. An unrecognised code
// falls back rather than leaking a raw SCREAMING_SNAKE token into the UI.
func reasonText(code, fallback string) string {
	if code == "" {
		return fallback
	}
	if msg, ok := reasonMessages[code]; ok {
		return msg
	}
	return fallback
}

// contractLabel describes a contract the *server* resolved, rather than
// looking one up by deal number. The deal-to-contract table used to live in
// this package, duplicating server/internal/rules/profiles.go, so a profile
// with a different rotation silently printed the wrong requirement.
func contractLabel(c api.Contract) string {
	var parts []string
	if c.Sets > 0 {
		parts = append(parts, countLabel(c.Sets, "Set"))
	}
	if c.Runs > 0 {
		parts = append(parts, countLabel(c.Runs, "Run"))
	}
	if len(parts) == 0 {
		if c.RequireCleanRun {
			return "Any mix of sets and runs, one run joker-free"
		}
		return "Any valid meld"
	}
	base := strings.Join(parts, ", ")
	if c.RequireCleanRun {
		return base + " (one run joker-free)"
	}
	return base
}

func countLabel(n int, noun string) string {
	words := []string{"Zero", "One", "Two", "Three", "Four", "Five"}
	word := fmt.Sprint(n)
	if n >= 0 && n < len(words) {
		word = words[n]
	}
	plural := "s"
	if n == 1 {
		plural = ""
	}
	return fmt.Sprintf("%s %s%s", word, strings.ToLower(noun), plural)
}

// availableMovesLine renders what the player may do right now, straight from
// the server's offer list — and, when nothing is available, the engine's own
// reason.
//
// The terminal client has no buttons to grey out, so before the offer list
// existed it simply never told the player why a command would be refused:
// they typed it and read the error afterwards. This is that same
// information, up front, and it costs no rule knowledge on this side.
func availableMovesLine(state api.GameState) string {
	if len(state.LegalActions) == 0 {
		return ""
	}
	labels := []struct {
		id    string
		label string
	}{
		{OfferDrawDeck, "draw"},
		{OfferDrawDiscard, "take discard"},
		{OfferLayMeld, "meld"},
		{OfferDiscard, "discard"},
		{OfferUndoTurn, "undo turn"},
	}

	var available []string
	for _, l := range labels {
		if can(state, l.id) {
			available = append(available, l.label)
		}
	}
	if canLayOffAnywhere(state) {
		available = append(available, "lay off")
	}

	if len(available) > 0 {
		return "MOVES │ " + strings.Join(available, " · ")
	}
	// Nothing is available: say why. The deck-draw offer is present in every
	// phase, so its gate is the most generally informative one to report.
	reason := reasonText(whyNot(state, OfferDrawDeck), "")
	if reason == "" {
		reason = reasonText(whyNot(state, OfferDiscard), "waiting")
	}
	return "MOVES │ none — " + reason
}

// previewLine renders the server's verdict on the current selection: what it
// would be, what it is worth, and whether it clears the initial-meld floor.
//
// This replaces approximateNaturalValue, which was a second copy of the card
// scoring table (server/internal/rules/scoring.go) kept in this client purely
// to render this one line. It disagreed with the server about aces, and the
// only way to notice was to lay the meld and be refused.
func previewLine(p *api.MeldPreview) string {
	if p == nil || len(p.Cards) == 0 {
		return ""
	}
	shape := "not a meld"
	if p.Valid {
		shape = p.Type
		if p.WildCount > 0 {
			shape = fmt.Sprintf("%s, %d wild", shape, p.WildCount)
		}
	} else if reason := reasonText(p.WhyNot, ""); reason != "" {
		shape = reason
	}

	// The floor is measured across the whole table, so showing the selection's
	// value alone next to it invites the wrong subtraction: "30 pts (min 35)"
	// looks 5 short even when 26 are already laid. Spell the sum out whenever
	// there is something already on the table to add.
	value := fmt.Sprintf("%d pts", p.NaturalValue)
	if p.AlreadyLaidValue > 0 {
		value = fmt.Sprintf("%d + %d laid = %d pts", p.NaturalValue, p.AlreadyLaidValue, p.NaturalValue+p.AlreadyLaidValue)
	}
	line := fmt.Sprintf("SELECTION │ %s · %s", shape, value)
	if p.InitialMeldMinimum > 0 {
		flag := "✗"
		if p.MeetsMinimum {
			flag = "✓"
		}
		line += fmt.Sprintf(" (min %d %s)", p.InitialMeldMinimum, flag)
	}
	// Only append a playability reason when the cards *are* a meld. When they
	// are not, the shape half of the line already said so, and repeating it
	// after a dash reads as two separate problems.
	if !p.Playable && p.Valid {
		if reason := reasonText(p.WhyNotPlayable, ""); reason != "" {
			line += " — " + reason
		}
	}
	return line
}
