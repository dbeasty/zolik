package holdem

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"strconv"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
)

// Hold'em as the learning machinery sees it (internal/learn).
//
// Three translations and nothing else: a decision into numbers (Encode), the
// offers into a short list of concrete moves (Candidates), and a finished hand
// into chips (Reward). The rules stay in the engine; every candidate here is
// built from an enabled offer, clamped to the range that offer declares,
// exactly the way the hand-written bot builds its moves (bot.go's menu).
//
// The encoder is written to the same discipline as the bot, and more strictly,
// because a network cannot be read to check what it looked at. It is handed
// the whole state and reads only what the seat could know: its own two cards,
// the board, the chips, the public record (history.go). The equity it reports
// is a rollout over the cards this seat cannot see, dealt from a fresh deck
// minus its own cards and the board, with a coin seeded from public facts —
// never from the state's deck, whose order *is* the future, and never from the
// match seed, which fixes it. TestEncodeDoesNotPeek pins all of that.
type learnGame struct{}

var (
	_ learn.Game       = learnGame{}
	_ learn.Styled     = learnGame{}
	_ learn.Positional = learnGame{}
)

func init() { learn.Register(learnGame{}) }

// errNotHoldem is the encoder declining a poker game that is not Hold'em.
//
// The network was trained on two hole cards and a board, and the observation
// has a slot for exactly that: given Five-Card Draw's five cards it would read
// two of them and an empty board and play a hand that does not exist. Refusing
// the position is what hands the decision back to the rule bot (learn.NetBot
// falls back on any error), which knows every game this module deals.
var errNotHoldem = errors.New("holdem: the learned model plays Texas Hold'em only")

func (learnGame) Name() string              { return "holdem" }
func (learnGame) Module() module.GameModule { return New() }
func (learnGame) Heuristic() module.Bot     { return bot{} }

// Config is the table the bot ladder has always been measured on
// (bot_strategy_test.go's headToHead): a 15-hand timed match, fifty big blinds
// deep. Short enough that a sweep of hundreds of seeds is quick, long enough
// that a hand's result is not the whole match.
func (learnGame) Config(_ int, variation string) module.MatchConfig {
	if variation == "" {
		variation = "timed"
	}
	return module.MatchConfig{
		Variation: variation,
		Options:   module.Options{OptStartingStack: 1000, OptBigBlind: 20, OptHandLimit: 15},
	}
}

// ShortConfig is Config at fifteen to twenty-five big blinds deep, a step of
// one big blind picked by roll: the depths where the jam-or-fold chart starts
// to be the game (pushfold.go), for training only (learn.EnvOptions).
func (g learnGame) ShortConfig(seats int, variation string, roll float64) module.MatchConfig {
	cfg := g.Config(seats, variation)
	bb := cfg.Options[OptBigBlind]
	depth := 15 + int(roll*11)
	opts := module.Options{}
	for k, v := range cfg.Options {
		opts[k] = v
	}
	opts[OptStartingStack] = min(depth, 25) * bb
	cfg.Options = opts
	return cfg
}

// Outcome is chips won or lost, in big blinds.
func (learnGame) Outcome(raw module.State, seat string) (float64, error) {
	s, err := decode(raw)
	if err != nil {
		return 0, err
	}
	for _, st := range s.Seats {
		if st.PlayerID == seat {
			return float64(st.Stack-s.StartingStack) / float64(s.BigBlind), nil
		}
	}
	return 0, fmt.Errorf("holdem: no seat %q", seat)
}

// --- the observation ---------------------------------------------------------

// The observation's layout. Every block starts where the last one ended, and
// the Python trainer reads these offsets, so a change here is a new encoder:
// StateDim moves and every saved network refuses to load, which is the point.
const (
	// Hole cards, highest rank first: a rank one-hot and a suit one-hot each.
	encHole     = 0
	encHoleSlot = 13 + 4
	// Pair, suited, and the gap between the ranks.
	encShape = encHole + 2*encHoleSlot
	// The board as a 52-card presence vector (rank*4 + suit).
	encBoard = encShape + 3
	// Street one-hot: preflop, flop, turn, river.
	encStreet = encBoard + 52
	// The made hand's category, one-hot over the nine (high card .. straight
	// flush). Before the flop, a pocket pair is a pair.
	encMade = encStreet + 4
	// Chen score, rollout equity against random hands, draw outs.
	encStrength = encMade + 9
	// Seats still to act behind, opponents in the hand, opponents able to
	// act, seats left in the match, on the button, share of the match left.
	encTable = encStrength + 3
	// Pot, own stack, to call, effective stack — each as bb/50 and as
	// log1p(bb)/5 — then pot odds, stack-to-pot ratio, own chips in the hand.
	encMoney = encTable + 6
	// This seat raised last on this street; someone else did; this seat was
	// the last to raise before the flop; raises so far this street.
	encAggr = encMoney + 11
	// The last encLogLen public actions, newest first, relative to this seat.
	encLog      = encAggr + 4
	encLogLen   = 12
	encLogEntry = 13
	// Opponent reads: the mean over opponents still in the hand, then the
	// opponent who last raised into this seat, then whether there is one.
	encReads    = encLog + encLogLen*encLogEntry
	encReadsLen = 6

	stateDim = encReads + 2*encReadsLen + 1
)

// Candidate features.
const (
	// Kind one-hot: fold, check, call, raise to half, three quarters, one and
	// two pots, all in.
	candKind  = 0
	candKinds = 8
	// log1p(raise-to in bb)/5, the raise's size as a fraction of the pot
	// after the call (capped at 4, halved), log1p(the raise in bb)/5, share
	// of the seat's chips in the hand once this is in, all in, log1p(chips
	// put in by this move, in bb)/5.
	candNum = candKind + candKinds

	candDim = candNum + 6
)

// Candidate kinds, in one-hot order.
const (
	kindFold = iota
	kindCheck
	kindCall
	kindHalfPot
	kindThreeQuarterPot
	kindPot
	kindTwoPots
	kindAllIn
)

// potSizes are the raises offered, as fractions of the pot after the call.
var potSizes = []struct {
	kind     int
	fraction float64
}{{kindHalfPot, 0.5}, {kindThreeQuarterPot, 0.75}, {kindPot, 1}, {kindTwoPots, 2}}

func (learnGame) StateDim() int { return stateDim }
func (learnGame) CandDim() int  { return candDim }

// encodeTrials is the rollout count behind the equity feature. Under the
// bot's own (rollouts: 160-400), because Encode runs on every decision the
// trainer sees and a network can average out noise that a threshold cannot:
// at 200 the standard error is about three and a half points.
const encodeTrials = 200

// --- one decode, any seat ---------------------------------------------------

// position is a state decoded for learn.Positional: the whole GameState when
// a seat's encoding or candidates are asked for, and only the hand tally when
// a reward is — each at most once, however many seats ask.
type position struct {
	raw   module.State
	full  learn.Memo[*GameState]
	tally learn.Memo[*handTally]
}

func (learnGame) Position(raw module.State) (learn.Position, error) {
	return &position{raw: raw}, nil
}

func positionOf(p learn.Position) (*position, error) {
	if pos, ok := p.(*position); ok && pos != nil {
		return pos, nil
	}
	return nil, fmt.Errorf("holdem: %T is not a Hold'em position", p)
}

// state is the decoded GameState, shared by every reader: read it, never
// write it.
func (p *position) state() (*GameState, error) {
	return p.full.Get(func() (*GameState, error) { return decode(p.raw) })
}

func (p *position) hands() (*handTally, error) {
	return p.tally.Get(func() (*handTally, error) {
		var t handTally
		if err := json.Unmarshal(p.raw, &t); err != nil {
			return nil, fmt.Errorf("holdem: decode state: %w", err)
		}
		return &t, nil
	})
}

// Encode is the decision as this seat sees it. See the layout above.
func (g learnGame) Encode(raw module.State, playerID string) ([]float32, error) {
	p, err := g.Position(raw)
	if err != nil {
		return nil, err
	}
	return g.EncodeFor(p, playerID)
}

// EncodeFor is Encode from a decoded position.
func (learnGame) EncodeFor(p learn.Position, playerID string) ([]float32, error) {
	pos, err := positionOf(p)
	if err != nil {
		return nil, err
	}
	s, err := pos.state()
	if err != nil {
		return nil, err
	}
	if !s.rules().board {
		return nil, errNotHoldem
	}
	me := s.seatIndex(playerID)
	if me < 0 {
		return nil, fmt.Errorf("holdem: no seat %q", playerID)
	}
	seat := &s.Seats[me]
	out := make([]float32, stateDim)
	bb := float64(max(s.BigBlind, 1))

	// --- cards ---
	hole := seat.Hole
	if len(hole) == 2 && rankValue[rankOf(hole[1])] > rankValue[rankOf(hole[0])] {
		hole = []string{hole[1], hole[0]}
	}
	for i, c := range hole {
		if i >= 2 {
			break
		}
		if r, su, ok := cardIndex(c); ok {
			out[encHole+i*encHoleSlot+r] = 1
			out[encHole+i*encHoleSlot+13+su] = 1
		}
	}
	if len(hole) == 2 {
		hi, lo := rankValue[rankOf(hole[0])], rankValue[rankOf(hole[1])]
		out[encShape] = b2f(hi == lo)
		out[encShape+1] = b2f(suitOf(hole[0]) == suitOf(hole[1]))
		out[encShape+2] = float32(hi-lo) / 12
	}
	for _, c := range s.Board {
		if r, su, ok := cardIndex(c); ok {
			out[encBoard+r*4+su] = 1
		}
	}
	if st := streetIndex(s.Street); st < 4 {
		out[encStreet+st] = 1
	}
	out[encMade+madeCategory(hole, s.Board)] = 1

	// --- strength ---
	opponents := opponentsOf(s)
	out[encStrength] = float32((chen(hole) + 1.5) / 21.5)
	if len(hole) == 2 {
		rnd := seededRand(publicSeed(s, me))
		out[encStrength+1] = float32(equity(hole, s.Board, opponents, encodeTrials, highCard, 0, rnd))
		releaseRand(rnd)
	}
	out[encStrength+2] = float32(drawOuts(hole, s.Board)) / 17

	// --- the table ---
	able, live := 0, len(s.liveSeats())
	for i := range s.Seats {
		if i != me && s.Seats[i].canAct() {
			able++
		}
	}
	out[encTable] = float32(behindOf(s, me)) / 8
	out[encTable+1] = float32(opponents) / 8
	out[encTable+2] = float32(able) / 8
	out[encTable+3] = float32(live) / 9
	out[encTable+4] = b2f(s.Button == me)
	if s.HandLimit > 0 {
		out[encTable+5] = float32(math.Max(0, float64(s.HandLimit-s.HandNumber)) / float64(s.HandLimit))
	}

	// --- chips ---
	pot, owed := potNow(s), s.toCall(seat)
	// Effective stack: what this seat could still lose to the deepest
	// opponent in the hand, which is the most either can win off the other.
	deepest := 0
	for i := range s.Seats {
		if i != me && s.Seats[i].inHand() && s.Seats[i].Stack+s.Seats[i].Bet > deepest {
			deepest = s.Seats[i].Stack + s.Seats[i].Bet
		}
	}
	effective := max(min(deepest, seat.Stack+seat.Bet)-seat.Bet, 0)
	for i, chips := range []int{pot, seat.Stack, owed, effective} {
		x := float64(chips) / bb
		out[encMoney+2*i] = float32(x / 50)
		out[encMoney+2*i+1] = float32(math.Log1p(x) / 5)
	}
	if owed > 0 {
		out[encMoney+8] = float32(owed) / float32(pot+owed)
	}
	if pot > 0 {
		out[encMoney+9] = float32(math.Min(float64(effective)/float64(pot), 20) / 20)
	}
	out[encMoney+10] = float32(float64(seat.Committed) / bb / 50)

	// --- who has been betting ---
	out[encAggr] = b2f(s.Aggressor == me)
	out[encAggr+1] = b2f(s.Aggressor >= 0 && s.Aggressor != me)
	raises, lastPre := 0, -1
	for _, a := range s.HandLog {
		if a.Verb != VerbRaise {
			continue
		}
		if a.Street == s.Street {
			raises++
		}
		if a.Street == streetPreflop {
			lastPre = a.Seat
		}
	}
	out[encAggr+2] = b2f(lastPre == me)
	out[encAggr+3] = float32(math.Min(float64(raises), 4) / 4)

	// --- the hand so far ---
	n := len(s.Seats)
	for k := 0; k < encLogLen && k < len(s.HandLog); k++ {
		a := s.HandLog[len(s.HandLog)-1-k]
		f := out[encLog+k*encLogEntry : encLog+(k+1)*encLogEntry]
		f[0] = 1
		f[1] = b2f(a.Seat == me)
		f[2] = float32((a.Seat-me+n)%n) / float32(n)
		if st := streetIndex(a.Street); st < 4 {
			f[3+st] = 1
		}
		switch a.Verb {
		case VerbFold:
			f[7] = 1
		case VerbCheck:
			f[8] = 1
		case VerbCall:
			f[9] = 1
		case VerbRaise:
			f[10] = 1
		}
		switch {
		case a.Verb == VerbRaise:
			f[11] = float32(math.Min(a.sizeOfPot(), 4) / 2)
		case a.Amount > 0 && a.Pot+a.Amount > 0:
			// A call's price: the share of the pot it paid for.
			f[11] = float32(a.Amount) / float32(a.Pot+a.Amount)
		}
		f[12] = float32(math.Log1p(float64(a.Amount)/bb) / 5)
	}

	// --- the players ---
	var sum [encReadsLen]float64
	count := 0
	for i := range s.Seats {
		if i == me || !s.Seats[i].inHand() {
			continue
		}
		r := readsOf(s.Seats[i].Reads)
		for j := range sum {
			sum[j] += r[j]
		}
		count++
	}
	mean := readsOf(nil)
	if count > 0 {
		for j := range sum {
			mean[j] = sum[j] / float64(count)
		}
	}
	for j, v := range mean {
		out[encReads+j] = float32(v)
	}
	facing := readsOf(nil)
	if f := facingSeat(s, me); f >= 0 {
		facing = readsOf(s.Seats[f].Reads)
		out[encReads+2*encReadsLen] = 1
	}
	for j, v := range facing {
		out[encReads+encReadsLen+j] = float32(v)
	}
	return out, nil
}

// cardIndex is a card's rank (0 = two .. 12 = ace) and suit (in `suits`
// order).
func cardIndex(c string) (int, int, bool) {
	v, ok := rankValue[rankOf(c)]
	if !ok {
		return 0, 0, false
	}
	for i, su := range suits {
		if suitOf(c) == su {
			return v - 2, i, true
		}
	}
	return 0, 0, false
}

// madeCategory is what the seat holds right now. Best needs five cards, so
// before the flop the only thing to know is whether the two make a pair.
func madeCategory(hole, board []string) int {
	if len(hole) < 2 {
		return highCard
	}
	if len(board) < 3 {
		if rankOf(hole[0]) == rankOf(hole[1]) {
			return pair
		}
		return highCard
	}
	return Best(append(append(make([]string, 0, 7), hole...), board...)).Category
}

// behindOf is `behind` for any seat: players who still have a decision after
// it on this street. The same count as the bot's when it is this seat's turn.
func behindOf(s *GameState, me int) int {
	n := 0
	for i := range s.Seats {
		if i != me && s.Seats[i].canAct() && !s.Seats[i].Acted {
			n++
		}
	}
	return n
}

// facingSeat is the opponent whose raise this seat is answering: the last one
// to raise this hand who is not this seat and is still in it. -1 when nobody
// has raised at it.
func facingSeat(s *GameState, me int) int {
	for i := len(s.HandLog) - 1; i >= 0; i-- {
		a := s.HandLog[i]
		if a.Verb != VerbRaise || a.Seat == me {
			continue
		}
		if a.Seat >= 0 && a.Seat < len(s.Seats) && s.Seats[a.Seat].inHand() {
			return a.Seat
		}
	}
	return -1
}

// Priors for the reads, with the weight of evidence each is worth: a rate is
// (seen + k·prior) / (chances + k), so a seat with no history reads as a
// typical player and one with a long history as itself. Roughly what a
// sensible player at a small table looks like.
var readPriors = [encReadsLen - 1]struct{ prior, k float64 }{
	{0.30, 8}, // VPIP per hand
	{0.15, 8}, // PFR per hand
	{0.50, 6}, // aggression: bets and raises per postflop bet, raise or call
	{0.45, 6}, // folds per bet faced
	{0.30, 4}, // bluffs per big bet shown
}

// readsOf turns counters into blended rates, plus how much history there is.
func readsOf(r *SeatReads) [encReadsLen]float64 {
	var c SeatReads
	if r != nil {
		c = *r
	}
	blend := func(i, seen, chances int) float64 {
		p := readPriors[i]
		return (float64(seen) + p.k*p.prior) / (float64(chances) + p.k)
	}
	return [encReadsLen]float64{
		blend(0, c.VPIP, c.Hands),
		blend(1, c.PFR, c.Hands),
		blend(2, c.PostBets, c.PostBets+c.PostCalls),
		blend(3, c.FoldedToBet, c.FacedBet),
		blend(4, c.BluffsShown, c.BigBetsShown),
		math.Log1p(float64(c.Hands)) / 5,
	}
}

// publicSeed is the rollout's coin, from nothing this seat cannot see.
//
// The bot's seedFor mixes in the match seed, which is fine for a bot that
// only ever reads its own cards but is not something to feed a network: the
// seed fixes the deal, so the rollout's noise would be a function of the
// cards to come. This hashes the seat, its own cards, the board and the
// public betting, so the same decision always encodes the same way and two
// decisions that differ only in hidden cards encode identically.
func publicSeed(s *GameState, me int) int64 {
	h := fnv.New64a()
	w := func(parts ...string) {
		for _, p := range parts {
			_, _ = h.Write([]byte(p))
			_, _ = h.Write([]byte{0})
		}
	}
	w(s.Seats[me].PlayerID, strconv.Itoa(s.HandNumber), s.Street)
	w(s.Seats[me].Hole...)
	w(s.Board...)
	for i := range s.Seats {
		w(strconv.Itoa(s.Seats[i].Stack), strconv.Itoa(s.Seats[i].Bet))
	}
	w(strconv.Itoa(s.Pot), strconv.Itoa(len(s.HandLog)))
	return int64(h.Sum64())
}

func b2f(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

// --- the moves ---------------------------------------------------------------

// Candidates are the moves a person at this seat would reach for: fold, check
// or call, a raise to half, three quarters, one or two pots, and all in.
//
// Each is built through the bot's own menu, so a raise is clamped into the
// offer's declared range exactly as the bot's are — raiseTarget, which also
// turns a raise that would leave a token stack behind into the shove it
// really is. Sizes that clamp to the same total are one candidate, not
// several: two indices for one move would split its probability between them
// and teach the network that the move is worth less than it is.
//
// Fold is not offered when checking is free. Folding for nothing is never
// right, and leaving it in only gives exploration a way to throw away hands.
//
// Outside a betting decision — a showdown stop, or not this seat's turn — the
// answer is the one move the heuristic would make, so the environment plays it
// without asking: go on rather than show, the same preference and for the same
// reason as the bot.
func (g learnGame) Candidates(raw module.State, playerID string, offers []module.ActionOffer) ([]learn.Candidate, error) {
	p, err := g.Position(raw)
	if err != nil {
		return nil, err
	}
	return g.CandidatesFor(p, playerID, offers)
}

// CandidatesFor is Candidates from a decoded position.
func (learnGame) CandidatesFor(p learn.Position, playerID string, offers []module.ActionOffer) ([]learn.Candidate, error) {
	pos, err := positionOf(p)
	if err != nil {
		return nil, err
	}
	s, err := pos.state()
	if err != nil {
		return nil, err
	}
	if !s.rules().board {
		return nil, errNotHoldem
	}
	if s.Break.Open || s.Status != "active" || s.Current < 0 || s.Seats[s.Current].PlayerID != playerID {
		a, ok := module.ChooseAction(offers, []string{module.VerbContinue})
		if !ok {
			return nil, nil
		}
		return []learn.Candidate{{Action: a, Features: make([]float32, candDim)}}, nil
	}
	seat := &s.Seats[s.Current]
	mn := menuOf(offers)

	var out []learn.Candidate
	seen := map[string]bool{}
	add := func(kind int, c choice) {
		a, ok := mn.action(c)
		if !ok || a.Verb != c.verb {
			// The menu degraded it to something else, which another
			// candidate already covers under its own name.
			return
		}
		key := a.Verb + "/" + a.Params[ParamAmount]
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, learn.Candidate{Action: a, Features: candFeatures(s, seat, kind, a)})
	}
	if !mn.can(VerbCheck) {
		add(kindFold, choice{verb: VerbFold})
	}
	add(kindCheck, choice{verb: VerbCheck})
	add(kindCall, choice{verb: VerbCall})
	if mn.can(VerbRaise) {
		for _, p := range potSizes {
			add(p.kind, choice{verb: VerbRaise, to: raiseTarget(s, mn, betOf(s, seat, p.fraction))})
		}
		add(kindAllIn, choice{verb: VerbRaise, to: shove(mn)})
	}
	return out, nil
}

// candFeatures describes one move. See the layout above.
//
// A raise that the clamp turned into everything is labelled all in whatever
// size asked for it, so the kind always says what the move does.
func candFeatures(s *GameState, seat *Seat, kind int, a module.Action) []float32 {
	f := make([]float32, candDim)
	bb := float64(max(s.BigBlind, 1))
	owed := s.toCall(seat)
	put := 0
	switch a.Verb {
	case VerbCall:
		put = owed
	case VerbRaise:
		to, _ := strconv.Atoi(a.Params[ParamAmount])
		put = to - seat.Bet
		raise := to - s.CurrentBet
		f[candNum] = float32(math.Log1p(float64(to)/bb) / 5)
		if base := potNow(s) + owed; base > 0 {
			f[candNum+1] = float32(math.Min(float64(raise)/float64(base), 4) / 2)
		}
		f[candNum+2] = float32(math.Log1p(float64(raise)/bb) / 5)
	}
	allIn := put > 0 && put >= seat.Stack
	if allIn && a.Verb == VerbRaise {
		kind = kindAllIn
	}
	f[candKind+kind] = 1
	if total := seat.Committed + seat.Stack; total > 0 {
		f[candNum+3] = float32(seat.Committed+put) / float32(total)
	}
	f[candNum+4] = b2f(allIn)
	f[candNum+5] = float32(math.Log1p(float64(put)/bb) / 5)
	return f
}

// --- the outcome -------------------------------------------------------------

// Reward is paid once per hand, when the hand ends: the seat's net chips over
// it, in big blinds — winnings less everything it put in, which is exactly
// HandSummary.Deltas. Nothing is paid for the chips going in along the way;
// a bet is not a loss until the hand says it is.
//
// The boundary is the summary list growing, which happens in endHand and
// nowhere else, so each hand is credited exactly once whatever action closed
// it — a fold, a call on the river, or a "go on" at a showdown that dealt a
// hand the blinds ended on the spot. Zero-sum by construction: every chip a
// summary pays one seat came out of another's Committed.
//
// The episode closes for every seat dealt into the hand, and for nobody who
// was already out of the match.
func (g learnGame) Reward(beforeRaw, afterRaw module.State, playerID string) (float32, bool, error) {
	before, err := g.Position(beforeRaw)
	if err != nil {
		return 0, false, err
	}
	after, err := g.Position(afterRaw)
	if err != nil {
		return 0, false, err
	}
	return g.RewardFor(before, after, playerID)
}

// RewardFor is Reward between two decoded positions.
func (learnGame) RewardFor(beforePos, afterPos learn.Position, playerID string) (float32, bool, error) {
	bp, err := positionOf(beforePos)
	if err != nil {
		return 0, false, err
	}
	ap, err := positionOf(afterPos)
	if err != nil {
		return 0, false, err
	}
	before, err := bp.hands()
	if err != nil {
		return 0, false, err
	}
	after, err := ap.hands()
	if err != nil {
		return 0, false, err
	}
	if len(after.Hands) <= len(before.Hands) {
		return 0, false, nil
	}
	dealt := false
	for _, st := range before.Seats {
		if st.PlayerID == playerID {
			dealt = !st.Out
		}
	}
	if !dealt {
		return 0, false, nil
	}
	bb := float64(max(after.BigBlind, 1))
	net := 0
	for _, h := range after.Hands[len(before.Hands):] {
		net += h.Deltas[playerID]
	}
	return float32(float64(net) / bb), true, nil
}

// handTally is the slice of GameState that Reward reads. The environment asks
// for a reward for every learning seat after every action at every table, so
// this is decoded far more often than anything else here, and a full decode —
// the deck, the log, every seat's cards — was a fifth of a training step.
type handTally struct {
	BigBlind int `json:"bigBlind"`
	Seats    []struct {
		PlayerID string `json:"playerId"`
		Out      bool   `json:"out"`
	} `json:"seats"`
	Hands []struct {
		Deltas map[string]int `json:"deltas"`
	} `json:"hands"`
}
