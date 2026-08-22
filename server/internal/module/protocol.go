// Package module defines the contract a card game implements so the runtime
// can host it without knowing which game it is.
//
// This is Phase 3 of docs/extensibility-plan.md. It exists because
// `rules.GameState`, `rules.Action` and the draw→meld→discard turn model are
// rummy vocabulary baked into the type system: no amount of configuration
// makes them describe a trick, a bid, or a shed.
//
// # Designed against two games, not one
//
// The plan's own warning is that an interface designed against a single game
// fits exactly one game. So this was written with Žolíky and Prší (Czech
// Mau-Mau) side by side, and Prší immediately bent it in one place worth
// naming: an offer needs *parameters that are not cards*. Playing a Queen in
// Prší changes the suit in play, and "which suit" is a choice with no card to
// drag. That is what ParamSpec is for, and nothing in rummy would have
// suggested it.
//
// # What is deliberately generic
//
// Zones holding cards, and groups within zones. That is enough vocabulary for
// a rummy meld, a trick pile, a bidding box and a fanned hand alike. A game
// with no melds simply never emits a meld zone.
//
// # What is deliberately opaque
//
// State. The runtime persists and passes it around; only the module reads it.
// That is what removes the 30-field hand-mapping between the engine and the
// database, and its whole class of forgot-to-map-a-field bug.
package module

import "encoding/json"

// State is a module's own game state, opaque to everything outside it.
//
// JSON rather than `any` on purpose: the runtime has to persist it, ship it
// between processes and hand it back later, and a concrete encoding makes all
// three trivial while keeping the runtime unable to peek inside.
type State = json.RawMessage

// PlayerRef is who is playing, as much as the runtime knows.
type PlayerRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	IsAI bool   `json:"isAI"`
}

// Options are the numeric lobby settings a match starts with, keyed by the
// option names the module's descriptor declares.
type Options map[string]int

// MatchConfig is everything a lobby chose: which variation, and the values of
// the options that variation exposes.
//
// Variation is separate from Options rather than squeezed into it because a
// variation is a named ruleset, not a number — and both games have them. The
// adapter for Žolíky was where this surfaced: it needed a profile name, and
// an int-keyed map had nowhere to put one.
type MatchConfig struct {
	Variation string  `json:"variation,omitempty"`
	Options   Options `json:"options,omitempty"`
}

// Opt reads a numeric option, or returns fallback when it is unset.
func (c MatchConfig) Opt(name string, fallback int) int {
	if v, ok := c.Options[name]; ok {
		return v
	}
	return fallback
}

// Action is one move, in vocabulary no specific game owns.
//
// Verb is the module's own action name. Cards, Target and Params carry
// whatever that verb needs — a rummy lay-off uses Cards plus a Target meld;
// a Prší suit choice uses Params alone.
type Action struct {
	// OfferID is the offer this action exercises, echoed back verbatim.
	//
	// Added because the second game exposed the gap: Žolíky has two distinct
	// draws (from the deck, from the discard pile) that share the verb
	// "draw", so a verb alone cannot say which was chosen. Prší has no such
	// ambiguity and ignores the field. Optional, so a caller that constructs
	// an action directly rather than from an offer still works.
	OfferID string            `json:"offerId,omitempty"`
	Verb    string            `json:"verb"`
	Cards   []string          `json:"cards,omitempty"`
	Target  string            `json:"target,omitempty"`
	Params  map[string]string `json:"params,omitempty"`
}

// Event is something that happened, for clients and the action log.
type Event struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
}

// Error is a refusal, carrying a stable code clients render from a locale
// bundle. The message is for logs and developers, never for players.
type Error struct {
	Code    string
	Message string
}

func (e Error) Error() string {
	if e.Message != "" {
		return e.Code + ": " + e.Message
	}
	return e.Code
}

// CodeOf extracts a module error's code, or a generic one.
func CodeOf(err error) string {
	if err == nil {
		return ""
	}
	if me, ok := err.(Error); ok {
		return me.Code
	}
	return "ERROR"
}

// --- the board -------------------------------------------------------------

// ZoneKind tells a client how to lay a zone out without knowing what it means.
type ZoneKind string

const (
	// ZoneHand: a fanned, owned, usually private set of cards.
	ZoneHand ZoneKind = "hand"
	// ZoneStack: a face-down pile where only the count matters (a draw pile).
	ZoneStack ZoneKind = "stack"
	// ZonePile: a face-up pile where the top card matters (a discard pile).
	ZonePile ZoneKind = "pile"
	// ZoneSpread: cards laid out side by side, in groups (melds, tricks).
	ZoneSpread ZoneKind = "spread"
)

// CardView is one visible card.
type CardView struct {
	Card string `json:"card"`
}

// Group is a run of cards within a zone that belong together — a meld, a
// trick, a staged combination.
type Group struct {
	ID    string   `json:"id"`
	Kind  string   `json:"kind,omitempty"`
	Cards []string `json:"cards"`
	// BadgeKeys are message keys for anything worth marking on the group
	// ("clean run", "trump"). Keys, never rendered text.
	BadgeKeys []string `json:"badgeKeys,omitempty"`
}

// Zone is one area of the board.
//
// Hidden zones send Count instead of Cards — which is where per-viewer
// filtering happens, and why it belongs to the module: only the module knows
// what is secret in *that* game.
type Zone struct {
	ID       string     `json:"id"`
	Kind     ZoneKind   `json:"kind"`
	OwnerID  string     `json:"ownerId,omitempty"`
	LabelKey string     `json:"labelKey,omitempty"`
	Cards    []CardView `json:"cards,omitempty"`
	Count    int        `json:"count"`
	Groups   []Group    `json:"groups,omitempty"`
}

// Fact is a labelled value for a header or scoreboard — pre-resolved by the
// module, rendered by the client, interpreted by neither.
type Fact struct {
	LabelKey string         `json:"labelKey"`
	Value    string         `json:"value,omitempty"`
	Params   map[string]any `json:"params,omitempty"`
}

// ViewModel is the whole board as one viewer sees it.
type ViewModel struct {
	Zones   []Zone `json:"zones"`
	Header  []Fact `json:"header,omitempty"`
	Status  []Fact `json:"status,omitempty"`
	Prompts []Fact `json:"prompts,omitempty"`
}

// --- affordances -----------------------------------------------------------

// SelectorZone names where an offer's cards come from or go to.
type SelectorZone string

const (
	FromHand        SelectorZone = "hand"
	FromDeck        SelectorZone = "deck"
	FromDiscardPile SelectorZone = "discard_pile"
	ToMeld          SelectorZone = "meld"
	ToTable         SelectorZone = "table"
)

// Placement is one card an offer accepts, plus any positional hint (which end
// of a run it may extend). Empty Positions means "send no position".
type Placement struct {
	Card      string   `json:"card"`
	Positions []string `json:"positions,omitempty"`
}

// Selector describes where an offer's cards come from, or where they land.
type Selector struct {
	Zone    SelectorZone `json:"zone"`
	OwnerID string       `json:"ownerId,omitempty"`
	MeldID  string       `json:"meldId,omitempty"`

	Cards      []string    `json:"cards,omitempty"`
	Placements []Placement `json:"placements,omitempty"`

	MinCards int `json:"minCards,omitempty"`
	MaxCards int `json:"maxCards,omitempty"`
}

// ParamSpec declares a non-card input an offer needs.
//
// This is the field Prší added. Playing a Queen changes the suit in play, and
// the player must say which — a choice with no card to drag and no zone to
// drop on. A rummy-only design would never have grown it, which is precisely
// why the second game had to be written before the interface was fixed.
type ParamSpec struct {
	// Name is the key the client sends back in Action.Params.
	Name string `json:"name"`
	// LabelKey names the prompt; Choices are the allowed values, each with
	// its own key. Keys, not sentences — the wording is the client's.
	LabelKey string        `json:"labelKey"`
	Choices  []ParamChoice `json:"choices,omitempty"`
}

type ParamChoice struct {
	Value    string `json:"value"`
	LabelKey string `json:"labelKey"`
}

// ActionOffer is one affordance the interface may present.
//
// The full set is always returned, disabled entries included, each carrying
// the engine's own reason — "greyed out, and here is why" is a UI
// requirement, and an omitted offer is indistinguishable from a client bug.
type ActionOffer struct {
	ID      string `json:"id"`
	Verb    string `json:"verb"`
	Enabled bool   `json:"enabled"`
	// WhyNot is a stable error code, never a sentence.
	WhyNot string `json:"whyNot,omitempty"`

	Source *Selector   `json:"source,omitempty"`
	Target *Selector   `json:"target,omitempty"`
	Params []ParamSpec `json:"params,omitempty"`
}

// FindOffer returns the offer with this ID, or nil.
func FindOffer(offers []ActionOffer, id string) *ActionOffer {
	for i := range offers {
		if offers[i].ID == id {
			return &offers[i]
		}
	}
	return nil
}

// --- the module ------------------------------------------------------------

// GameModule is everything the runtime needs to host a game.
//
// The runtime knows how to persist State, route Actions, fan out ViewModels
// and drive an agent. It knows nothing about melds, tricks, suits or turns.
type GameModule interface {
	// Descriptor identifies the module and declares what a lobby may set.
	Descriptor() ModuleDescriptor

	// NewMatch deals a fresh match. seed makes it reproducible.
	NewMatch(cfg MatchConfig, players []PlayerRef, seed int64) (State, error)

	// Apply validates and applies one action, returning the new state and
	// what happened. Pure: no I/O, no clock beyond what the caller supplies.
	Apply(s State, playerID string, a Action) (State, []Event, error)

	// View is the board as one viewer may see it — the only place hidden
	// information is filtered, because only the module knows what is hidden.
	View(s State, viewerID string) (ViewModel, error)

	// LegalActions is what this player may do right now, complete with
	// disabled entries and their reasons.
	LegalActions(s State, playerID string) ([]ActionOffer, error)

	// Finished reports whether the match is over and who won.
	Finished(s State) (done bool, winnerID string, err error)
}
