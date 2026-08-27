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
	// Break is the pause between deals. The engine says the table has stopped
	// — it sets its own intermission phase — and the adapter owns the ready-up,
	// because "who has agreed to go on" is protocol vocabulary and not rummy's.
	Break module.Intermission `json:"break,omitempty"`
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
				rules.OptRequireCleanRun:     rules.BoolOpt(cfg.ContractFor(1).RequireCleanRun),
				rules.OptDealStarter:         rules.DealStarterOpt(cfg.DealStarter),
				module.OptPauseBetweenRounds: module.OptOn,
			},
		})
	}
	// Declared here rather than in the rummy descriptor: pausing between deals
	// is a property of how a match is presented, which the runtime owns, and
	// the engine's own option list stays about rules.
	out.Options = append(out.Options, module.PauseOption())
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

// resolveConfig is the one place a lobby's variation+options become a
// rules.RulesConfig. NewMatch deals from it and Rules (rules.go) writes from
// it, so the two cannot disagree about what a setting means.
func resolveConfig(mc module.MatchConfig) rules.RulesConfig {
	// ResolveProfile already falls back to the default ruleset for an empty or
	// unknown name, so an unspecified variation needs no special case here.
	cfg := rules.ResolveProfile(mc.Variation)
	cfg.InitialMeldMinimum = mc.Opt(rules.OptInitialMeldMinimum, cfg.InitialMeldMinimum)
	cfg.DiscardDrawMinRound = mc.Opt(rules.OptDiscardDrawMinRound, cfg.DiscardDrawMinRound)
	// StaticContract is where the clean-run rule lives for every profile,
	// rotating or not — ContractFor reads it back out for both.
	cfg.StaticContract.RequireCleanRun = mc.Opt(
		rules.OptRequireCleanRun, rules.BoolOpt(cfg.StaticContract.RequireCleanRun),
	) == rules.OptOn
	cfg.DealStarter = rules.ParseDealStarterOpt(
		mc.Opt(rules.OptDealStarter, rules.DealStarterOpt(cfg.DealStarter)),
	)
	cfg.PauseBetweenDeals = mc.PauseBetweenRounds(true)
	return cfg
}

// NewMatch deals the opening hand through the engine's own dealer.
func (m *Module) NewMatch(mc module.MatchConfig, players []module.PlayerRef, seed int64) (module.State, error) {
	cfg := resolveConfig(mc)

	order := make([]string, 0, len(players))
	for _, p := range players {
		order = append(order, p.ID)
	}
	starter := order[module.StartingSeat(seed, len(order))]
	gs, err := rules.StartMatch(cfg, order, seed, starter)
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
	// Between deals the engine has no turn to check an actor against, so the
	// intermission does it: it refuses a stranger, and a second press, with
	// codes of its own.
	if s.Break.Open {
		if a.Verb != module.VerbContinue {
			return raw, nil, module.Error{Code: "GAME_NOT_ACTIVE", Message: a.Verb}
		}
		if err := s.Break.Mark(s.Rules.TurnOrder, playerID); err != nil {
			return raw, nil, err
		}
		if s.Break.Settled(s.Rules.TurnOrder) {
			s.Break.Close()
			gs, err := rules.ResumeAfterIntermission(s.Rules)
			if err != nil {
				return raw, nil, err
			}
			s.Rules = gs
		}
		next, err := encode(s)
		return next, nil, err
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
	// The engine stopped between deals; the adapter opens the ready-up.
	if s.Rules.Phase == rules.PhaseIntermission && !s.Break.Open {
		s.Break.Begin(s.Rules.GameNumber + 1)
	}
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
	// Between deals the only thing anybody may do is agree to go on, and the
	// control for it is an ordinary offer — which is what lets the runtime's
	// bot loop ready a seat without knowing intermissions exist.
	if s.Break.Open {
		return s.Break.Offers(s.Rules.TurnOrder, playerID), nil
	}

	src := rules.LegalActions(s.Rules, playerID)
	out := make([]module.ActionOffer, 0, len(src))
	for _, o := range src {
		out = append(out, module.ActionOffer{
			ID: o.ID, Verb: string(o.Verb), Enabled: o.Enabled, WhyNot: string(o.WhyNot),
			LabelKey: o.LabelKey, Facts: toFacts(o.Facts),
			Source: toSelector(o.Source, playerID), Target: toSelector(o.Target, playerID),
			// Laying a meld is the one thing here a person has to compose. The
			// offer lists which cards are eligible but not which *combination*
			// of them is a run or a set, because enumerating rummy shapes is
			// the offer explosion extensibility-plan.md §1.1 refuses. Saying so
			// explicitly is new: a client used to have to infer it.
			Composite: o.Verb == rules.VerbLayMeld,
		})
	}
	// Why each disabled offer is disabled, in terms a player can act on: the
	// rules that justify the refusal, and the move that gets round it. Both
	// read off what was just built rather than being worked out again — see
	// remedy.go.
	annotate(s.Rules, playerID, out)
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

// toFacts carries the engine's control captions across unchanged — they are
// keys and values, so there is nothing to translate.
func toFacts(in []rules.OfferFact) []module.Fact {
	if len(in) == 0 {
		return nil
	}
	out := make([]module.Fact, 0, len(in))
	for _, f := range in {
		out = append(out, module.Fact{LabelKey: f.LabelKey, Value: f.Value})
	}
	return out
}

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
	// The card a pickup obliges this player to lay down, if they are the one
	// who owes it. Marked on the card itself as well as said in a prompt —
	// see badgedCardViews.
	owed := ""
	if gs.CurrentTurn == viewerID {
		owed = gs.DiscardDrawnCardPendingMeld
	}
	vm.Zones = append(vm.Zones, module.Zone{
		ID: handZoneID(viewerID), Kind: module.ZoneHand, OwnerID: viewerID,
		LabelKey: "zone.yourHand", Cards: badgedCardViews(own, owed), Count: len(own),
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
			Active: s.Break.Open && !s.Break.Ready[p] ||
				gs.CurrentTurn == p && gs.Status == rules.StatusActive,
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
	// What this deal asks for, in the one phrasing that is true of it. A
	// single key with sets and runs in it read "Needs 0 sets and 0 runs" on a
	// Žolík Classic table, where the whole requirement is a joker-free run —
	// so the module picks the sentence, rather than a client trying to make
	// one template cover three different demands.
	contract := cfg.ContractFor(gs.GameNumber)
	switch {
	case contract.Sets > 0 || contract.Runs > 0:
		vm.Header = append(vm.Header, module.Fact{
			LabelKey: "header.contract",
			Params: map[string]any{
				"sets": contract.Sets, "runs": contract.Runs, "cleanRun": contract.RequireCleanRun,
			},
		})
	case contract.RequireCleanRun:
		vm.Header = append(vm.Header, module.Fact{LabelKey: "header.contract.cleanRunOnly"})
	}
	if gs.DiscardDrawnCardPendingMeld != "" && gs.CurrentTurn == viewerID {
		vm.Prompts = append(vm.Prompts, module.Fact{
			LabelKey: "prompt.pickupMustBeMelded",
			Value:    gs.DiscardDrawnCardPendingMeld,
		})
	}
	return vm, nil
}

func cardViews(cards []string) []module.CardView {
	return badgedCardViews(cards, "")
}

// badgedCardViews marks `owed`, if it is in this hand: the card a discard-pile
// pickup obliges the player to lay down this turn.
//
// Marked rather than only refused later. The rule is enforced at the discard,
// which is the last possible moment to hear about it — by then the player has
// already decided what their turn was for. On the card, it is an instruction
// while there is still a turn left to act on it.
func badgedCardViews(cards []string, owed string) []module.CardView {
	out := make([]module.CardView, 0, len(cards))
	marked := false
	for _, c := range cards {
		cv := module.CardView{Card: c}
		// Once: two decks put a second copy of every card in play, and only
		// one of them is the one that came off the pile.
		if owed != "" && c == owed && !marked {
			cv.BadgeKeys = []string{"zolik.badge.owedToMeld"}
			marked = true
		}
		out = append(out, cv)
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

// Standings ranks by match penalty total, lowest first — rummy is a game you
// try to score *little* at, which is why the score is negated before ranking
// rather than the ranking growing a direction.
//
// It ranks on the two measures rules.DetermineMatchWinner decides on, and in
// the same order: lowest total, then fewest deals won. That is deliberate and
// load-bearing. This used to rank on handPenalty — the cards a player happened
// to be *holding at the instant the match ended* — which is not the match
// score at all: it is the leftovers of one deal out of seven, and it is what
// the runtime then recorded as the player's score for the whole match. The
// scoreboard and the engine could name different winners, and every lifetime
// figure derived from it (ScoreSum, BestScore, AvgScore, head-to-head) was
// built on the wrong number.
//
// The old figure is still worth seeing, so it is kept as a fact on the row. It
// is simply no longer mistaken for the score.
func (m *Module) Standings(raw module.State) ([]module.Standing, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	gs := s.Rules
	dealsWon := rules.DealsWonByPlayer(gs.TurnOrder, gs.GameScores)

	out := module.RankByScores(gs.TurnOrder, "zolik.unit.penalty",
		func(id string) int { return -gs.TotalScores[id] },
		// Fewer deals won is better here, the same way fewer points is: this is
		// the engine's own tiebreak, not a second opinion about one.
		func(id string) int { return -dealsWon[id] },
	)
	for i := range out {
		id := out[i].PlayerID
		// Score stays negated so the runtime can rank and record it without a
		// sense of direction; Shown is the penalty as rummy writes it, because
		// "-29 Penalty" is the negation showing through to the player.
		shown := gs.TotalScores[id]
		out[i].Shown = &shown
		out[i].Facts = []module.Fact{
			{LabelKey: "zolik.standing.dealsWon", Params: map[string]any{"n": dealsWon[id]}},
			{LabelKey: "zolik.standing.inHand", Params: map[string]any{"n": handPenalty(gs, id)}},
		}
	}
	return out, nil
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
