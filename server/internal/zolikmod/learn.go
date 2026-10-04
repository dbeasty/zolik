package zolikmod

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"zolik/server/internal/ai"
	"zolik/server/internal/cardinfer"
	"zolik/server/internal/learn"
	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// Žolíky as the learning machinery sees it (internal/learn).
//
//	Encode      the position as one seat may know it: its own hand, the table,
//	            the pile, and the public memory of the deal (ai.Ledger, the same
//	            one the heuristic plays from) — never another hand or the order
//	            of the stock. TestLearnEncodeDoesNotPeek pins it.
//	Candidates  the moves on offer. Laying a meld is a Composite offer here: the
//	            engine does not enumerate meld shapes, so every meld candidate
//	            is found by a search over the hand and then put to the engine
//	            (rules.ApplyAction, the function Module.Apply calls) before it
//	            is offered. Nothing in this file decides a move is legal.
//	Reward      a deal's result, once per seat, at the end of the deal.
//
// The one rule every candidate keeps is that it never ends in a dead position:
// after it the seat can still end its turn — a discard the engine accepts — or
// the deal is over. That is what makes the first meld a whole plan (a partial
// lay-down cannot be discarded out of), and a joker reclaim a two-step move
// (a reclaimed joker must be played again before the discard).
//
// The vector layouts below are a contract with the trainer (ml/). A change to
// any offset is a new encoder, which NetBot's StateDim/CandDim check refuses
// to load an old network against.
type learnGame struct{}

func init() { learn.Register(learnGame{}) }

var (
	_ learn.Game       = learnGame{}
	_ learn.Positional = learnGame{}
)

// position is a state decoded for learn.Positional: the whole matchState when
// a seat's encoding or candidates are asked for, and only the score sheet when
// a reward is — each at most once, however many seats ask. The candidate
// search branches through rules.ApplyAction, which clones the state it is
// handed, so the decoded state here is never written.
type position struct {
	raw    module.State
	full   learn.Memo[*matchState]
	scores learn.Memo[*scoresOnly]
	match  learn.Memo[*matchOnly] // learn_train.go
}

func (learnGame) Position(raw module.State) (learn.Position, error) {
	return &position{raw: raw}, nil
}

func positionOf(at learn.Position) (*position, error) {
	if pos, ok := at.(*position); ok && pos != nil {
		return pos, nil
	}
	return nil, fmt.Errorf("zolik: %T is not a Žolíky position", at)
}

// state is the decoded matchState, shared by every reader: read it, never
// write it.
func (p *position) state() (*matchState, error) {
	return p.full.Get(func() (*matchState, error) { return decode(p.raw) })
}

func (p *position) scoreSheet() (*scoresOnly, error) {
	return p.scores.Get(func() (*scoresOnly, error) {
		var sc scoresOnly
		if err := json.Unmarshal(p.raw, &sc); err != nil {
			return nil, fmt.Errorf("zolik: decode state: %w", err)
		}
		return &sc, nil
	})
}

// Name is the module id, which is also what gamebench and gameenv call it.
func (learnGame) Name() string              { return rules.Descriptor().ID }
func (learnGame) Module() module.GameModule { return New() }

// Heuristic is the bot the module plays its AI seats with (internal/ai).
func (learnGame) Heuristic() module.Bot { return heuristicBot{} }

// The rulesets a trainer or bench can name. Two are the shipped profiles; the
// third is the house-rule combination cmd/aibench sweeps (a 35-point floor
// and the pile locked until the third lap on Žolík Classic), because a policy
// that is good under one of them can be bad under another.
const (
	varClassic     = "zolik_classic"
	varFloor35     = "zolik_classic+floor35"
	varContinental = "continental"
)

var learnVariations = []string{varClassic, varFloor35, varContinental}

// maxSeats is the widest table the encoder describes. Two to four seats share
// two packs (rules.DeckCountForPlayers); five and up change the deck, and the
// card counts below would mean something else.
const maxSeats = 4

// Config is the ruleset as a lobby deals it: the named profile with its own
// defaults (pause between deals included), plus the two option values that
// make floor35 what cmd/aibench means by it.
func (learnGame) Config(seats int, variation string) module.MatchConfig {
	switch variation {
	case varFloor35:
		return module.MatchConfig{Variation: varClassic, Options: module.Options{
			rules.OptInitialMeldMinimum:  35,
			rules.OptDiscardDrawMinRound: 3,
		}}
	case "":
		return module.MatchConfig{Variation: varClassic}
	}
	return module.MatchConfig{Variation: variation}
}

// Outcome is the seat's match penalty total, negated: rummy is scored in
// penalty points and fewer is better, so -TotalScores makes higher better, as
// the bench expects. A seat that went out of every deal scores 0; the bench's
// A-minus-B is then "how many fewer penalty points A took per match".
func (learnGame) Outcome(raw module.State, seat string) (float64, error) {
	s, err := decode(raw)
	if err != nil {
		return 0, err
	}
	if !contains(s.Rules.TurnOrder, seat) {
		return 0, fmt.Errorf("zolik: no seat %q", seat)
	}
	return -float64(s.Rules.TotalScores[seat]), nil
}

// --- cards, to a network ------------------------------------------------------

// A card's slot: rank (2..A) × suit (H C D S), and one slot for the joker, the
// only wild in this game. Aces are a real rank here — natural at a run's ends
// and in a set of aces — and are encoded as the rank they are.
const (
	numRanks    = 13
	numSuits    = 4
	jokerSlot   = numRanks * numSuits
	numCardSlot = jokerSlot + 1
)

var learnSuits = [numSuits]string{"H", "C", "D", "S"}

func cardSlot(c string) int {
	if rules.IsJoker(c) {
		return jokerSlot
	}
	r := rules.CardRank(c)
	if r < 0 {
		return -1
	}
	for i, s := range learnSuits {
		if rules.CardSuit(c) == s {
			return r*numSuits + i
		}
	}
	return -1
}

// slotCard is the card a slot names; the joker slot names JOKER1.
func slotCard(i int) string {
	if i == jokerSlot {
		return "JOKER1"
	}
	return string(rules.RankOrder[i/numSuits]) + learnSuits[i%numSuits]
}

// --- Encode ------------------------------------------------------------------

// The state vector. Offsets are absolute; ml/ reads them. Card blocks are
// numCardSlot (53) wide in cardSlot order, and counts are over two packs.
//
//	[0,53)      my hand, copies per card (/2)
//	[53,56)     my hand size (/15), its penalty (/150), jokers in it (/4)
//	[56,109)    every meld on the table, copies per card (/2)
//	[109,162)   cards that would extend some meld on the table now (0/1)
//	[162,215)   the discard pile, copies per card (/2)
//	[215,269)   the pile's top card one-hot, slot 53 = empty pile
//	269         pile size (/30)
//	[270,323)   what the other seats have discarded this deal (/2) — the
//	            public ledger, order folded away
//	[323,376)   copies of each card not seen anywhere I can see (/2): my hand,
//	            the table, the pile and the cards others were seen to take
//	[376,535)   three opponent blocks, the order they play after me: the cards
//	            each was seen to take off the pile (or the table) and has not
//	            put down again (/2)
//	[535,595)   four seat blocks of seatDim, me first then the order of play;
//	            an absent seat is zero
//	[595,608)   the turn: stock (/100), deal number (/10), lap (/10), draw
//	            phase, meld phase, my turn, between deals, melds laid this turn
//	            toward going down (/3), a pickup owed to the opening, reclaimed
//	            jokers owed (/2), a card taken off the pile this turn, the pile
//	            locked this lap, reshuffles (/3)
//	[608,617)   the requirement: contract sets (/3), runs (/3), clean run
//	            required, point floor (/70), final deal; what I still owe of
//	            each: sets (/3), runs (/3), clean run, points (/70) — zero once
//	            I am down
//	[617,630)   the rules: variation one-hot (classic, classic+floor35,
//	            continental), any card from the pile, pile lock lap (/3),
//	            target (/200), fixed deals (/7), reclaimed joker must be played,
//	            joker discard restricted, seats one-hot (2..4), min run (/4)
//	[630,639)   how far my hand is from going down (distanceOf; all zero once
//	            I am down): natural cards missing from the nearest clean run
//	            window whose missing cards are all still unseen (/4, 1 when
//	            none is), whether any clean run is completable at all, the
//	            unseen copies of that window's missing cards (/8), cards the
//	            contract's runs still want (/6), cards its sets still want
//	            (/6), natural value of the melds the hand holds now (/70),
//	            points the floor still wants beyond that (/70), all of it
//	            together in cards (/8, the floor at ten a card), the clean
//	            run already in hand
//	[639,900)   three opponent blocks of rivalDim (87), the order they play
//	            after me — what each has shown it is collecting (rivalOf):
//	            cards seen taken off the pile and still held, per suit H C D
//	            S (/4) and per rank 2..A (/4); cards on the table in its
//	            melds, per suit (/8) and per rank (/4); and its wants, per
//	            card slot: 1 where the card lays off onto its own melds or
//	            completes a pair it took, 0.5 next to (or the rank of) a
//	            single card it took
//
//	[900,1161)  three inference blocks of inferDim (87), the order they play
//	            after me — what each probably holds and wants
//	            (internal/cardinfer via ai.Infer, public information only):
//	            expected copies held per rank 2..A (/2) and per suit H C D S
//	            (/8), expected jokers (/2); how much more than a random card
//	            it wants each rank (the most over the suits) and each suit
//	            (the most over the ranks), and what it wants of a random card;
//	            its three most wanted unseen cards, each a rank one-hot (13)
//	            and a suit one-hot (4)
//
// A seat block (seatDim = 15):
//
//	present, is me, hand size (/15), is down, melds (/6), sets (/4), runs (/4),
//	has a clean run, natural value of its melds (/100), match penalty
//	(/target), deals won (/5), discards this deal (/15), cards known held
//	(/5), last deal's penalty (/100), is on turn
const (
	offHand     = 0
	offHandMisc = offHand + numCardSlot
	offTable    = offHandMisc + 3
	offExtends  = offTable + numCardSlot
	offPile     = offExtends + numCardSlot
	offPileTop  = offPile + numCardSlot
	offPileMisc = offPileTop + numCardSlot + 1
	offDiscards = offPileMisc + 1
	offUnseen   = offDiscards + numCardSlot
	offHeld     = offUnseen + numCardSlot
	offSeats    = offHeld + (maxSeats-1)*numCardSlot
	seatDim     = 15
	offTurn     = offSeats + maxSeats*seatDim
	offReq      = offTurn + 13
	offRules    = offReq + 9
	offDist     = offRules + 3 + 6 + 3 + 1
	distDim     = 9
	offRivals   = offDist + distDim
	rivalDim    = 2*(numSuits+numRanks) + numCardSlot
	offInfer    = offRivals + (maxSeats-1)*rivalDim
	inferTop    = 3
	inferDim    = 2*(numRanks+numSuits) + 2 + inferTop*(numRanks+numSuits)
	learnState  = offInfer + (maxSeats-1)*inferDim
)

func (learnGame) StateDim() int { return learnState }

func (g learnGame) Encode(raw module.State, seat string) ([]float32, error) {
	at, err := g.Position(raw)
	if err != nil {
		return nil, err
	}
	return g.EncodeFor(at, seat)
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
	gs := s.Rules
	if !contains(gs.TurnOrder, seat) {
		return nil, fmt.Errorf("zolik: no seat %q", seat)
	}
	if len(gs.TurnOrder) > maxSeats {
		return nil, fmt.Errorf("zolik: the encoder describes at most %d seats, not %d", maxSeats, len(gs.TurnOrder))
	}
	cfg := rules.ResolveConfig(gs.Rules)
	// Everything below except my own hand comes off the agent's public view —
	// the same snapshot the heuristic plays from, whose own no-peek test pins
	// that it carries no other hand.
	vis := ai.VisibleFor(gs, s.Ledger, seat)
	hand := gs.Hands[seat]
	v := make([]float32, learnState)

	for _, c := range hand {
		if k := cardSlot(c); k >= 0 {
			v[offHand+k] += 0.5
		}
	}
	jokers := 0
	for _, c := range hand {
		if rules.IsJoker(c) {
			jokers++
		}
	}
	v[offHandMisc] = float32(len(hand)) / 15
	v[offHandMisc+1] = float32(rules.HandPenaltyTotal(hand, cfg)) / 150
	v[offHandMisc+2] = float32(jokers) / 4

	for _, melds := range vis.Melds {
		for _, m := range melds {
			for _, c := range m {
				if k := cardSlot(c); k >= 0 {
					v[offTable+k] += 0.5
				}
			}
		}
	}
	ext := extenders(vis.Melds, cfg)
	for k, ok := range ext {
		v[offExtends+k] = b2f(ok)
	}

	for _, c := range vis.DiscardPile {
		if k := cardSlot(c); k >= 0 {
			v[offPile+k] += 0.5
		}
	}
	if n := len(vis.DiscardPile); n == 0 {
		v[offPileTop+numCardSlot] = 1
	} else if k := cardSlot(vis.DiscardPile[n-1]); k >= 0 {
		v[offPileTop+k] = 1
	}
	v[offPileMisc] = float32(len(vis.DiscardPile)) / 30

	for _, d := range vis.DealDiscards {
		if d.Player == seat {
			continue
		}
		if k := cardSlot(d.Card); k >= 0 {
			v[offDiscards+k] += 0.5
		}
	}
	unseen := unseenCounts(vis, hand, seat)
	for k, n := range unseen {
		v[offUnseen+k] = float32(n) / 2
	}

	order := seatsFrom(gs.TurnOrder, seat)
	for i, id := range order[1:] {
		for _, c := range vis.KnownHeld[id] {
			if k := cardSlot(c); k >= 0 {
				v[offHeld+i*numCardSlot+k] += 0.5
			}
		}
	}

	target := cfg.TargetScore
	if target <= 0 {
		target = 300
	}
	won := rules.DealsWonByPlayer(gs.TurnOrder, gs.GameScores)
	discards := map[string]int{}
	for _, d := range vis.DealDiscards {
		discards[d.Player]++
	}
	for i, id := range order {
		b := v[offSeats+i*seatDim : offSeats+(i+1)*seatDim]
		sets, runs, clean := rules.PlayerMeldCounts(gs, id)
		b[0] = 1
		b[1] = b2f(id == seat)
		b[2] = float32(vis.HandCounts[id]) / 15
		b[3] = b2f(vis.RoundReqMet[id])
		b[4] = float32(len(vis.Melds[id])) / 6
		b[5] = float32(sets) / 4
		b[6] = float32(runs) / 4
		b[7] = b2f(clean)
		b[8] = float32(rules.PlayerInitialMeldNaturalValue(gs, id)) / 100
		b[9] = float32(gs.TotalScores[id]) / float32(target)
		b[10] = float32(won[id]) / 5
		b[11] = float32(discards[id]) / 15
		b[12] = float32(len(vis.KnownHeld[id])) / 5
		if sc := gs.GameScores[id]; len(sc) > 0 {
			b[13] = float32(sc[len(sc)-1]) / 100
		}
		b[14] = b2f(gs.CurrentTurn == id)
	}

	t := v[offTurn:]
	t[0] = float32(len(gs.DrawPile)) / 100
	t[1] = float32(gs.GameNumber) / 10
	t[2] = float32(gs.Round) / 10
	t[3] = b2f(gs.Phase == rules.PhaseDraw)
	t[4] = b2f(gs.Phase == rules.PhaseMeld || gs.Phase == rules.PhaseDiscard)
	t[5] = b2f(gs.CurrentTurn == seat)
	t[6] = b2f(s.Break.Open)
	// The rest are the current turn's, and only mine to be told about when it
	// is my turn; they are public (the pickup and the reclaim happened in front
	// of everybody), but an opponent's debts are not what this seat decides.
	if gs.CurrentTurn == seat {
		t[7] = float32(gs.MeldsLaidThisTurn) / 3
		t[8] = b2f(gs.DiscardDrawnCardPendingMeld != "")
		t[9] = float32(len(gs.JokersReclaimedPendingMeld)) / 2
		t[10] = b2f(gs.DiscardTakenCard != "")
	}
	t[11] = b2f(rules.IsDiscardLocked(gs.Round, cfg.DiscardDrawMinRound))
	t[12] = float32(gs.ReshuffleCount) / 3

	req := cfg.ContractFor(gs.GameNumber)
	r := v[offReq:]
	r[0] = float32(req.Sets) / 3
	r[1] = float32(req.Runs) / 3
	r[2] = b2f(req.RequireCleanRun)
	r[3] = float32(cfg.InitialMeldMinimum) / 70
	r[4] = b2f(cfg.IsFinalDeal(gs.GameNumber))
	if !gs.RoundReqMet[seat] {
		sets, runs, clean := rules.PlayerMeldCounts(gs, seat)
		r[5] = float32(max(0, req.Sets-sets)) / 3
		r[6] = float32(max(0, req.Runs-runs)) / 3
		r[7] = b2f(req.RequireCleanRun && !clean)
		r[8] = float32(max(0, cfg.InitialMeldMinimum-rules.PlayerInitialMeldNaturalValue(gs, seat))) / 70
	}

	o := v[offRules:]
	o[variationIndex(cfg)] = 1
	o = o[3:]
	o[0] = b2f(cfg.DiscardPickupMode == rules.DiscardPickupAnyFromPile)
	o[1] = float32(cfg.DiscardDrawMinRound) / 3
	o[2] = float32(cfg.TargetScore) / 200
	o[3] = float32(cfg.FixedDealCount) / 7
	o[4] = b2f(cfg.JokerReclaimMustPlay)
	o[5] = b2f(cfg.JokerDiscardRestricted)
	o = o[6:]
	if n := len(gs.TurnOrder); n >= 2 && n <= maxSeats {
		o[n-2] = 1
	}
	o[3] = float32(cfg.MinRunSize) / 4

	if !gs.RoundReqMet[seat] {
		encodeDistance(v[offDist:offDist+distDim], distanceOf(hand, cfg, req, &unseen), req)
	}
	for i, id := range order[1:] {
		r := rivalOf(vis, id, cfg)
		b := v[offRivals+i*rivalDim : offRivals+(i+1)*rivalDim]
		for k := 0; k < numSuits; k++ {
			b[k] = float32(r.takenSuit[k]) / 4
			b[numSuits+numRanks+k] = float32(r.meldSuit[k]) / 8
		}
		for k := 0; k < numRanks; k++ {
			b[numSuits+k] = float32(r.takenRank[k]) / 4
			b[2*numSuits+numRanks+k] = float32(r.meldRank[k]) / 4
		}
		copy(b[2*(numSuits+numRanks):], r.wants[:])
	}
	var est cardinfer.Estimate
	ai.Infer(vis, hand, seat, &est)
	for i := 0; i < est.N && i < maxSeats-1; i++ {
		encodeInference(v[offInfer+i*inferDim:offInfer+(i+1)*inferDim], &est, i)
	}
	return v, nil
}

// encodeInference writes one seat's inference block (see the layout above).
func encodeInference(b []float32, est *cardinfer.Estimate, i int) {
	hold := &est.Hold[i]
	for k := 0; k < jokerSlot; k++ {
		r, su := k/numSuits, k%numSuits
		b[r] += float32(hold[k]) / 2
		b[numRanks+su] += float32(hold[k]) / 8
		x := float32(est.Excess(i, k))
		o := numRanks + numSuits + 1
		b[o+r] = max(b[o+r], x)
		b[o+numRanks+su] = max(b[o+numRanks+su], x)
	}
	b[numRanks+numSuits] = float32(hold[jokerSlot]) / 2
	b[2*(numRanks+numSuits)+1] = float32(est.Base[i])
	var top [inferTop]int
	n := est.TopWanted(i, top[:])
	for j := 0; j < n; j++ {
		if top[j] == jokerSlot {
			continue // a joker is wanted by everybody; it says nothing
		}
		o := 2*(numRanks+numSuits) + 2 + j*(numRanks+numSuits)
		b[o+top[j]/numSuits] = 1
		b[o+numRanks+top[j]%numSuits] = 1
	}
}

func encodeDistance(b []float32, d distance, req rules.ContractRequirement) {
	b[0] = float32(min(d.clean, 4)) / 4
	b[1] = b2f(d.cleanLive)
	b[2] = float32(min(d.cleanOuts, 16)) / 8
	b[3] = float32(min(d.runs, 12)) / 6
	b[4] = float32(min(d.sets, 12)) / 6
	b[5] = float32(min(d.value, 140)) / 70
	b[6] = float32(d.short) / 70
	b[7] = float32(min(d.total(req), 16)) / 8
	b[8] = b2f(d.clean == 0)
}

// variationIndex reads which of learnVariations a table plays off its resolved
// rules rather than its name, so a lobby that set the same options by hand is
// the same game to the network.
func variationIndex(cfg rules.RulesConfig) int {
	switch {
	case cfg.FixedDealCount > 0:
		return 2
	case cfg.InitialMeldMinimum > 0:
		return 1
	}
	return 0
}

// seatsFrom is every seat, starting with this one and going round in the order
// of play.
func seatsFrom(order []string, seat string) []string {
	at := 0
	for i, p := range order {
		if p == seat {
			at = i
		}
	}
	out := make([]string, 0, len(order))
	for i := 0; i < len(order) && i < maxSeats; i++ {
		out = append(out, order[(at+i)%len(order)])
	}
	return out
}

// unseenCounts is, per card, the copies not accounted for by anything this
// seat can see — knowledge.go's outs, from the same public view: the packs in
// play, less my hand, the table, the pile as it stands and the cards others
// were seen to take. What is left is in the stock or in a hand nobody showed.
func unseenCounts(vis ai.VisibleState, hand []string, seat string) [numCardSlot]int {
	packs := vis.DeckCount
	if packs <= 0 {
		packs = 2
	}
	var n [numCardSlot]int
	for k := range n {
		n[k] = packs
	}
	n[jokerSlot] = 2 * packs
	see := func(c string) {
		if k := cardSlot(c); k >= 0 && n[k] > 0 {
			n[k]--
		}
	}
	for _, c := range hand {
		see(c)
	}
	for _, melds := range vis.Melds {
		for _, m := range melds {
			for _, c := range m {
				see(c)
			}
		}
	}
	for _, c := range vis.DiscardPile {
		see(c)
	}
	for id, cards := range vis.KnownHeld {
		if id == seat {
			continue
		}
		for _, c := range cards {
			see(c)
		}
	}
	return n
}

// extenders marks every card that would lay off onto some meld on the table
// as it stands (ai.TableExtenders: the card slots are the same layout).
func extenders(melds map[string][][]string, cfg rules.RulesConfig) [numCardSlot]bool {
	return ai.TableExtenders(melds, cfg)
}

// runRankCard is the card at a run position: 1 and 14 are the ace, 2..13 the
// ranks between.
func runRankCard(r int, suit string) string {
	switch {
	case suit == "":
		return ""
	case r == 1 || r == 14:
		return "A" + suit
	case r >= 2 && r <= 13:
		return string(rules.RankOrder[r-2]) + suit
	}
	return ""
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
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
//	[0,9)    kind: draw from the stock, take from the pile, take from the pile
//	         and go down with it (a composite), go down (an opening, composite),
//	         lay a meld, lay off, reclaim a joker and play it again (composite),
//	         discard, forced (between deals, nothing to choose)
//	[9,27)   the card the move is about — the one taken, laid off or
//	         discarded: rank one-hot 2..A (13), suit one-hot H C D S (4), is a
//	         joker (1)
//	27       actions it takes (/5)
//	28       cards it puts on the table (/10)
//	29       jokers it puts on the table (/4)
//	30       natural value it puts on the table (/50)
//	31       new melds it lays (/3)
//	32       it lays a joker-free run
//	33       it takes me down (my contract and floor met by it)
//	34       my melds' natural value afterwards, over the floor (/floor; 1
//	         where there is no floor) — whether it meets the minimum
//	35       it goes out
//	36       jokers it takes back off the table
//	37       cards it takes off the pile (/10)
//	38       penalty it takes off the pile (/100)
//	39       cards in hand afterwards (/15)
//	40       penalty in hand afterwards (/150)
//	41       penalty it sheds from the hand, net of what it takes in (/100)
//	42-50    discards and pile takes: the card is a joker, an ace, extends a
//	         meld on the table (feeds the next player / lays off for me), some
//	         opponent was seen collecting it or its neighbours, copies still
//	         unseen (/2), other copies of its rank in hand (/4), its suit
//	         neighbours in hand within two ranks (/4), it is part of a meld the
//	         hand could lay now, another seat discarded its rank this deal
//	51       pile takes only: the card makes a meld with the hand
//	52-55    discards (and pile takes, of the card taken): how clearly the next
//	         seat wants the card (its rivalOf wants, 0..1), the most any
//	         opponent wants it, the cards of its suit within two ranks the
//	         next seat was seen to take (/2), and of its rank (/2)
//	56-59    pile takes and discards, before going down: how much the move
//	         brings me closer to going down, per contract part — cards
//	         missing from the clean run, the runs and the sets (/4 each), and
//	         points the floor still wants (/70); negative is further away
//	60       pile takes only: a building card — it brings a part closer (or,
//	         once down, joins a card of its rank or suit in hand) without
//	         making a meld now
//	61       every part of going down the hand still wants afterwards, in
//	         cards (/8; zero once down or out)
//	62-63    the card the move is about, by the card inference
//	         (cardinfer.Estimate.Feed): how much more than a random card the
//	         next seat wants it, and the most any opponent does
//
// Every kind but forced also carries the penalty left in hand afterwards
// (40): the deck draw its hand as it stands, the unknown card aside.
const (
	kindDrawDeck = iota
	kindTakePile
	kindTakeOpen
	kindOpening
	kindLayMeld
	kindLayOff
	kindReclaim
	kindDiscard
	kindForced
	numKinds

	fCard       = numKinds
	fSteps      = fCard + numRanks + numSuits + 1
	fCards      = fSteps + 1
	fWilds      = fCards + 1
	fValue      = fWilds + 1
	fNewMelds   = fValue + 1
	fCleanRun   = fNewMelds + 1
	fGoesDown   = fCleanRun + 1
	fMeetsMin   = fGoesDown + 1
	fGoesOut    = fMeetsMin + 1
	fReclaims   = fGoesOut + 1
	fTaken      = fReclaims + 1
	fTakenPen   = fTaken + 1
	fHandAfter  = fTakenPen + 1
	fPenAfter   = fHandAfter + 1
	fPenShed    = fPenAfter + 1
	fIsJoker    = fPenShed + 1
	fIsAce      = fIsJoker + 1
	fExtends    = fIsAce + 1
	fWanted     = fExtends + 1
	fUnseen     = fWanted + 1
	fSameRank   = fUnseen + 1
	fSuitNear   = fSameRank + 1
	fInMeld     = fSuitNear + 1
	fPassed     = fInMeld + 1
	fMakesMeld  = fPassed + 1
	fFeedsNext  = fMakesMeld + 1
	fFeedsAny   = fFeedsNext + 1
	fNextSuit   = fFeedsAny + 1
	fNextRank   = fNextSuit + 1
	fDClean     = fNextRank + 1
	fDRuns      = fDClean + 1
	fDSets      = fDRuns + 1
	fDShort     = fDSets + 1
	fBuilds     = fDShort + 1
	fDistAfter  = fBuilds + 1
	fInferNext  = fDistAfter + 1
	fInferAny   = fInferNext + 1
	learnCand   = fInferAny + 1
	maxCands    = 64
	maxLayMelds = 16
	// maxLoosePickups bounds a down seat's loose pickups in a deal: see
	// pileTakes.
	maxLoosePickups = 3
	maxLayOffs      = 24
	maxReclaims     = 8
)

func (learnGame) CandDim() int { return learnCand }

var _ learn.Appending = learnGame{}

// EncoderPrefixes are the earlier encoders this one appended to: 900/62,
// before the card inference blocks (offInfer) and the two inference
// candidate features (fInferNext, fInferAny) — the zolik-v2 model's encoder.
// TestNarrowedV2ChoosesAsOnItsOwnEncoder holds the claim to recorded play.
func (learnGame) EncoderPrefixes() [][2]int { return [][2]int{{offInfer, fInferNext}} }

// How hard the opening search looks, in engine applications — an opening is
// found by laying it, not by reasoning about whether it could be laid.
const (
	openBudget     = 160 // from the meld phase
	takeOpenBudget = 50  // after each pile pickup, before going down
	maxOpenings    = 8
	maxTakeOpens   = 2
	maxOpenDepth   = 4
)

// Candidates are the moves this seat may make now.
//
// Undos never: a network, like the heuristic, is a function of the position,
// and an undo hands it back the position it just left. What keeps it out of
// the positions an undo exists to rescue is that no candidate leads into one:
// every candidate ends where a discard is accepted or the deal is over.
//
// The ones that need more than one action to get there are offered whole
// (learn.Candidate.Then): the first meld, laid meld by meld until the seat is
// down; a pickup from the pile before going down, which obliges the opening
// that spends it; and a joker bought back off the table, which must be played
// again before the turn can end. A NetBot plays them one step at a time and is
// asked again part-way; from there the only candidates that finish are the
// ways of finishing, because nothing that leaves the turn unfinishable is ever
// a candidate.
func (g learnGame) Candidates(raw module.State, seat string, offers []module.ActionOffer) ([]learn.Candidate, error) {
	at, err := g.Position(raw)
	if err != nil {
		return nil, err
	}
	return g.CandidatesFor(at, seat, offers)
}

// CandidatesFor is Candidates from a decoded position.
func (learnGame) CandidatesFor(at learn.Position, seat string, offers []module.ActionOffer) ([]learn.Candidate, error) {
	pos, err := positionOf(at)
	if err != nil {
		return nil, err
	}
	s, err := pos.state()
	if err != nil {
		return nil, err
	}
	offers = withoutUndos(offers)
	gs := s.Rules
	if s.Break.Open || gs.Status != rules.StatusActive || gs.CurrentTurn != seat {
		for _, o := range offers {
			if a, ok := module.SubmissionFor(o); ok {
				return []learn.Candidate{{Action: a, Features: kindOnly(kindForced)}}, nil
			}
		}
		return nil, nil
	}
	b := newBuilder(s, seat)
	var out []learn.Candidate
	switch gs.Phase {
	case rules.PhaseDraw:
		out = b.draws(offers)
	case rules.PhaseMeld, rules.PhaseDiscard:
		out = b.meldPhase(offers)
	}
	if len(out) > maxCands {
		out = out[:maxCands]
	}
	return out, nil
}

func withoutUndos(offers []module.ActionOffer) []module.ActionOffer {
	out := make([]module.ActionOffer, 0, len(offers))
	for _, o := range offers {
		if !o.Undo {
			out = append(out, o)
		}
	}
	return out
}

func kindOnly(kind int) []float32 {
	f := make([]float32, learnCand)
	f[kind] = 1
	return f
}

// builder holds what every candidate at one decision is described against.
type builder struct {
	gs     rules.GameState
	seat   string
	cfg    rules.RulesConfig
	hand   []string
	ext    [numCardSlot]bool
	unseen [numCardSlot]int
	wanted map[string]bool
	passed map[int]bool
	inMeld map[string]bool
	// req is this deal's contract and down whether I have met it; dist is my
	// hand's distance from it (unset once down). rivals are the other seats
	// in the order they play after me, the first of them the one my discard
	// is offered to.
	req    rules.ContractRequirement
	down   bool
	dist   distance
	rivals []*rival
	// pickups is how many times I have taken from the pile this deal.
	pickups int
	// est is the card inference for every other seat, in rivals' order.
	est cardinfer.Estimate
	// seenAfter dedupes plans by the position they leave: a lay-off that
	// happens to buy a joker back and an explicit swap of the same card are
	// one move.
	seenAfter map[string]bool
}

func newBuilder(s *matchState, seat string) *builder {
	gs := s.Rules
	cfg := rules.ResolveConfig(gs.Rules)
	vis := ai.VisibleFor(gs, s.Ledger, seat)
	b := &builder{gs: gs, seat: seat, cfg: cfg, hand: gs.Hands[seat],
		ext: extenders(gs.Melds, cfg), unseen: unseenCounts(vis, gs.Hands[seat], seat),
		wanted: map[string]bool{}, passed: map[int]bool{}, inMeld: map[string]bool{},
		seenAfter: map[string]bool{}}
	b.req = cfg.ContractFor(gs.GameNumber)
	b.down = gs.RoundReqMet[seat]
	b.pickups = vis.Pickups[seat]
	if !b.down {
		b.dist = distanceOf(b.hand, cfg, b.req, &b.unseen)
	}
	for _, id := range seatsFrom(gs.TurnOrder, seat)[1:] {
		b.rivals = append(b.rivals, rivalOf(vis, id, cfg))
	}
	ai.Infer(vis, b.hand, seat, &b.est)
	for id, cards := range vis.KnownHeld {
		if id == seat {
			continue
		}
		for _, c := range cards {
			if rules.IsJoker(c) {
				continue
			}
			b.wanted[c] = true
			r := rules.CardRank(c)
			for _, su := range learnSuits {
				b.wanted[string(rules.RankOrder[r])+su] = true
			}
			for _, adj := range []int{r - 1, r + 1} {
				if adj >= 0 && adj < numRanks {
					b.wanted[string(rules.RankOrder[adj])+rules.CardSuit(c)] = true
				}
			}
		}
	}
	for _, d := range vis.DealDiscards {
		if d.Player != seat {
			b.passed[rules.CardRank(d.Card)] = true
		}
	}
	for _, m := range handMelds(b.hand, cfg) {
		for _, c := range m.cards {
			b.inMeld[c] = true
		}
	}
	return b
}

// --- the engine, asked ---------------------------------------------------------

// apply puts one action to the engine. rules.ApplyAction is exactly what
// Module.Apply calls — the module adds the ledger and the ready-up between
// deals around it, neither of which can make it refuse — and it clones the
// state it is handed, so a search can branch from one position freely.
func (b *builder) apply(gs rules.GameState, a rules.Action) (rules.GameState, bool) {
	out, err := rules.ApplyAction(gs, b.seat, a)
	if err != nil {
		return gs, false
	}
	return out.State, true
}

func dealEnded(before, after rules.GameState) bool {
	return len(after.DealWinners) > len(before.DealWinners)
}

// finishable reports that the turn can still end from here: the deal is over,
// or some card in the hand is a discard the engine will take. The property
// every candidate is built to have, so it is asked of the engine rather than
// argued; a refusal that is not about the card tried (an unfinished opening,
// an unplayed joker) is a refusal of every card, and stops the asking.
func (b *builder) finishable(ns rules.GameState) bool {
	if dealEnded(b.gs, ns) || ns.Status != rules.StatusActive {
		return true
	}
	if ns.CurrentTurn != b.seat || (ns.Phase != rules.PhaseMeld && ns.Phase != rules.PhaseDiscard) {
		return false
	}
	hand := ns.Hands[b.seat]
	cards := uniqueCards(hand)
	// Naturals first: they are the cards a discard is almost always made of.
	sort.SliceStable(cards, func(i, j int) bool { return !rules.IsJoker(cards[i]) && rules.IsJoker(cards[j]) })
	for _, c := range cards {
		_, err := rules.ApplyAction(ns, b.seat, rules.Action{Type: rules.ActionDiscard, Card: c})
		if err == nil {
			return true
		}
		switch codeOfErr(err) {
		case rules.ErrJokerDiscard, rules.ErrCardNotInHand, rules.ErrDiscardTakenCard:
			continue
		}
		return false
	}
	return false
}

func codeOfErr(err error) rules.RulesErrorCode {
	if re, ok := err.(rules.RulesError); ok {
		return re.Code
	}
	return ""
}

// posKey names the position a plan leaves, for deduplication.
func posKey(gs rules.GameState, seat string) string {
	var sb strings.Builder
	sb.WriteString(strings.Join(sortedCopy(gs.Hands[seat]), ","))
	for _, owner := range sortedKeys(gs.Melds) {
		for _, m := range gs.Melds[owner] {
			sb.WriteString("|")
			sb.WriteString(strings.Join(sortedCopy(m), ","))
		}
	}
	return sb.String()
}

func sortedCopy(xs []string) []string {
	out := append([]string(nil), xs...)
	sort.Strings(out)
	return out
}

func uniqueCards(hand []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(hand))
	for _, c := range hand {
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	return out
}

// toModule is the module action for a rules action, with the offer id the
// module's own offer carries.
func toModule(a rules.Action) module.Action {
	out, _ := toModuleAction(a)
	switch a.Type {
	case rules.ActionLayMeld:
		out.OfferID = rules.OfferLayMeld
	case rules.ActionLayOff:
		out.OfferID = rules.LayOffOfferID(a.MeldID)
	case rules.ActionSwapJoker:
		out.OfferID = rules.SwapJokerOfferID(a.MeldID)
	case rules.ActionDiscard:
		out.OfferID = rules.OfferDiscard
	}
	return out
}

func candidateOf(steps []rules.Action, f []float32) learn.Candidate {
	c := learn.Candidate{Action: toModule(steps[0]), Features: f}
	for _, a := range steps[1:] {
		c.Then = append(c.Then, toModule(a))
	}
	return c
}

// --- the draw ------------------------------------------------------------------

// draws are the ways to start a turn: the stock, and every pickup the engine
// allows this seat. Before going down a pickup is an obligation — the card has
// to go into the opening this turn (rules.ErrDiscardCardNotMelded) — so it is
// offered only together with an opening that spends it, found from the
// position the pickup actually produces. Once down a pickup carries no
// obligation, and every one is offered: whether a card that lands nowhere yet
// is worth a turn is the policy's to learn, not this file's to decide.
func (b *builder) draws(offers []module.ActionOffer) []learn.Candidate {
	var out []learn.Candidate
	for _, o := range offers {
		if !o.Enabled {
			continue
		}
		switch o.ID {
		case rules.OfferDrawDeck:
			f := kindOnly(kindDrawDeck)
			f[fSteps] = 0.2
			f[fHandAfter] = float32(len(b.hand)+1) / 15
			f[fPenAfter] = float32(rules.HandPenaltyTotal(b.hand, b.cfg)) / 150
			if !b.down {
				f[fDistAfter] = float32(min(b.dist.total(b.req), 16)) / 8
			}
			out = append(out, learn.Candidate{Action: toModule(rules.Action{Type: rules.ActionDrawCard, DrawFrom: rules.DrawFromDeck}), Features: f})
		case rules.OfferDrawDiscard:
			if o.Source != nil {
				out = append(out, b.pileTakes(o.Source.Cards)...)
			}
		}
	}
	if len(out) > maxCands {
		// More pickups than the network is shown: keep the stock, and cut the
		// pickups that say least — deepest first among those that neither
		// make a meld, lay off, nor build. Stable, so the pile's own order
		// (nearest the top first) decides the rest.
		rank := func(c learn.Candidate) int {
			f := c.Features
			switch {
			case f[kindDrawDeck] == 1:
				return 3
			case f[fMakesMeld] == 1 || f[fExtends] == 1 || f[kindTakeOpen] == 1:
				return 2
			case f[fBuilds] == 1:
				return 1
			}
			return 0
		}
		sort.SliceStable(out, func(i, j int) bool { return rank(out[i]) > rank(out[j]) })
		out = out[:maxCands]
	}
	return out
}

// pileTakes are the pickups, nearest the top first. Under any_from_pile a
// card is taken with everything above it, and the engine takes the deepest
// copy of a card named twice, so what each costs is read off the pile the way
// the engine will read it.
func (b *builder) pileTakes(drawable []string) []learn.Candidate {
	pile := b.gs.DiscardPile
	anyFrom := b.cfg.DiscardPickupMode == rules.DiscardPickupAnyFromPile
	type take struct {
		card string
		at   int
	}
	var takes []take
	seen := map[string]bool{}
	for _, c := range drawable {
		if seen[c] {
			continue
		}
		seen[c] = true
		at := len(pile) - 1
		if anyFrom {
			for i, p := range pile {
				if p == c {
					at = i
					break
				}
			}
		}
		takes = append(takes, take{c, at})
	}
	sort.SliceStable(takes, func(i, j int) bool { return takes[i].at > takes[j].at })
	var out []learn.Candidate
	for _, t := range takes {
		a := rules.Action{Type: rules.ActionDrawCard, DrawFrom: rules.DrawFromDiscard}
		if anyFrom {
			a.Card = t.card
		}
		taken := pile[t.at:]
		combined := append(append([]string(nil), b.hand...), taken...)
		makes := false
		for _, m := range handMelds(combined, b.cfg) {
			if containsCard(m.cards, t.card) {
				makes = true
				break
			}
		}
		// Before going down the rule is that the card taken goes into this
		// turn's opening, so a card that makes no meld with the hand has no
		// opening to find: the engine would refuse every discard after it.
		if !b.down && !makes {
			continue
		}
		// Once down any pickup is legal, and every one is offered — but a
		// loose one (a card that neither makes a meld with the hand nor lays
		// off) only while this deal's pickups are few. Not a judgement of the
		// card: a bound on the deal. With every loose pickup open, a table of
		// down seats can trade the pile round for ever without a card leaving
		// the stock; random self-play did, for tens of thousands of actions.
		if loose := !makes && !b.ext[cardSlot(t.card)]; loose && b.pickups >= maxLoosePickups {
			continue
		}
		ns, ok := b.apply(b.gs, a)
		if !ok {
			continue
		}
		describe := func(f []float32) {
			b.markCard(f, t.card)
			f[fTaken] = float32(len(taken)) / 10
			f[fTakenPen] = float32(rules.HandPenaltyTotal(taken, b.cfg)) / 100
			f[fMakesMeld] = b2f(makes)
			f[fExtends] = b2f(b.ext[cardSlot(t.card)])
			closer := b.distanceChange(f, combined)
			if f[fGoesDown] == 1 {
				f[fDistAfter] = 0
			}
			joins := b.down && (f[fSameRank] > 0 || f[fSuitNear] > 0)
			f[fBuilds] = b2f(!makes && (closer || joins))
		}
		if b.down || dealEnded(b.gs, ns) || ns.RoundReqMet[b.seat] {
			if !b.finishable(ns) {
				continue
			}
			f := b.describe(kindTakePile, []rules.Action{a}, ns)
			describe(f)
			out = append(out, candidateOf([]rules.Action{a}, f))
			continue
		}
		for _, p := range b.openings(ns, []rules.Action{a}, takeOpenBudget, maxTakeOpens, true) {
			f := b.describe(kindTakeOpen, p.steps, p.after)
			describe(f)
			out = append(out, candidateOf(p.steps, f))
		}
	}
	return out
}

// --- the meld phase --------------------------------------------------------------

func (b *builder) meldPhase(offers []module.ActionOffer) []learn.Candidate {
	var out []learn.Candidate
	if !b.gs.RoundReqMet[b.seat] {
		for _, p := range b.openings(b.gs, nil, openBudget, maxOpenings, true) {
			out = append(out, candidateOf(p.steps, b.describe(kindOpening, p.steps, p.after)))
		}
	} else {
		out = append(out, b.layMelds()...)
		out = append(out, b.layOffs(offers)...)
		out = append(out, b.swaps(offers)...)
	}
	// Discards last in the building but never cut: they are what ends a turn,
	// and every one the engine lists is a candidate.
	disc := b.discards(offers)
	room := maxCands - len(disc)
	if room < 0 {
		room = 0
	}
	if len(out) > room {
		out = out[:room]
	}
	return append(out, disc...)
}

// layMelds are the new melds a down seat can lay, one per candidate — each
// its own decision, since the seat is asked again after it.
func (b *builder) layMelds() []learn.Candidate {
	var out []learn.Candidate
	for _, m := range handMelds(b.hand, b.cfg) {
		if len(out) >= maxLayMelds {
			break
		}
		steps := []rules.Action{{Type: rules.ActionLayMeld, Cards: m.cards}}
		ns, ok := b.apply(b.gs, steps[0])
		if !ok {
			continue
		}
		out = append(out, b.settle(kindLayMeld, steps, ns, "")...)
	}
	return out
}

// layOffs are the lay-offs the engine's offers list: each placement, with the
// company it needs where it cannot go alone, and each alternative company.
func (b *builder) layOffs(offers []module.ActionOffer) []learn.Candidate {
	var out []learn.Candidate
	tried := map[string]bool{}
	for _, o := range offers {
		if !o.Enabled || o.Verb != string(rules.VerbLayOff) || o.Source == nil || o.Target == nil {
			continue
		}
		for _, p := range o.Source.Placements {
			sets := [][]string{p.Requires}
			sets = append(sets, p.Alternatives...)
			for _, company := range sets {
				if len(out) >= maxLayOffs {
					return out
				}
				cards := append(append([]string(nil), company...), p.Card)
				key := o.Target.MeldID + ":" + strings.Join(sortedCopy(cards), ",")
				if tried[key] {
					continue
				}
				tried[key] = true
				steps := []rules.Action{{Type: rules.ActionLayOff, MeldID: o.Target.MeldID, Cards: cards}}
				ns, ok := b.apply(b.gs, steps[0])
				if !ok {
					continue
				}
				out = append(out, b.settle(kindLayOff, steps, ns, p.Card)...)
			}
		}
	}
	return out
}

// swaps are the explicit joker reclaims the engine offers, each completed by
// playing the joker again.
func (b *builder) swaps(offers []module.ActionOffer) []learn.Candidate {
	var out []learn.Candidate
	for _, o := range offers {
		if !o.Enabled || o.Verb != string(rules.VerbSwapJoker) || o.Source == nil || o.Target == nil {
			continue
		}
		for _, c := range o.Source.Cards {
			if len(out) >= maxReclaims {
				return out
			}
			steps := []rules.Action{{Type: rules.ActionSwapJoker, MeldID: o.Target.MeldID, Card: c}}
			ns, ok := b.apply(b.gs, steps[0])
			if !ok {
				continue
			}
			out = append(out, b.settle(kindReclaim, steps, ns, c)...)
		}
	}
	return out
}

// settle turns one applied step into candidates. A step that leaves the turn
// finishable is a candidate as it is. One that bought a joker back — an
// explicit swap, or a lay-off whose card took a joker's exact place, which the
// engine turns into a swap — leaves a joker that must be played before the
// turn can end, so it is offered only together with a place for that joker,
// found by laying it. Anything else unfinishable is not a candidate at all.
func (b *builder) settle(kind int, steps []rules.Action, ns rules.GameState, focus string) []learn.Candidate {
	reclaimed := len(ns.JokersReclaimedPendingMeld) > len(b.gs.JokersReclaimedPendingMeld)
	if b.finishable(ns) {
		if reclaimed {
			kind = kindReclaim
		}
		return b.keep(kind, steps, ns, focus)
	}
	if !reclaimed && len(ns.JokersReclaimedPendingMeld) == 0 {
		return nil
	}
	var out []learn.Candidate
	for _, next := range b.jokerHomes(ns) {
		if len(out) >= 2 {
			break
		}
		after, ok := b.apply(ns, next)
		if !ok || !b.finishable(after) {
			continue
		}
		out = append(out, b.keep(kindReclaim, append(append([]rules.Action(nil), steps...), next), after, focus)...)
	}
	return out
}

// jokerHomes are the single actions that could play an owed joker: onto each
// meld on the table, or in a new meld the hand can make with it.
func (b *builder) jokerHomes(ns rules.GameState) []rules.Action {
	hand := ns.Hands[b.seat]
	var jokers []string
	for _, j := range uniqueCards(ns.JokersReclaimedPendingMeld) {
		if containsCard(hand, j) {
			jokers = append(jokers, j)
		}
	}
	var out []rules.Action
	for _, j := range jokers {
		for _, owner := range sortedKeys(ns.MeldMeta) {
			for _, mi := range ns.MeldMeta[owner] {
				out = append(out, rules.Action{Type: rules.ActionLayOff, MeldID: mi.MeldID, Cards: []string{j}})
			}
		}
		for _, m := range handMelds(hand, b.cfg) {
			if containsCard(m.cards, j) {
				out = append(out, rules.Action{Type: rules.ActionLayMeld, Cards: m.cards})
			}
		}
	}
	return out
}

func (b *builder) keep(kind int, steps []rules.Action, ns rules.GameState, focus string) []learn.Candidate {
	key := posKey(ns, b.seat)
	if b.seenAfter[key] {
		return nil
	}
	b.seenAfter[key] = true
	f := b.describe(kind, steps, ns)
	if focus != "" {
		b.markCard(f, focus)
		f[fExtends] = b2f(b.ext[cardSlot(focus)])
	}
	return []learn.Candidate{candidateOf(steps, f)}
}

// discards are one candidate per distinct card the engine will take as this
// turn's discard. The offer lists exactly those, jokers included where the
// engine allows one — which is what lets the network learn not to.
//
// Read from the offer's card list even when the offer itself says disabled.
// The list is the engine's own per-card probe (rules.discardableCards); the
// verb's flag comes from rules.probeDiscard, which stops at the first card
// refused for a reason it does not count as card-specific — and
// DISCARD_TAKEN_CARD_FORBIDDEN, which is about exactly one card, is not in its
// list. So a down seat that took a card off the pile, and holds that card
// first, is shown the discard greyed out with other legal discards listed
// under it. Trusting the flag there would leave the turn with no candidate
// that ends it.
func (b *builder) discards(offers []module.ActionOffer) []learn.Candidate {
	var out []learn.Candidate
	for _, o := range offers {
		if o.Verb != string(rules.VerbDiscard) || o.Source == nil {
			continue
		}
		for _, card := range uniqueCards(o.Source.Cards) {
			rest := removeOnce(b.hand, card)
			f := kindOnly(kindDiscard)
			b.markCard(f, card)
			f[fSteps] = 0.2
			f[fGoesOut] = b2f(len(rest) == 0)
			f[fHandAfter] = float32(len(rest)) / 15
			f[fPenAfter] = float32(rules.HandPenaltyTotal(rest, b.cfg)) / 150
			f[fPenShed] = float32(rules.PenaltyPoints(card, false)) / 100
			f[fExtends] = b2f(b.ext[cardSlot(card)])
			if len(rest) > 0 {
				b.distanceChange(f, rest)
			}
			out = append(out, learn.Candidate{
				Action:   toModule(rules.Action{Type: rules.ActionDiscard, Card: card}),
				Features: f,
			})
		}
	}
	return out
}

// markCard describes the card a move is about: what it is, and what the table
// and the hand say about it.
func (b *builder) markCard(f []float32, card string) {
	if rules.IsJoker(card) {
		f[fCard+numRanks+numSuits] = 1
		f[fIsJoker] = 1
	} else {
		r := rules.CardRank(card)
		if r >= 0 {
			f[fCard+r] = 1
		}
		for i, s := range learnSuits {
			if rules.CardSuit(card) == s {
				f[fCard+numRanks+i] = 1
			}
		}
		f[fIsAce] = b2f(rules.IsAce(card))
		f[fWanted] = b2f(b.wanted[card])
		f[fPassed] = b2f(b.passed[r])
	}
	if k := cardSlot(card); k >= 0 {
		f[fUnseen] = float32(b.unseen[k]) / 2
	}
	same, near := 0, 0
	skipped := false
	for _, h := range b.hand {
		if h == card && !skipped {
			skipped = true
			continue
		}
		if rules.IsJoker(h) || rules.IsJoker(card) {
			continue
		}
		if rules.CardRank(h) == rules.CardRank(card) {
			same++
		} else if rules.CardSuit(h) == rules.CardSuit(card) {
			if d := rules.CardRank(h) - rules.CardRank(card); d >= -2 && d <= 2 {
				near++
			}
		}
	}
	f[fSameRank] = float32(same) / 4
	f[fSuitNear] = float32(near) / 4
	f[fInMeld] = b2f(b.inMeld[card])
	if k := cardSlot(card); k >= 0 {
		for i, r := range b.rivals {
			if i == 0 {
				f[fFeedsNext] = r.wants[k]
				sn, sr := r.nearTaken(card)
				f[fNextSuit] = float32(min(sn, 4)) / 2
				f[fNextRank] = float32(min(sr, 4)) / 2
			}
			f[fFeedsAny] = max(f[fFeedsAny], r.wants[k])
		}
		next, anyone := b.est.Feed(k)
		f[fInferNext], f[fInferAny] = float32(next), float32(anyone)
	}
}

// distanceChange describes a hand change before going down: how much nearer
// each contract part is with the new hand than with mine now (negative is
// further), and how far it all still is. Nothing once down.
func (b *builder) distanceChange(f []float32, after []string) (closer bool) {
	if b.down {
		return false
	}
	d := distanceOf(after, b.cfg, b.req, &b.unseen)
	f[fDClean] = float32(b.dist.clean-d.clean) / 4
	f[fDRuns] = float32(b.dist.runs-d.runs) / 4
	f[fDSets] = float32(b.dist.sets-d.sets) / 4
	f[fDShort] = float32(b.dist.short-d.short) / 70
	f[fDistAfter] = float32(min(d.total(b.req), 16)) / 8
	return d.clean < b.dist.clean || d.runs < b.dist.runs || d.sets < b.dist.sets || d.short < b.dist.short
}

// describe is what a plan did, read off the table before it and after it —
// both of which exist, because a plan is found by playing it.
func (b *builder) describe(kind int, steps []rules.Action, after rules.GameState) []float32 {
	f := kindOnly(kind)
	f[fSteps] = float32(len(steps)) / 5
	before := b.gs
	cardsLaid, wildsLaid, value, newMelds := 0, 0, 0, 0
	clean := false
	was := map[string][]string{}
	for owner, metas := range before.MeldMeta {
		for i, mi := range metas {
			if i < len(before.Melds[owner]) {
				was[mi.MeldID] = before.Melds[owner][i]
			}
		}
	}
	for owner, metas := range after.MeldMeta {
		for i, mi := range metas {
			if i >= len(after.Melds[owner]) {
				continue
			}
			now := after.Melds[owner][i]
			old, existed := was[mi.MeldID]
			if existed && len(old) == len(now) && jokersIn(old) == jokersIn(now) {
				continue
			}
			if !existed {
				newMelds++
			}
			cardsLaid += len(now) - len(old)
			wildsLaid += jokersIn(now) - jokersIn(old)
			nv, _ := rules.ValidateMeld(now, b.cfg)
			value += nv.NaturalValue
			if existed {
				ov, _ := rules.ValidateMeld(old, b.cfg)
				value -= ov.NaturalValue
			}
			if !existed && nv.Type == rules.MeldRun && nv.WildCount == 0 {
				clean = true
			}
		}
	}
	f[fCards] = float32(cardsLaid) / 10
	f[fWilds] = float32(wildsLaid) / 4
	f[fValue] = float32(value) / 50
	f[fNewMelds] = float32(newMelds) / 3
	f[fCleanRun] = b2f(clean)
	ended := dealEnded(before, after)
	f[fGoesDown] = b2f(!before.RoundReqMet[b.seat] && (after.RoundReqMet[b.seat] || ended))
	if floor := b.cfg.InitialMeldMinimum; floor > 0 && !ended {
		f[fMeetsMin] = float32(min(rules.PlayerInitialMeldNaturalValue(after, b.seat), 2*floor)) / float32(floor)
	} else {
		f[fMeetsMin] = 1
	}
	f[fGoesOut] = b2f(ended && len(after.DealWinners) > 0 && after.DealWinners[len(after.DealWinners)-1] == b.seat)
	reclaims := 0
	for _, a := range steps {
		if a.Type == rules.ActionSwapJoker {
			reclaims++
		}
	}
	if after.LastLayOff != nil {
		reclaims += len(after.LastLayOff.ReclaimedJokers)
	}
	f[fReclaims] = float32(reclaims)
	penBefore := rules.HandPenaltyTotal(b.hand, b.cfg)
	if !ended {
		hand := after.Hands[b.seat]
		f[fHandAfter] = float32(len(hand)) / 15
		pen := rules.HandPenaltyTotal(hand, b.cfg)
		f[fPenAfter] = float32(pen) / 150
		f[fPenShed] = float32(penBefore-pen) / 100
	} else {
		f[fPenShed] = float32(penBefore) / 100
	}
	return f
}

func jokersIn(cards []string) int {
	n := 0
	for _, c := range cards {
		if rules.IsJoker(c) {
			n++
		}
	}
	return n
}

func containsCard(cards []string, c string) bool {
	for _, x := range cards {
		if x == c {
			return true
		}
	}
	return false
}

func removeOnce(hand []string, card string) []string {
	out := make([]string, 0, len(hand))
	done := false
	for _, c := range hand {
		if c == card && !done {
			done = true
			continue
		}
		out = append(out, c)
	}
	return out
}

// --- going down ------------------------------------------------------------------

type openingFound struct {
	steps []rules.Action
	after rules.GameState
	keys  []string
}

// openings are the complete first melds from a position: the melds, laid back
// to back, that take the seat down with a discard (or the deal's end) still to
// be had, each prefixed with the steps that reached the position — a pickup,
// or nothing.
//
// The heuristic's own plan (ai.InitialMeldPlan, its searchMeldCombo) is tried
// first where asked for: it is the opening the hand-written bot would lay, so
// the network always has that one to compare against. Then an iterative-
// deepening walk over every meld the hand holds, one meld deep before two, so
// the openings found are the smallest ones — a plan that merely adds a meld to
// an opening already found is the same opening followed by a lay, and the seat
// can make that lay as its next decision. Every step of every plan is put to
// the engine from the position the step before it left; the engine is the only
// judge, and a plan counts only once it has said the seat is down and a
// discard is still accepted.
//
// Every position part-way through a plan found here is one this search can
// finish from — the rest of the plan is a shallower walk over the same melds —
// which is what a NetBot playing a plan a step at a time relies on.
func (b *builder) openings(from rules.GameState, prefix []rules.Action, budget, max int, seed bool) []openingFound {
	o := &openingSearch{b: b, budget: budget, max: max, seenPlan: map[string]bool{}}
	if seed {
		o.seedHeuristic(from, prefix)
	}
	for depth := 1; depth <= maxOpenDepth && o.budget > 0 && len(o.plans) < o.max; depth++ {
		o.visited = map[string]bool{}
		o.walk(from, prefix, nil, depth)
	}
	return o.plans
}

type openingSearch struct {
	b        *builder
	budget   int
	max      int
	plans    []openingFound
	seenPlan map[string]bool
	visited  map[string]bool
}

func (o *openingSearch) seedHeuristic(from rules.GameState, prefix []rules.Action) {
	b := o.b
	must := ""
	if !from.RoundReqMet[b.seat] {
		must = from.DiscardDrawnCardPendingMeld
	}
	plan, ok := ai.InitialMeldPlan(from, b.seat, from.Hands[b.seat], must)
	if !ok || len(plan) == 0 || len(plan) > maxOpenDepth+1 {
		return
	}
	state := from
	path := append([]rules.Action(nil), prefix...)
	var keys []string
	for _, m := range plan {
		o.budget--
		a := rules.Action{Type: rules.ActionLayMeld, Cards: append([]string(nil), m...)}
		ns, ok := b.apply(state, a)
		if !ok {
			return
		}
		state, path, keys = ns, append(path, a), append(keys, meldKey(m))
		if dealEnded(from, ns) || ns.RoundReqMet[b.seat] {
			o.record(path, ns, keys)
			return
		}
	}
}

func (o *openingSearch) record(path []rules.Action, after rules.GameState, keys []string) {
	if !o.b.finishable(after) {
		return
	}
	ks := sortedCopy(keys)
	id := strings.Join(ks, " ")
	if o.seenPlan[id] {
		return
	}
	for _, p := range o.plans {
		if subset(p.keys, ks) {
			return
		}
	}
	o.seenPlan[id] = true
	o.plans = append(o.plans, openingFound{steps: append([]rules.Action(nil), path...), after: after, keys: ks})
}

func (o *openingSearch) walk(state rules.GameState, path []rules.Action, keys []string, depth int) {
	b := o.b
	for _, m := range handMelds(state.Hands[b.seat], b.cfg) {
		if o.budget <= 0 || len(o.plans) >= o.max {
			return
		}
		ks := sortedCopy(append(append([]string(nil), keys...), m.key))
		id := strings.Join(ks, " ")
		if o.visited[id] {
			continue
		}
		o.visited[id] = true
		if o.coveredBy(ks) {
			continue
		}
		o.budget--
		a := rules.Action{Type: rules.ActionLayMeld, Cards: m.cards}
		ns, ok := b.apply(state, a)
		if !ok {
			continue
		}
		p := append(append([]rules.Action(nil), path...), a)
		if dealEnded(b.gs, ns) || ns.RoundReqMet[b.seat] {
			if depth == 1 {
				o.record(p, ns, ks)
			}
			continue // down: building on it is the next decision
		}
		if depth > 1 {
			o.walk(ns, p, ks, depth-1)
		}
	}
}

// coveredBy reports that these melds already contain a whole opening found.
func (o *openingSearch) coveredBy(ks []string) bool {
	for _, p := range o.plans {
		if subset(p.keys, ks) {
			return true
		}
	}
	return false
}

// subset reports that every key of a is in b, counting duplicates.
func subset(a, b []string) bool {
	left := map[string]int{}
	for _, k := range b {
		left[k]++
	}
	for _, k := range a {
		if left[k] == 0 {
			return false
		}
		left[k]--
	}
	return true
}

// --- what melds a hand holds ------------------------------------------------------

type handMeld struct {
	cards []string
	key   string
	wilds int
	value int
}

func meldKey(cards []string) string { return strings.Join(sortedCopy(cards), ",") }

// handMelds is every distinct meld the hand could lay, cheapest in jokers
// first, then worth most, then longest — the order the heuristic's own search
// prefers, for its reason: an opening that keeps the joker is the better one.
//
// Proposed by shape and kept only if rules.ValidateMeld accepts it, so the
// shapes below are a way of not trying every combination of fourteen cards,
// never a statement of what is legal. Sets: each rank's distinct suits, any
// non-empty subset, with up to as many jokers as naturals. Runs: each suit's
// windows of the run ranks 1..14 (the ace at either end), every gap a joker.
func handMelds(hand []string, cfg rules.RulesConfig) []handMeld {
	minSet, minRun := cfg.MinSetSize, cfg.MinRunSize
	if minSet == 0 {
		minSet = 3
	}
	if minRun == 0 {
		minRun = 4
	}
	var jokers []string
	byRank := map[byte]map[string]string{}
	bySuit := map[string]map[int]int{} // suit -> run rank -> copies
	for _, c := range hand {
		if rules.IsJoker(c) {
			jokers = append(jokers, c)
			continue
		}
		r, s := c[0], rules.CardSuit(c)
		if byRank[r] == nil {
			byRank[r] = map[string]string{}
		}
		byRank[r][s] = c
		if bySuit[s] == nil {
			bySuit[s] = map[int]int{}
		}
		if rules.IsAce(c) {
			bySuit[s][1]++
			bySuit[s][14]++
		} else {
			bySuit[s][rules.CardRank(c)+2]++
		}
	}
	seen := map[string]bool{}
	var out []handMeld
	add := func(cards []string) {
		key := meldKey(cards)
		if seen[key] {
			return
		}
		seen[key] = true
		mv, err := rules.ValidateMeld(cards, cfg)
		if err != nil {
			return
		}
		out = append(out, handMeld{cards: cards, key: key, wilds: mv.WildCount, value: mv.NaturalValue})
	}
	for i := 0; i < len(rules.RankOrder); i++ {
		suits := byRank[rules.RankOrder[i]]
		if len(suits) == 0 {
			continue
		}
		var naturals []string
		for _, s := range learnSuits {
			if c, ok := suits[s]; ok {
				naturals = append(naturals, c)
			}
		}
		for mask := 1; mask < 1<<len(naturals); mask++ {
			var pick []string
			for j, c := range naturals {
				if mask&(1<<j) != 0 {
					pick = append(pick, c)
				}
			}
			for w := 0; w <= len(jokers) && w <= len(pick) && len(pick)+w <= rules.MaxSetSize; w++ {
				if len(pick)+w < minSet {
					continue
				}
				add(append(append([]string(nil), pick...), jokers[:w]...))
			}
		}
	}
	for _, s := range learnSuits {
		have := bySuit[s]
		if len(have) == 0 {
			continue
		}
		for start := 1; start <= 14; start++ {
			for end := start + minRun - 1; end <= 14; end++ {
				if start == 1 && end == 14 {
					continue
				}
				var cards []string
				naturals, wilds := 0, 0
				prevWild, ok := false, true
				for r := start; r <= end; r++ {
					if have[r] > 0 {
						cards = append(cards, runRankCard(r, s))
						naturals++
						prevWild = false
						continue
					}
					if prevWild || wilds >= len(jokers) {
						ok = false
						break
					}
					cards = append(cards, jokers[wilds])
					wilds++
					prevWild = true
				}
				if !ok || naturals == 0 || wilds > naturals {
					continue
				}
				add(cards)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].wilds != out[j].wilds {
			return out[i].wilds < out[j].wilds
		}
		if out[i].value != out[j].value {
			return out[i].value > out[j].value
		}
		if len(out[i].cards) != len(out[j].cards) {
			return len(out[i].cards) > len(out[j].cards)
		}
		return out[i].key < out[j].key
	})
	return out
}

// --- Reward --------------------------------------------------------------------

// scoresOnly is the part of a state Reward reads: decoding the whole of it
// twice per learner seat per action would be most of what the environment
// spends its time on.
type scoresOnly struct {
	Rules struct {
		TurnOrder  []string
		GameScores map[string][]int
	} `json:"rules"`
}

// rewardScale turns penalty points into a reward of order one: a deal's
// penalty is tens of points, a bad one a hundred or more.
const rewardScale = 100

// Reward is paid once a deal, when it is scored: the best (lowest) deal
// penalty among the other seats, minus this seat's own, over rewardScale. So
// going out (a penalty of zero) is paid what the nearest rival was left
// holding, and every point kept in hand costs the same one-hundredth. Nothing
// in between — a meld laid is only worth something if the deal ends with the
// hand emptier for it.
//
// Every deal closes an episode, for every seat, exactly once: the action that
// scores the deal (a go-out discard, or a meld-out on a final deal) appends one
// row to every seat's GameScores, and nothing else does.
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
	b, err := bp.scoreSheet()
	if err != nil {
		return 0, false, err
	}
	a, err := ap.scoreSheet()
	if err != nil {
		return 0, false, err
	}
	mine := a.Rules.GameScores[seat]
	if len(mine) <= len(b.Rules.GameScores[seat]) {
		return 0, false, nil
	}
	own := mine[len(mine)-1]
	best, found := 0, false
	for _, id := range a.Rules.TurnOrder {
		if id == seat {
			continue
		}
		sc := a.Rules.GameScores[id]
		if len(sc) == 0 {
			continue
		}
		if p := sc[len(sc)-1]; !found || p < best {
			best, found = p, true
		}
	}
	return float32(best-own) / rewardScale, true, nil
}
