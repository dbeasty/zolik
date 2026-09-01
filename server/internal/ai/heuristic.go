package ai

import (
	"math/rand"
	"sort"

	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// The bot roster used to live here, as an AINames map that nothing ever read —
// so every opponent was called "Bot 4F". It is now module.Persona, because
// naming a seat nobody is sitting at is a runtime job and Prší's bots want
// names too.

// HeuristicAgent picks moves from fixed heuristics at a declared strength.
//
// It is a pure function of (VisibleState, hand, seed): the same state, the same
// hand and the same seed always produce the same action. That is a slightly
// weaker promise than the one it used to make — it was previously
// deterministic outright, with no randomness at all — and the weakening is on
// purpose. A weak opponent has to make mistakes, and a mistake needs a coin to
// flip. What matters for tests and for bug reports is *reproducibility*, not
// the absence of dice, so the coin is seeded from the match rather than from
// the clock and nothing about replaying a game changes.
//
// The seed is mixed with the position (deal, round, hand size) rather than
// held as mutable state, so the agent has no memory between calls and two
// agents built the same way from the same seed cannot drift apart.
type HeuristicAgent struct {
	prof Profile
	seed int64
}

// NewHeuristicAgent builds an agent at a named strength, unseeded.
//
// Kept in its original shape because a great deal of the test suite calls it,
// and because an unseeded agent is a perfectly good thing to want: seed zero
// is as reproducible as any other.
func NewHeuristicAgent(difficulty string) *HeuristicAgent {
	skill, _ := module.ParseSkill(difficulty)
	return &HeuristicAgent{prof: ProfileFor(skill)}
}

// NewAgent builds an agent for a seat: a strength, and the seed its mistakes
// come out of.
func NewAgent(skill module.Skill, seed int64) *HeuristicAgent {
	return &HeuristicAgent{prof: ProfileFor(skill), seed: seed}
}

// NewAgentWithProfile builds an agent from a profile directly, bypassing the
// ladder.
//
// For tuning. The whole design says a strength is a set of knobs, and the only
// way to find out what a knob is worth is to change one and play the same
// seeds again — which needs a profile that is not one of the four. Nothing in
// the server calls this; internal/ai/sim's ablation does.
func NewAgentWithProfile(p Profile, seed int64) *HeuristicAgent {
	return &HeuristicAgent{prof: p, seed: seed}
}

func (a *HeuristicAgent) Difficulty() string { return string(a.prof.Skill) }

// Profile is the strength this agent plays at.
func (a *HeuristicAgent) Profile() Profile { return a.prof }

// rngFor is the coin for one decision.
//
// Derived from the position rather than carried, so that ChooseAction stays a
// function of its arguments: an agent asked the same question twice in a turn
// — which happens, because the runtime re-derives the state between each of a
// turn's actions — answers it the same way both times.
func (a *HeuristicAgent) rngFor(v VisibleState, hand []string) *rand.Rand {
	mix := a.seed
	mix = mix*1103515245 + int64(v.GameNumber)
	mix = mix*1103515245 + int64(v.Round)
	mix = mix*1103515245 + int64(len(hand))
	mix = mix*1103515245 + int64(len(v.DealDiscards))
	return rand.New(rand.NewSource(mix))
}

func (a *HeuristicAgent) ChooseAction(visible VisibleState, hand []string) rules.Action {
	visible.Rules = rules.ResolveConfig(visible.Rules)
	actor := visible.CurrentTurn
	k := newKnowledge(visible, hand, actor, a.prof)
	rng := a.rngFor(visible, hand)

	// Priority order (simplified v1):
	// 1) Meld phase: only start laying toward the round requirement if the
	// current hand can complete it entirely (pattern + minimum value) this
	// turn — a player who starts but can't finish gets stuck (the server
	// won't let them discard until they finish), so never begin unless a
	// full plan exists. Re-derived fresh each call so it naturally picks up
	// where a previous meld this turn left off.
	if visible.Phase == string(rules.PhaseMeld) {
		if !visible.RoundReqMet[actor] {
			// Dithering is only ever allowed *before* the first card of a
			// plan is on the table. A plan is laid one meld at a time across
			// successive calls, so hesitating halfway through would end the
			// turn with melds down and the contract unmet — cards stranded
			// where they help every opponent and can never bring this player
			// down. That is the exact failure the self-play gate calls a
			// stranded lay, and it must not be reachable by a dice roll.
			midPlan := len(visible.Melds[actor]) > 0
			// A card taken off the pile is a debt: the engine will not accept
			// this turn's discard until that exact card is part of the initial
			// meld. Dithering while owing it would end the turn with no legal
			// move in it at all, so the debt overrides the dice — and, once
			// there is one, the plan has to be a plan that *spends* it.
			owed := visible.PendingMeldCard
			if midPlan || owed != "" || !a.dithered(rng) {
				st := rulesStateForAI(visible, actor)
				combo, ok := findInitialMeldPlan(st, actor, hand)
				if owed != "" {
					// Not merely a preference. findInitialMeldPlan looks for
					// any qualifying combination, and a hand that owes the
					// pile a card usually has several — most of which do not
					// use it. Laying one of those satisfies the contract and
					// wedges the turn: down, still owing, and refused the
					// discard that would end it. This is the search that
					// asks the right question.
					combo, ok = findInitialMeldPlanRequiring(st, actor, hand, owed)
				}
				if ok && len(combo) > 0 {
					return rules.Action{Type: rules.ActionLayMeld, Cards: combo[0]}
				}
			}
			// The plan search only looks for what the contract still *needs*,
			// so it finds nothing at a table whose contract asks for nothing
			// (Žolík Classic with the clean-run house rule turned off, say) —
			// and the agent would then never lay a first meld at all. The
			// fallback covers exactly that: a meld the plan did not ask for is
			// worth laying if it brings the player down on its own.
			if meld, ok := findAnyValidMeld(hand, visible.Rules); ok {
				rest := removeCardsOnce(hand, meld)
				// Same debt, same rule: a fallback meld that does not spend
				// the card owed to the pile leaves the turn unfinishable.
				spendsDebt := visible.PendingMeldCard == "" || containsCard(meld, visible.PendingMeldCard)
				if spendsDebt && len(rest) >= 1 && handCanStillDiscard(rest, visible.Rules, true) &&
					meldWouldBringDown(visible, actor, meld) {
					return rules.Action{Type: rules.ActionLayMeld, Cards: meld}
				}
			}
		} else {
			// Already down. First pay off any joker debt: a joker this agent
			// took off the table must be played again before the engine will
			// accept its discard, so placing it outranks every other shed.
			// findLayOff's own reclaim guard only ever takes a joker it can
			// place immediately, and nothing intervenes between the take and
			// this branch, so a placement is there to find.
			if pending := presentIn(hand, visible.PendingJokers); len(pending) > 0 {
				if meldID, card, ok := findLayOffAmong(visible.MeldMeta, visible.Melds, hand, pending, visible.Rules, visible.GameNumber); ok {
					return rules.Action{Type: rules.ActionLayOff, MeldID: meldID, Card: card}
				}
			}
			// Shed cards one at a time onto any table meld (own or another
			// player's) before trying a brand-new meld — otherwise a hand
			// with no full new meld left in it can never shrink to zero and
			// the deal never ends.
			//
			// MissRate is applied here and only here: the beginner's real
			// failure is not bad judgement but not *seeing* that the seven in
			// hand goes on the run at the far end of the table. Skipping the
			// lay-off costs a turn; it never costs legality, because the
			// discard below is always available.
			if !a.missed(rng) {
				if meldID, card, ok := a.chooseLayOff(visible, hand, k); ok {
					return rules.Action{Type: rules.ActionLayOff, MeldID: meldID, Card: card}
				}
			}
			if meld, ok := findAnyValidMeld(hand, visible.Rules); ok && len(hand) > len(meld) && handCanStillDiscard(removeCardsOnce(hand, meld), visible.Rules, visible.RoundReqMet[actor]) {
				return rules.Action{Type: rules.ActionLayMeld, Cards: meld}
			}
		}
		// Otherwise discard.
		canDiscardJoker := len(hand) == 1 && visible.RoundReqMet[actor]
		return rules.Action{Type: rules.ActionDiscard, Card: a.pickDiscard(hand, visible, actor, k, rng, canDiscardJoker)}
	}
	// 2) Draw phase: prefer discard if available and allowed this round, else deck.
	if visible.Phase == string(rules.PhaseDraw) {
		// The shared helper, reading the lock round off the resolved
		// ruleset: VisibleState deliberately no longer carries its own
		// DiscardDrawMinRound copy to drift out of sync with Rules.
		discardLocked := rules.IsDiscardLocked(visible.Round, visible.Rules.DiscardDrawMinRound)
		if len(visible.DiscardPile) > 0 && !discardLocked {
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
				// There was a DrawSpeculative setting here that let a strong
				// agent also take a card which merely improved the hand's
				// *shape* — turning a pair into a triple, extending a
				// two-card run. It is deleted rather than disabled, because
				// the sweep was unambiguous: it lost 141 of 200 deals to the
				// profile without it and gave up nearly sixty penalty points
				// a match. The reason is not subtle in hindsight. Taking a
				// card off the pile commits to it for the turn (the engine
				// will not accept it straight back), tells the table what you
				// are building, and buys a maybe with a certainty — and the
				// card on the pile is one an opponent has already judged
				// worthless. "Useful now" is a good rule; "useful later" is
				// not.
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
		canDiscardJoker := len(hand) == 1 && visible.RoundReqMet[actor]
		return rules.Action{Type: rules.ActionDiscard, Card: a.pickDiscard(hand, visible, actor, k, rng, canDiscardJoker)}
	}
	return rules.Action{Type: rules.ActionDiscard, Card: ""}
}

// missed rolls the profile's chance of not noticing an available lay-off.
func (a *HeuristicAgent) missed(rng *rand.Rand) bool {
	return a.prof.MissRate > 0 && rng.Float64() < a.prof.MissRate
}

// dithered rolls the profile's chance of putting off going down this turn.
func (a *HeuristicAgent) dithered(rng *rand.Rand) bool {
	return a.prof.MeldDither > 0 && rng.Float64() < a.prof.MeldDither
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
	minValue := 0
	if cfg.InitialMeldMinimum > 0 {
		minValue = cfg.InitialMeldMinimum
	}
	alreadyValue := rules.PlayerInitialMeldNaturalValue(state, playerID)
	// Nothing left to plan only when the shape is complete *and* the point
	// floor is already covered *and* no particular card has to be placed.
	// Meeting the shape alone is not the end of the search: the floor is
	// summed across every meld the player lays (see the RoundReqMet
	// assignment in rules.ValidateMeldAction), so a contract-complete but
	// under-value position still needs more melds to top the total up.
	if needSets == 0 && needRuns == 0 && !needCleanRun && alreadyValue >= minValue && mustInclude == "" {
		return nil, false
	}

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

// maxNaturalValue is the most natural value any collection of melds built
// from these cards could be worth: every card's own natural value, with a
// wild joker worth 0 and an ace worth its best case of 1. An upper bound,
// used only to prune a search that cannot reach the floor.
func maxNaturalValue(cards []string) int {
	sum := 0
	for _, c := range cards {
		sum += rules.NaturalCardValue(c, true)
	}
	return sum
}

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
	shapeDone := needSets == 0 && needRuns == 0 && !needCleanRun
	if shapeDone && valueSoFar >= minValue && satisfied {
		return [][]string{}, true
	}
	// Prune: melding every remaining card still leaves the total short of
	// the floor, so no arrangement of what is left can succeed. Sound
	// because a meld's NaturalValue is exactly the sum of its cards'
	// natural values (rules.ValidateMeldValue), and an ace counts at most 1.
	if valueSoFar+maxNaturalValue(hand) < minValue {
		return nil, false
	}
	n := len(hand)
	minSet, minRun := meldSizes(cfg)
	// Once the contract's shape is complete the search is no longer after a
	// particular kind of meld — it is topping the natural-value total up to
	// the floor (or still looking for somewhere to place mustInclude), and
	// either kind of meld does that. Before then, each branch is entered
	// only if the contract still wants what it lays.
	//
	// Topping up is only on the table under a profile with no per-type quota.
	// The predicate is FixedDealCount == 0 because that is exactly the
	// condition rules.MeldContributesTowardRequirement short-circuits on: a
	// fixed-contract profile (Continental) rejects any meld past the quota
	// with MELD_NO_CONTRIBUTION, so planning one would only get the action
	// bounced and strand the turn.
	canTopUp := shapeDone && cfg.FixedDealCount == 0
	wantSet := needSets > 0 || canTopUp
	wantRun := needRuns > 0 || needCleanRun || canTopUp
	if wantSet && n >= minSet {
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
			nextNeedSets := needSets
			if nextNeedSets > 0 {
				nextNeedSets--
			}
			if combo, ok := searchMeldCombo(rest, cfg, nextNeedSets, needRuns, needCleanRun, valueSoFar+mv.NaturalValue, minValue, candSatisfied, mustInclude, budget); ok {
				return append([][]string{cand}, combo...), true
			}
		}
	}
	if wantRun && n >= minRun {
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
	// max(0, ...): remove may be longer than hand — the ledger asks to drop
	// every card an action played from a list of only the ones it happened to
	// know about — and a negative capacity is a panic, not a smaller slice.
	capacity := len(hand) - len(remove)
	if capacity < 0 {
		capacity = 0
	}
	out := make([]string, 0, capacity)
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
	return findLayOffAmong(meldMeta, melds, hand, hand, cfg, gameNumber)
}

// findLayOffAmong is findLayOff restricted to candidates (a subset of hand) —
// the pending-joker branch of ChooseAction uses it to place exactly the joker
// it owes the table.
func findLayOffAmong(meldMeta map[string][]rules.MeldInfo, melds map[string][][]string, hand []string, candidates []string, cfg rules.RulesConfig, gameNumber int) (meldID string, card string, ok bool) {
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
			for _, c := range candidates {
				cand := append(append([]string(nil), existing...), c)
				if _, err := rules.ValidateMeld(cand, cfg); err != nil {
					continue
				}
				// Never shed the last card the player could legally discard.
				// ValidateLayOff refuses a lay-off from a player who is not
				// down (ROUND_REQ_NOT_MET), so reaching here means they are.
				if !handCanStillDiscard(removeCardsOnce(hand, []string{c}), cfg, true) {
					continue
				}
				// The engine treats a single natural dropped into a joker's
				// exact place as buying the joker back (swap-before-lay-off,
				// see rules.ApplyAction), and under JokerReclaimMustPlay
				// that joker must be played again before the turn can end.
				// Only take it when a place for it demonstrably exists —
				// otherwise this "lay-off" walks the agent into a discard
				// the engine will refuse, with no undo in its vocabulary.
				if cfg.JokerReclaimMustPlay {
					if joker, replaced, would := layOffWouldReclaim(existing, mi, c, cfg); would {
						postHand := append(removeCardsOnce(hand, []string{c}), joker)
						if !reclaimedJokerPlayable(meldMeta, melds, owner, i, replaced, postHand, joker, cfg, gameNumber) {
							continue
						}
					}
				}
				return mi.MeldID, c, true
			}
		}
	}
	return "", "", false
}

// layOffWouldReclaim mirrors the engine's swap-before-lay-off decision for a
// single card dropped onto a meld: a non-joker that takes the meld's first
// joker's exact place (the meld re-validates as the same type with the joker
// removed and the card added) releases that joker into the hand instead of
// piling in alongside it.
func layOffWouldReclaim(existing []string, mi rules.MeldInfo, c string, cfg rules.RulesConfig) (joker string, replaced []string, would bool) {
	if rules.IsJoker(c) {
		return "", nil, false
	}
	jokerPos := -1
	for i, mc := range existing {
		if rules.IsJoker(mc) {
			jokerPos = i
			break
		}
	}
	if jokerPos == -1 {
		return "", nil, false
	}
	joker = existing[jokerPos]
	replaced = append(append([]string(nil), existing[:jokerPos]...), existing[jokerPos+1:]...)
	replaced = append(replaced, c)
	mv, err := rules.ValidateMeld(replaced, cfg)
	if err != nil || mv.Type != mi.Type {
		return "", nil, false
	}
	return joker, replaced, true
}

// reclaimedJokerPlayable reports whether the joker a lay-off would buy back
// still has somewhere to go on the table as it will stand afterwards — the
// changed meld swapped in, every other meld as-is. A one-card post-hand needs
// no placement at all: discarding the last card of an already-down hand goes
// out, which the take-and-replay rule exempts.
func reclaimedJokerPlayable(meldMeta map[string][]rules.MeldInfo, melds map[string][][]string, changedOwner string, changedIdx int, replaced []string, postHand []string, joker string, cfg rules.RulesConfig, gameNumber int) bool {
	if len(postHand) == 1 {
		return true
	}
	rest := removeCardsOnce(postHand, []string{joker})
	if !cfg.IsFinalDeal(gameNumber) && len(rest) == 0 {
		return false
	}
	if !handCanStillDiscard(rest, cfg, true) {
		return false
	}
	for _, owner := range sortedOwners(meldMeta) {
		metas := meldMeta[owner]
		ownerMelds := melds[owner]
		for i := range metas {
			if i >= len(ownerMelds) {
				continue
			}
			target := ownerMelds[i]
			if owner == changedOwner && i == changedIdx {
				target = replaced
			}
			cand := append(append([]string(nil), target...), joker)
			if _, err := rules.ValidateMeld(cand, cfg); err == nil {
				return true
			}
		}
	}
	return false
}

// presentIn returns the entries of want that the hand actually holds,
// respecting duplicates — the guard that keeps a stale pending list from
// naming a card the agent no longer has.
func presentIn(hand []string, want []string) []string {
	counts := map[string]int{}
	for _, c := range hand {
		counts[c]++
	}
	out := make([]string, 0, len(want))
	for _, w := range want {
		if counts[w] > 0 {
			counts[w]--
			out = append(out, w)
		}
	}
	return out
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

// pickDiscard is pickWorstDiscard made table-aware, hand-aware and fallible.
//
// The ordering it applies is in smarterDiscardBetter; what this function does
// is score every legal candidate against the agent's own profile. Three
// signals separate the strengths:
//
//	dangerous   the card extends a meld already on the table, so shedding it
//	            is a gift. findLayOff has already claimed anything the agent
//	            could use itself, so whatever reaches here and fits a live
//	            meld is pure charity. This is the single biggest source of
//	            "the AI just fed me my run".
//	wanted      some seat was *seen* to take that card, or one next to it,
//	            off the pile — so it is not merely useful to somebody, it is
//	            useful to somebody who has shown you they are building there.
//	seenBefore  a rank another seat has already thrown away is a rank they
//	            are unlikely to want now. Weak evidence, so it only breaks
//	            exact points ties.
//
// Keeping meld material is not a difficulty setting — an agent that breaks up
// its own finished set every turn never gets one onto the table at all, which
// reads as "the AI doesn't meld" rather than as a beatable opponent. What
// varies by strength is how much *unfinished* material counts as material at
// all; see keepValue.
func (a *HeuristicAgent) pickDiscard(hand []string, visible VisibleState, actor string, k knowledge, rng *rand.Rand, canDiscardJoker bool) string {
	cands := a.discardCandidates(hand, visible, actor, k, canDiscardJoker)
	if len(cands) == 0 {
		// Every card is a forbidden joker (pathological, but has to return
		// something) — the caller/server is left to reject it.
		return hand[0]
	}
	sort.SliceStable(cands, func(i, j int) bool { return smarterDiscardBetter(cands[i], cands[j]) })
	// A blunder is the *second*-best card off the same list, never a card
	// from outside it. That matters more than it looks: the list has already
	// been filtered for legality, for the joker restriction and for the card
	// taken off the pile this turn, so a weak agent throws away value without
	// ever proposing a move the engine will refuse — and, crucially, without
	// ever stranding its own turn. A bad player is still a player.
	if len(cands) > 1 && a.prof.BlunderRate > 0 && rng.Float64() < a.prof.BlunderRate {
		return cands[1].card
	}
	return cands[0].card
}

// pickSmartDiscard is the strength-free entry point kept for the engine's own
// fallback path and for tests that name a difficulty rather than build an
// agent.
func pickSmartDiscard(hand []string, visible VisibleState, actor string, difficulty string, canDiscardJoker bool) string {
	a := NewHeuristicAgent(difficulty)
	visible.Rules = rules.ResolveConfig(visible.Rules)
	k := newKnowledge(visible, hand, actor, a.prof)
	if len(hand) == 0 {
		return pickWorstDiscard(hand, visible.Rules, canDiscardJoker, visible.DiscardTakenCard)
	}
	// No rng: this path is the deterministic one, so a blunder never fires.
	return a.pickDiscard(hand, visible, actor, k, rand.New(rand.NewSource(0)), canDiscardJoker)
}

// discardCandidates is every card the engine would accept as this turn's
// discard, scored.
func (a *HeuristicAgent) discardCandidates(hand []string, visible VisibleState, actor string, k knowledge, canDiscardJoker bool) []discardCandidate {
	cfg := visible.Rules
	if len(hand) == 0 {
		return nil
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
			keep := keepValue(hand, i, ownMeld[i], k, cfg)
			// Somebody is about to go out. Every point still in hand is a
			// point about to be scored against this seat, and a fragment that
			// was an investment two turns ago is now just an expensive card
			// nobody will pay for. So stop protecting anything unfinished —
			// a complete meld still goes down rather than out, because that
			// one can still be laid.
			if k.endgame && keep < keepFinished {
				keep = 0
			}
			// And, for a profile that goes that far, stop protecting the
			// *table* too. Feeding somebody's run is a real cost right up
			// until the deal is about to end, at which point the ten points
			// in hand are the certainty and the gift is the hypothetical.
			danger := a.prof.ReadTableDanger && extendsAnyLiveMeld(c, visible.Melds, cfg)
			if danger && k.endgame && a.prof.EndgameDumpsUnsafe {
				danger = false
			}
			cands = append(cands, discardCandidate{
				card:       c,
				pts:        rules.PenaltyPoints(c, false),
				keep:       keep,
				dangerous:  danger,
				wanted:     a.prof.ReadPickups && k.dangerousToOpponents(c),
				seenBefore: a.prof.Recall > 0 && k.rankPassed(c),
			})
		}
		if len(cands) > 0 {
			break
		}
	}
	return cands
}

type discardCandidate struct {
	card string
	pts  int
	// keep is how badly the agent wants to hold this card; see keepValue.
	keep       int
	dangerous  bool
	wanted     bool
	seenBefore bool
}

// smarterDiscardBetter orders candidates, best-to-discard first:
//
//  1. lower keep-value wins. This outranks everything else because meld
//     material is the whole point of the hand: face cards are simultaneously
//     the highest-penalty cards and the likeliest set material, so a
//     points-first ordering dismantled a ready-to-lay set of kings one card
//     per turn and the agent never got it onto the table. Denying an opponent
//     a single lay-off is worth much less than keeping your own meld intact.
//  2. safe (doesn't feed a live meld) beats dangerous.
//  3. a card nobody has shown an interest in beats one somebody has been
//     seen collecting. Below table danger, because a meld on the table is a
//     certainty and a pickup is an inference.
//  4. then, same as the plain worst-card heuristic, higher penalty points
//     win (shed the costliest card first).
//  5. an already-passed-on rank only breaks an exact points tie, so the
//     history signal fine-tunes which equally-costly card to let go of
//     rather than overriding the basic "get rid of the expensive card" goal.
func smarterDiscardBetter(c, best discardCandidate) bool {
	if c.keep != best.keep {
		return c.keep < best.keep
	}
	if c.dangerous != best.dangerous {
		return !c.dangerous
	}
	if c.wanted != best.wanted {
		return !c.wanted
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

// meldWouldBringDown reports whether laying these cards would leave the
// player down — the same question ValidateDiscard's gate asks when the turn
// is ended. Answered by putting the meld on a *copy* of the table and asking
// rules.PlayerIsDown, so what "down" means is never restated here and the
// real table is never touched (VisibleState hands its maps out by reference).
func meldWouldBringDown(visible VisibleState, playerID string, meld []string) bool {
	mv, err := rules.ValidateMeld(meld, visible.Rules)
	if err != nil {
		return false
	}
	melds := map[string][][]string{}
	for k, ms := range visible.Melds {
		melds[k] = append([][]string(nil), ms...)
	}
	metas := map[string][]rules.MeldInfo{}
	for k, ms := range visible.MeldMeta {
		metas[k] = append([]rules.MeldInfo(nil), ms...)
	}
	melds[playerID] = append(melds[playerID], append([]string(nil), meld...))
	metas[playerID] = append(metas[playerID], rules.MeldInfo{
		Type: mv.Type, OwnerID: playerID, WildCount: mv.WildCount,
	})
	return rules.PlayerIsDown(rules.GameState{
		Rules:      visible.Rules,
		GameNumber: visible.GameNumber,
		Melds:      melds,
		MeldMeta:   metas,
	}, playerID)
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
	if extendsAnyLiveMeld(card, visible.Melds, cfg) {
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
func extendsAnyLiveMeld(card string, melds map[string][][]string, cfg rules.RulesConfig) bool {
	// Order-independent in principle (this only asks "does any meld take it?"),
	// but iterated in a fixed order anyway so that a future early-return or
	// scoring change here can't reintroduce map-order-dependent play.
	for _, owner := range sortedMeldOwners(melds) {
		ownerMelds := melds[owner]
		for _, existing := range ownerMelds {
			cand := append(append([]string(nil), existing...), card)
			if _, err := rules.ValidateMeld(cand, cfg); err != nil {
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
