package ai

import (
	"sort"

	"zolik/server/internal/rules"
)

var AINames = map[string][]string{
	"easy":   {"Rookie Rita", "Lucky Lukáš", "Wobbly Wanda", "Slow Štefan"},
	"medium": {"Clever Karel", "Sharp Šárka", "Steady Stanislav", "Crafty Klára"},
	"hard":   {"Master Miroslav", "Shark Soňa", "Iron Ivan", "Relentless Radka"},
}

// HeuristicAgent picks moves from fixed heuristics with no randomness: the
// same VisibleState and hand always produce the same action. That is
// deliberate — it makes the agent's behaviour reproducible in tests and in bug
// reports. (It previously carried a wall-clock-seeded *rand.Rand that no
// decision ever read, which implied a variability that was never there.) The
// only jitter a player perceives is the "thinking" delay, which belongs to the
// game loop that drives the agent, not to the agent itself.
type HeuristicAgent struct {
	difficulty string
}

func NewHeuristicAgent(difficulty string) *HeuristicAgent {
	return &HeuristicAgent{difficulty: difficulty}
}

func (a *HeuristicAgent) Difficulty() string { return a.difficulty }

func (a *HeuristicAgent) ChooseAction(visible VisibleState, hand []string) rules.Action {
	visible.Rules = rules.ResolveConfig(visible.Rules)
	// Priority order (simplified v1):
	// 1) Meld phase: only start laying toward the round requirement if the
	// current hand can complete it entirely (pattern + minimum value) this
	// turn — a player who starts but can't finish gets stuck (the server
	// won't let them discard until they finish), so never begin unless a
	// full plan exists. Re-derived fresh each call so it naturally picks up
	// where a previous meld this turn left off.
	if visible.Phase == string(rules.PhaseMeld) {
		actor := visible.CurrentTurn
		if !visible.RoundReqMet[actor] {
			st := rulesStateForAI(visible, actor)
			if combo, ok := findInitialMeldPlan(st, actor, hand); ok && len(combo) > 0 {
				return rules.Action{Type: rules.ActionLayMeld, Cards: combo[0]}
			}
			// A profile with no per-type quota (FixedDealCount == 0, e.g.
			// Žolík Classic) lets any complete meld go down at any time:
			// MeldContributesTowardRequirement short-circuits to true, and
			// ValidateDiscard deliberately skips its "finish what you
			// started" gate there, so laying one cannot strand the turn.
			// Only the contract itself (e.g. a clean run) decides who is
			// down. Without this the agent held finished sets it was
			// allowed to lay — and eventually discarded them away — because
			// the plan search only ever looks for what the contract still
			// needs, which under this profile is never a set.
			if visible.Rules.FixedDealCount == 0 {
				if meld, ok := findAnyValidMeld(hand, visible.Rules); ok {
					rest := removeCardsOnce(hand, meld)
					// A player who is not down cannot go out (ValidateDiscard
					// only lets a player end the deal once RoundReqMet), so
					// emptying the hand buys nothing and costs everything:
					// meld away the cards the outstanding contract still
					// needs and it can never be completed this deal. Keep
					// enough material for it, plus one card to discard.
					// This is also what keeps the agent clear of the engine's
					// one true dead end — a hand of nothing but jokers, which
					// can be neither melded nor discarded nor passed on.
					need := contractCardsStillNeeded(st, actor)
					if len(rest) >= need+1 && handCanStillDiscard(rest, visible.Rules, false) {
						return rules.Action{Type: rules.ActionLayMeld, Cards: meld}
					}
				}
			}
		} else {
			// Already down: shed cards one at a time onto any table meld
			// (own or another player's) before trying a brand-new meld —
			// otherwise a hand with no full new meld left in it can never
			// shrink to zero and the deal never ends.
			if meldID, card, ok := findLayOff(visible.MeldMeta, visible.Melds, hand, visible.Rules, visible.GameNumber); ok {
				return rules.Action{Type: rules.ActionLayOff, MeldID: meldID, Card: card}
			}
			if meld, ok := findAnyValidMeld(hand, visible.Rules); ok && len(hand) > len(meld) && handCanStillDiscard(removeCardsOnce(hand, meld), visible.Rules, visible.RoundReqMet[actor]) {
				return rules.Action{Type: rules.ActionLayMeld, Cards: meld}
			}
		}
		// Otherwise discard.
		canDiscardJoker := len(hand) == 1 && visible.RoundReqMet[actor]
		return rules.Action{Type: rules.ActionDiscard, Card: pickSmartDiscard(hand, visible, actor, a.difficulty, canDiscardJoker)}
	}
	// 2) Draw phase: prefer discard if available and allowed this round, else deck.
	if visible.Phase == string(rules.PhaseDraw) {
		// The shared helper, reading the lock round off the resolved
		// ruleset: VisibleState deliberately no longer carries its own
		// DiscardDrawMinRound copy to drift out of sync with Rules.
		discardLocked := rules.IsDiscardLocked(visible.Round, visible.Rules.DiscardDrawMinRound)
		if len(visible.DiscardPile) > 0 && !discardLocked {
			actor := visible.CurrentTurn
			topDiscard := visible.DiscardPile[len(visible.DiscardPile)-1]
			if visible.RoundReqMet[actor] {
				// Already down: the pickup is unrestricted by the rules, but
				// "allowed" is not "useful". Taking a card the agent cannot
				// place means pickSmartDiscard names that same card the worst
				// in hand and throws it straight back next phase. With every
				// agent down, one useless card then circulates forever — no
				// melds, no lay-offs, nobody going out — while the deck still
				// holds dozens of untouched cards. Only take it when it
				// actually lands somewhere.
				if discardPickupUseful(topDiscard, hand, visible) {
					return rules.Action{Type: rules.ActionDrawCard, DrawFrom: rules.DrawFromDiscard}
				}
				return rules.Action{Type: rules.ActionDrawCard, DrawFrom: rules.DrawFromDeck}
			}
			// Not yet down: picking up the discard obligates laying that
			// exact card into the initial meld this turn (server-enforced).
			// Only take it if a full plan using it actually exists —
			// otherwise take the deck instead, which carries no obligation.
			st := rulesStateForAI(visible, actor)
			candidateHand := append(append([]string(nil), hand...), topDiscard)
			if _, ok := findInitialMeldPlanRequiring(st, actor, candidateHand, topDiscard); ok {
				return rules.Action{Type: rules.ActionDrawCard, DrawFrom: rules.DrawFromDiscard}
			}
		}
		return rules.Action{Type: rules.ActionDrawCard, DrawFrom: rules.DrawFromDeck}
	}
	// Fallback: discard.
	if len(hand) > 0 {
		canDiscardJoker := len(hand) == 1 && visible.RoundReqMet[visible.CurrentTurn]
		return rules.Action{Type: rules.ActionDiscard, Card: pickSmartDiscard(hand, visible, visible.CurrentTurn, a.difficulty, canDiscardJoker)}
	}
	return rules.Action{Type: rules.ActionDiscard, Card: ""}
}

func rulesStateForAI(visible VisibleState, playerID string) rules.GameState {
	return rules.GameState{
		Rules:       visible.Rules,
		GameNumber:  visible.GameNumber,
		RoundReqMet: visible.RoundReqMet,
		Melds:       visible.Melds,
		MeldMeta:    visible.MeldMeta,
	}
}

// combinations returns every k-length subset of items, as index tuples
// materialized into card slices. Used so the meld search works under any
// profile's MinSetSize/MinRunSize instead of hardcoding 3/4.
func combinations(items []string, k int) [][]string {
	idxs := indexCombinations(len(items), k)
	out := make([][]string, 0, len(idxs))
	for _, idx := range idxs {
		cand := make([]string, k)
		for i, v := range idx {
			cand[i] = items[v]
		}
		out = append(out, cand)
	}
	return out
}

// indexCombinations returns every k-length subset of [0,n) as index tuples.
// Callers that need to know *which positions* a candidate came from (rather
// than just the cards) use this directly — duplicate cards are common in a
// two-deck game, so positions, not values, are what identify a card.
func indexCombinations(n, k int) [][]int {
	if k <= 0 || k > n {
		return nil
	}
	var out [][]int
	idx := make([]int, k)
	for i := range idx {
		idx[i] = i
	}
	for {
		out = append(out, append([]int(nil), idx...))

		i := k - 1
		for i >= 0 && idx[i] == n-k+i {
			i--
		}
		if i < 0 {
			break
		}
		idx[i]++
		for j := i + 1; j < k; j++ {
			idx[j] = idx[j-1] + 1
		}
	}
	return out
}

// findInitialMeldPlan looks for a full decomposition of the player's hand
// that completes their remaining round requirement (accounting for any
// sets/runs they've already laid, e.g. earlier this same turn) with total
// natural value at least the round's minimum. It returns the melds to lay,
// in order — the caller lays them one at a time (across successive
// ChooseAction calls), but only ever starts if a full plan already exists,
// so the server's "must finish what you start" rule never strands it.
func findInitialMeldPlan(state rules.GameState, playerID string, hand []string) ([][]string, bool) {
	return findInitialMeldPlanRequiring(state, playerID, hand, "")
}

// findInitialMeldPlanRequiring is findInitialMeldPlan, but when mustInclude
// is non-empty, only returns a plan that actually uses that specific card in
// one of its melds — used to check whether picking up a discard card (which
// obligates melding that exact card this turn) is actually going to work
// before committing to the draw.
func findInitialMeldPlanRequiring(state rules.GameState, playerID string, hand []string, mustInclude string) ([][]string, bool) {
	cfg := state.Rules
	req := cfg.ContractFor(state.GameNumber)
	setsBefore, runsBefore, hasCleanRun := rules.PlayerMeldCounts(state, playerID)
	needSets := req.Sets - setsBefore
	needRuns := req.Runs - runsBefore
	if needSets < 0 {
		needSets = 0
	}
	if needRuns < 0 {
		needRuns = 0
	}
	needCleanRun := req.RequireCleanRun && !hasCleanRun
	if needSets == 0 && needRuns == 0 && !needCleanRun {
		return nil, false
	}
	minValue := 0
	if cfg.InitialMeldMinimum > 0 {
		minValue = cfg.InitialMeldMinimum
	}
	alreadyValue := rules.PlayerInitialMeldNaturalValue(state, playerID)

	budget := &searchBudget{remaining: 200000}
	satisfied := mustInclude == ""
	combo, ok := searchMeldCombo(hand, cfg, needSets, needRuns, needCleanRun, alreadyValue, minValue, satisfied, mustInclude, budget)
	if !ok {
		return nil, false
	}
	if !cfg.IsFinalDeal(state.GameNumber) {
		used := 0
		for _, m := range combo {
			used += len(m)
		}
		if len(hand)-used < 1 {
			// Would meld away the entire hand with no card left to discard.
			return nil, false
		}
	}
	return combo, true
}

type searchBudget struct{ remaining int }

func containsCard(cards []string, card string) bool {
	for _, c := range cards {
		if c == card {
			return true
		}
	}
	return false
}

func searchMeldCombo(
	hand []string,
	cfg rules.RulesConfig,
	needSets, needRuns int,
	needCleanRun bool,
	valueSoFar, minValue int,
	satisfied bool,
	mustInclude string,
	budget *searchBudget,
) ([][]string, bool) {
	if needSets == 0 && needRuns == 0 && !needCleanRun {
		if valueSoFar >= minValue && satisfied {
			return [][]string{}, true
		}
		return nil, false
	}
	n := len(hand)
	minSet, minRun := meldSizes(cfg)
	if needSets > 0 && n >= minSet {
		for _, cand := range combinations(hand, minSet) {
			if budget.remaining <= 0 {
				return nil, false
			}
			budget.remaining--
			mv, err := rules.ValidateMeld(cand, cfg)
			if err != nil || mv.Type != rules.MeldSet {
				continue
			}
			rest := removeCardsOnce(hand, cand)
			candSatisfied := satisfied || containsCard(cand, mustInclude)
			if combo, ok := searchMeldCombo(rest, cfg, needSets-1, needRuns, needCleanRun, valueSoFar+mv.NaturalValue, minValue, candSatisfied, mustInclude, budget); ok {
				return append([][]string{cand}, combo...), true
			}
		}
	}
	if (needRuns > 0 || needCleanRun) && n >= minRun {
		for _, cand := range combinations(hand, minRun) {
			if budget.remaining <= 0 {
				return nil, false
			}
			budget.remaining--
			mv, err := rules.ValidateMeld(cand, cfg)
			if err != nil || mv.Type != rules.MeldRun {
				continue
			}
			rest := removeCardsOnce(hand, cand)
			candSatisfied := satisfied || containsCard(cand, mustInclude)
			nextNeedRuns := needRuns
			if nextNeedRuns > 0 {
				nextNeedRuns--
			}
			nextNeedCleanRun := needCleanRun && mv.WildCount > 0 // still need one if this run wasn't clean
			if combo, ok := searchMeldCombo(rest, cfg, needSets, nextNeedRuns, nextNeedCleanRun, valueSoFar+mv.NaturalValue, minValue, candSatisfied, mustInclude, budget); ok {
				return append([][]string{cand}, combo...), true
			}
		}
	}
	return nil, false
}

// removeCardsOnce removes exactly one occurrence of each card in remove from
// hand (order-preserving), matching removeAtIndices' semantics but keyed off
// a variable-length candidate slice instead of fixed index arguments.
func removeCardsOnce(hand []string, remove []string) []string {
	toRemove := append([]string(nil), remove...)
	out := make([]string, 0, len(hand)-len(remove))
	for _, c := range hand {
		removed := false
		for i, r := range toRemove {
			if r == c {
				toRemove = append(toRemove[:i], toRemove[i+1:]...)
				removed = true
				break
			}
		}
		if !removed {
			out = append(out, c)
		}
	}
	return out
}

// findLayOff looks for a card in hand that can extend any table meld (own
// or another player's). Skips a lay-off that would empty the hand on a deal
// that requires a final discard to go out (see RulesConfig.IsFinalDeal).
func findLayOff(meldMeta map[string][]rules.MeldInfo, melds map[string][][]string, hand []string, cfg rules.RulesConfig, gameNumber int) (meldID string, card string, ok bool) {
	if !cfg.IsFinalDeal(gameNumber) && len(hand) == 1 {
		return "", "", false
	}
	// Owners are visited in a fixed order: ranging a map directly made which
	// meld the agent extended depend on Go's randomised map iteration, so the
	// same position could produce different play on different runs.
	for _, owner := range sortedOwners(meldMeta) {
		metas := meldMeta[owner]
		ownerMelds := melds[owner]
		for i, mi := range metas {
			if i >= len(ownerMelds) {
				continue
			}
			existing := ownerMelds[i]
			for _, c := range hand {
				cand := append(append([]string(nil), existing...), c)
				if _, err := rules.ValidateMeld(cand, cfg); err != nil {
					continue
				}
				// Never dirty the owner's only clean run with a wild — the
				// server rejects it, and the card belongs in its own meld.
				if rules.LayOffBreaksCleanRun(cfg, gameNumber, ownerMelds, i, []string{c}) {
					continue
				}
				// Never shed the last card the player could legally discard.
				// ValidateLayOff refuses a lay-off from a player who is not
				// down (ROUND_REQ_NOT_MET), so reaching here means they are.
				if !handCanStillDiscard(removeCardsOnce(hand, []string{c}), cfg, true) {
					continue
				}
				return mi.MeldID, c, true
			}
		}
	}
	return "", "", false
}

func findAnyValidMeld(hand []string, cfg rules.RulesConfig) ([]string, bool) {
	minSet, minRun := meldSizes(cfg)
	n := len(hand)
	if n >= minSet {
		for _, cand := range combinations(hand, minSet) {
			if _, err := rules.ValidateMeld(cand, cfg); err == nil {
				return cand, true
			}
		}
	}
	if n >= minRun {
		for _, cand := range combinations(hand, minRun) {
			if _, err := rules.ValidateMeld(cand, cfg); err == nil {
				return cand, true
			}
		}
	}
	return nil, false
}

// pickSmartDiscard is pickWorstDiscard made meld-aware. "easy" AIs keep the
// old blind behavior (highest penalty points, full stop). "medium" and
// "hard" first drop any candidate that would let *any* player extend a meld
// already on the table — the single biggest source of "the AI just fed me
// my run" complaints, since findLayOff already claims such cards for the
// AI's own hand before reaching this fallback, so anything still here that
// fits a live meld is pure gift to an opponent. "hard" additionally prefers,
// among the safe candidates, a card whose rank someone has already
// discarded this game: if a player passed on that rank before, they're
// unlikely to want it now, which is the same "read the discards" signal a
// careful human opponent would use.
func pickSmartDiscard(hand []string, visible VisibleState, actor string, difficulty string, canDiscardJoker bool) string {
	cfg := visible.Rules
	if len(hand) == 0 {
		return pickWorstDiscard(hand, cfg, canDiscardJoker, visible.DiscardTakenCard)
	}
	allowJoker := canDiscardJoker || !cfg.JokerDiscardRestricted
	ownMeld := meldMaterialPositions(hand, cfg)

	// The engine refuses to take this turn's discard-pile pickup straight
	// back (rules.ErrDiscardTakenCard) while the hand holds anything else it
	// could legally shed — taking a card commits you to it for the turn.
	// Both halves of that are mirrored here: skip the taken card while other
	// candidates exist, then stop skipping it if that leaves nothing, so the
	// agent still names the move the engine would actually accept instead of
	// stalling on a rejection it can't diagnose.
	var cands []discardCandidate
	for _, banTaken := range [2]bool{true, false} {
		for i, c := range hand {
			if !allowJoker && rules.IsJoker(c) {
				continue
			}
			if banTaken && visible.DiscardTakenCard != "" && c == visible.DiscardTakenCard {
				continue
			}
			cands = append(cands, discardCandidate{
				card: c,
				pts:  rules.PenaltyPoints(c, false),
				// Keeping a meld you are holding is not a difficulty setting —
				// an agent that breaks up its own finished set every turn never
				// gets one onto the table at all, which reads as "the AI doesn't
				// meld" rather than as a beatable opponent. Every difficulty
				// protects its own melds; what separates them is reading the
				// table (dangerous) and the discard history (seenBefore).
				ownMeld:    ownMeld[i],
				dangerous:  difficulty != "easy" && extendsAnyLiveMeld(c, visible.Melds, cfg, visible.GameNumber),
				seenBefore: difficulty == "hard" && rankAlreadyDiscardedByOthers(c, visible.PlayerDiscards, actor),
			})
		}
		if len(cands) > 0 {
			break
		}
	}
	if len(cands) == 0 {
		// Every card is a forbidden joker (pathological, but has to return
		// something) — the caller/server is left to reject it.
		return hand[0]
	}
	best := cands[0]
	for _, c := range cands[1:] {
		if smarterDiscardBetter(c, best) {
			best = c
		}
	}
	return best.card
}

type discardCandidate struct {
	card       string
	pts        int
	ownMeld    bool
	dangerous  bool
	seenBefore bool
}

// smarterDiscardBetter orders candidates:
//
//  1. a card that isn't part of a finished meld in hand beats one that is.
//     This outranks everything else because meld material is the whole
//     point of the hand: face cards are simultaneously the highest-penalty
//     cards and the likeliest set material, so a points-first ordering
//     dismantled a ready-to-lay set of kings one card per turn and the
//     agent never got it onto the table. Denying an opponent a single
//     lay-off is worth much less than keeping your own meld intact.
//  2. safe (doesn't feed a live meld) beats dangerous.
//  3. then, same as the plain worst-card heuristic, higher penalty points
//     win (shed the costliest card first).
//  4. an already-passed-on rank only breaks an exact points tie, so the
//     history signal fine-tunes which equally-costly card to let go of
//     rather than overriding the basic "get rid of the expensive card" goal.
func smarterDiscardBetter(c, best discardCandidate) bool {
	if c.ownMeld != best.ownMeld {
		return !c.ownMeld
	}
	if c.dangerous != best.dangerous {
		return !c.dangerous
	}
	if c.pts != best.pts {
		return c.pts > best.pts
	}
	return c.seenBefore && !best.seenBefore
}

// meldSizes returns the profile's minimum set and run lengths, defaulting a
// zero-value config to the classic 3/4 rather than letting a 0 through.
func meldSizes(cfg rules.RulesConfig) (minSet, minRun int) {
	minSet, minRun = cfg.MinSetSize, cfg.MinRunSize
	if minSet == 0 {
		minSet = 3
	}
	if minRun == 0 {
		minRun = 4
	}
	return minSet, minRun
}

// contractCardsStillNeeded is the smallest number of cards from hand that
// could still satisfy the player's outstanding contract, given what they
// have already laid. Used to stop a not-yet-down agent melding away the very
// material it still owes.
func contractCardsStillNeeded(state rules.GameState, playerID string) int {
	cfg := rules.ResolveConfig(state.Rules)
	req := cfg.ContractFor(state.GameNumber)
	setsBefore, runsBefore, hasCleanRun := rules.PlayerMeldCounts(state, playerID)
	minSet, minRun := meldSizes(cfg)

	need := 0
	if n := req.Sets - setsBefore; n > 0 {
		need += n * minSet
	}
	runsNeeded := req.Runs - runsBefore
	if runsNeeded < 0 {
		runsNeeded = 0
	}
	// A clean-run requirement costs a run's worth of cards unless a run is
	// already owed (that one can be the clean one) or already satisfied.
	if req.RequireCleanRun && !hasCleanRun && runsNeeded == 0 {
		runsNeeded = 1
	}
	return need + runsNeeded*minRun
}

// handCanStillDiscard reports whether the player would still have a legal
// discard left after shedding cards down to rest.
//
// Under JokerDiscardRestricted (ValidateDiscard) a joker may be discarded
// only as the exact card that empties an *already-down* player's hand. So a
// hand of nothing but jokers has no legal move at all — it cannot be melded,
// cannot be discarded, and the engine has no "pass" — with the single
// exception of one joker held by a player who is down, which is their
// go-out discard. An agent that melds or lays off its last natural card
// walks into that dead end and wedges the deal for the whole table, so
// every play that sheds cards has to check this first.
func handCanStillDiscard(rest []string, cfg rules.RulesConfig, down bool) bool {
	if len(rest) == 0 {
		return true // melded out; nothing left to discard
	}
	if !cfg.JokerDiscardRestricted {
		return true
	}
	for _, c := range rest {
		if !rules.IsJoker(c) {
			return true
		}
	}
	// Jokers only: legal exactly when a lone joker is the discard that ends
	// the hand, which the engine allows only once the player is down.
	return len(rest) == 1 && down
}

// discardPickupUseful reports whether a player who is already down has
// anywhere to put the top discard: onto a meld already on the table, or
// into a new meld it completes with cards already in hand. Anything else is
// a card the agent would immediately discard again.
func discardPickupUseful(card string, hand []string, visible VisibleState) bool {
	cfg := visible.Rules
	if extendsAnyLiveMeld(card, visible.Melds, cfg, visible.GameNumber) {
		return true
	}
	combined := append(append([]string(nil), hand...), card)
	return meldMaterialPositions(combined, cfg)[len(combined)-1]
}

// meldMaterialPositions marks each position in hand that takes part in at
// least one complete, valid meld formable from the hand as it stands. It is
// keyed by position rather than by card because a two-deck game deals
// duplicates (two 5C are different cards holding different jobs).
//
// Deliberately only *complete* melds count. Protecting partial material
// (pairs, two-thirds of a run) would freeze most of the hand and leave the
// agent nothing safe to discard.
func meldMaterialPositions(hand []string, cfg rules.RulesConfig) []bool {
	out := make([]bool, len(hand))
	minSet, minRun := meldSizes(cfg)
	sizes := []int{minSet}
	if minRun != minSet {
		sizes = append(sizes, minRun)
	}
	for _, k := range sizes {
		for _, idx := range indexCombinations(len(hand), k) {
			cand := make([]string, k)
			for i, v := range idx {
				cand[i] = hand[v]
			}
			if _, err := rules.ValidateMeld(cand, cfg); err != nil {
				continue
			}
			for _, v := range idx {
				out[v] = true
			}
		}
	}
	return out
}

// extendsAnyLiveMeld reports whether card can be laid off onto any meld
// currently on the table (any owner) — i.e. discarding it hands the next
// player a free lay-off.
func extendsAnyLiveMeld(card string, melds map[string][][]string, cfg rules.RulesConfig, gameNumber int) bool {
	// Order-independent in principle (this only asks "does any meld take it?"),
	// but iterated in a fixed order anyway so that a future early-return or
	// scoring change here can't reintroduce map-order-dependent play.
	for _, owner := range sortedMeldOwners(melds) {
		ownerMelds := melds[owner]
		for i, existing := range ownerMelds {
			cand := append(append([]string(nil), existing...), card)
			if _, err := rules.ValidateMeld(cand, cfg); err != nil {
				continue
			}
			if rules.LayOffBreaksCleanRun(cfg, gameNumber, ownerMelds, i, []string{card}) {
				continue
			}
			return true
		}
	}
	return false
}

// sortedOwners returns meld owners in a stable order, so meld search never
// depends on Go's randomised map iteration.
func sortedOwners(meldMeta map[string][]rules.MeldInfo) []string {
	out := make([]string, 0, len(meldMeta))
	for owner := range meldMeta {
		out = append(out, owner)
	}
	sort.Strings(out)
	return out
}

func sortedMeldOwners(melds map[string][][]string) []string {
	out := make([]string, 0, len(melds))
	for owner := range melds {
		out = append(out, owner)
	}
	sort.Strings(out)
	return out
}

// rankAlreadyDiscardedByOthers reports whether some player other than actor
// has already discarded a card of the same rank this game.
func rankAlreadyDiscardedByOthers(card string, playerDiscards map[string][]string, actor string) bool {
	rank := rules.CardRank(card)
	for player, discards := range playerDiscards {
		if player == actor {
			continue
		}
		for _, d := range discards {
			if rules.CardRank(d) == rank {
				return true
			}
		}
	}
	return false
}

// PickWorstDiscard exposes pickWorstDiscard for callers that need an emergency
// fallback discard outside of ChooseAction (e.g. when a chosen action was rejected).
func PickWorstDiscard(hand []string, cfg rules.RulesConfig, canDiscardJoker bool, takenCard string) string {
	return pickWorstDiscard(hand, cfg, canDiscardJoker, takenCard)
}

// pickWorstDiscard picks the highest-penalty card, but a joker is the
// highest-penalty card there is (rules.PenaltyPoints: 50) — under a profile
// with JokerDiscardRestricted, blindly picking it produces a discard the
// server always rejects, and the caller has no fallback, so the AI loop
// just retries the same illegal move until it gives up (made no progress).
// canDiscardJoker mirrors the server's own exception: true only when this
// would be the player's last card and they're already down.
//
// takenCard mirrors the other exception the engine enforces: the card taken
// off the discard pile this turn can't go straight back on it while anything
// else in hand is legally discardable (rules.ErrDiscardTakenCard). This is a
// fallback used precisely when the agent's real choice was already rejected,
// so naming a second card the server also refuses leaves the turn — and the
// deal — stuck. Pass "" when the turn's draw came from the deck.
func pickWorstDiscard(hand []string, cfg rules.RulesConfig, canDiscardJoker bool, takenCard string) string {
	if len(hand) == 0 {
		return ""
	}
	allowJoker := canDiscardJoker || !cfg.JokerDiscardRestricted
	worst := ""
	worstPts := -1
	for _, banTaken := range [2]bool{true, false} {
		for _, c := range hand {
			if !allowJoker && rules.IsJoker(c) {
				continue
			}
			if banTaken && takenCard != "" && c == takenCard {
				continue
			}
			pts := rules.PenaltyPoints(c, false)
			if pts > worstPts {
				worst = c
				worstPts = pts
			}
		}
		if worst != "" {
			break
		}
	}
	if worst == "" {
		// Every card is a forbidden joker (pathological, but has to return
		// something) — the caller/server is left to reject it.
		return hand[0]
	}
	return worst
}
