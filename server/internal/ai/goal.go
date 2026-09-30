package ai

import (
	"math/rand"
	"sort"

	"zolik/server/internal/rules"
)

// What the hand is for, this deal.
//
// The discard used to value material in the abstract: a finished meld was
// worth keeping, a pair or a part-run was worth keeping at the strengths that
// look at fragments, and the tie between loose cards went to shedding the most
// expensive one. Every one of those rules is right in Žolík Classic, where any
// meld helps, and every one of them is wrong somewhere in Continental, where
// the deal names what counts:
//
//   - Deal seven asks for three runs. A set of twos is not material for that
//     contract, it is three cards the hand cannot use, and protecting it while
//     shedding the eights and nines drove whole tables into hands of 2s, 3s, 4s
//     and jokers that could never come down. Nobody went down, so nobody went
//     out, and the deal never ended.
//   - Every deal has a 35-point floor. A hand whose protected material is sets
//     of low cards cannot reach it however the cards fall, and "shed the most
//     expensive loose card" throws away exactly the kings that would lift it.
//     The agent held its plan and discarded its way out of it, forever.
//   - Once down, a pair is only worth holding while the hand is big enough to
//     finish it *and still have a card to discard*. A down seat holding two
//     cards that make a pair can never lay the set it is waiting for — the
//     third card arrives, and laying all three would leave nothing to end the
//     turn with — so it kept the pair, threw away every card it drew, and
//     never went out.
//
// goal is the reading of the contract the discard needs to avoid all three. It
// is derived fresh from the visible state on every decision, like everything
// else the agent knows, and it is the same for every strength: following the
// contract is not a skill level, it is playing the game.
type goal struct {
	// quota is set while a per-type contract (Continental's rotating one) is
	// still unmet. Only then does the kind of meld matter: before a player is
	// down under a quota the engine refuses any meld the contract did not ask
	// for, and once they are down every meld and every lay-off is welcome.
	quota      bool
	sets, runs bool
	// needSets and needRuns are how many of each the quota still asks for.
	needSets, needRuns int

	// floor is the initial-meld point floor still to be reached, and slots is
	// how many cards the contract's melds take between them — so floor/slots
	// is what an average card of a qualifying hand has to be worth.
	floor int
	slots int

	down bool
	// room is, for a player already down, whether the hand is big enough for a
	// fragment in it to be finished into a new meld with a card left over to
	// discard. Without it a fragment is not an investment, it is a card that
	// can never leave the hand except as a discard.
	room bool
}

func goalFor(v VisibleState, actor string, hand []string) goal {
	cfg := v.Rules
	minSet, minRun := meldSizes(cfg)
	g := goal{down: v.RoundReqMet[actor]}
	if g.down {
		// After this turn's discard the hand holds len(hand)-1 cards, and the
		// card that finishes a fragment arrives with the next draw. Laying a
		// minimum set then has to leave a card to discard — except on a final
		// deal, which may be gone out of by melding the whole hand.
		after := len(hand) - 1
		need := minSet
		if cfg.IsFinalDeal(v.GameNumber) {
			need = minSet - 1
		}
		g.room = after >= need
		return g
	}

	st := rulesStateForAI(v, actor)
	req := cfg.ContractFor(v.GameNumber)
	setsBefore, runsBefore, hasClean := rules.PlayerMeldCounts(st, actor)
	needSets := max(req.Sets-setsBefore, 0)
	needRuns := max(req.Runs-runsBefore, 0)
	needClean := req.RequireCleanRun && !hasClean
	if cfg.FixedDealCount > 0 && (needSets > 0 || needRuns > 0 || needClean) {
		g.quota = true
		g.sets = needSets > 0
		g.runs = needRuns > 0 || needClean
		g.needSets, g.needRuns = needSets, max(needRuns, b2i(needClean))
	}
	if cfg.InitialMeldMinimum > 0 {
		g.floor = cfg.InitialMeldMinimum - rules.PlayerInitialMeldNaturalValue(st, actor)
		g.slots = needSets*minSet + needRuns*minRun
		if needClean && needRuns == 0 {
			g.slots += minRun
		}
		// A contract that names no shape (Žolík Classic under a house floor)
		// still has to be reached in some number of melds; two minimum sets
		// is the least a floor worth having takes.
		g.slots = max(g.slots, 2*minSet)
	}
	return g
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// wants reports whether a meld of this kind is material for the goal.
func (g goal) wants(t rules.MeldType) bool {
	if !g.quota {
		return true
	}
	switch t {
	case rules.MeldSet:
		return g.sets
	case rules.MeldRun:
		return g.runs
	}
	return false
}

// pullsWeight reports whether a card is worth enough to carry its share of the
// point floor.
func (g goal) pullsWeight(card string) bool {
	return g.floor <= 0 || floorValue(card)*g.slots >= g.floor
}

// floorValue is the most a card can put toward the initial-meld floor. An ace
// is counted at its set value, which is also what it is worth at the top of a
// run.
func floorValue(card string) int {
	return rules.NaturalSetCardValue(card)
}

// shortOfFloor reports that the material the hand is protecting cannot reach
// the point floor even if every piece of it is finished: its whole natural
// value, added up, is still under the line. keep holds each position's keep
// value as the goal-blind scoring left it.
//
// It is an optimistic test — a fragment is counted at the value of the cards
// already in it, and nothing is deducted for material the contract has no slot
// for — so when it fires the hand is genuinely behind, and the discard should
// stop defending low material and start holding the cards that can lift it.
func (g goal) shortOfFloor(hand []string, keep []int) bool {
	if g.down || g.floor <= 0 {
		return false
	}
	total := 0
	for i, c := range hand {
		if keep[i] > 0 && !rules.IsJoker(c) {
			total += floorValue(c)
		}
	}
	return total < g.floor
}

// staleEndgameRound is the lap past which a hand that is short of the floor
// stops trusting the endgame reading. Beyond the longest deal the sweep
// finishes normally (Continental's 99th percentile is under sixty laps with
// every seat playing Hard), so it changes nothing about a deal that is ending
// and only acts on one that has stopped.
const staleEndgameRound = 60

// contractPlan is the hand read against the contract: which cards are working
// toward one of the melds it still asks for, and what those melds would be
// worth toward the floor once finished.
//
// Deciding what a card is for one card at a time is what went wrong. A hand
// holding four fours and four sixes against a contract of one set and two runs
// saw eight cards of set material and protected every one of them, when the
// contract has room for three; the runs it actually lacked had nowhere to grow,
// because every card it drew was the loosest card in hand and went straight
// back. The plan assigns cards to the contract's slots — needSets sets and
// needRuns runs, disjoint — so material past what the contract can use is
// loose, however finished it looks.
type contractPlan struct {
	used []bool
	// value is what the planned melds would put toward the floor once every
	// one of them is finished, jokers standing in for the missing cards.
	value int
}

// planGroup is one slot's worth of material: the hand positions of the
// naturals that would go into it.
type planGroup struct {
	idx   []int
	value int
}

// maxPlanGroups is how many of the strongest candidates of each kind the plan
// search considers. A hand has thirteen-odd cards; the groups below the top
// dozen are single cards the stronger ones already cover.
const maxPlanGroups = 12

func bestContractPlan(hand []string, cfg rules.RulesConfig, g goal) contractPlan {
	needSets, needRuns := g.needSets, g.needRuns
	minSet, minRun := meldSizes(cfg)
	sets := setGroups(hand, minSet)
	runs := runGroups(hand, minRun)

	best := contractPlan{used: make([]bool, len(hand))}
	bestCover, bestReach := -1, false
	used := make([]bool, len(hand))
	var chosen []planGroup

	var pick func(pool []planGroup, from, left int, next func())
	pick = func(pool []planGroup, from, left int, next func()) {
		if left == 0 {
			next()
			return
		}
		placed := false
		for i := from; i < len(pool); i++ {
			grp := pool[i]
			if overlaps(used, grp.idx) {
				continue
			}
			placed = true
			mark(used, grp.idx, true)
			chosen = append(chosen, grp)
			pick(pool, i+1, left-1, next)
			chosen = chosen[:len(chosen)-1]
			mark(used, grp.idx, false)
		}
		if !placed {
			// Nothing left to fill this slot with: the slot is simply empty
			// in this plan, and the rest of the plan still counts.
			next()
		}
	}
	score := func() {
		cover, value := 0, 0
		for _, grp := range chosen {
			cover += len(grp.idx)
			value += grp.value
		}
		reach := g.floor <= 0 || value >= g.floor
		// A plan that can reach the floor beats one that cannot, whatever
		// else is true of either: the one that cannot is a plan for never
		// going down. Then the most cards already in place, then value.
		better := false
		switch {
		case reach != bestReach:
			better = reach
		case cover != bestCover:
			better = cover > bestCover
		default:
			better = value > best.value
		}
		if bestCover < 0 || better {
			bestCover, bestReach = cover, reach
			best.value = value
			copy(best.used, used)
		}
	}
	pick(sets, 0, needSets, func() { pick(runs, 0, needRuns, score) })
	return best
}

func overlaps(used []bool, idx []int) bool {
	for _, i := range idx {
		if used[i] {
			return true
		}
	}
	return false
}

func mark(used []bool, idx []int, v bool) {
	for _, i := range idx {
		used[i] = v
	}
}

// setGroups is, per rank, the naturals that could share one set — one card of
// each suit — and a second group from the duplicates two packs deal.
func setGroups(hand []string, minSet int) []planGroup {
	byRank := map[int][]int{}
	for i, c := range hand {
		if rules.IsJoker(c) {
			continue
		}
		byRank[rules.CardRank(c)] = append(byRank[rules.CardRank(c)], i)
	}
	var out []planGroup
	for _, idxs := range byRank {
		taken := map[int]bool{}
		for len(taken) < len(idxs) {
			suits := map[string]bool{}
			var grp []int
			for _, i := range idxs {
				s := rules.CardSuit(hand[i])
				if taken[i] || suits[s] {
					continue
				}
				suits[s] = true
				taken[i] = true
				grp = append(grp, i)
			}
			size := min(max(len(grp), minSet), rules.MaxSetSize)
			out = append(out, planGroup{idx: grp, value: size * rules.NaturalSetCardValue(hand[grp[0]])})
		}
	}
	return strongest(out)
}

// runGroups is, per suit and per window of minRun consecutive ranks, the
// naturals that fall in it — one card per rank. The ace sits at both ends of
// the order, as it does in a run.
func runGroups(hand []string, minRun int) []planGroup {
	const low, high = 1, 14 // the ace, at each end
	var out []planGroup
	for _, suit := range suits {
		// slots[r] is the hand positions holding rank r of this suit, r in
		// run numbering: 1 the low ace, 2..13 two to king, 14 the high ace.
		var slots [high + 1][]int
		for i, c := range hand {
			if rules.IsJoker(c) || rules.CardSuit(c) != suit {
				continue
			}
			r := rules.CardRank(c) + 2
			if r == high {
				slots[low] = append(slots[low], i)
			}
			slots[r] = append(slots[r], i)
		}
		for start := low; start+minRun-1 <= high; start++ {
			var grp []int
			value := 0
			for r := start; r < start+minRun; r++ {
				value += runSlotValue(r)
				if len(slots[r]) > 0 {
					grp = append(grp, slots[r][0])
				}
			}
			if len(grp) == 0 {
				continue
			}
			out = append(out, planGroup{idx: grp, value: value})
		}
	}
	// A window with both aces in it would name one card twice.
	return strongest(dedupe(out))
}

func runSlotValue(r int) int {
	switch {
	case r == 1:
		return rules.AceRunLowValue
	case r == 14:
		return rules.AceMeldValue
	case r >= 10:
		return 10
	}
	return r
}

func dedupe(groups []planGroup) []planGroup {
	out := groups[:0]
	for _, grp := range groups {
		seen := map[int]bool{}
		clean := grp.idx[:0:0]
		for _, i := range grp.idx {
			if !seen[i] {
				seen[i] = true
				clean = append(clean, i)
			}
		}
		grp.idx = clean
		out = append(out, grp)
	}
	return out
}

// strongest keeps the maxPlanGroups groups with the most cards in them, then
// the most value, in a stable order so the plan never depends on map order.
func strongest(groups []planGroup) []planGroup {
	sort.SliceStable(groups, func(i, j int) bool {
		a, b := groups[i], groups[j]
		if len(a.idx) != len(b.idx) {
			return len(a.idx) > len(b.idx)
		}
		if a.value != b.value {
			return a.value > b.value
		}
		return a.idx[0] < b.idx[0]
	})
	if len(groups) > maxPlanGroups {
		groups = groups[:maxPlanGroups]
	}
	return groups
}

// staleRelease reports that this discard should be any legal card rather than
// the best one, because the deal has run far past any normal length.
//
// A deal that long is a table where the cards somebody needs to go out are
// sitting in hands that cannot use them: every down seat keeps its cheapest
// and its safest card, every seat still building keeps the half-run it is
// waiting on, and they are the same cards every lap. So the seats that cannot
// make anything of their hands stop choosing, and the cards start moving round
// the table again:
//
//   - a down seat with no room for a new meld, always: nothing it holds can
//     leave its hand except as a discard, so which discard is only a question
//     of who else might want it;
//   - a seat still building toward the contract, half the time, so that it
//     keeps some of its plan.
//
// A down seat that does have room keeps choosing, and keeps its fragments at
// every strength (see discardCandidates): it is the one seat that can open a
// new meld, which is a new opening for everybody.
func staleRelease(v VisibleState, actor string, hand []string, rng *rand.Rand) bool {
	if v.Round <= staleEndgameRound {
		return false
	}
	if !v.RoundReqMet[actor] {
		return rng.Intn(2) == 0
	}
	return !goalFor(v, actor, hand).room
}
