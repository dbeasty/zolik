package canasta

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
)

// Canasta as the learning machinery sees it (internal/learn).
//
// Three translations, and each has one rule it may not break.
//
//	Encode      the position as one seat may know it: its own hand and what
//	            the table shows, never another hand or the order of the stock.
//	            TestEncodeDoesNotPeek pins it.
//	Candidates  the moves on offer, built only from enabled offers, and never
//	            one that can strand the seat. That second rule is what most of
//	            this file is about — see candidateOpenings.
//	Reward      a deal's result, once, at the end of the deal.
//
// The layouts below are a contract with the trainer (ml/), which slices the
// vectors by these offsets. A change to any of them is a new encoder, and a
// network trained against the old one refuses to load against it (NetBot's
// StateDim/CandDim check) rather than playing on misread numbers.
type learnGame struct{}

func init() { learn.Register(learnGame{}) }

var (
	_ learn.Game        = learnGame{}
	_ learn.Equivalence = learnGame{}
	_ learn.Positional  = learnGame{}
)

// position is a state decoded for learn.Positional: the whole GameState when
// a seat's encoding or candidates are asked for, and only the deal results
// when a reward is — each at most once, however many seats ask. The raw
// document stays alongside, because the candidate search plays moves through
// the engine, which takes a document, and never through the decoded state.
type position struct {
	raw   module.State
	full  learn.Memo[*GameState]
	deals learn.Memo[*dealsOnly]
}

func (learnGame) Position(raw module.State) (learn.Position, error) {
	return &position{raw: raw}, nil
}

func positionOf(p learn.Position) (*position, error) {
	if pos, ok := p.(*position); ok && pos != nil {
		return pos, nil
	}
	return nil, fmt.Errorf("canasta: %T is not a Canasta position", p)
}

// state is the decoded GameState, shared by every reader: read it, never
// write it.
func (p *position) state() (*GameState, error) {
	return p.full.Get(func() (*GameState, error) { return decode(p.raw) })
}

func (p *position) dealResults() (*dealsOnly, error) {
	return p.deals.Get(func() (*dealsOnly, error) {
		var d dealsOnly
		if err := json.Unmarshal(p.raw, &d); err != nil {
			return nil, fmt.Errorf("canasta: decode state: %w", err)
		}
		return &d, nil
	})
}

func (learnGame) Name() string              { return "canasta" }
func (learnGame) Module() module.GameModule { return New() }
func (learnGame) Heuristic() module.Bot     { return bot{} }

// Config is the variation as a lobby deals it, unchanged: a bench on shortened
// rules would measure a different game.
//
// The one thing it decides is which variation "no preference" means at a table
// Classic cannot seat. Six seats is Samba's alone (ruleset.MaxSeats), and a
// trainer asking for a six-handed table without naming a variation is asking for
// the only game that has one.
func (learnGame) Config(seats int, variation string) module.MatchConfig {
	if variation == "" {
		variation = "classic"
		if seats > variations["classic"].MaxSeats {
			variation = "samba"
		}
	}
	return module.MatchConfig{Variation: variation}
}

// Outcome is the partnership's score. Partners share it, which is what makes
// the bench's per-side averages mean what they should.
func (learnGame) Outcome(raw module.State, seat string) (float64, error) {
	s, err := decode(raw)
	if err != nil {
		return 0, err
	}
	t, ok := s.TeamOf[seat]
	if !ok || t < 0 || t >= len(s.Teams) {
		return 0, fmt.Errorf("canasta: no team for %q", seat)
	}
	return float64(s.Teams[t].Score), nil
}

// --- what a card is, to a network ------------------------------------------------

// A card's category: which of the fifteen things it can be in this game. The
// eleven natural ranks a group or a sequence is built from (runRanks, four to
// ace, indices 0-10), then the two wilds kept apart — a joker is fifty points
// and a deuce twenty — then the two threes, which are not ranks so much as
// rules.
const (
	catJoker      = 11
	catDeuce      = 12
	catBlackThree = 13
	catRedThree   = 14
	numCats       = 15
)

func cardCat(c string) int {
	switch {
	case rankOf(c) == rankJoker:
		return catJoker
	case rankOf(c) == rankTwo:
		return catDeuce
	case isBlackThree(c):
		return catBlackThree
	case isRedThree(c):
		return catRedThree
	}
	if i, ok := runRankIndex[rankOf(c)]; ok {
		return i
	}
	return -1
}

// The widest table there is: Samba's six seats are three partnerships, and
// three seats of Classic are three sides of one. Narrower tables zero-pad.
const (
	maxTeams  = 3
	maxOthers = 5
)

var learnVariations = []string{"classic", "modern_american", "samba"}

// --- Encode ------------------------------------------------------------------

// The state vector, block by block. Offsets are absolute; ml/ reads them.
//
//	[0,15)    my hand, count per card category (/4)
//	[15,59)   my hand by suit × run rank, H C D S × 4..A (count/2) — what a
//	          sequence is made of; zero in a variation without them
//	[59,61)   my hand size (/15), its card value (/300)
//	[61,316)  three team blocks of teamDim, mine first, then the other sides
//	          in the order they play after me; an absent side is all zero
//	[316,350) the discard pile: size (/30), top card category one-hot with a
//	          sixteenth slot for an empty pile, frozen against everyone,
//	          frozen against me, count per category (/4)
//	[350,365) the other seats in the order they play after me, three each:
//	          present, hand size (/15), is my partner
//	[365,373) stock (/100), deal number (/10), draw phase, meld phase, value
//	          laid this turn (/150), took the pile this turn, my turn, the
//	          table is between deals
//	[373,383) variation one-hot (classic, modern_american, samba), seats
//	          one-hot (2..6), canastas to go out (/2), target (/10000)
//
// A team block (teamDim = 85, offsets within it):
//
//	[0,55)    per group rank 4..A, five each: size (/7), wilds (/3), is a
//	          canasta, has no wild in it, groups of that rank (/2 — Samba
//	          keeps several). Where a side has several groups of a rank the
//	          open one is described, else the largest.
//	[55,75)   per suit H C D S, five each: the longest unfinished sequence's
//	          length (/7), its low and high rank index (/10), sequences in
//	          that suit (/2), sambas finished in it
//	[75,85)   the side exists at this table, red threes (/4), a black-three
//	          meld is down, has opened this deal, score (/target), the opening
//	          minimum at that score (/150), how much of it is still owed this
//	          deal (/150; zero once open), canastas (/canastas to go out), may
//	          go out, what its table is worth with bonuses (/1000)
const (
	offHand       = 0
	offHandSuit   = offHand + numCats
	offHandMisc   = offHandSuit + 4*11
	offTeams      = offHandMisc + 2
	teamGroups    = 0
	teamRuns      = teamGroups + 11*5
	teamMisc      = teamRuns + 4*5
	teamDim       = teamMisc + 10
	offPile       = offTeams + maxTeams*teamDim
	offOthers     = offPile + 1 + (numCats + 1) + 2 + numCats
	offTurn       = offOthers + maxOthers*3
	offRules      = offTurn + 8
	learnStateDim = offRules + 3 + 5 + 2
)

func (learnGame) StateDim() int { return learnStateDim }

func (g learnGame) Encode(raw module.State, seat string) ([]float32, error) {
	p, err := g.Position(raw)
	if err != nil {
		return nil, err
	}
	return g.EncodeFor(p, seat)
}

// EncodeFor is Encode from a decoded position.
func (learnGame) EncodeFor(at learn.Position, seat string) ([]float32, error) {
	pos, err := positionOf(at)
	if err != nil {
		return nil, err
	}
	s, err := pos.state()
	if err != nil {
		return nil, err
	}
	mine := s.team(seat)
	if mine == nil {
		return nil, fmt.Errorf("canasta: no team for %q", seat)
	}
	r := s.rules()
	v := make([]float32, learnStateDim)

	// My hand.
	hand := s.Hands[seat]
	for _, c := range hand {
		if k := cardCat(c); k >= 0 {
			v[offHand+k] += 0.25
		}
		if i, ok := runIndexOf(c); ok {
			if si := suitIndex(suitOf(c)); si >= 0 {
				v[offHandSuit+si*11+i] += 0.5
			}
		}
	}
	v[offHandMisc] = float32(len(hand)) / 15
	v[offHandMisc+1] = float32(handValue(hand)) / 300

	// The sides, mine first.
	for i, t := range teamsFrom(s, seat) {
		encodeTeam(v[offTeams+i*teamDim:offTeams+(i+1)*teamDim], s, r, t, t.ID == mine.ID)
	}

	// The discard pile.
	//
	// Counted whole, although a table dealt with the pile folded — the default,
	// see GameState.OpenDiscard and view.go's shownPile — publishes only its top
	// card. Every card in it went down face up in front of everybody, so what it
	// holds is something a player at this table can know by remembering, and a
	// good one does: what the pile is worth is half of whether to take it.
	// Handing the network that memory rather than making it grow one is a
	// choice about how much it has to learn, not about what it may see.
	p := offPile
	v[p] = float32(len(s.DiscardPile)) / 30
	p++
	if top := s.top(); top == "" {
		v[p+numCats] = 1
	} else if k := cardCat(top); k >= 0 {
		v[p+k] = 1
	}
	p += numCats + 1
	v[p] = b2f(s.Frozen || r.PileAlwaysFrozen)
	v[p+1] = b2f(s.Frozen || r.PileAlwaysFrozen || !mine.HasMelded)
	p += 2
	for _, c := range s.DiscardPile {
		if k := cardCat(c); k >= 0 {
			v[p+k] += 0.25
		}
	}

	// The other seats: how many cards each holds, which is on every screen, and
	// never which.
	for i, id := range othersFrom(s, seat) {
		o := offOthers + i*3
		v[o] = 1
		v[o+1] = float32(len(s.Hands[id])) / 15
		v[o+2] = b2f(s.TeamOf[id] == s.TeamOf[seat])
	}

	// The turn.
	t := offTurn
	v[t] = float32(len(s.DrawPile)) / 100
	v[t+1] = float32(s.DealNumber) / 10
	v[t+2] = b2f(s.Phase == phaseDraw)
	v[t+3] = b2f(s.Phase == phaseMeld)
	v[t+4] = float32(s.LaidThisTurn) / 150
	v[t+5] = b2f(s.TookPileThisTurn)
	v[t+6] = b2f(s.Current == seat)
	v[t+7] = b2f(s.Break.Open)

	// The rules.
	o := offRules
	for i, name := range learnVariations {
		if variationName(s) == name {
			v[o+i] = 1
		}
	}
	o += len(learnVariations)
	if n := len(s.TurnOrder); n >= 2 && n <= 6 {
		v[o+n-2] = 1
	}
	o += 5
	v[o] = float32(s.CanastasToGoOut) / 2
	v[o+1] = float32(s.TargetScore) / 10000
	return v, nil
}

func encodeTeam(v []float32, s *GameState, r ruleset, t *Team, mine bool) {
	for i, rank := range runRanks {
		groups := t.groupsOfRank(rank)
		if len(groups) == 0 {
			continue
		}
		m := t.openGroup(r, rank)
		if m == nil {
			m = groups[0]
			for _, g := range groups {
				if len(g.Cards) > len(m.Cards) {
					m = g
				}
			}
		}
		g := v[teamGroups+i*5:]
		g[0] = float32(len(m.Cards)) / 7
		g[1] = float32(m.wilds()) / 3
		g[2] = b2f(m.isCanasta())
		g[3] = b2f(m.isNatural())
		g[4] = float32(len(groups)) / 2
	}
	for si, suit := range suits {
		var best *Meld
		count, sambas := 0, 0
		for i := range t.Melds {
			m := &t.Melds[i]
			if m.kind() != meldRun || m.Suit != suit {
				continue
			}
			count++
			if m.isCanasta() {
				sambas++
				continue
			}
			if best == nil || len(m.Cards) > len(best.Cards) {
				best = m
			}
		}
		g := v[teamRuns+si*5:]
		if best != nil {
			low, high := runSpan(best.Cards)
			g[0] = float32(len(best.Cards)) / 7
			g[1] = float32(low) / 10
			g[2] = float32(high) / 10
		}
		g[3] = float32(count) / 2
		g[4] = float32(sambas)
	}
	blackThrees := false
	for _, m := range t.Melds {
		if m.Rank == rankThree {
			blackThrees = true
		}
	}
	floor := r.meldFloor(t.Score)
	owed := 0
	if !t.HasMelded {
		owed = floor
		// Only my own side's turn is mine to know about in progress; the
		// laid-this-turn total is public, but it is only ever my side's when I
		// am the one being asked.
		if mine && s.Current != "" && s.TeamOf[s.Current] == t.ID {
			owed -= s.LaidThisTurn
		}
	}
	target := s.TargetScore
	if target <= 0 {
		target = 5000
	}
	quota := s.CanastasToGoOut
	if quota <= 0 {
		quota = 1
	}
	g := v[teamMisc:]
	g[0] = 1
	g[1] = float32(len(t.RedThrees)) / 4
	g[2] = b2f(blackThrees)
	g[3] = b2f(t.HasMelded)
	g[4] = float32(t.Score) / float32(target)
	g[5] = float32(floor) / 150
	g[6] = float32(owed) / 150
	g[7] = float32(t.canastas()) / float32(quota)
	g[8] = b2f(canGoOut(s, t))
	g[9] = float32(meldedValue(r, t)) / 1000
}

// teamsFrom is every side, mine first and the rest in the order they play
// after me — so "the side on my left" is the same block whichever seat this
// is.
func teamsFrom(s *GameState, seat string) []*Team {
	out := []*Team{s.team(seat)}
	seen := map[int]bool{s.TeamOf[seat]: true}
	for _, id := range othersFrom(s, seat) {
		if tid := s.TeamOf[id]; !seen[tid] {
			seen[tid] = true
			out = append(out, s.team(id))
		}
	}
	if len(out) > maxTeams {
		out = out[:maxTeams]
	}
	return out
}

// othersFrom is every other seat, starting with the one who plays next.
func othersFrom(s *GameState, seat string) []string {
	at := 0
	for i, p := range s.TurnOrder {
		if p == seat {
			at = i
		}
	}
	var out []string
	for i := 1; i < len(s.TurnOrder) && len(out) < maxOthers; i++ {
		out = append(out, s.TurnOrder[(at+i)%len(s.TurnOrder)])
	}
	return out
}

func variationName(s *GameState) string {
	if s.Variation == "" {
		return "classic"
	}
	return s.Variation
}

func suitIndex(suit string) int {
	for i, s := range suits {
		if s == suit {
			return i
		}
	}
	return -1
}

func b2f(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

// --- Candidates ----------------------------------------------------------------

// What each candidate's features mean. Offsets within the vector; ml/ reads
// them.
//
//	[0,8)    kind: draw, take the pile, take the top card (Samba), lay a meld,
//	         lay off, discard, open the account (a whole opening, from the meld
//	         phase), forced (between deals, or nothing to choose)
//	[8,23)   the category of the card the move is about — a group's rank, the
//	         card laid off or discarded, the pile's top card
//	23       it is about a sequence
//	24       cards it puts on the table (/10)
//	25       wilds it puts on the table (/4)
//	26       the size of the largest meld it touches, afterwards (/7)
//	27       canastas it completes
//	28       natural canastas it completes
//	29       card points it puts on the table (/100)
//	30       cards in the discard pile it captures (/20)
//	31       it goes out
//	32       it opens the partnership's account
//	33       how many actions it takes (/4)
//	34-37    discards only: the card is wild, is a black three, is of a rank
//	         the next player's side has a group of, freezes the pile
//	38       cards left in my hand afterwards (/15)
//	39       discards only: cards of the same rank still in hand after it (/4)
const (
	kindDraw = iota
	kindTakePile
	kindTakeTop
	kindLayMeld
	kindLayOff
	kindDiscard
	kindOpening
	kindForced
	numKinds

	fCat          = numKinds
	fRun          = fCat + numCats
	fCards        = fRun + 1
	fWilds        = fCards + 1
	fMeldSize     = fWilds + 1
	fCanastas     = fMeldSize + 1
	fNaturals     = fCanastas + 1
	fPoints       = fNaturals + 1
	fPile         = fPoints + 1
	fGoesOut      = fPile + 1
	fOpens        = fGoesOut + 1
	fSteps        = fOpens + 1
	fDiscardWild  = fSteps + 1
	fDiscardBlack = fDiscardWild + 1
	fDiscardFeeds = fDiscardBlack + 1
	fDiscardFreez = fDiscardFeeds + 1
	fHandAfter    = fDiscardFreez + 1
	fSameRank     = fHandAfter + 1
	learnCandDim  = fSameRank + 1
)

func (learnGame) CandDim() int { return learnCandDim }

// maxLayOffs bounds the lay-off candidates at one decision. Collapsed as they
// are (layOffActions), a Classic table never reaches it; a Samba side with a
// dozen melds down and a fat hand could, and past this many the network is
// being asked to tell apart moves that differ in nothing it can use.
const maxLayOffs = 40

// Candidates are the moves this seat may make now.
//
// Undos never: a network, like the hand-written bot, is a function of the
// position, and an undo hands it back the position it just left (see bot.Act).
// What keeps it out of the one position an undo exists to rescue — an opening
// laid part-way and then refused — is that the opening is never offered in
// parts. Before a partnership has opened, every candidate that puts a card on
// the table is a whole opening: the melds and lay-offs that between them reach
// the minimum, applied back to back as one decision (learn.Candidate.Then),
// each found by putting it to the engine before it was offered. A pile capture
// before opening is the same: offered only with an opening that finishes from
// the hand it leaves.
func (g learnGame) Candidates(raw module.State, seat string, offers []module.ActionOffer) ([]learn.Candidate, error) {
	p, err := g.Position(raw)
	if err != nil {
		return nil, err
	}
	return g.CandidatesFor(p, seat, offers)
}

// CandidatesFor is Candidates from a decoded position.
func (learnGame) CandidatesFor(p learn.Position, seat string, offers []module.ActionOffer) ([]learn.Candidate, error) {
	pos, err := positionOf(p)
	if err != nil {
		return nil, err
	}
	s, err := pos.state()
	if err != nil {
		return nil, err
	}
	raw := pos.raw
	offers = withoutUndos(offers)

	// Between deals, or on somebody else's turn, there is at most one thing to
	// do and no reason to ask about it.
	if s.Break.Open || s.Status != "active" || s.Current != seat {
		for _, o := range offers {
			if a, ok := module.SubmissionFor(o); ok {
				return []learn.Candidate{{Action: a, Features: kindOnly(kindForced)}}, nil
			}
		}
		return nil, nil
	}
	t := s.team(seat)
	if t == nil {
		return nil, fmt.Errorf("canasta: no team for %q", seat)
	}
	c := candidateBuilder{m: New(), raw: raw, s: s, seat: seat, t: t}
	if s.Phase == phaseDraw {
		return c.draws(offers), nil
	}
	if !t.HasMelded {
		// Part-way through an opening — which the environment never shows a
		// network, because it applies an opening whole, but a NetBot plays one
		// a step at a time and is asked again here. The only candidates are the
		// ways of finishing it (see learn.Candidate.Then); a discard would be
		// refused, and anything else is a part of some other opening.
		if s.LaidThisTurn > 0 {
			return c.openings(raw, s, nil, kindOpening), nil
		}
		return append(c.openings(raw, s, nil, kindOpening), c.discards(offers)...), nil
	}
	var out []learn.Candidate
	layOffs := 0
	for _, o := range offers {
		if !o.Enabled {
			continue
		}
		switch o.Verb {
		case VerbLayMeld:
			if a, ok := module.SubmissionFor(o); ok {
				out = append(out, c.single(a, kindLayMeld))
			}
		case VerbLayOff:
			for _, a := range layOffActions(t, o) {
				if layOffs == maxLayOffs {
					break
				}
				layOffs++
				out = append(out, c.single(a, kindLayOff))
			}
		}
	}
	return append(out, c.discards(offers)...), nil
}

// layOffActions is one lay-off per card that makes a different move.
//
// A group takes any natural of its rank, and a seven of hearts on the sevens
// is the same move as a seven of spades, so the naturals collapse to one — and
// the wilds to two, a joker and a deuce, which differ in what they cost. A
// sequence is different: each card it would take goes on a different end or
// a different distance out, so each is its own move.
func layOffActions(t *Team, o module.ActionOffer) []module.Action {
	if o.Source == nil || o.Target == nil {
		return nil
	}
	target := t.meldByID(o.Target.MeldID)
	if target == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []module.Action
	for _, card := range sortedUnique(o.Source.Cards) {
		key := card
		if target.kind() != meldRun {
			switch {
			case rankOf(card) == rankJoker:
				key = "joker"
			case isWild(card):
				key = "deuce"
			default:
				key = "natural"
			}
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, module.Action{OfferID: o.ID, Verb: VerbLayOff, Target: target.ID, Cards: []string{card}})
	}
	return out
}

type candidateBuilder struct {
	m    *Module
	raw  module.State
	s    *GameState
	seat string
	t    *Team
}

// draws are the ways to start a turn: the stock, Samba's top card, and every
// capture of the pile the engine offers — the last, before this side has
// opened, only as the capture and an opening that finishes after it.
func (c candidateBuilder) draws(offers []module.ActionOffer) []learn.Candidate {
	var out []learn.Candidate
	for _, o := range offers {
		a, ok := module.SubmissionFor(o)
		if !ok {
			continue
		}
		switch o.Verb {
		case VerbDraw:
			f := kindOnly(kindDraw)
			f[fSteps] = 0.25
			f[fHandAfter] = float32(len(c.s.Hands[c.seat])+c.s.rules().DrawCount) / 15
			out = append(out, learn.Candidate{Action: a, Features: f})
		case VerbTakeTop:
			out = append(out, c.single(a, kindTakeTop))
		case VerbTakePile:
			if c.t.HasMelded {
				if c.captureLeavesATurn(a) {
					out = append(out, c.single(a, kindTakePile))
				}
				continue
			}
			out = append(out, c.openingCapture(a)...)
		}
	}
	return out
}

// captureLeavesATurn reports that a capture by an opened side can still be
// followed by a discard.
//
// Not always so. A capture onto a meld already on the table takes nothing out
// of the hand, and the pile under a lone top card adds nothing to it, so a
// seat holding one card that makes one ends its draw still holding one — and
// discarding it is going out, which a side short of its canastas may not do.
// applyTakePile does not ask checkLeavesPlayable the question applyTakeTop
// does, so the engine offers the move, and the only thing left after it is
// undo_take_pile: the dead end, arrived at without an opening in sight. The
// capture is made here, and kept only if a discard is still to be had.
func (c candidateBuilder) captureLeavesATurn(take module.Action) bool {
	next, _, err := c.m.Apply(c.raw, c.seat, take)
	if err != nil {
		return false
	}
	ns, err := decode(next)
	if err != nil {
		return false
	}
	return finishable(c.m, next, c.s, ns, c.seat)
}

// openingCapture is a capture by a side that has not opened, offered only
// together with an opening that finishes from the hand the capture leaves.
//
// The engine already refuses a capture it believes cannot lead to an opening
// (capturePlayable), and that belief is a bound, not a fact — meld.go says as
// much about reachableValue, and the heuristic's known wedge is a capture it
// allowed and a hand that then would not cooperate. So the capture is made,
// here, and the opening looked for in the position it actually produces. None
// found, no candidate: this side draws instead.
func (c candidateBuilder) openingCapture(take module.Action) []learn.Candidate {
	next, _, err := c.m.Apply(c.raw, c.seat, take)
	if err != nil {
		return nil
	}
	ns, err := decode(next)
	if err != nil {
		return nil
	}
	if dealEnded(c.s, ns) || ns.team(c.seat).HasMelded {
		if !finishable(c.m, next, c.s, ns, c.seat) {
			return nil
		}
		return []learn.Candidate{c.composite([]module.Action{take}, ns, kindTakePile)}
	}
	return c.openingsAfter(next, ns, []module.Action{take}, kindTakePile, captureBudget, maxCaptureOpens)
}

// discards are one candidate per card the engine will take as this turn's
// discard. The offer already lists each distinct card once.
func (c candidateBuilder) discards(offers []module.ActionOffer) []learn.Candidate {
	var out []learn.Candidate
	hand := c.s.Hands[c.seat]
	next := c.s.team(c.s.nextPlayer(c.seat))
	for _, o := range offers {
		if !o.Enabled || o.Verb != VerbDiscard || o.Source == nil {
			continue
		}
		for _, card := range o.Source.Cards {
			f := kindOnly(kindDiscard)
			if k := cardCat(card); k >= 0 {
				f[fCat+k] = 1
			}
			rest, _ := removeCards(hand, []string{card})
			f[fSteps] = 0.25
			f[fGoesOut] = b2f(len(rest) == 0)
			f[fDiscardWild] = b2f(isWild(card))
			f[fDiscardBlack] = b2f(isBlackThree(card))
			f[fDiscardFeeds] = b2f(next != nil && next.ID != c.t.ID && !isWild(card) && len(next.groupsOfRank(rankOf(card))) > 0)
			f[fDiscardFreez] = b2f(isWild(card) && !c.s.Frozen)
			f[fHandAfter] = float32(len(rest)) / 15
			same := 0
			for _, h := range rest {
				if rankOf(h) == rankOf(card) {
					same++
				}
			}
			f[fSameRank] = float32(same) / 4
			out = append(out, learn.Candidate{
				Action:   module.Action{OfferID: o.ID, Verb: VerbDiscard, Cards: []string{card}},
				Features: f,
			})
		}
	}
	return out
}

// single is a one-action candidate that puts cards on the table, described by
// working out the table it leaves rather than by applying it: there are
// dozens of these at a decision, and each is one small arithmetic step from
// the state already decoded.
func (c candidateBuilder) single(a module.Action, kind int) learn.Candidate {
	s, t := c.s, c.t
	r := s.rules()
	f := kindOnly(kind)
	hand := s.Hands[c.seat]

	var laid, before []string // cards reaching the table; the meld they join
	fromHand := a.Cards
	run := false
	about := ""
	switch a.Verb {
	case VerbTakePile:
		top := s.top()
		laid, about = append([]string{top}, a.Cards...), top
		if a.Target != "" {
			if m := t.meldByID(a.Target); m != nil {
				before = m.Cards
			}
		} else if m := t.openGroup(r, rankOf(top)); m != nil {
			before = m.Cards
		}
		f[fPile] = float32(len(s.DiscardPile)) / 20
	case VerbTakeTop:
		top := s.top()
		laid, fromHand, run, about = []string{top}, nil, true, top
		if m := t.meldByID(a.Target); m != nil {
			before = m.Cards
		}
	case VerbLayMeld:
		laid = a.Cards
		run = meldKindOf(r, a.Cards) == meldRun
		if !run {
			about = firstNatural(a.Cards)
		}
	case VerbLayOff:
		laid = a.Cards
		if m := t.meldByID(a.Target); m != nil {
			before = m.Cards
			run = m.kind() == meldRun
		}
		if len(a.Cards) > 0 {
			about = a.Cards[0]
		}
	}
	if k := cardCat(about); about != "" && k >= 0 {
		f[fCat+k] = 1
	}
	after := append(append([]string(nil), before...), laid...)
	f[fRun] = b2f(run)
	f[fCards] = float32(len(laid)) / 10
	f[fWilds] = float32(countWilds(laid)) / 4
	f[fMeldSize] = float32(len(after)) / 7
	if len(before) < canastaSize && len(after) >= canastaSize {
		f[fCanastas] = 1
		f[fNaturals] = b2f(countWilds(after) == 0)
	}
	f[fPoints] = float32(handValue(laid)) / 100
	f[fSteps] = 0.25
	rest, _ := removeCards(hand, fromHand)
	left := len(rest)
	if a.Verb == VerbTakePile {
		left += len(s.DiscardPile) - 1
	}
	// Only a meld or a lay-off can empty a hand here; a capture always leaves
	// the discard to come.
	f[fGoesOut] = b2f((a.Verb == VerbLayMeld || a.Verb == VerbLayOff) && left == 0)
	f[fHandAfter] = float32(left) / 15
	return learn.Candidate{Action: a, Features: f}
}

// SameMove is the other side of the collapsing Candidates does: a move that
// differs from a candidate only in which of several interchangeable cards it
// used is the move the candidate stands for.
//
// Interchangeable means what layOffActions means by it. On a group — a meld of
// one rank — every natural of that rank is the same card, and a wild is a
// joker or a deuce, which differ in what they cost; so two moves onto a group,
// or two groups laid, are the same when they put the same number of naturals
// of the same rank and the same wilds there. A sequence is not a group: its
// suit is the meld, and every card in it is its own move. Nothing else is
// collapsed, so nothing else is equivalent.
func (learnGame) SameMove(raw module.State, seat string, played, cand module.Action) bool {
	if played.Verb != cand.Verb || played.Target != cand.Target || len(played.Cards) != len(cand.Cards) {
		return false
	}
	s, err := decode(raw)
	if err != nil {
		return false
	}
	t := s.team(seat)
	if t == nil {
		return false
	}
	r := s.rules()
	onGroup := func(target string, cards []string) bool {
		if target != "" {
			m := t.meldByID(target)
			return m != nil && m.kind() != meldRun
		}
		return meldKindOf(r, cards) != meldRun
	}
	switch played.Verb {
	case VerbLayOff:
		if played.Target == "" {
			return false
		}
	case VerbLayMeld:
	case VerbTakePile:
		// The capture's cards go with the top card, which is the same for
		// both: the meld they make is that card's.
		top := s.top()
		if !onGroup(played.Target, append([]string{top}, played.Cards...)) ||
			!onGroup(cand.Target, append([]string{top}, cand.Cards...)) {
			return false
		}
		return sameClasses(played.Cards, cand.Cards)
	default:
		return false
	}
	return onGroup(played.Target, played.Cards) && onGroup(cand.Target, cand.Cards) &&
		sameClasses(played.Cards, cand.Cards)
}

// sameClasses compares two sets of cards bound for a group: naturals by rank,
// wilds as a joker or a deuce.
func sameClasses(a, b []string) bool {
	count := map[string]int{}
	class := func(c string) string {
		switch {
		case rankOf(c) == rankJoker:
			return "joker"
		case isWild(c):
			return "deuce"
		}
		return rankOf(c)
	}
	for _, c := range a {
		count[class(c)]++
	}
	for _, c := range b {
		count[class(c)]--
	}
	for _, n := range count {
		if n != 0 {
			return false
		}
	}
	return true
}

func firstNatural(cards []string) string {
	for _, c := range cards {
		if !isWild(c) {
			return c
		}
	}
	return ""
}

func countWilds(cards []string) int {
	n := 0
	for _, c := range cards {
		if isWild(c) {
			n++
		}
	}
	return n
}

// composite describes a candidate of several actions by comparing the table
// before it with the table after it, both of which exist: a composite has been
// played through the engine to be found at all.
func (c candidateBuilder) composite(steps []module.Action, after *GameState, kind int) learn.Candidate {
	f := kindOnly(kind)
	was := map[string]Meld{}
	for _, m := range c.t.Melds {
		was[m.ID] = m
	}
	cards, wilds, largest, canastas, naturals, points := 0, 0, 0, 0, 0, 0
	for _, m := range after.team(c.seat).Melds {
		old := was[m.ID]
		if len(m.Cards) == len(old.Cards) {
			continue
		}
		cards += len(m.Cards) - len(old.Cards)
		wilds += m.wilds() - old.wilds()
		points += handValue(m.Cards) - handValue(old.Cards)
		if len(m.Cards) > largest {
			largest = len(m.Cards)
		}
		if !old.isCanasta() && m.isCanasta() {
			canastas++
			if m.isNatural() {
				naturals++
			}
		}
	}
	if kind == kindTakePile {
		f[fPile] = float32(len(c.s.DiscardPile)) / 20
		if k := cardCat(c.s.top()); k >= 0 {
			f[fCat+k] = 1
		}
	}
	f[fCards] = float32(cards) / 10
	f[fWilds] = float32(wilds) / 4
	f[fMeldSize] = float32(largest) / 7
	f[fCanastas] = float32(canastas)
	f[fNaturals] = float32(naturals)
	f[fPoints] = float32(points) / 100
	ended := dealEnded(c.s, after)
	f[fGoesOut] = b2f(ended && after.Deals[len(after.Deals)-1].WentOut == c.seat)
	f[fOpens] = 1
	f[fSteps] = float32(len(steps)) / 4
	if !ended {
		f[fHandAfter] = float32(len(after.Hands[c.seat])) / 15
	}
	return learn.Candidate{Action: steps[0], Then: steps[1:], Features: f}
}

func kindOnly(kind int) []float32 {
	f := make([]float32, learnCandDim)
	f[kind] = 1
	return f
}

// --- opening the account -------------------------------------------------------

// How hard to look for openings. Measured in engine applications, because that
// is what one costs: an opening is found by laying it, not by reasoning about
// whether it could be laid.
const (
	openingBudget   = 240 // from the meld phase
	captureBudget   = 120 // after each pile capture
	maxOpenings     = 6   // plans offered from the meld phase
	maxCaptureOpens = 2   // plans offered per capture
	maxOpeningSteps = 4
)

// openings are the complete openings from the meld phase of this turn.
func (c candidateBuilder) openings(raw module.State, s *GameState, prefix []module.Action, kind int) []learn.Candidate {
	return c.openingsAfter(raw, s, prefix, kind, openingBudget, maxOpenings)
}

// openingsAfter are complete openings from a position, each prefixed with the
// steps that reached it — a pile capture, or nothing.
func (c candidateBuilder) openingsAfter(raw module.State, s *GameState, prefix []module.Action, kind, budget, max int) []learn.Candidate {
	search := candidateOpenings{m: c.m, seat: c.seat, origin: c.s, budget: budget, max: max, seen: map[string]bool{}}
	search.walk(raw, s, prefix, nil, 0)
	out := make([]learn.Candidate, 0, len(search.plans))
	for _, p := range search.plans {
		out = append(out, c.composite(p.steps, p.after, kind))
	}
	return out
}

// candidateOpenings finds whole openings by laying them.
//
// The same discipline as the heuristic's own opening search (opening.go) —
// never start an opening you have not found the whole of, and let the engine,
// not reachableValue's bound, judge every step. Every step of a plan is put to
// the engine in order, from the position the step before it left, and a plan
// counts only once the engine has marked the side opened and a discard (or the
// deal's end) is still to be had after it.
//
// It is kept separate from opening.go because the two answer different
// questions. The bot needs one plan, and its first move; a network needs a
// handful of plans at once, each offered as a whole candidate it can compare
// with the others, so this one enumerates several and returns them all.
//
// Depth-first, naturals before wilds and valuable before cheap, for the
// heuristic's reason: an opening that keeps the joker is the
// better one. Orders that reach the same melds are walked once (seen), which is
// what keeps three melds from costing six searches, and an order the engine
// refuses part-way leaves the others to be tried — checkInitialMeld can accept
// a set of melds in one order and refuse it in another.
//
// Every position part-way through a plan found here is a position this search
// can finish from, which is what a NetBot playing the plan a step at a time
// relies on (learn.Candidate.Then): the rest of the plan is a smaller search
// over the same steps. TestEveryCandidateApplies checks it at every step.
type candidateOpenings struct {
	m      *Module
	seat   string
	origin *GameState
	budget int
	max    int
	seen   map[string]bool
	plans  []openingFound
}

type openingFound struct {
	steps []module.Action
	after *GameState
}

type openingStep struct {
	a     module.Action
	key   string
	value int
	wilds int
}

func (o *candidateOpenings) walk(raw module.State, s *GameState, path []module.Action, keys []string, depth int) {
	for _, st := range openingSteps(s, o.seat) {
		if o.budget <= 0 || len(o.plans) >= o.max {
			return
		}
		ks := append(append([]string(nil), keys...), st.key)
		sort.Strings(ks)
		id := strings.Join(ks, " ")
		if o.seen[id] {
			continue
		}
		o.budget--
		next, _, err := o.m.Apply(raw, o.seat, st.a)
		if err != nil {
			continue
		}
		o.seen[id] = true
		ns, err := decode(next)
		if err != nil {
			continue
		}
		p := append(append([]module.Action(nil), path...), st.a)
		if dealEnded(o.origin, ns) || ns.team(o.seat).HasMelded {
			if finishable(o.m, next, o.origin, ns, o.seat) {
				o.plans = append(o.plans, openingFound{steps: p, after: ns})
			}
			continue // an opening is complete; building on it is the next decision
		}
		if depth+1 < maxOpeningSteps {
			o.walk(next, ns, p, ks, depth+1)
		}
	}
}

// openingSteps are the single actions an opening can be built from here: every
// new meld the hand holds — a group with the wilds it needs and again with
// each extra wild it could carry, and every sequence — and a card onto each
// meld already down this turn. Black threes are not among them: that meld only
// ever goes down as the move that empties a hand, never towards an opening.
func openingSteps(s *GameState, seat string) []openingStep {
	r := s.rules()
	t := s.team(seat)
	hand := s.Hands[seat]
	var out []openingStep
	add := func(a module.Action) {
		out = append(out, openingStep{a: a, key: stepKey(a), value: handValue(a.Cards), wilds: countWilds(a.Cards)})
	}
	// Jokers before deuces: an extra wild carried for its points should be the
	// one worth more of them.
	wilds := handWilds(hand)
	sort.SliceStable(wilds, func(i, j int) bool { return cardValue(wilds[i]) > cardValue(wilds[j]) })
	for _, c := range allMeldCandidates(r, hand, t) {
		id := OfferLayMeld + ":" + c.offerKey()
		add(module.Action{OfferID: id, Verb: VerbLayMeld, Cards: c.Cards})
		spare, ok := removeCards(hand, c.Cards)
		if !ok {
			continue
		}
		cards := append([]string(nil), c.Cards...)
		for _, w := range wilds {
			next, ok := removeCards(spare, []string{w})
			if !ok {
				continue
			}
			if validateMeld(r, append(append([]string(nil), cards...), w)) != nil {
				break
			}
			spare, cards = next, append(cards, w)
			add(module.Action{OfferID: id, Verb: VerbLayMeld, Cards: append([]string(nil), cards...)})
		}
	}
	for _, c := range runCandidates(r, hand, t) {
		add(module.Action{OfferID: OfferLayMeld + ":" + c.offerKey(), Verb: VerbLayMeld, Cards: c.Cards})
	}
	for i := range t.Melds {
		m := t.Melds[i]
		o := module.ActionOffer{ID: "lay_off:" + m.ID, Verb: VerbLayOff,
			Source: &module.Selector{Cards: layOffCards(r, hand, &m)},
			Target: &module.Selector{MeldID: m.ID}}
		for _, a := range layOffActions(t, o) {
			add(a)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if (out[i].wilds == 0) != (out[j].wilds == 0) {
			return out[i].wilds == 0
		}
		if out[i].value != out[j].value {
			return out[i].value > out[j].value
		}
		return out[i].key < out[j].key
	})
	return out
}

func stepKey(a module.Action) string {
	return a.Verb + ":" + a.Target + ":" + strings.Join(sortedCards(a.Cards), ",")
}

// finishable reports that a turn can still end from here: the deal is over, or
// some card in the hand is a discard the engine will take. The second is true
// of every opened side holding a card, and asked anyway — it is the property
// the whole search exists to guarantee, so it is checked rather than argued.
func finishable(m *Module, raw module.State, origin, s *GameState, seat string) bool {
	if dealEnded(origin, s) {
		return true
	}
	if s.Current != seat || s.Phase != phaseMeld {
		return false
	}
	for _, card := range sortedUnique(s.Hands[seat]) {
		if ok, _ := probe(m, raw, seat, module.Action{Verb: VerbDiscard, Cards: []string{card}}); ok {
			return true
		}
	}
	return false
}

// dealEnded reports that a deal was scored between two positions. Deals grows
// by one in endDeal and nowhere else, which is what makes this exact: it cannot
// miss a deal, and it cannot count one twice.
func dealEnded(before, after *GameState) bool { return len(after.Deals) > len(before.Deals) }

// --- Reward --------------------------------------------------------------------

// dealsOnly is the part of a state Reward reads. Decoding the whole of it twice
// for every learner seat after every action would be most of what the
// environment spends its time on.
type dealsOnly struct {
	Deals  []DealResult   `json:"deals"`
	TeamOf map[string]int `json:"teamOf"`
}

// Reward is paid once a deal, when it is scored: this side's deal total minus
// the best other side's, in thousands. Nothing in between — points on the
// table are only points if the deal ends with them there, and a network paid
// for melding as it goes learns to meld rather than to win deals.
//
// Every deal closes an episode. A match to 5000 is a dozen deals, and the deal
// is the unit whose outcome the moves in it decide. Partners read the same
// result off the same row, so they are paid the same.
func (g learnGame) Reward(before, after module.State, seat string) (float32, bool, error) {
	bp, err := g.Position(before)
	if err != nil {
		return 0, false, err
	}
	ap, err := g.Position(after)
	if err != nil {
		return 0, false, err
	}
	return g.RewardFor(bp, ap, seat)
}

// RewardFor is Reward between two decoded positions.
func (learnGame) RewardFor(before, after learn.Position, seat string) (float32, bool, error) {
	bp, err := positionOf(before)
	if err != nil {
		return 0, false, err
	}
	ap, err := positionOf(after)
	if err != nil {
		return 0, false, err
	}
	b, err := bp.dealResults()
	if err != nil {
		return 0, false, err
	}
	a, err := ap.dealResults()
	if err != nil {
		return 0, false, err
	}
	if len(a.Deals) <= len(b.Deals) {
		return 0, false, nil
	}
	mine, ok := a.TeamOf[seat]
	if !ok {
		return 0, false, fmt.Errorf("canasta: no team for %q", seat)
	}
	res := a.Deals[len(a.Deals)-1]
	own, best, found := 0, 0, false
	for _, tr := range res.Teams {
		if tr.TeamID == mine {
			own = tr.Total
			continue
		}
		if !found || tr.Total > best {
			best, found = tr.Total, true
		}
	}
	return float32(own-best) / 1000, true, nil
}
