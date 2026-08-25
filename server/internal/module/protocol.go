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

import (
	"encoding/json"
	"sort"
)

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
	//
	// Cards are ordered bottom to top, so the *last* one is the top card. A
	// module may send the whole pile or only the top; an interface shows the
	// top and lets a player look under it, so sending the rest is a choice
	// about what is public rather than about what is drawn.
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

// Seat is one player as the board shows them.
//
// Added for Hold'em, which is the first game whose board carries numbers that
// are not cards: a stack, the chips you have pushed forward this street,
// whether you have folded or are all in. None of that fits a Zone, because
// none of it is a card.
//
// It earns its place beyond poker, though, which is why it is a first-class
// field rather than a poker escape hatch. Canasta puts partnership standings
// here, Prší the size of a hand, Žolíky whose turn it is. And Active means a
// client no longer has to work out whose turn it is by scanning everyone's
// offers for an enabled one — which worked, and was a strange way to learn
// something the module knew all along.
type Seat struct {
	PlayerID string `json:"playerId"`
	// Active marks the player the module is waiting on.
	Active bool `json:"active,omitempty"`
	// LabelKeys mark states a client should show against the seat
	// ("seat.dealer", "seat.folded", "seat.allIn"). Keys, never text.
	LabelKeys []string `json:"labelKeys,omitempty"`
	// Facts are the module's own numbers for this seat — chips, score,
	// canastas. Pre-resolved by the module, rendered by the client,
	// interpreted by neither.
	Facts []Fact `json:"facts,omitempty"`
}

// ViewModel is the whole board as one viewer sees it.
type ViewModel struct {
	Zones   []Zone `json:"zones"`
	Seats   []Seat `json:"seats,omitempty"`
	Header  []Fact `json:"header,omitempty"`
	Status  []Fact `json:"status,omitempty"`
	Prompts []Fact `json:"prompts,omitempty"`
}

// SeatOf returns the seat for this player, or nil.
func (v ViewModel) SeatOf(playerID string) *Seat {
	for i := range v.Seats {
		if v.Seats[i].PlayerID == playerID {
			return &v.Seats[i]
		}
	}
	return nil
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
//
// Positions are listed in the order the group's own cards are rendered, so a
// client can map "dropped on the left half" to the first entry and "the right
// half" to the last without knowing what either one means. A chosen position
// travels back in Action.Params under the key "position".
type Placement struct {
	Card      string   `json:"card"`
	Positions []string `json:"positions,omitempty"`
}

// PositionParam is the parameter a chosen Placement position is submitted
// under. Named here rather than in any module so that a client can honour a
// placement hint generically, the same way it renders a declared ParamSpec.
const PositionParam = "position"

// Selector describes where an offer's cards come from, or where they land.
type Selector struct {
	// Zone says what kind of place this is, in the abstract: a hand, a deck, a
	// discard pile, a meld, the table.
	Zone    SelectorZone `json:"zone"`
	OwnerID string       `json:"ownerId,omitempty"`
	MeldID  string       `json:"meldId,omitempty"`

	// ZoneID names the *rendered* zone — the same string as the matching
	// Zone.ID in the view — where Zone alone only says what sort of place it
	// is. The two are deliberately not the same string: a game may render two
	// discard piles, and "discard_pile" cannot say which.
	//
	// It exists so an interface can offer a place to drop a card. Which zone a
	// move lands on is a fact about the game, so the module answers it; a
	// client that had to guess from the verb, or from a convention like
	// "discard_pile means the zone called discard", would be deriving a rule.
	//
	// MeldID addresses a group inside a zone and is enough on its own, since
	// groups are rendered under their own ids. Set ZoneID when a drop lands on
	// a whole zone.
	ZoneID string `json:"zoneId,omitempty"`

	Cards      []string    `json:"cards,omitempty"`
	Placements []Placement `json:"placements,omitempty"`

	MinCards int `json:"minCards,omitempty"`
	MaxCards int `json:"maxCards,omitempty"`
}

// ParamKind is the shape of a non-card input.
type ParamKind string

const (
	// ParamKindChoice: one value from a declared list. The original kind, and
	// the zero value, so an offer written before kinds existed still means
	// this.
	ParamKindChoice ParamKind = "choice"
	// ParamKindInt: a whole number inside a declared range.
	ParamKindInt ParamKind = "int"
)

// ParamSpec declares a non-card input an offer needs.
//
// This is the field Prší added. Playing a Queen changes the suit in play, and
// the player must say which — a choice with no card to drag and no zone to
// drop on. A rummy-only design would never have grown it, which is precisely
// why the second game had to be written before the interface was fixed.
//
// Hold'em then bent it a second time, in the way the first bend did not
// anticipate: "raise to how much" is a *number*, drawn from a range the engine
// computes (the minimum legal raise up to the player's whole stack), not one of
// a handful of named options. Enumerating a no-limit betting range is the
// offer-explosion problem in its purest form — a thousand offers for a thousand
// chips — so the range is declared and the engine still validates the concrete
// submission on arrival.
type ParamSpec struct {
	// Name is the key the client sends back in Action.Params.
	Name string `json:"name"`
	// Kind says how to render and validate this input. Empty means
	// ParamKindChoice, so nothing that predates this field changed meaning.
	Kind ParamKind `json:"kind,omitempty"`
	// LabelKey names the prompt; Choices are the allowed values, each with
	// its own key. Keys, not sentences — the wording is the client's.
	LabelKey string        `json:"labelKey"`
	Choices  []ParamChoice `json:"choices,omitempty"`

	// Min, Max and Step describe a ParamKindInt's range, inclusive. Step is the
	// granularity a control should move in; zero means one.
	Min  int `json:"min,omitempty"`
	Max  int `json:"max,omitempty"`
	Step int `json:"step,omitempty"`
	// Default is the value a control should start on — the minimum legal
	// raise, say, rather than an arbitrary end of the range.
	Default int `json:"default,omitempty"`
}

// ParamChoice is one selectable value of a ParamKindChoice.
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

	// LabelKey names this control when the verb cannot.
	//
	// A client labels a control from its verb, which works right up until a
	// module offers two of the same verb at once — and then a player is looking
	// at two buttons that say the same word and do different things. Žolíky
	// draws from the deck and from the discard pile, both "draw"; Canasta offers
	// a meld per rank and a capture per way of capturing.
	//
	// A key, never a sentence, like every other label in this protocol. Empty
	// means "the verb says it well enough", which is true of most offers.
	LabelKey string `json:"labelKey,omitempty"`

	Source *Selector   `json:"source,omitempty"`
	Target *Selector   `json:"target,omitempty"`
	Params []ParamSpec `json:"params,omitempty"`

	// Facts are things worth showing *on the control itself* — what this move
	// costs, what it is worth. Output, not input: Params is what the player
	// supplies, Facts is what the engine already knows.
	//
	// Hold'em is what asked for it. "Call" is a button whose whole meaning is
	// the number on it, and a client that had to work out that the number is
	// the highest bet minus its own would be deriving a rule again — the one
	// thing this protocol exists to stop.
	Facts []Fact `json:"facts,omitempty"`

	// Composite marks an offer whose concrete submission this list does not
	// enumerate: a *combination* a person has to compose, from a set of cards
	// the offer does list.
	//
	// It exists because one game genuinely cannot be enumerated. A Žolíky meld
	// is a run or a set of a shape the offer protocol deliberately refuses to
	// expand (extensibility-plan.md §1.1's offer-explosion note), whereas a
	// Canasta meld is n cards of one rank and ships as exact cards. Both are
	// "lay_meld"; only one can be pressed like a button.
	//
	// Before this field the difference was implicit — a client could infer it
	// from MinCards not matching the card list — and inferring is precisely
	// what this protocol exists to stop clients doing. A shell renders a
	// composite offer as a selection, a bot skips it, and neither has to guess.
	Composite bool `json:"composite,omitempty"`
}

// Standing is one player's place in a match, in a shape no game owns.
//
// Rank is 1-based and ties share a rank, because ties are real: a Canasta
// partnership's two members share first place and a split poker pot leaves two
// seats level. Score is the module's own number — points, chips, deals won —
// and LabelKey says which, so a client can put a unit on it without knowing
// what game it is showing.
type Standing struct {
	PlayerID string `json:"playerId"`
	Rank     int    `json:"rank"`
	Score    int    `json:"score"`
	Won      bool   `json:"won"`
	// LabelKey names what Score measures ("holdem.unit.chips").
	LabelKey string `json:"labelKey,omitempty"`
	// Facts are anything else worth putting in a row of a scoreboard.
	Facts []Fact `json:"facts,omitempty"`
}

// Ranked is implemented by a module that can produce a scoreboard.
//
// Optional, and every module here implements it — but optional on purpose: a
// game where "who is ahead" is not a meaningful question mid-match should be
// able to decline rather than invent a number.
type Ranked interface {
	Standings(s State) ([]Standing, error)
}

// StandingsFor returns a module's scoreboard, or nil if it does not keep one.
func StandingsFor(m GameModule, s State) []Standing {
	r, ok := m.(Ranked)
	if !ok {
		return nil
	}
	out, err := r.Standings(s)
	if err != nil {
		return nil
	}
	return out
}

// RankByScore turns per-player scores into standings, highest first.
//
// Shared because every module that has a score wants exactly this, ties
// included, and four copies of a ranking loop is four chances to handle a tie
// differently.
func RankByScore(order []string, score func(string) int, labelKey string) []Standing {
	out := make([]Standing, 0, len(order))
	for _, id := range order {
		out = append(out, Standing{PlayerID: id, Score: score(id), LabelKey: labelKey})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })

	for i := range out {
		if i > 0 && out[i].Score == out[i-1].Score {
			out[i].Rank = out[i-1].Rank
		} else {
			out[i].Rank = i + 1
		}
		out[i].Won = out[i].Rank == 1
	}
	return out
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
	//
	// Winners is a list because a match can genuinely have more than one. Two
	// games in a row said so: Canasta is won by a *partnership*, and returning
	// one of its two players was the first place this interface was visibly
	// the wrong shape rather than merely unfamiliar. Hold'em then made it
	// unavoidable — a split pot has no single winner in any sense at all.
	//
	// An unfinished match returns nil. A finished one returns at least one id,
	// in no significant order.
	Finished(s State) (done bool, winners []string, err error)
}
