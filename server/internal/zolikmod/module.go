// Package zolikmod presents the existing Žolíky rules engine as a game
// module.
//
// It is an adapter, not a rewrite. internal/rules keeps its own vocabulary —
// melds, contracts, phases — and this package translates it into the
// game-agnostic contract in internal/module. That is deliberate: the rummy
// engine is the mature, heavily-tested part of this codebase and rewriting it
// to fit a new interface would have risked exactly the behaviour the interface
// is supposed to preserve.
//
// The adapter's job is also the honest test of the interface. Anywhere the
// translation needs a special case, the interface is leaking rummy or missing
// something a real game needs. There is one such place, and it is recorded on
// Apply: Žolíky has two draws sharing one verb, which is why module.Action
// grew an OfferID.
package zolikmod

import (
	"encoding/json"
	"fmt"

	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// Module adapts internal/rules to module.GameModule.
type Module struct{}

func New() *Module { return &Module{} }

var _ module.GameModule = (*Module)(nil)

// matchState is what gets persisted: the engine's own state, plus the players
// the runtime handed us. rules.GameState carries a turn order but not names,
// and View needs to label opponents.
type matchState struct {
	Rules   rules.GameState    `json:"rules"`
	Players []module.PlayerRef `json:"players"`
}

func decode(raw module.State) (*matchState, error) {
	var s matchState
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("zolik: decode state: %w", err)
	}
	return &s, nil
}

func encode(s *matchState) (module.State, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("zolik: encode state: %w", err)
	}
	return raw, nil
}

// Descriptor translates the rummy descriptor into the shared shape.
func (m *Module) Descriptor() module.ModuleDescriptor {
	d := rules.Descriptor()
	out := module.ModuleDescriptor{
		ID:         d.ID,
		Label:      d.Label,
		MinPlayers: d.MinPlayers,
		MaxPlayers: d.MaxPlayers,
	}
	for _, p := range d.Profiles {
		cfg := rules.ResolveConfig(p.Rules)
		out.Variations = append(out.Variations, module.VariationSpec{
			ID: p.ID, Label: p.Label,
			Defaults: map[string]int{
				rules.OptInitialMeldMinimum:  cfg.InitialMeldMinimum,
				rules.OptDiscardDrawMinRound: cfg.DiscardDrawMinRound,
			},
		})
	}
	for _, o := range d.Options {
		spec := module.OptionSpec{
			Name: o.Name, Type: module.OptionType(o.Type), Label: o.Label, Help: o.Help,
		}
		for _, c := range o.Choices {
			spec.Choices = append(spec.Choices, module.OptionChoice{Value: c.Value, Label: c.Label})
		}
		out.Options = append(out.Options, spec)
	}
	return out
}

// NewMatch deals the opening hand through the engine's own dealer.
func (m *Module) NewMatch(mc module.MatchConfig, players []module.PlayerRef, seed int64) (module.State, error) {
	// ResolveProfile already falls back to the default ruleset for an empty or
	// unknown name, so an unspecified variation needs no special case here.
	cfg := rules.ResolveProfile(mc.Variation)
	cfg.InitialMeldMinimum = mc.Opt(rules.OptInitialMeldMinimum, cfg.InitialMeldMinimum)
	cfg.DiscardDrawMinRound = mc.Opt(rules.OptDiscardDrawMinRound, cfg.DiscardDrawMinRound)

	order := make([]string, 0, len(players))
	for _, p := range players {
		order = append(order, p.ID)
	}
	gs, err := rules.StartMatch(cfg, order, seed)
	if err != nil {
		return nil, err
	}
	return encode(&matchState{Rules: gs, Players: players})
}

// Apply translates a generic action into the engine's vocabulary.
//
// The OfferID is what disambiguates: Žolíky's two draws share the verb "draw"
// and differ only in where the card comes from. That is the one place the
// generic protocol had to grow to fit this game, and it grew a field rather
// than a special case — a caller that constructs actions directly can still
// name the source through Target.
func (m *Module) Apply(raw module.State, playerID string, a module.Action) (module.State, []module.Event, error) {
	s, err := decode(raw)
	if err != nil {
		return raw, nil, err
	}
	act, err := toRulesAction(a)
	if err != nil {
		return raw, nil, err
	}
	// No clone: decode() unmarshalled a fresh matchState, so the engine's
	// in-place mutation cannot reach the caller's bytes. Making state opaque
	// removed the aliasing hazard that BuildGameStateMsg needed a regression
	// test for.
	out, err := rules.ApplyAction(s.Rules, playerID, act)
	if err != nil {
		return raw, nil, module.Error{Code: string(codeOf(err)), Message: err.Error()}
	}
	s.Rules = out.State
	next, err := encode(s)
	if err != nil {
		return raw, nil, err
	}
	events := make([]module.Event, 0, len(out.Events))
	for _, e := range out.Events {
		events = append(events, module.Event{Type: e.Type, Data: e.Data})
	}
	return next, events, nil
}

func toRulesAction(a module.Action) (rules.Action, error) {
	switch a.Verb {
	case string(rules.VerbDraw):
		from := rules.DrawFromDeck
		// The offer ID says which pile; Target is the fallback for a caller
		// that built the action by hand.
		if a.OfferID == rules.OfferDrawDiscard || a.Target == string(rules.ZoneDiscardPile) {
			from = rules.DrawFromDiscard
		}
		card := ""
		if len(a.Cards) > 0 {
			card = a.Cards[0]
		}
		return rules.Action{Type: rules.ActionDrawCard, DrawFrom: from, Card: card}, nil
	case string(rules.VerbLayMeld):
		return rules.Action{Type: rules.ActionLayMeld, Cards: a.Cards}, nil
	case string(rules.VerbLayOff):
		return rules.Action{
			Type: rules.ActionLayOff, MeldID: a.Target, Cards: a.Cards,
			Position: a.Params["position"],
		}, nil
	case string(rules.VerbSwapJoker):
		card := ""
		if len(a.Cards) > 0 {
			card = a.Cards[0]
		}
		return rules.Action{Type: rules.ActionSwapJoker, MeldID: a.Target, Card: card}, nil
	case string(rules.VerbDiscard):
		card := ""
		if len(a.Cards) > 0 {
			card = a.Cards[0]
		}
		return rules.Action{Type: rules.ActionDiscard, Card: card}, nil
	case string(rules.VerbUndo):
		switch a.OfferID {
		case rules.OfferUndoDrawDiscard:
			return rules.Action{Type: rules.ActionUndoDrawDiscard}, nil
		case rules.OfferUndoLayOff:
			return rules.Action{Type: rules.ActionUndoLayOff}, nil
		case rules.OfferUndoLayMeld:
			return rules.Action{Type: rules.ActionUndoLayMeld}, nil
		case rules.OfferUndoTurn:
			return rules.Action{Type: rules.ActionUndoTurn}, nil
		}
		return rules.Action{}, module.Error{Code: "UNKNOWN_ACTION", Message: "undo without an offer id"}
	}
	return rules.Action{}, module.Error{Code: "UNKNOWN_ACTION", Message: a.Verb}
}

func codeOf(err error) rules.RulesErrorCode {
	if re, ok := err.(rules.RulesError); ok {
		return re.Code
	}
	return rules.ErrInvalidMeld
}

// LegalActions forwards the engine's own offer list, retyped.
//
// Nothing is recomputed here: the rummy offers were already built by probing
// the real validators, and this is a field-for-field translation. If it had to
// decide anything, that decision would be a second implementation of a rule.
func (m *Module) LegalActions(raw module.State, playerID string) ([]module.ActionOffer, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	src := rules.LegalActions(s.Rules, playerID)
	out := make([]module.ActionOffer, 0, len(src))
	for _, o := range src {
		out = append(out, module.ActionOffer{
			ID: o.ID, Verb: string(o.Verb), Enabled: o.Enabled, WhyNot: string(o.WhyNot),
			Source: toSelector(o.Source, playerID), Target: toSelector(o.Target, playerID),
			// Laying a meld is the one thing here a person has to compose. The
			// offer lists which cards are eligible but not which *combination*
			// of them is a run or a set, because enumerating rummy shapes is
			// the offer explosion extensibility-plan.md §1.1 refuses. Saying so
			// explicitly is new: a client used to have to infer it.
			Composite: o.Verb == rules.VerbLayMeld,
		})
	}
	return out, nil
}

// The ids View renders zones under. They are named here, next to the mapping
// that hands them out to offers, because an offer that points at a zone id no
// zone has is a drop target a player can never hit — and nothing else would
// notice.
const (
	drawZoneID    = "draw"
	discardZoneID = "discard"
)

func handZoneID(playerID string) string  { return "hand:" + playerID }
func meldsZoneID(playerID string) string { return "melds:" + playerID }

func toSelector(s *rules.Selector, playerID string) *module.Selector {
	if s == nil {
		return nil
	}
	out := &module.Selector{
		Zone: module.SelectorZone(s.Zone), OwnerID: s.OwnerID, MeldID: s.MeldID,
		ZoneID: zoneIDFor(s, playerID),
		Cards:  s.Cards, MinCards: s.MinCards, MaxCards: s.MaxCards,
	}
	for _, p := range s.Placements {
		out.Placements = append(out.Placements, module.Placement{Card: p.Card, Positions: p.Positions})
	}
	return out
}

// zoneIDFor resolves the abstract zone an engine selector names into the
// rendered zone a person can actually drop a card on.
//
// The engine says "discard_pile" because that is what the rule is about; the
// board has a zone called "discard" because that is what is drawn. Translating
// between the two is exactly the sort of thing that belongs in this adapter
// and nowhere else — a client doing it would be guessing at a rule, and doing
// it in the engine would give the rules a view to know about.
func zoneIDFor(s *rules.Selector, playerID string) string {
	switch s.Zone {
	case rules.ZoneHand:
		if s.OwnerID == "" {
			return ""
		}
		return handZoneID(s.OwnerID)
	case rules.ZoneDeck:
		return drawZoneID
	case rules.ZoneDiscardPile:
		return discardZoneID
	case rules.ZoneMeld:
		if s.OwnerID == "" {
			return ""
		}
		return meldsZoneID(s.OwnerID)
	case rules.ZoneTable:
		// A meld is laid down into the layer's own spread, which is why this
		// is the one case that needs to know whose offers these are.
		return meldsZoneID(playerID)
	}
	return ""
}

// View renders the rummy board in generic zones.
//
// This is where the projection that used to live in BuildGameStateMsg belongs:
// hidden-information filtering is a property of the game, so the game decides
// it. The runtime never learns that a hand is secret and a meld is not.
func (m *Module) View(raw module.State, viewerID string) (module.ViewModel, error) {
	s, err := decode(raw)
	if err != nil {
		return module.ViewModel{}, err
	}
	gs := s.Rules
	vm := module.ViewModel{}

	own := gs.Hands[viewerID]
	vm.Zones = append(vm.Zones, module.Zone{
		ID: handZoneID(viewerID), Kind: module.ZoneHand, OwnerID: viewerID,
		LabelKey: "zone.yourHand", Cards: cardViews(own), Count: len(own),
	})
	for _, p := range gs.TurnOrder {
		if p == viewerID {
			continue
		}
		vm.Zones = append(vm.Zones, module.Zone{
			ID: handZoneID(p), Kind: module.ZoneHand, OwnerID: p,
			LabelKey: "zone.opponentHand", Count: len(gs.Hands[p]),
		})
	}

	vm.Zones = append(vm.Zones,
		module.Zone{ID: drawZoneID, Kind: module.ZoneStack, LabelKey: "zone.drawPile", Count: len(gs.DrawPile)},
		module.Zone{
			ID: discardZoneID, Kind: module.ZonePile, LabelKey: "zone.discardPile",
			Cards: cardViews(gs.DiscardPile), Count: len(gs.DiscardPile),
		},
	)

	// Melds are a spread of groups — the vocabulary a trick-taking game would
	// use for tricks, and a climbing game for played combinations.
	for _, p := range gs.TurnOrder {
		melds := gs.Melds[p]
		// An opponent with nothing down gets no empty box, but the viewer
		// always gets theirs: laying a meld targets it, and a target that is
		// not drawn until after the first meld is a place to drop a card that
		// only appears once you no longer need it.
		if len(melds) == 0 && p != viewerID {
			continue
		}
		z := module.Zone{
			ID: meldsZoneID(p), Kind: module.ZoneSpread, OwnerID: p, LabelKey: "zone.melds",
		}
		metas := gs.MeldMeta[p]
		for i, cards := range melds {
			g := module.Group{ID: fmt.Sprintf("meld_%d", i), Cards: cards}
			if i < len(metas) {
				g.ID = metas[i].MeldID
				g.Kind = string(metas[i].Type)
				if metas[i].WildCount == 0 && metas[i].Type == rules.MeldRun {
					g.BadgeKeys = append(g.BadgeKeys, "badge.cleanRun")
				}
			}
			z.Groups = append(z.Groups, g)
			z.Count += len(cards)
		}
		vm.Zones = append(vm.Zones, z)
	}

	// The seats. Žolíky's are the plainest of the four modules — a hand size,
	// a score and whose turn it is — but emitting them is what lets one shell
	// draw a rummy table and a poker table with the same code.
	for _, p := range gs.TurnOrder {
		seat := module.Seat{
			PlayerID: p,
			Active:   gs.CurrentTurn == p && gs.Status == rules.StatusActive,
			Facts: []module.Fact{{
				LabelKey: "seat.cards", Value: fmt.Sprint(len(gs.Hands[p])),
				Params: map[string]any{"n": len(gs.Hands[p])},
			}},
		}
		// Whether this player has met the deal's contract is the one piece of
		// per-seat state a rummy table shows that a hand count does not, and
		// it is public.
		if gs.RoundReqMet[p] {
			seat.LabelKeys = append(seat.LabelKeys, "zolik.seat.contractMet")
		}
		vm.Seats = append(vm.Seats, seat)
	}

	cfg := rules.ResolveConfig(gs.Rules)
	vm.Header = []module.Fact{
		{LabelKey: "header.deal", Params: map[string]any{"n": gs.GameNumber}},
		{LabelKey: "header.round", Params: map[string]any{"n": gs.Round}},
		{LabelKey: "header.deck", Value: fmt.Sprint(len(gs.DrawPile))},
	}
	contract := cfg.ContractFor(gs.GameNumber)
	vm.Header = append(vm.Header, module.Fact{
		LabelKey: "header.contract",
		Params: map[string]any{
			"sets": contract.Sets, "runs": contract.Runs, "cleanRun": contract.RequireCleanRun,
		},
	})
	if gs.DiscardDrawnCardPendingMeld != "" && gs.CurrentTurn == viewerID {
		vm.Prompts = append(vm.Prompts, module.Fact{
			LabelKey: "prompt.pickupMustBeMelded",
			Value:    gs.DiscardDrawnCardPendingMeld,
		})
	}
	return vm, nil
}

func cardViews(cards []string) []module.CardView {
	out := make([]module.CardView, 0, len(cards))
	for _, c := range cards {
		out = append(out, module.CardView{Card: c})
	}
	return out
}

// Finished reports whether the match is over.
//
// Žolíky always has exactly one winner; the list is the shared shape, not a
// hint that rummy might one day have two.
func (m *Module) Finished(raw module.State) (bool, []string, error) {
	s, err := decode(raw)
	if err != nil {
		return false, nil, err
	}
	done := s.Rules.Status == rules.StatusCompleted
	if !done || s.Rules.WinnerID == "" {
		return done, nil, nil
	}
	return true, []string{s.Rules.WinnerID}, nil
}

// Standings ranks by penalty score, lowest first — rummy is a game you try to
// score *little* at, which is why the score is negated before ranking rather
// than the ranking growing a direction.
func (m *Module) Standings(raw module.State) ([]module.Standing, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	return module.RankByScore(s.Rules.TurnOrder, func(id string) int {
		return -handPenalty(s.Rules, id)
	}, "zolik.unit.penalty"), nil
}

// handPenalty is what this player is currently holding, in points. It is the
// live version of what the scoreboard settles at the end of a deal.
func handPenalty(gs rules.GameState, playerID string) int {
	total := 0
	for _, c := range gs.Hands[playerID] {
		// Aces score as wild here: a card left in hand is a penalty, and an
		// ace only counts as one when it is doing work inside a run.
		total += rules.PenaltyPoints(c, false)
	}
	return total
}

// StateFromRules wraps an existing rummy state as module state.
//
// The one entry point for putting a game the module did not deal into the
// module's hands — used by the `games` → `matches` migration, which has a
// rummy state read out of a legacy document and needs it in the shape the
// runtime persists.
//
// It exists here rather than in the migration so the matchState shape stays
// this package's business: a migration that constructed the JSON itself would
// be a second definition of this module's state, and would drift the first
// time a field is added.
func StateFromRules(gs rules.GameState, players []models.Player) (module.State, error) {
	refs := make([]module.PlayerRef, 0, len(players))
	for _, p := range players {
		refs = append(refs, module.PlayerRef{ID: p.ID, Name: p.Name, IsAI: p.IsAI})
	}
	return encode(&matchState{Rules: gs, Players: refs})
}

// RulesStateOf reads the rummy state back out of module state.
//
// The inverse of StateFromRules, for the migration's equivalence test: the one
// legitimate way to look inside this module's state from outside it, so a test
// can compare a migrated game against the document it came from without
// hand-parsing the JSON and inventing a third definition of the shape.
func RulesStateOf(raw module.State) (rules.GameState, error) {
	s, err := decode(raw)
	if err != nil {
		return rules.GameState{}, err
	}
	return s.Rules, nil
}
