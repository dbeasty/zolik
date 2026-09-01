package rummytiles

import (
	"fmt"
	"strconv"

	"zolik/server/internal/module"
)

// Module is the Rummy Tiles game module.
type Module struct{}

// New returns the module. Stateless: every method takes the state it works on.
func New() *Module { return &Module{} }

var _ module.GameModule = (*Module)(nil)

// NewMatch deals the first round of a fresh match.
func (m *Module) NewMatch(cfg module.MatchConfig, players []module.PlayerRef, seed int64) (module.State, error) {
	if len(players) < 2 || len(players) > 4 {
		return nil, module.Error{Code: ErrWrongPlayerCount, Message: "rummy tiles seats two to four"}
	}

	s := &GameState{
		Status:      "active",
		Variation:   cfg.Variation,
		Hands:       map[string][]string{},
		InitialMeld: map[string]bool{},
		Scores:      map[string]int{},
		Seed:        seed,
	}
	for _, p := range players {
		s.Players = append(s.Players, p.ID)
	}

	v := resolveVariation(cfg)
	s.TargetScore = cfg.Opt(OptTargetScore, v.targetScore)
	s.RoundLimit = cfg.Opt(OptRoundLimit, v.roundLimit)
	s.PoolExhaustionLowestWins = cfg.Opt(OptPoolExhaustion, module.BoolOpt(v.poolExhaustionLowestWins)) == module.OptOn
	s.Pause = cfg.PauseBetweenRounds(true)

	dealRound(s)
	return encode(s)
}

// dealRound shuffles and deals one round: fourteen tiles each, the rest in
// the pool, no sets on the table, nobody's initial meld yet made.
func dealRound(s *GameState) {
	tiles := shuffle(buildTiles(), s.Seed+int64(s.RoundNumber)*7919+1)

	s.Hands = map[string][]string{}
	for _, p := range s.Players {
		s.Hands[p] = nil
	}
	for i := 0; i < 14; i++ {
		for _, p := range s.Players {
			s.Hands[p] = append(s.Hands[p], tiles[len(tiles)-1])
			tiles = tiles[:len(tiles)-1]
		}
	}
	s.Pool = tiles
	s.Sets = nil
	s.NextSetID = 0
	s.InitialMeld = map[string]bool{}

	s.Current = s.Players[module.StartingSeat(s.Seed+int64(s.RoundNumber)*7919+2, len(s.Players))]
	beginTurn(s)
}

// beginTurn opens a fresh workspace for whoever is on turn — a scratch copy
// of the table, per the package doc.
func beginTurn(s *GameState) {
	s.Workspace = &Workspace{Sets: cloneSets(s.Sets)}
}

func cloneSets(sets []Set) []Set {
	out := make([]Set, len(sets))
	for i, set := range sets {
		out[i] = Set{ID: set.ID, Kind: set.Kind, Cards: append([]string(nil), set.Cards...)}
	}
	return out
}

// Apply validates and applies one move. The state is decoded fresh on every
// call, so a refused action returns the caller's own bytes untouched.
func (m *Module) Apply(raw module.State, playerID string, a module.Action) (module.State, []module.Event, error) {
	s, err := decode(raw)
	if err != nil {
		return raw, nil, err
	}
	if s.Status != "active" {
		return raw, nil, errCode(ErrGameNotActive)
	}

	if s.Intermission.Open {
		if a.Verb != module.VerbContinue {
			return raw, nil, errCode(ErrGameNotActive)
		}
		if err := s.Intermission.Mark(s.Players, playerID); err != nil {
			return raw, nil, err
		}
		var events []module.Event
		if s.Intermission.Settled(s.Players) {
			s.Intermission.Close()
			dealRound(s)
			events = []module.Event{{Type: "round_started", Data: map[string]any{"roundNumber": s.RoundNumber}}}
		}
		out, err := encode(s)
		return out, events, err
	}

	if s.Current != playerID {
		return raw, nil, errCode(ErrNotYourTurn)
	}
	if s.Workspace == nil {
		return raw, nil, errCode(ErrGameNotActive)
	}

	var events []module.Event
	switch a.Verb {
	case VerbPlace:
		events, err = applyPlace(s, playerID, a)
	case VerbAdd:
		events, err = applyAdd(s, playerID, a)
	case VerbTake:
		events, err = applyTake(s, playerID, a)
	case VerbSplit:
		events, err = applySplit(s, playerID, a)
	case VerbSwapJoker:
		events, err = applySwapJoker(s, playerID, a)
	case VerbResetTurn:
		events, err = applyResetTurn(s, playerID)
	case VerbCommit:
		events, err = applyCommit(s, playerID)
	case VerbDraw:
		events, err = applyDraw(s, playerID)
	default:
		err = module.Error{Code: ErrUnknownAction, Message: a.Verb}
	}
	if err != nil {
		return raw, nil, err
	}
	out, err := encode(s)
	return out, events, err
}

// touchable reports whether playerID may add/take/split/swap_joker setID this
// turn: their own in-progress lay always qualifies, but a set that predates
// this turn is off limits until their initial meld is made.
func touchable(s *GameState, playerID, setID string) bool {
	return s.Workspace.NewSetIDs[setID] || s.InitialMeld[playerID]
}

func applyPlace(s *GameState, playerID string, a module.Action) ([]module.Event, error) {
	if len(a.Cards) == 0 {
		return nil, errCode(ErrTileDoesNotFit)
	}
	hand, ok := removeTiles(s.Hands[playerID], a.Cards)
	if !ok {
		return nil, errCode(ErrTileNotInHand)
	}
	s.Hands[playerID] = hand

	ws := s.Workspace
	id := fmt.Sprintf("s%d", s.NextSetID)
	s.NextSetID++
	ws.Sets = append(ws.Sets, Set{ID: id, Cards: append([]string(nil), a.Cards...)})
	if ws.NewSetIDs == nil {
		ws.NewSetIDs = map[string]bool{}
	}
	ws.NewSetIDs[id] = true
	ws.PlayedFromHand = append(ws.PlayedFromHand, a.Cards...)

	return []module.Event{{Type: "tiles_placed", Data: map[string]any{
		"playerId": playerID, "setId": id, "count": len(a.Cards),
	}}}, nil
}

// applyAdd moves tiles onto an existing workspace set, from the hand or from
// the loose tray — one action, one source, never a mix of the two.
func applyAdd(s *GameState, playerID string, a module.Action) ([]module.Event, error) {
	if len(a.Cards) == 0 {
		return nil, errCode(ErrTileDoesNotFit)
	}
	ws := s.Workspace
	idx := setIndex(ws.Sets, a.Target)
	if idx < 0 {
		return nil, errCode(ErrNoSuchSet)
	}
	if !touchable(s, playerID, a.Target) {
		return nil, errCode(ErrInitialMeldOnly)
	}

	if hand, ok := removeTiles(s.Hands[playerID], a.Cards); ok {
		s.Hands[playerID] = hand
		ws.PlayedFromHand = append(ws.PlayedFromHand, a.Cards...)
	} else if tray, ok := removeTiles(ws.Tray, a.Cards); ok {
		ws.Tray = tray
	} else {
		return nil, errCode(ErrTileNotInHand)
	}
	ws.Sets[idx].Cards = append(ws.Sets[idx].Cards, a.Cards...)

	return []module.Event{{Type: "tiles_added", Data: map[string]any{
		"playerId": playerID, "setId": a.Target, "count": len(a.Cards),
	}}}, nil
}

// applyTake pulls tiles out of a workspace set into the loose tray. Any tile,
// unrestricted — a set left too small to be legal is caught at commit, not
// here, which is what makes "may not leave a group of three as a pair" a
// consequence of the general validity check rather than a rule of its own.
func applyTake(s *GameState, playerID string, a module.Action) ([]module.Event, error) {
	if len(a.Cards) == 0 {
		return nil, errCode(ErrTileDoesNotFit)
	}
	ws := s.Workspace
	idx := setIndex(ws.Sets, a.Target)
	if idx < 0 {
		return nil, errCode(ErrNoSuchSet)
	}
	if !touchable(s, playerID, a.Target) {
		return nil, errCode(ErrInitialMeldOnly)
	}
	cards, ok := removeTiles(ws.Sets[idx].Cards, a.Cards)
	if !ok {
		return nil, errCode(ErrTileDoesNotFit)
	}
	ws.Sets[idx].Cards = cards
	ws.Tray = append(ws.Tray, a.Cards...)

	if len(ws.Sets[idx].Cards) == 0 {
		delete(ws.NewSetIDs, a.Target)
		ws.Sets = append(ws.Sets[:idx:idx], ws.Sets[idx+1:]...)
	}

	return []module.Event{{Type: "tiles_taken", Data: map[string]any{
		"playerId": playerID, "setId": a.Target, "count": len(a.Cards),
	}}}, nil
}

// applySplit breaks a run in two at Params["position"] — the index, in the
// run's own canonical order, of the first tile of the new second run. Only a
// currently-valid run may be split; a workspace set mid-repair has nothing
// coherent to split it at.
func applySplit(s *GameState, playerID string, a module.Action) ([]module.Event, error) {
	ws := s.Workspace
	idx := setIndex(ws.Sets, a.Target)
	if idx < 0 {
		return nil, errCode(ErrNoSuchSet)
	}
	if !touchable(s, playerID, a.Target) {
		return nil, errCode(ErrInitialMeldOnly)
	}
	kind, canonical, ok := validateSet(ws.Sets[idx].Cards)
	if !ok || kind != "run" {
		return nil, errCode(ErrNotARun)
	}
	pos, err := strconv.Atoi(a.Params["position"])
	if err != nil || pos < 1 || pos >= len(canonical) {
		return nil, errCode(ErrBadSplitPosition)
	}
	left, right := canonical[:pos], canonical[pos:]
	if len(left) < 3 || len(right) < 3 {
		return nil, errCode(ErrBadSplitPosition)
	}

	ws.Sets[idx].Cards = left
	newID := fmt.Sprintf("s%d", s.NextSetID)
	s.NextSetID++
	ws.Sets = append(ws.Sets, Set{ID: newID, Cards: right})
	if ws.NewSetIDs[a.Target] {
		ws.NewSetIDs[newID] = true
	}

	return []module.Event{{Type: "run_split", Data: map[string]any{
		"playerId": playerID, "setId": a.Target, "newSetId": newID,
	}}}, nil
}

// applySwapJoker replaces a joker in a set with the hand tile it stands for.
// The freed joker joins the tray, exactly as a take would leave it — which is
// what makes "the joker must be used in a set this turn" fall out of the same
// "the tray must be empty" commit rule as everything else the tray holds,
// rather than needing tracking of its own.
func applySwapJoker(s *GameState, playerID string, a module.Action) ([]module.Event, error) {
	if len(a.Cards) != 1 || isJoker(a.Cards[0]) {
		return nil, errCode(ErrTileDoesNotFit)
	}
	replacement := a.Cards[0]
	ws := s.Workspace
	idx := setIndex(ws.Sets, a.Target)
	if idx < 0 {
		return nil, errCode(ErrNoSuchSet)
	}
	if !touchable(s, playerID, a.Target) {
		return nil, errCode(ErrInitialMeldOnly)
	}
	if !hasTile(s.Hands[playerID], replacement) {
		return nil, errCode(ErrTileNotInHand)
	}
	kind, canonical, ok := validateSet(ws.Sets[idx].Cards)
	if !ok {
		return nil, errCode(ErrTableNotValid)
	}

	jokerToSwap, matchErr := jokerMatching(kind, canonical, replacement)
	if matchErr != "" {
		return nil, errCode(matchErr)
	}

	cards, _ := removeTile(ws.Sets[idx].Cards, jokerToSwap)
	ws.Sets[idx].Cards = append(cards, replacement)
	s.Hands[playerID], _ = removeTile(s.Hands[playerID], replacement)
	ws.PlayedFromHand = append(ws.PlayedFromHand, replacement)
	ws.Tray = append(ws.Tray, jokerToSwap)

	return []module.Event{{Type: "joker_swapped", Data: map[string]any{
		"playerId": playerID, "setId": a.Target, "replacement": replacement,
	}}}, nil
}

// jokerMatching finds the joker in a valid, canonicalized set that replacement
// may take the place of — a group's number with a free colour, or a run's
// one positionally-implied number. Returns "" and an error code if none does.
func jokerMatching(kind string, canonical []string, replacement string) (string, string) {
	switch kind {
	case "group":
		number := 0
		usedColours := map[string]bool{}
		for _, c := range canonical {
			if !isJoker(c) {
				number = numberOf(c)
				usedColours[colourOf(c)] = true
			}
		}
		if numberOf(replacement) != number || usedColours[colourOf(replacement)] {
			return "", ErrJokerSwapMismatch
		}
	case "run":
		colour := colourOf(realTileIn(canonical))
		if colourOf(replacement) != colour {
			return "", ErrJokerSwapMismatch
		}
		want := numberOf(replacement) - runStartNumber(canonical)
		if want < 0 || want >= len(canonical) || !isJoker(canonical[want]) {
			return "", ErrJokerSwapMismatch
		}
		return canonical[want], ""
	}
	for _, c := range canonical {
		if isJoker(c) {
			return c, ""
		}
	}
	return "", ErrNoJokerInSet
}

func applyResetTurn(s *GameState, playerID string) ([]module.Event, error) {
	ws := s.Workspace
	s.Hands[playerID] = append(s.Hands[playerID], ws.PlayedFromHand...)
	beginTurn(s)
	return []module.Event{{Type: "turn_reset", Data: map[string]any{"playerId": playerID}}}, nil
}

// applyDraw is the fallback: whatever was rearranged this turn is put back —
// matching a real player returning the table to how they found it — one tile
// is taken from the pool, and the turn passes. If the pool is empty, nobody
// can be dealt a fallback tile, so the round ends instead (a simplification
// of the physical rule, which cycles every seat first; see rounds.go).
func applyDraw(s *GameState, playerID string) ([]module.Event, error) {
	s.Hands[playerID] = append(s.Hands[playerID], s.Workspace.PlayedFromHand...)
	if len(s.Pool) == 0 {
		return endRoundPoolExhausted(s), nil
	}
	drawn := s.Pool[len(s.Pool)-1]
	s.Pool = s.Pool[:len(s.Pool)-1]
	s.Hands[playerID] = append(s.Hands[playerID], drawn)

	s.Current = nextPlayer(s.Players, playerID)
	beginTurn(s)
	return []module.Event{{Type: "tile_drawn", Data: map[string]any{"playerId": playerID}}}, nil
}

// applyCommit is where the rules live — see the package doc. Every workspace
// set must validate, the tray must be empty, at least one tile must have come
// from hand, and a player's first successful commit must total 30 points from
// hand-only new sets.
func applyCommit(s *GameState, playerID string) ([]module.Event, error) {
	ws := s.Workspace
	if len(ws.Tray) > 0 {
		return nil, errCode(ErrTrayNotEmpty)
	}
	if len(ws.PlayedFromHand) == 0 {
		return nil, errCode(ErrNothingPlayed)
	}

	canonicalSets := make([]Set, 0, len(ws.Sets))
	newSetsValue := 0
	for _, set := range ws.Sets {
		kind, canonical, ok := validateSet(set.Cards)
		if !ok {
			return nil, errCode(ErrTableNotValid)
		}
		canonicalSets = append(canonicalSets, Set{ID: set.ID, Kind: kind, Cards: canonical})
		if ws.NewSetIDs[set.ID] {
			newSetsValue += setValueOf(kind, canonical)
		}
	}
	if !s.InitialMeld[playerID] {
		if newSetsValue < 30 {
			return nil, errCode(ErrInitialMeldLow)
		}
		s.InitialMeld[playerID] = true
	}

	s.Sets = canonicalSets
	s.Workspace = nil
	events := []module.Event{{Type: "turn_committed", Data: map[string]any{"playerId": playerID}}}

	if len(s.Hands[playerID]) == 0 {
		return append(events, endRoundOut(s, playerID)...), nil
	}
	s.Current = nextPlayer(s.Players, playerID)
	beginTurn(s)
	return events, nil
}

// Finished reports whether the match is over and who won.
func (m *Module) Finished(raw module.State) (bool, []string, error) {
	s, err := decode(raw)
	if err != nil {
		return false, nil, err
	}
	if s.Status != "completed" || s.WinnerID == "" {
		return s.Status == "completed", nil, nil
	}
	return true, []string{s.WinnerID}, nil
}
