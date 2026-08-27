package rules

// PlayerMeldCounts returns qualifying sets and runs the player has laid this
// game, and whether at least one of those runs is "clean" (zero wild cards).
func PlayerMeldCounts(state GameState, playerID string) (sets, runs int, hasCleanRun bool) {
	cfg := effectiveRules(state)
	metas := state.MeldMeta[playerID]
	melds := state.Melds[playerID]
	for i, mi := range metas {
		if i >= len(melds) {
			break
		}
		switch mi.Type {
		case MeldSet:
			if len(melds[i]) >= cfg.MinSetSize {
				sets++
			}
		case MeldRun:
			if len(melds[i]) >= cfg.MinRunSize {
				runs++
				if meldWildCount(melds[i], mi, cfg) == 0 {
					hasCleanRun = true
				}
			}
		}
	}
	return sets, runs, hasCleanRun
}

// meldWildCount re-derives a meld's wild count from the cards on the table,
// falling back to the recorded count only if the meld no longer validates.
// Melds grow through lay-offs, so the count captured when the meld was laid
// is not necessarily the count it has now.
func meldWildCount(cards []string, mi MeldInfo, cfg RulesConfig) int {
	if mv, err := ValidateMeld(cards, cfg); err == nil {
		return mv.WildCount
	}
	return mi.WildCount
}

// IsCleanRun reports whether cards form a qualifying joker-free run.
func IsCleanRun(cards []string, cfg RulesConfig) bool {
	if len(cards) < cfg.MinRunSize {
		return false
	}
	mv, err := ValidateMeld(cards, cfg)
	return err == nil && mv.Type == MeldRun && mv.WildCount == 0
}

// PlayerMeetsRoundRequirement reports whether the player has the required melds on table.
func PlayerMeetsRoundRequirement(state GameState, playerID string) bool {
	cfg := effectiveRules(state)
	req := cfg.ContractFor(state.GameNumber)
	sets, runs, hasCleanRun := PlayerMeldCounts(state, playerID)
	if sets < req.Sets || runs < req.Runs {
		return false
	}
	if req.RequireCleanRun && !hasCleanRun {
		return false
	}
	return true
}

// PlayerIsDown re-derives, from nothing but the table, whether playerID has
// satisfied everything needed to be "down": the deal's contract (sets/runs
// and, where configured, a joker-free run) and the initial-meld point floor.
//
// This is the single definition of down-ness. GameState.RoundReqMet caches
// its answer, and every place that can change a player's laid melds has to
// keep that cache honest — see refreshRoundReqMet.
func PlayerIsDown(state GameState, playerID string) bool {
	if !PlayerMeetsRoundRequirement(state, playerID) {
		return false
	}
	cfg := effectiveRules(state)
	return cfg.InitialMeldMinimum <= 0 ||
		PlayerInitialMeldNaturalValue(state, playerID) >= cfg.InitialMeldMinimum
}

// refreshRoundReqMet brings RoundReqMet[ownerID] back in line with the table
// after ownerID's melds changed.
//
// RoundReqMet used to be written in exactly one place — ValidateLayMeld — as
// if a player's own lay_meld were the only thing that could ever change their
// melds. It isn't: a lay_off grows a meld (and any down player may lay off
// onto *anyone's* meld, including one belonging to a player who is not down
// yet), and a joker swap can turn a wild-carrying run into a joker-free one.
// Either can be the event that finally satisfies the contract or lifts the
// player over the point floor, and neither used to re-derive the flag. The
// player was then locked out of laying off for the rest of the deal — the
// engine refusing with ROUND_REQ_NOT_MET while the table plainly showed the
// requirement met — with no way back, since only a further lay_meld would
// have recomputed it and a player with no meldable cards left has none.
//
// Monotonic on purpose: going down is permanent for the deal, so this only
// ever sets the flag. Nothing here may take a player back up.
func refreshRoundReqMet(state GameState, ownerID string) {
	if state.RoundReqMet == nil || state.RoundReqMet[ownerID] {
		return
	}
	if PlayerIsDown(state, ownerID) {
		state.RoundReqMet[ownerID] = true
	}
}

// PlayerInitialMeldNaturalValue sums natural card values across all melds the player laid this game.
func PlayerInitialMeldNaturalValue(state GameState, playerID string) int {
	cfg := effectiveRules(state)
	total := 0
	melds := state.Melds[playerID]
	for _, cards := range melds {
		mv, err := ValidateMeld(cards, cfg)
		if err != nil {
			continue
		}
		total += mv.NaturalValue
	}
	return total
}

// MeldContributesTowardRequirement reports whether a new qualifying meld moves the
// player toward the current deal's pattern before RoundReqMet is set.
// wildCount is the wild-card count of the meld being laid (0 = clean run).
func MeldContributesTowardRequirement(state GameState, playerID string, meldType MeldType, cardCount int, wildCount int) bool {
	if state.RoundReqMet[playerID] {
		return true
	}
	cfg := effectiveRules(state)
	req := cfg.ContractFor(state.GameNumber)
	setsBefore, runsBefore, _ := PlayerMeldCounts(state, playerID)

	addsSet := meldType == MeldSet && cardCount >= cfg.MinSetSize
	addsRun := meldType == MeldRun && cardCount >= cfg.MinRunSize
	if !addsSet && !addsRun {
		return false
	}

	if cfg.FixedDealCount == 0 {
		// A non-rotating profile (e.g. Žolík Classic) has no per-type count
		// quota at all — every valid meld may be laid freely at any time.
		// The only thing gating "down" status is PlayerMeetsRoundRequirement
		// (e.g. its clean-run rule), not what gets laid along the way.
		return true
	}

	if addsSet && setsBefore < req.Sets {
		return true
	}
	if addsRun && runsBefore < req.Runs {
		return true
	}
	return false
}

// AllTableMelds returns every meld currently on the table (all players).
func AllTableMelds(state GameState) [][]string {
	var out [][]string
	for _, melds := range state.Melds {
		for _, m := range melds {
			out = append(out, append([]string(nil), m...))
		}
	}
	return out
}

// HandPenaltyTotal scores leftover cards at round end.
// Aces count as 1 when they sit in a natural run fragment in hand or can extend a table run.
func HandPenaltyTotal(hand []string, cfg RulesConfig) int {
	return HandPenaltyTotalWithMelds(hand, nil, cfg)
}

func HandPenaltyTotalWithMelds(hand []string, tableMelds [][]string, cfg RulesConfig) int {
	sum := 0
	for _, c := range hand {
		sum += handCardPenalty(c, hand, tableMelds, cfg)
	}
	return sum
}

func handCardPenalty(card string, hand []string, tableMelds [][]string, cfg RulesConfig) int {
	if IsAce(card) {
		if aceCountsAsNaturalInHand(card, hand) {
			return 1
		}
		for _, meld := range tableMelds {
			if aceExtendsRunAsNatural(card, meld, cfg) {
				return 1
			}
		}
		return 25
	}
	return PenaltyPoints(card, false)
}

func aceCountsAsNaturalInHand(ace string, hand []string) bool {
	suit := CardSuit(ace)
	has2, hasQ, hasK := false, false, false
	for _, c := range hand {
		if c == ace || IsJoker(c) {
			continue
		}
		if IsAce(c) {
			continue
		}
		if CardSuit(c) != suit {
			continue
		}
		switch CardRank(c) {
		case 0:
			has2 = true
		case 10:
			hasQ = true
		case 11:
			hasK = true
		}
	}
	if has2 {
		return true
	}
	return hasQ && hasK
}

func minMaxRunRank(cards []string) (min, max int) {
	min, max = 99, 0
	for _, c := range cards {
		if IsJoker(c) || IsAce(c) {
			continue
		}
		r := cardToRunRank(c)
		if r < 2 {
			continue
		}
		if r < min {
			min = r
		}
		if r > max {
			max = r
		}
	}
	return min, max
}

func aceExtendsRunAsNatural(ace string, meld []string, cfg RulesConfig) bool {
	minRun := cfg.MinRunSize
	if minRun == 0 {
		minRun = 4
	}
	if len(meld) < minRun || !IsAce(ace) {
		return false
	}
	if _, err := ValidateMeld(meld, cfg); err != nil {
		return false
	}
	suit := CardSuit(ace)
	minR, maxR := minMaxRunRank(meld)

	try := func(extended []string, atEnd bool) bool {
		if len(extended) != len(meld)+1 {
			return false
		}
		mv, err := ValidateMeld(extended, cfg)
		if err != nil || mv.Type != MeldRun {
			return false
		}
		if mv.ResolvedSuit != "" && mv.ResolvedSuit != suit {
			return false
		}
		if atEnd {
			return maxR == 13 && extended[len(extended)-1] == ace
		}
		return minR == 2 && extended[0] == ace
	}

	appended := append(append([]string(nil), meld...), ace)
	if try(appended, true) {
		return true
	}
	prepended := append([]string{ace}, meld...)
	return try(prepended, false)
}
