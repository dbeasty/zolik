package canasta

import (
	"fmt"

	"zolik/server/internal/module"
)

// Module is the Canasta game module.
type Module struct{}

// New returns the module. Stateless: every method takes the state it works on.
func New() *Module { return &Module{} }

var _ module.GameModule = (*Module)(nil)

// NewMatch deals the first deal of a fresh match.
func (m *Module) NewMatch(cfg module.MatchConfig, players []module.PlayerRef, seed int64) (module.State, error) {
	r := resolveVariation(cfg.Variation)
	if len(players) < 2 || len(players) > r.MaxSeats {
		return nil, module.Error{
			Code:    "WRONG_PLAYER_COUNT",
			Message: fmt.Sprintf("this canasta seats two to %d", r.MaxSeats),
		}
	}

	s := &GameState{
		Status:    "active",
		Variation: cfg.Variation,
		Hands:     map[string][]string{},
		TeamOf:    map[string]int{},
		Seed:      seed,
		// On by default here. A deal's settlement has six components, the swing
		// is routinely in the thousands, and dealNew clears every meld off the
		// table the moment the next deal starts — so not stopping is what makes
		// the score unreadable rather than merely brisk.
		Pause: cfg.PauseBetweenRounds(true),
	}
	for _, p := range players {
		s.Players = append(s.Players, p.ID)
		s.TurnOrder = append(s.TurnOrder, p.ID)
	}

	// Partnerships are seat parity, computed once and stored, so nothing else
	// in the package re-derives who is on whose side. Where the seats cannot be
	// split evenly — three players, five — everyone is their own partnership,
	// which needs no special case anywhere else: a team simply has one member.
	//
	// Heads-up is two sides of one rather than one side of two, which is why the
	// evenness test is not enough on its own.
	teams := seatsToTeams(len(players))
	for i := 0; i < teams; i++ {
		s.Teams = append(s.Teams, Team{ID: i})
	}
	for i, p := range s.TurnOrder {
		team := i % teams
		s.TeamOf[p] = team
		s.Teams[team].Players = append(s.Teams[team].Players, p)
	}

	// The ruleset is stored, not just read: a deal is played under the rules the
	// match was created with, so shipping a change to a variation cannot alter a
	// match already in flight. The three scalars stay on the state beside it
	// because they were there first and a client reads them.
	r.HandSize = cfg.Opt(OptHandSize, r.handSizeFor(len(players)))
	r.TargetScore = cfg.Opt(OptTargetScore, r.TargetScore)
	r.CanastasToGoOut = cfg.Opt(OptCanastasToGoOut, r.CanastasToGoOut)
	s.Rules = &r
	s.HandSize = r.HandSize
	s.TargetScore = r.TargetScore
	s.CanastasToGoOut = r.CanastasToGoOut

	// dealNew leads from Dealer+1, so the dealer is seeded one seat behind a
	// random opening seat rather than always seat 0 (opening seat 1).
	n := len(s.TurnOrder)
	s.Dealer = (module.StartingSeat(seed, n) - 1 + n) % n

	if err := dealNew(s); err != nil {
		return nil, err
	}
	return encode(s)
}

var _ module.Seated = (*Module)(nil)

// Sides is who plays with whom, answered for a table that has not been dealt.
//
// It is the lobby's question — "if we start now, who is my partner?" — and it
// is answered here, from seatsToTeams and the same seat arithmetic NewMatch
// uses, rather than by a client counting to two. A rule with two
// implementations is a rule that will eventually have two answers.
//
// Nil where nobody has a partner: at two, three or five seats every player is
// their own side, and a lobby showing five "teams" of one would be noise.
func (m *Module) Sides(cfg module.MatchConfig, players []module.PlayerRef) [][]string {
	teams := seatsToTeams(len(players))
	if teams == len(players) {
		return nil
	}
	out := make([][]string, teams)
	for i, p := range players {
		out[i%teams] = append(out[i%teams], p.ID)
	}
	return out
}

// seatsToTeams is how a table of this size divides into sides.
//
// Even and four or more: partnerships, half as many sides as seats, partner i
// and i+teams — so four seats are 0+2 against 1+3 and six are 0+3, 1+4, 2+5,
// which puts partners opposite each other at both. Anything else — heads-up,
// three, five — is every seat for itself.
func seatsToTeams(seats int) int {
	if seats >= 4 && seats%2 == 0 {
		return seats / 2
	}
	return seats
}

// dealNew shuffles and deals one deal, leaving the first player to move.
//
// The seed is varied per deal so a match is reproducible from its match seed
// yet does not deal the same cards every deal — the same trick prsi uses for
// its reshuffles, and for the same reason.
func dealNew(s *GameState) error {
	r := s.rules()
	deck := shuffle(buildDeck(r), s.Seed+int64(s.DealNumber)*7919+1)

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

	// Between deals there is no player on turn, so the turn check below cannot
	// stand in for "may you do this" — module.Intermission.Mark does it
	// instead, and refuses a stranger and a second press with codes of its own.
	if s.Break.Open {
		if a.Verb != module.VerbContinue {
			return raw, nil, errCode(ErrGameNotActive)
		}
		if err := s.Break.Mark(s.TurnOrder, playerID); err != nil {
			return raw, nil, err
		}
		var events []module.Event
		if s.Break.Settled(s.TurnOrder) {
			s.Break.Close()
			events = nextDeal(s)
		}
		out, err := encode(s)
		return out, events, err
	}

	if s.Current != playerID {
		return raw, nil, errCode(ErrNotYourTurn)
	}

	// Every verb but the undo itself spends or replaces whatever it touches,
	// so a pile capture stops being undoable the moment anything else
	// happens — cleared here, up front, rather than separately in each of
	// them. Safe because a refusal below returns the caller's own `raw`
	// untouched (see Apply's own doc comment): this clear only survives when
	// the action it guarded actually went through.
	if a.Verb != VerbUndoTakePile {
		s.PileTaken = nil
	}
	// The same window for lay-offs, one verb wider: a second lay-off does not
	// close the first's, it stacks on top of it (see LaidOff), so laying off is
	// the one other verb that leaves the stack standing.
	if a.Verb != VerbLayOff && a.Verb != VerbUndoLayOff {
		s.LaidOff = nil
	}

	var events []module.Event
	switch a.Verb {
	case VerbDraw:
		events, err = applyDraw(s, playerID)
	case VerbTakePile:
		events, err = applyTakePile(s, playerID, a)
	case VerbTakeTop:
		events, err = applyTakeTop(s, playerID, a)
	case VerbLayMeld:
		events, err = applyLayMeld(s, playerID, a)
	case VerbLayOff:
		events, err = applyLayOff(s, playerID, a)
	case VerbDiscard:
		events, err = applyDiscard(s, playerID, a)
	case VerbUndoTakePile:
		events, err = applyUndoTakePile(s, playerID)
	case VerbUndoLayOff:
		events, err = applyUndoLayOff(s, playerID)
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

	// How many is the variation's: Canasta takes one, Samba two. A red three
	// drawn is laid down and replaced rather than counted, so the loop is over
	// cards that reached the hand, not over cards off the stock.
	var drawn []string
	var reds []string
	for len(drawn) < s.rules().DrawCount {
		if len(s.DrawPile) == 0 {
			// The stock ran out mid-draw. The deal simply ends, which is the
			// same answer as running out at the top of a turn — and the cards
			// already drawn stay in the hand to be counted against it.
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
	r := s.rules()
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
	// Samba's pile is frozen against everyone for the whole deal, so a capture
	// there always costs two naturals from hand: the same rule the buried wild
	// imposes here, made permanent rather than made separately.
	frozen := s.Frozen || !t.HasMelded || r.PileAlwaysFrozen

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
	// has: the cheapest way in, since it spends nothing from hand. It needs an
	// unfrozen pile *and* a variation that allows the move at all — Modern
	// American removed it outright, so there the pile always costs two cards
	// out of a hand no matter how the table looks. `openGroup` is what makes
	// the meld have to be incomplete: a closed canasta is not an open group,
	// so it cannot reach up and take anything.
	if !frozen && r.PileMeldCapture {
		if m := t.openGroup(r, rank); m != nil {
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
	r := s.rules()
	t := s.team(playerID)
	top := s.top()
	rank := rankOf(top)

	combined := append([]string{top}, fromHand...)
	existing := t.openGroup(r, rank)
	if existing == nil && t.rankIsFull(r, rank) {
		return false // the only group of this rank is a closed canasta
	}
	if existing != nil {
		combined = append(append([]string(nil), existing.Cards...), combined...)
	}
	if validateMeld(r, combined) != nil {
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
	return laid+reachableValue(r, rest, t) >= r.meldFloor(t.Score)
}

func applyTakePile(s *GameState, playerID string, a module.Action) ([]module.Event, error) {
	r := s.rules()
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
	// Samba's pile is frozen against everyone for the whole deal, so a capture
	// there always costs two naturals from hand: the same rule the buried wild
	// imposes here, made permanent rather than made separately.
	frozen := s.Frozen || !t.HasMelded || r.PileAlwaysFrozen

	// Resolve which of the two captures this is, and refuse with the reason
	// that actually applies rather than a generic one.
	var fromHand []string
	target := a.Target
	if target != "" {
		// Frozen first, then whether the variation has this capture at all.
		// The order is what keeps Samba answering `PILE_FROZEN` — the rule its
		// players were actually told, and the one its rules screen prints —
		// while Modern American, whose pile is not frozen and refuses anyway,
		// gets the reason that is true there.
		if frozen {
			return nil, errCode(ErrPileFrozen)
		}
		if !r.PileMeldCapture {
			return nil, errCode(ErrMeldCaptureNotAllowed)
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
		if m.closed(r) {
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
		existing := t.openGroup(r, rank)
		if existing == nil && t.rankIsFull(r, rank) {
			return nil, errCode(ErrMeldClosed)
		}
		if existing != nil {
			combined = append(append([]string(nil), existing.Cards...), combined...)
		}
		if err := validateMeld(r, combined); err != nil {
			return nil, err
		}
		if !t.HasMelded {
			rest, _ := removeCards(s.Hands[playerID], fromHand)
			laid := s.LaidThisTurn + handValue(append([]string{top}, fromHand...))
			if laid+reachableValue(r, rest, t) < r.meldFloor(t.Score) {
				return nil, errCode(ErrInitialMeldNotMet)
			}
		}
	}

	// Snapshotted before anything moves, so an undo — see PileTaken — can put
	// it all back exactly rather than reconstructing it from what the table
	// looks like afterward.
	pileSnapshot := append([]string(nil), s.DiscardPile...)
	priorHand := append([]string(nil), s.Hands[playerID]...)
	priorFrozen := s.Frozen
	priorHasMelded := t.HasMelded
	priorLaidThisTurn := s.LaidThisTurn
	var meldIDForUndo string
	var meldWasNew bool
	var priorMeldCards []string
	if target != "" {
		_, m := s.findMeld(target)
		meldIDForUndo, priorMeldCards = m.ID, append([]string(nil), m.Cards...)
	} else if existing := t.openGroup(r, rank); existing != nil {
		meldIDForUndo, priorMeldCards = existing.ID, append([]string(nil), existing.Cards...)
	} else {
		// The id is read back from the meld once it exists rather than predicted
		// here: a side's *second* group of a rank is not `t0-K` (see newMeldID),
		// so guessing it would leave the undo pointing at the wrong meld.
		meldWasNew = true
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
	} else if existing := t.openGroup(r, rank); existing != nil {
		existing.Cards = append(existing.Cards, meldCards...)
	} else {
		t.Melds = append(t.Melds, Meld{
			ID: t.newMeldID(meldSet, rank), TeamID: t.ID,
			Kind: meldSet, Rank: rank, Cards: meldCards,
		})
		meldIDForUndo = t.Melds[len(t.Melds)-1].ID
	}

	var redThreesGained []string
	rest := s.DiscardPile[:len(s.DiscardPile)-1]
	for _, c := range rest {
		// The only red three that can be buried here is the deal's opening
		// upcard. It goes to the row like any other, with no replacement:
		// there is no draw to replace.
		if isRedThree(c) {
			t.RedThrees = append(t.RedThrees, c)
			redThreesGained = append(redThreesGained, c)
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

	s.PileTaken = &PileTaken{
		Pile:              pileSnapshot,
		PriorHand:         priorHand,
		MeldID:            meldIDForUndo,
		MeldWasNew:        meldWasNew,
		PriorCards:        priorMeldCards,
		RedThreesGained:   redThreesGained,
		PriorHasMelded:    priorHasMelded,
		PriorLaidThisTurn: priorLaidThisTurn,
		PriorFrozen:       priorFrozen,
	}

	return []module.Event{{Type: "pile_taken", Data: map[string]any{
		"playerId": playerID, "cards": len(rest) + 1, "top": top,
	}}}, nil
}

// --- taking the top card onto a sequence ------------------------------------

// topCardRuns lists the sequences on this side's table that the top card of the
// discard pile would continue.
//
// Samba's second way into the pile, and a genuinely different move from taking
// it: one card comes off, the pile stays where it is, and it replaces the draw
// rather than following it. Only where the variation has sequences at all.
func topCardRuns(s *GameState, playerID string) []string {
	r := s.rules()
	if !r.Sequences {
		return nil
	}
	top := s.top()
	if top == "" || isWild(top) {
		return nil
	}
	// A black three on top blocks the pile, and neither three nor wild can be
	// in a sequence anyway — the index lookup is what says so.
	idx, ok := runIndexOf(top)
	if !ok {
		return nil
	}
	t := s.team(playerID)
	if t == nil {
		return nil
	}

	var out []string
	for i := range t.Melds {
		m := &t.Melds[i]
		if m.kind() != meldRun || m.Suit != suitOf(top) || m.closed(r) {
			continue
		}
		low, high := runSpan(m.Cards)
		if idx == low-1 || idx == high+1 {
			out = append(out, m.ID)
		}
	}
	return out
}

func applyTakeTop(s *GameState, playerID string, a module.Action) ([]module.Event, error) {
	r := s.rules()
	if !r.Sequences {
		return nil, module.Error{Code: ErrUnknownAction, Message: VerbTakeTop}
	}
	if s.Phase != phaseDraw {
		return nil, errCode(ErrWrongPhase)
	}
	if len(s.DiscardPile) == 0 {
		return nil, errCode(ErrPileEmpty)
	}

	t := s.team(playerID)
	owner, m := s.findMeld(a.Target)
	if m == nil {
		return nil, errCode(ErrNoSuchMeld)
	}
	if owner.ID != t.ID {
		return nil, errCode(ErrNotYourMeld)
	}
	if m.kind() != meldRun {
		return nil, errCode(ErrWrongRank)
	}
	if m.closed(r) {
		return nil, errCode(ErrMeldClosed)
	}

	top := s.top()
	grown := sortRun(append(append([]string(nil), m.Cards...), top))
	if err := validateRun(r, grown); err != nil {
		return nil, err
	}

	// This is the one move that takes a card without putting one in the hand —
	// it replaces the draw rather than following it. So a player holding a
	// single card could take it and then have no way to end the turn: the
	// discard would be their last card, which is going out, which their side
	// may not be able to do. Refused here rather than discovered at the discard,
	// because by then there is no way back.
	if err := checkLeavesPlayable(s, t, s.Hands[playerID]); err != nil {
		return nil, err
	}

	// No initial-meld check: this takes nothing out of the hand and only adds
	// to what is on the table, so it cannot make a floor unreachable — which is
	// the dead end checkInitialMeld exists to prevent.
	s.DiscardPile = s.DiscardPile[:len(s.DiscardPile)-1]
	m.Cards = grown
	s.Phase = phaseMeld
	s.LaidThisTurn += cardValue(top)
	noteInitialMeld(s, t)

	return []module.Event{{Type: "top_card_taken", Data: map[string]any{
		"playerId": playerID, "card": top, "meldId": m.ID,
	}}}, nil
}

// applyUndoTakePile reverses the current turn's pile capture — see PileTaken
// for why this is the one move this module lets a player take back, and how
// narrowly it is scoped.
func applyUndoTakePile(s *GameState, playerID string) ([]module.Event, error) {
	pt := s.PileTaken
	if pt == nil {
		return nil, errCode(ErrNothingToUndo)
	}
	t := s.team(playerID)
	m := t.meldByID(pt.MeldID)
	if m == nil {
		return nil, errCode(ErrNothingToUndo)
	}
	if !pt.MeldWasNew && len(m.Cards) <= len(pt.PriorCards) {
		return nil, errCode(ErrNothingToUndo)
	}

	if pt.MeldWasNew {
		t.Melds = t.Melds[:len(t.Melds)-1] // appended last by applyTakePile; nothing since has touched it
	} else {
		m.Cards = append([]string(nil), pt.PriorCards...)
	}

	s.Hands[playerID] = append([]string(nil), pt.PriorHand...)
	if len(pt.RedThreesGained) > 0 {
		t.RedThrees, _ = removeCards(t.RedThrees, pt.RedThreesGained)
	}

	s.DiscardPile = append([]string(nil), pt.Pile...)
	s.Frozen = pt.PriorFrozen
	s.TookPileThisTurn = false
	s.Phase = phaseDraw
	s.LaidThisTurn = pt.PriorLaidThisTurn
	t.HasMelded = pt.PriorHasMelded
	s.PileTaken = nil

	return []module.Event{{Type: "take_pile_undone", Data: map[string]any{
		"playerId": playerID,
	}}}, nil
}

// --- melding ---------------------------------------------------------------

func applyLayMeld(s *GameState, playerID string, a module.Action) ([]module.Event, error) {
	r := s.rules()
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
		if err := validateMeld(r, a.Cards); err != nil {
			return nil, err
		}
	}

	rank, _ := meldRank(a.Cards)
	kind := meldSet
	if !blackThrees {
		kind = meldKindOf(r, a.Cards)
	}
	// A cap on groups of a rank is Canasta's; Samba keeps them separate instead.
	// Sequences have no such cap in either — two runs in one suit are two melds.
	if kind == meldSet && t.rankIsFull(r, rank) {
		return nil, errCode(ErrRankAlreadyMelded)
	}

	rest, _ := removeCards(s.Hands[playerID], a.Cards)
	value := handValue(a.Cards)

	if !blackThrees {
		if err := checkInitialMeld(s, t, value, rest); err != nil {
			return nil, err
		}
	}

	laid := Meld{
		ID: t.newMeldID(kind, rank), TeamID: t.ID, Kind: kind, Rank: rank,
		Cards: append([]string(nil), a.Cards...),
	}
	if kind == meldRun {
		laid.Rank = ""
		laid.Suit = suitOf(a.Cards[0])
		laid.Cards = sortRun(laid.Cards)
	}
	// Provisionally place it, so "can this partnership go out now" is asked of
	// the table as it will actually be — a meld that completes a canasta is
	// what licenses going out on the same action.
	t.Melds = append(t.Melds, laid)
	if err := checkLeavesPlayable(s, t, rest); err != nil {
		t.Melds = t.Melds[:len(t.Melds)-1]
		return nil, err
	}

	s.Hands[playerID] = rest
	s.LaidThisTurn += value
	noteInitialMeld(s, t)

	events := []module.Event{{Type: "meld_laid", Data: map[string]any{
		"playerId": playerID, "meldId": laid.ID, "cards": a.Cards,
	}}}
	if len(rest) == 0 {
		return append(events, endDeal(s, playerID, wasConcealed(s), false)...), nil
	}
	return events, nil
}

func applyLayOff(s *GameState, playerID string, a module.Action) ([]module.Event, error) {
	r := s.rules()
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
	if m.closed(r) {
		return nil, errCode(ErrMeldClosed)
	}
	// What "fits" means depends on the kind. A group takes its own rank and
	// wilds; a sequence takes the cards that continue it, in its suit, and no
	// wild ever. Saying WRONG_RANK to somebody offering the nine of hearts to a
	// heart run would send them to fix a rank that is not the problem.
	if m.kind() == meldRun {
		for _, c := range a.Cards {
			if isWild(c) {
				return nil, errCode(ErrSequenceNoWilds)
			}
			if suitOf(c) != m.Suit {
				return nil, errCode(ErrSequenceNeedsOneSuit)
			}
		}
	} else {
		for _, c := range a.Cards {
			if !isWild(c) && rankOf(c) != m.Rank {
				return nil, errCode(ErrWrongRank)
			}
		}
	}
	grown := append(append([]string(nil), m.Cards...), a.Cards...)
	if m.kind() == meldRun {
		grown = sortRun(grown)
		if err := validateRun(r, grown); err != nil {
			return nil, err
		}
	} else if err := validateMeld(r, grown); err != nil {
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

	// Pushed before the hand moves, so what it holds is the table as it stood
	// rather than as it is about to be — see LaidOff.
	s.LaidOff = append(s.LaidOff, LaidOff{
		MeldID:            m.ID,
		PriorCards:        append([]string(nil), before...),
		Cards:             append([]string(nil), a.Cards...),
		PriorHand:         append([]string(nil), s.Hands[playerID]...),
		PriorLaidThisTurn: s.LaidThisTurn,
		PriorHasMelded:    t.HasMelded,
	})

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

// applyUndoLayOff takes back the most recent lay-off still standing — see
// LaidOff for why the window is exactly this narrow, and why it is a stack.
//
// Nothing here can strand anybody: an undo only ever puts cards back into a
// hand, so the hand grows and checkLeavesPlayable's "keep one to discard" can
// only become easier to satisfy. What it does take away is a canasta the
// lay-off completed — and that matters only at the discard, which asks
// canGoOut for itself.
func applyUndoLayOff(s *GameState, playerID string) ([]module.Event, error) {
	if len(s.LaidOff) == 0 {
		return nil, errCode(ErrNothingToUndo)
	}
	lo := s.LaidOff[len(s.LaidOff)-1]

	t := s.team(playerID)
	m := t.meldByID(lo.MeldID)
	if m == nil {
		return nil, errCode(ErrNothingToUndo)
	}
	// The stack is unwound last-first, so this entry's cards are still the top
	// of the meld and nothing has been laid on them. A meld that is not the
	// size this entry left it is one somebody built on, and putting it back
	// would throw their cards away — refuse instead, and let the turn stand.
	if len(m.Cards) != len(lo.PriorCards)+len(lo.Cards) {
		return nil, errCode(ErrNothingToUndo)
	}

	m.Cards = append([]string(nil), lo.PriorCards...)
	s.Hands[playerID] = append([]string(nil), lo.PriorHand...)
	s.LaidThisTurn = lo.PriorLaidThisTurn
	t.HasMelded = lo.PriorHasMelded
	s.LaidOff = s.LaidOff[:len(s.LaidOff)-1]

	return []module.Event{{Type: "lay_off_undone", Data: map[string]any{
		"playerId": playerID, "meldId": lo.MeldID, "cards": lo.Cards,
	}}}, nil
}

// checkInitialMeld enforces the opening minimum without creating a dead end.
//
// The minimum is a property of a whole turn, so a partnership may reach it with
// two melds — which means a lay that falls short has to be allowed. What is not
// allowed is a lay that puts the minimum out of reach, because there is no way
// to take cards back off the table. See meld.go's reachableValue.
func checkInitialMeld(s *GameState, t *Team, value int, rest []string) error {
	r := s.rules()
	if t.HasMelded {
		return nil
	}
	laid := s.LaidThisTurn + value
	if laid >= r.meldFloor(t.Score) {
		return nil
	}
	if laid+reachableValue(r, rest, t) < r.meldFloor(t.Score) {
		return errCode(ErrInitialMeldNotMet)
	}
	return nil
}

// noteInitialMeld promotes a partnership the moment this turn's total clears
// the floor.
func noteInitialMeld(s *GameState, t *Team) {
	r := s.rules()
	if !t.HasMelded && s.LaidThisTurn >= r.meldFloor(t.Score) {
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

	// Taking the top card onto a sequence is also a move, so a stock of nothing
	// is only a dead deal when that is unavailable too.
	if len(s.DrawPile) == 0 && len(pileTakeOptions(s, next)) == 0 && len(topCardRuns(s, next)) == 0 {
		return endDeal(s, "", false, true)
	}
	return nil
}

// --- ending a deal ---------------------------------------------------------

// endDeal scores the deal and either deals the next one or ends the match.
func endDeal(s *GameState, wentOut string, concealed bool, exhausted bool) []module.Event {
	// A deal ends in the middle of somebody's turn — going out is a lay-off or
	// a meld, not a discard — so the turn's take-backs are closed here rather
	// than left for the next action to clear. A settled deal is not a thing to
	// undo your way back into, and during a pause between deals there is nobody
	// on turn to be offered it.
	s.LaidOff = nil
	s.PileTaken = nil

	res := scoreDeal(s, wentOut, concealed, exhausted)
	s.LastDeal = &res
	s.Deals = append(s.Deals, res)

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

	if s.Pause {
		// Stop here and wait for the table. The board stays exactly as it
		// ended, which is the point: the melds and the caught cards that the
		// settlement is made of are still there to be looked at.
		s.Break.Begin(res.DealNumber + 1)
		s.Current = ""
		s.Phase = ""
		return events
	}
	return append(events, nextDeal(s)...)
}

// nextDeal advances to the deal after the one just scored. It is the tail of
// endDeal, split off so that resuming from an intermission and never pausing at
// all take the same path rather than two that can drift.
func nextDeal(s *GameState) []module.Event {
	s.DealNumber++
	s.Dealer = (s.Dealer + 1) % len(s.TurnOrder)
	if err := dealNew(s); err != nil {
		// The only way this fails is a deck too small for the table, which
		// NewMatch has already ruled out. Ending the match beats looping.
		s.Status = "completed"
		s.WinnerTeam = -1
		s.Current = ""
		return []module.Event{{Type: "match_ended", Data: map[string]any{"error": err.Error()}}}
	}
	return []module.Event{{Type: "deal_started", Data: map[string]any{
		"dealNumber": s.DealNumber,
	}}}
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
