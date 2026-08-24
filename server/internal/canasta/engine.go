package canasta

import (
	"zolik/server/internal/module"
)

// Module is the Canasta game module.
type Module struct{}

// New returns the module. Stateless: every method takes the state it works on.
func New() *Module { return &Module{} }

var _ module.GameModule = (*Module)(nil)

// NewMatch deals the first deal of a fresh match.
func (m *Module) NewMatch(cfg module.MatchConfig, players []module.PlayerRef, seed int64) (module.State, error) {
	if len(players) < 2 || len(players) > 4 {
		return nil, module.Error{Code: "WRONG_PLAYER_COUNT", Message: "canasta seats two to four"}
	}

	s := &GameState{
		Status:    "active",
		Variation: cfg.Variation,
		Hands:     map[string][]string{},
		TeamOf:    map[string]int{},
		Seed:      seed,
	}
	for _, p := range players {
		s.Players = append(s.Players, p.ID)
		s.TurnOrder = append(s.TurnOrder, p.ID)
	}

	// Partnerships are seat parity, computed once and stored, so nothing else
	// in the package re-derives who is on whose side. At two or three players
	// everyone is their own partnership, which needs no special case anywhere
	// else — a team simply has one member.
	teams := 2
	if len(players) == 3 {
		teams = 3
	}
	for i := 0; i < teams; i++ {
		s.Teams = append(s.Teams, Team{ID: i})
	}
	for i, p := range s.TurnOrder {
		team := i % teams
		s.TeamOf[p] = team
		s.Teams[team].Players = append(s.Teams[team].Players, p)
	}

	v := resolveVariation(cfg)
	s.HandSize = cfg.Opt(OptHandSize, v.handSize)
	s.TargetScore = cfg.Opt(OptTargetScore, v.targetScore)
	s.CanastasToGoOut = cfg.Opt(OptCanastasToGoOut, v.canastasToGoOut)

	if err := dealNew(s); err != nil {
		return nil, err
	}
	return encode(s)
}

// dealNew shuffles and deals one deal, leaving the first player to move.
//
// The seed is varied per deal so a match is reproducible from its match seed
// yet does not deal the same cards every deal — the same trick prsi uses for
// its reshuffles, and for the same reason.
func dealNew(s *GameState) error {
	deck := shuffle(buildDeck(), s.Seed+int64(s.DealNumber)*7919+1)

	for i := range s.Teams {
		s.Teams[i].Melds = nil
		s.Teams[i].RedThrees = nil
		s.Teams[i].HasMelded = false
	}
	s.Hands = map[string][]string{}
	s.Frozen = false
	s.LaidThisTurn = 0
	s.TookPileThisTurn = false
	s.DiscardPile = nil

	for i := 0; i < s.HandSize; i++ {
		for _, p := range s.TurnOrder {
			if len(deck) == 0 {
				return module.Error{Code: ErrNothingToDraw, Message: "deck exhausted during the deal"}
			}
			s.Hands[p] = append(s.Hands[p], deck[len(deck)-1])
			deck = deck[:len(deck)-1]
		}
	}
	s.DrawPile = deck

	// Red threes dealt into a hand never stay there: they go face up in front
	// of the partnership and are replaced, repeatedly, because a replacement
	// can itself be a red three.
	for _, p := range s.TurnOrder {
		for {
			moved := false
			for _, c := range s.Hands[p] {
				if !isRedThree(c) {
					continue
				}
				s.Hands[p], _ = removeCards(s.Hands[p], []string{c})
				s.team(p).RedThrees = append(s.team(p).RedThrees, c)
				if len(s.DrawPile) > 0 {
					s.Hands[p] = append(s.Hands[p], s.DrawPile[len(s.DrawPile)-1])
					s.DrawPile = s.DrawPile[:len(s.DrawPile)-1]
				}
				moved = true
				break
			}
			if !moved {
				break
			}
		}
	}

	// The upcard. A wild or a red three turned up here freezes the pile for
	// the whole deal — the pile starts hard to take, and the card that made it
	// so stays visible under everything discarded onto it.
	if len(s.DrawPile) > 0 {
		up := s.DrawPile[len(s.DrawPile)-1]
		s.DrawPile = s.DrawPile[:len(s.DrawPile)-1]
		s.DiscardPile = []string{up}
		if isWild(up) || isRedThree(up) {
			s.Frozen = true
		}
	}

	s.Current = s.TurnOrder[(s.Dealer+1)%len(s.TurnOrder)]
	s.Phase = phaseDraw
	s.MeldsAtTurnStart = false
	return nil
}

// Apply validates and applies one move.
//
// The state is decoded fresh on every call, so a refused action returns the
// caller's own bytes untouched and a half-applied mutation cannot escape. That
// property is free behind an opaque State, and is the reason this engine needs
// no equivalent of the rummy engine's do-not-mutate regression test.
func (m *Module) Apply(raw module.State, playerID string, a module.Action) (module.State, []module.Event, error) {
	s, err := decode(raw)
	if err != nil {
		return raw, nil, err
	}
	if s.Status != "active" {
		return raw, nil, errCode(ErrGameNotActive)
	}
	if s.Current != playerID {
		return raw, nil, errCode(ErrNotYourTurn)
	}

	var events []module.Event
	switch a.Verb {
	case VerbDraw:
		events, err = applyDraw(s, playerID)
	case VerbTakePile:
		events, err = applyTakePile(s, playerID, a)
	case VerbLayMeld:
		events, err = applyLayMeld(s, playerID, a)
	case VerbLayOff:
		events, err = applyLayOff(s, playerID, a)
	case VerbDiscard:
		events, err = applyDiscard(s, playerID, a)
	default:
		err = module.Error{Code: ErrUnknownAction, Message: a.Verb}
	}
	if err != nil {
		return raw, nil, err
	}
	out, err := encode(s)
	return out, events, err
}

// --- draw ------------------------------------------------------------------

func applyDraw(s *GameState, playerID string) ([]module.Event, error) {
	if s.Phase != phaseDraw {
		return nil, errCode(ErrWrongPhase)
	}
	if len(s.DrawPile) == 0 {
		return nil, errCode(ErrNothingToDraw)
	}

	var drawn []string
	var reds []string
	for {
		if len(s.DrawPile) == 0 {
			// The stock ran out mid-replacement. The deal simply ends, which
			// is the same answer as running out at the top of a turn.
			return endDeal(s, "", false, true), nil
		}
		card := s.DrawPile[len(s.DrawPile)-1]
		s.DrawPile = s.DrawPile[:len(s.DrawPile)-1]
		if isRedThree(card) {
			s.team(playerID).RedThrees = append(s.team(playerID).RedThrees, card)
			reds = append(reds, card)
			continue // and draw a replacement
		}
		s.Hands[playerID] = append(s.Hands[playerID], card)
		drawn = append(drawn, card)
		break
	}

	s.Phase = phaseMeld
	ev := []module.Event{{Type: "cards_drawn", Data: map[string]any{
		"playerId": playerID, "count": len(drawn),
	}}}
	if len(reds) > 0 {
		ev = append(ev, module.Event{Type: "red_threes_laid", Data: map[string]any{
			"playerId": playerID, "cards": reds,
		}})
	}
	return ev, nil
}

// --- taking the pile -------------------------------------------------------

// pileOption is one concrete, legal way to take the discard pile.
//
// Enumerated rather than described, for the same reason meld candidates are:
// there are at most a handful, so a client can be handed the exact cards
// instead of a rule to re-derive.
type pileOption struct {
	// MeldID is set when the top card is laid off onto an existing meld;
	// otherwise Cards are the hand cards that receive it.
	MeldID string
	Cards  []string
}

// pileTakeOptions lists every legal capture of the pile for this player.
//
// It is the single implementation of the pile's rules, used by `Apply`, by the
// offer list, and by the stock-exhaustion check — so those three cannot come
// to different conclusions about whether a pile is takeable.
func pileTakeOptions(s *GameState, playerID string) []pileOption {
	top := s.top()
	if top == "" {
		return nil
	}
	// A wild or a black three on top cannot be melded, so the pile under it
	// cannot be reached. The black three's block lasts exactly as long as it
	// is the top card, which is one turn.
	if isWild(top) || isBlackThree(top) || isRedThree(top) {
		return nil
	}

	t := s.team(playerID)
	if t == nil {
		return nil
	}
	hand := s.Hands[playerID]
	rank := rankOf(top)

	// A partnership that has not opened is frozen out of the easy captures
	// even when the pile itself is not frozen — the "personal freeze".
	frozen := s.Frozen || !t.HasMelded

	naturals := make([]string, 0, len(hand))
	var wilds []string
	for _, c := range hand {
		switch {
		case isWild(c):
			wilds = append(wilds, c)
		case rankOf(c) == rank:
			naturals = append(naturals, c)
		}
	}

	var out []pileOption

	// Capture by laying the top card off onto a meld the partnership already
	// has. Only available while the pile is unfrozen.
	if !frozen {
		if m := t.meld(rank); m != nil && !m.closed() {
			out = append(out, pileOption{MeldID: m.ID})
		}
	}

	// Capture by melding the top card with cards from hand. Two naturals
	// always work; a natural and a wild work only while the pile is unfrozen,
	// which is exactly what freezing means.
	if len(naturals) >= 2 {
		cards := append([]string(nil), naturals[:2]...)
		if capturePlayable(s, playerID, cards) {
			out = append(out, pileOption{Cards: cards})
		}
	}
	if !frozen && len(naturals) >= 1 && len(wilds) >= 1 {
		cards := []string{naturals[0], wilds[0]}
		if capturePlayable(s, playerID, cards) {
			out = append(out, pileOption{Cards: cards})
		}
	}
	return out
}

// capturePlayable checks the parts of a capture that are not about the pile:
// that the resulting meld is legal, and that a partnership still opening can
// actually reach its minimum from the top card and its hand.
func capturePlayable(s *GameState, playerID string, fromHand []string) bool {
	t := s.team(playerID)
	top := s.top()
	rank := rankOf(top)

	combined := append([]string{top}, fromHand...)
	if existing := t.meld(rank); existing != nil {
		if existing.closed() {
			return false
		}
		combined = append(append([]string(nil), existing.Cards...), combined...)
	}
	if validateMeld(combined) != nil {
		return false
	}
	if t.HasMelded {
		return true
	}
	// Opening off the pile: only the top card and the hand count toward the
	// minimum — never the cards buried under it, which are not yours yet.
	rest, ok := removeCards(s.Hands[playerID], fromHand)
	if !ok {
		return false
	}
	laid := s.LaidThisTurn + handValue(append([]string{top}, fromHand...))
	return laid+reachableValue(rest, t) >= initialMeldMinimum(t.Score)
}

func applyTakePile(s *GameState, playerID string, a module.Action) ([]module.Event, error) {
	if s.Phase != phaseDraw {
		return nil, errCode(ErrWrongPhase)
	}
	if len(s.DiscardPile) == 0 {
		return nil, errCode(ErrPileEmpty)
	}
	top := s.top()
	if isWild(top) || isRedThree(top) {
		return nil, errCode(ErrTopCardUnusable)
	}
	if isBlackThree(top) {
		return nil, errCode(ErrPileBlocked)
	}

	t := s.team(playerID)
	rank := rankOf(top)
	frozen := s.Frozen || !t.HasMelded

	// Resolve which of the two captures this is, and refuse with the reason
	// that actually applies rather than a generic one.
	var fromHand []string
	target := a.Target
	if target != "" {
		if frozen {
			return nil, errCode(ErrPileFrozen)
		}
		owner, m := s.findMeld(target)
		if m == nil {
			return nil, errCode(ErrNoSuchMeld)
		}
		if owner.ID != t.ID {
			return nil, errCode(ErrNotYourMeld)
		}
		if m.Rank != rank {
			return nil, errCode(ErrWrongRank)
		}
		if m.closed() {
			return nil, errCode(ErrMeldClosed)
		}
	} else {
		fromHand = a.Cards
		if len(fromHand) < 2 {
			return nil, errCode(ErrMeldTooSmall)
		}
		if !hasCards(s.Hands[playerID], fromHand) {
			return nil, errCode(ErrCardNotInHand)
		}
		if frozen {
			for _, c := range fromHand {
				if isWild(c) || rankOf(c) != rank {
					return nil, errCode(ErrPileFrozen)
				}
			}
		}
		combined := append([]string{top}, fromHand...)
		if existing := t.meld(rank); existing != nil {
			if existing.closed() {
				return nil, errCode(ErrMeldClosed)
			}
			combined = append(append([]string(nil), existing.Cards...), combined...)
		}
		if err := validateMeld(combined); err != nil {
			return nil, err
		}
		if !t.HasMelded {
			rest, _ := removeCards(s.Hands[playerID], fromHand)
			laid := s.LaidThisTurn + handValue(append([]string{top}, fromHand...))
			if laid+reachableValue(rest, t) < initialMeldMinimum(t.Score) {
				return nil, errCode(ErrInitialMeldNotMet)
			}
		}
	}

	// Committed from here. Melded cards leave the hand, the top card joins
	// them, and everything buried under it becomes the taker's problem.
	laidValue := cardValue(top)
	if len(fromHand) > 0 {
		s.Hands[playerID], _ = removeCards(s.Hands[playerID], fromHand)
		laidValue += handValue(fromHand)
	}

	meldCards := append([]string{top}, fromHand...)
	if target != "" {
		_, m := s.findMeld(target)
		m.Cards = append(m.Cards, top)
	} else if existing := t.meld(rank); existing != nil {
		existing.Cards = append(existing.Cards, meldCards...)
	} else {
		t.Melds = append(t.Melds, Meld{
			ID: meldID(t.ID, rank), TeamID: t.ID, Rank: rank, Cards: meldCards,
		})
	}

	rest := s.DiscardPile[:len(s.DiscardPile)-1]
	for _, c := range rest {
		// The only red three that can be buried here is the deal's opening
		// upcard. It goes to the row like any other, with no replacement:
		// there is no draw to replace.
		if isRedThree(c) {
			t.RedThrees = append(t.RedThrees, c)
			continue
		}
		s.Hands[playerID] = append(s.Hands[playerID], c)
	}
	s.DiscardPile = nil
	s.Frozen = false
	s.TookPileThisTurn = true
	s.Phase = phaseMeld

	s.LaidThisTurn += laidValue
	noteInitialMeld(s, t)

	return []module.Event{{Type: "pile_taken", Data: map[string]any{
		"playerId": playerID, "cards": len(rest) + 1, "top": top,
	}}}, nil
}

// --- melding ---------------------------------------------------------------

func applyLayMeld(s *GameState, playerID string, a module.Action) ([]module.Event, error) {
	if s.Phase != phaseMeld {
		return nil, errCode(ErrWrongPhase)
	}
	if !hasCards(s.Hands[playerID], a.Cards) {
		return nil, errCode(ErrCardNotInHand)
	}
	t := s.team(playerID)

	// Black threes are the one meld that is not about points: they may only
	// go down as the move that empties a hand.
	blackThrees := len(a.Cards) > 0 && isBlackThree(a.Cards[0])
	if blackThrees {
		if err := validateBlackThreeMeld(a.Cards); err != nil {
			return nil, err
		}
		rest, _ := removeCards(s.Hands[playerID], a.Cards)
		if len(rest) > 0 || !canGoOut(s, t) {
			return nil, errCode(ErrCannotMeldThree)
		}
	} else {
		if err := validateMeld(a.Cards); err != nil {
			return nil, err
		}
	}

	rank, _ := meldRank(a.Cards)
	if t.meld(rank) != nil {
		return nil, errCode(ErrRankAlreadyMelded)
	}

	rest, _ := removeCards(s.Hands[playerID], a.Cards)
	value := handValue(a.Cards)

	if !blackThrees {
		if err := checkInitialMeld(s, t, value, rest); err != nil {
			return nil, err
		}
	}

	// Provisionally place it, so "can this partnership go out now" is asked of
	// the table as it will actually be — a meld that completes a canasta is
	// what licenses going out on the same action.
	t.Melds = append(t.Melds, Meld{
		ID: meldID(t.ID, rank), TeamID: t.ID, Rank: rank, Cards: append([]string(nil), a.Cards...),
	})
	if err := checkLeavesPlayable(s, t, rest); err != nil {
		t.Melds = t.Melds[:len(t.Melds)-1]
		return nil, err
	}

	s.Hands[playerID] = rest
	s.LaidThisTurn += value
	noteInitialMeld(s, t)

	events := []module.Event{{Type: "meld_laid", Data: map[string]any{
		"playerId": playerID, "meldId": meldID(t.ID, rank), "cards": a.Cards,
	}}}
	if len(rest) == 0 {
		return append(events, endDeal(s, playerID, wasConcealed(s), false)...), nil
	}
	return events, nil
}

func applyLayOff(s *GameState, playerID string, a module.Action) ([]module.Event, error) {
	if s.Phase != phaseMeld {
		return nil, errCode(ErrWrongPhase)
	}
	if len(a.Cards) == 0 {
		return nil, errCode(ErrMeldTooSmall)
	}
	if !hasCards(s.Hands[playerID], a.Cards) {
		return nil, errCode(ErrCardNotInHand)
	}
	t := s.team(playerID)

	// Ownership is decided before anything else about this partnership's own
	// progress: laying off onto an opponent's meld is never legal, and saying
	// "you have not melded yet" there would send a player to fix the wrong
	// thing. A target that exists nowhere is reported as the missing initial
	// meld only when the partnership genuinely has no table to aim at.
	owner, m := s.findMeld(a.Target)
	if m == nil {
		if len(t.Melds) == 0 {
			return nil, errCode(ErrMustMeldFirst)
		}
		return nil, errCode(ErrNoSuchMeld)
	}
	if owner.ID != t.ID {
		return nil, errCode(ErrNotYourMeld)
	}
	if m.closed() {
		return nil, errCode(ErrMeldClosed)
	}
	for _, c := range a.Cards {
		if !isWild(c) && rankOf(c) != m.Rank {
			return nil, errCode(ErrWrongRank)
		}
	}
	grown := append(append([]string(nil), m.Cards...), a.Cards...)
	if err := validateMeld(grown); err != nil {
		return nil, err
	}

	rest, _ := removeCards(s.Hands[playerID], a.Cards)
	value := handValue(a.Cards)
	if err := checkInitialMeld(s, t, value, rest); err != nil {
		return nil, err
	}

	before := m.Cards
	m.Cards = grown
	if err := checkLeavesPlayable(s, t, rest); err != nil {
		m.Cards = before
		return nil, err
	}

	s.Hands[playerID] = rest
	s.LaidThisTurn += value
	noteInitialMeld(s, t)

	events := []module.Event{{Type: "cards_laid_off", Data: map[string]any{
		"playerId": playerID, "meldId": m.ID, "cards": a.Cards,
	}}}
	if len(rest) == 0 {
		return append(events, endDeal(s, playerID, wasConcealed(s), false)...), nil
	}
	return events, nil
}

// checkInitialMeld enforces the opening minimum without creating a dead end.
//
// The minimum is a property of a whole turn, so a partnership may reach it with
// two melds — which means a lay that falls short has to be allowed. What is not
// allowed is a lay that puts the minimum out of reach, because there is no way
// to take cards back off the table. See meld.go's reachableValue.
func checkInitialMeld(s *GameState, t *Team, value int, rest []string) error {
	if t.HasMelded {
		return nil
	}
	laid := s.LaidThisTurn + value
	if laid >= initialMeldMinimum(t.Score) {
		return nil
	}
	if laid+reachableValue(rest, t) < initialMeldMinimum(t.Score) {
		return errCode(ErrInitialMeldNotMet)
	}
	return nil
}

// noteInitialMeld promotes a partnership the moment this turn's total clears
// the floor.
func noteInitialMeld(s *GameState, t *Team) {
	if !t.HasMelded && s.LaidThisTurn >= initialMeldMinimum(t.Score) {
		t.HasMelded = true
	}
}

// checkLeavesPlayable stops a player melding themselves into a corner.
//
// A turn ends with a discard, and discarding your last card is going out. So a
// partnership that cannot go out must be left holding at least two cards: one
// to discard and one to keep. Fewer, and the player would be unable to finish
// their turn at all — the dead end this rule exists to prevent.
func checkLeavesPlayable(s *GameState, t *Team, rest []string) error {
	if len(rest) >= 2 {
		return nil
	}
	if canGoOut(s, t) {
		return nil
	}
	if len(rest) == 0 {
		return errCode(ErrCannotGoOutYet)
	}
	return errCode(ErrMustKeepACard)
}

// canGoOut reports whether the partnership has the canastas its variation
// requires. It is the only gate on going out.
func canGoOut(s *GameState, t *Team) bool {
	return t.canastas() >= s.CanastasToGoOut
}

// wasConcealed reports the 200-point go-out: a hand melded in one turn by a
// partnership that had nothing on the table when the turn began.
func wasConcealed(s *GameState) bool { return !s.MeldsAtTurnStart }

// --- discarding ------------------------------------------------------------

func applyDiscard(s *GameState, playerID string, a module.Action) ([]module.Event, error) {
	if s.Phase != phaseMeld {
		return nil, errCode(ErrWrongPhase)
	}
	if len(a.Cards) != 1 {
		return nil, errCode(ErrMeldTooSmall)
	}
	card := a.Cards[0]
	if !hasCards(s.Hands[playerID], []string{card}) {
		return nil, errCode(ErrCardNotInHand)
	}
	if isRedThree(card) {
		return nil, errCode(ErrCannotDiscardThree)
	}

	t := s.team(playerID)
	// Laid something but not enough: the turn cannot end here, and the offer
	// list will show that melding is still on. This is the "describing the way
	// out of a dead end" property the offer protocol was built for.
	if !t.HasMelded && s.LaidThisTurn > 0 {
		return nil, errCode(ErrInitialMeldNotMet)
	}

	rest, _ := removeCards(s.Hands[playerID], []string{card})
	if len(rest) == 0 && !canGoOut(s, t) {
		return nil, errCode(ErrCannotGoOutYet)
	}

	s.Hands[playerID] = rest
	s.DiscardPile = append(s.DiscardPile, card)
	// A wild discarded onto the pile freezes it against everyone, for the rest
	// of the deal or until somebody manages to take it.
	if isWild(card) {
		s.Frozen = true
	}

	events := []module.Event{{Type: "card_discarded", Data: map[string]any{
		"playerId": playerID, "card": card,
	}}}
	if len(rest) == 0 {
		return append(events, endDeal(s, playerID, wasConcealed(s), false)...), nil
	}
	return append(events, advanceTurn(s)...), nil
}

// advanceTurn hands the turn on and resets everything that is per-turn.
//
// It is also where the deal can end without anybody going out: a player who
// cannot draw and cannot take the pile has no move, so the deal is over.
func advanceTurn(s *GameState) []module.Event {
	next := s.nextPlayer(s.Current)
	s.Current = next
	s.Phase = phaseDraw
	s.LaidThisTurn = 0
	s.TookPileThisTurn = false
	s.MeldsAtTurnStart = len(s.team(next).Melds) > 0

	if len(s.DrawPile) == 0 && len(pileTakeOptions(s, next)) == 0 {
		return endDeal(s, "", false, true)
	}
	return nil
}

// --- ending a deal ---------------------------------------------------------

// endDeal scores the deal and either deals the next one or ends the match.
func endDeal(s *GameState, wentOut string, concealed bool, exhausted bool) []module.Event {
	res := scoreDeal(s, wentOut, concealed, exhausted)
	s.LastDeal = &res

	events := []module.Event{{Type: "deal_ended", Data: map[string]any{
		"dealNumber": res.DealNumber,
		"wentOut":    wentOut,
		"concealed":  concealed,
		"exhausted":  exhausted,
	}}}

	if winner := matchWinner(s); winner >= 0 {
		s.Status = "completed"
		s.WinnerTeam = winner
		s.WinnerID = s.Teams[winner].Players[0]
		s.Current = ""
		s.Phase = ""
		return append(events, module.Event{Type: "match_ended", Data: map[string]any{
			"winnerTeam": winner, "winnerId": s.WinnerID,
		}})
	}

	s.DealNumber++
	s.Dealer = (s.Dealer + 1) % len(s.TurnOrder)
	if err := dealNew(s); err != nil {
		// The only way this fails is a deck too small for the table, which
		// NewMatch has already ruled out. Ending the match beats looping.
		s.Status = "completed"
		s.WinnerTeam = -1
		s.Current = ""
		return append(events, module.Event{Type: "match_ended", Data: map[string]any{"error": err.Error()}})
	}
	return append(events, module.Event{Type: "deal_started", Data: map[string]any{
		"dealNumber": s.DealNumber,
	}})
}

// Finished reports whether the match is over and who won.
//
// The whole winning partnership, now that the interface can say so. This used
// to return the partnership's first seat and note in a comment that
// `winners []string` was the honest shape; Hold'em's split pots made that
// change unavoidable and it landed here first.
func (m *Module) Finished(raw module.State) (bool, []string, error) {
	s, err := decode(raw)
	if err != nil {
		return false, nil, err
	}
	if s.Status != "completed" || s.WinnerTeam < 0 || s.WinnerTeam >= len(s.Teams) {
		return s.Status == "completed", nil, nil
	}
	return true, append([]string(nil), s.Teams[s.WinnerTeam].Players...), nil
}
