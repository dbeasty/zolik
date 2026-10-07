package klondike

import (
	"zolik/server/internal/module"
)

// Refusal codes. Each is explained by a written rule (see offers.go
// ruleIDsFor), except the four that are not rules of play.
const (
	ErrWrongPlayerCount = "WRONG_PLAYER_COUNT"
	ErrGameNotActive    = "GAME_NOT_ACTIVE"
	ErrNotYourTurn      = "NOT_YOUR_TURN"
	ErrUnknownAction    = "UNKNOWN_ACTION"
	ErrNoSuchCard       = "NO_SUCH_CARD"
	ErrNotWholeRun      = "NOT_WHOLE_RUN"
	ErrBadTarget        = "BAD_TARGET"
	ErrDoesNotBuild     = "DOES_NOT_BUILD"
	ErrKingOnly         = "KING_ONLY"
	ErrFoundationOrder  = "FOUNDATION_ORDER"
	ErrTakeBackOff      = "TAKE_BACK_OFF"
	ErrStockEmpty       = "STOCK_EMPTY"
	ErrStockNotEmpty    = "STOCK_NOT_EMPTY"
	ErrWasteEmpty       = "WASTE_EMPTY"
	ErrNoRedeals        = "NO_REDEALS"
	ErrNotFinishable    = "NOT_FINISHABLE"
	ErrNothingToUndo    = "NOTHING_TO_UNDO"
)

// Module is the Solitaire (Klondike) game module.
type Module struct{}

// New returns the module. Stateless: every method takes the state it works on.
func New() *Module { return &Module{} }

var _ module.GameModule = (*Module)(nil)

// ServerDealt says the deal must never leave the server: the deck is the only
// opponent, and a player who knows or picks the deal has beaten it already.
func (m *Module) ServerDealt() bool { return true }

// NewMatch deals the tableau and leaves the other 24 cards as the stock.
func (m *Module) NewMatch(cfg module.MatchConfig, players []module.PlayerRef, seed int64) (module.State, error) {
	if len(players) != 1 {
		return nil, module.Error{Code: ErrWrongPlayerCount, Message: "solitaire seats one player"}
	}
	s := tableOf(cfg)
	s.Seed = seed
	s.Player = players[0].ID
	s.Status = statusActive
	if s.Scoring == scoreVegas {
		s.Score = -52
	}

	deck := newDeck(seed)
	s.Cols = make([]Column, numColumns)
	for c := 0; c < numColumns; c++ {
		s.Cols[c].Down = cloneList(deck[:c])
		s.Cols[c].Up = []string{deck[c]}
		deck = deck[c+1:]
	}
	s.Stock = cloneList(deck)
	s.Waste = []string{}
	s.Found = make([][]string, len(suits))
	for i := range s.Found {
		s.Found[i] = []string{}
	}
	return encode(s)
}

// Apply validates and applies one action. State is decoded fresh every call,
// so a refused action returns the caller's own bytes untouched.
func (m *Module) Apply(raw module.State, playerID string, a module.Action) (module.State, []module.Event, error) {
	s, err := decode(raw)
	if err != nil {
		return raw, nil, err
	}
	if s.Status != statusActive {
		return raw, nil, errCode(ErrGameNotActive)
	}
	if playerID != s.Player {
		return raw, nil, errCode(ErrNotYourTurn)
	}

	var events []module.Event
	switch a.Verb {
	case VerbDraw:
		events, err = applyDraw(s)
	case VerbRecycle:
		events, err = applyRecycle(s)
	case VerbMove:
		events, err = applyMove(s, a.Cards, a.Target)
	case VerbAutoFinish:
		events, err = applyAutoFinish(s)
	case VerbUndo:
		events, err = applyUndo(s)
	case VerbGiveUp:
		s.Status = statusLost
		events = []module.Event{{Type: "lost", Data: map[string]any{"reason": "gave_up"}}}
	default:
		err = module.Error{Code: ErrUnknownAction, Message: a.Verb}
	}
	if err != nil {
		return raw, nil, err
	}
	events = append(events, settle(s)...)
	out, err := encode(s)
	return out, events, err
}

// Finished is done once the game is won or lost. A win names the player; a
// loss names nobody — the "nobody won" answer blackjack already established.
func (m *Module) Finished(raw module.State) (bool, []string, error) {
	s, err := decode(raw)
	if err != nil {
		return false, nil, err
	}
	switch s.Status {
	case statusWon:
		return true, []string{s.Player}, nil
	case statusLost:
		return true, nil, nil
	}
	return false, nil, nil
}

// --- verbs -------------------------------------------------------------

// The refusal each verb would give right now, or "". Apply returns these and
// the offer list reads them, so the two cannot disagree.

func drawRefusal(s *GameState) string {
	if len(s.Stock) == 0 {
		return ErrStockEmpty
	}
	return ""
}

func recycleRefusal(s *GameState) string {
	switch {
	case len(s.Stock) > 0:
		return ErrStockNotEmpty
	case len(s.Waste) == 0:
		return ErrWasteEmpty
	case !s.unlimitedRedeals() && s.Recycles >= s.Redeals:
		return ErrNoRedeals
	}
	return ""
}

func finishRefusal(s *GameState) string {
	if !s.AutoFinish || !autoFinishable(s) {
		return ErrNotFinishable
	}
	return ""
}

func undoRefusal(s *GameState) string {
	if len(s.Undo) == 0 {
		return ErrNothingToUndo
	}
	return ""
}

func applyDraw(s *GameState) ([]module.Event, error) {
	if why := drawRefusal(s); why != "" {
		return nil, errCode(why)
	}
	n := s.Draw
	if n > len(s.Stock) {
		n = len(s.Stock)
	}
	var drawn []string
	for i := 0; i < n; i++ {
		c := s.Stock[len(s.Stock)-1]
		s.Stock = s.Stock[:len(s.Stock)-1]
		s.Waste = append(s.Waste, c)
		drawn = append(drawn, c)
	}
	s.Moves++
	s.Idle++
	// A draw shows cards the player had not seen; stepping back over it would
	// let them look and put it back.
	s.Undo = nil
	return []module.Event{{Type: "draw", Data: map[string]any{"cards": drawn}}}, nil
}

func applyRecycle(s *GameState) ([]module.Event, error) {
	if why := recycleRefusal(s); why != "" {
		return nil, errCode(why)
	}
	for i := len(s.Waste) - 1; i >= 0; i-- {
		s.Stock = append(s.Stock, s.Waste[i])
	}
	s.Waste = []string{}
	s.Recycles++
	s.Moves++
	s.Idle++
	s.Moved = false
	s.Undo = nil
	if s.Scoring == scoreStandard {
		penalty := 100
		if s.Draw >= 3 {
			penalty = 20
		}
		s.addScore(-penalty)
	}
	return []module.Event{{Type: "recycle"}}, nil
}

func applyUndo(s *GameState) ([]module.Event, error) {
	if why := undoRefusal(s); why != "" {
		return nil, errCode(why)
	}
	st := s.Undo[len(s.Undo)-1]
	s.Undo = s.Undo[:len(s.Undo)-1]
	s.takeOff(st.To, len(st.Cards))
	s.putOn(st.From, st.Cards)
	s.Score, s.Moves, s.Idle, s.Moved = st.Score, st.Moves, st.Idle, st.Moved
	return []module.Event{{Type: "undo", Data: map[string]any{"cards": st.Cards, "to": st.From}}}, nil
}

// addScore keeps standard scoring from going below zero, as the familiar
// game does; Vegas keeps a real running total.
func (s *GameState) addScore(n int) {
	s.Score += n
	if s.Scoring == scoreStandard && s.Score < 0 {
		s.Score = 0
	}
}

// --- moves -------------------------------------------------------------

type srcKind int

const (
	srcWaste srcKind = iota
	srcFound
	srcCol
)

type located struct {
	kind srcKind
	idx  int // foundation or column index
	from int // index into the column's Up
}

// locate finds where a submitted run is lifted from. The first card decides
// the source, and the cards must be exactly what that source can lift.
func locate(s *GameState, cards []string) (located, error) {
	if len(cards) == 0 {
		return located{}, errCode(ErrNoSuchCard)
	}
	head := cards[0]
	if n := len(s.Waste); n > 0 && s.Waste[n-1] == head {
		if len(cards) != 1 {
			return located{}, errCode(ErrNotWholeRun)
		}
		return located{kind: srcWaste}, nil
	}
	for i, f := range s.Found {
		if n := len(f); n > 0 && f[n-1] == head {
			if len(cards) != 1 {
				return located{}, errCode(ErrNotWholeRun)
			}
			return located{kind: srcFound, idx: i}, nil
		}
	}
	for c, col := range s.Cols {
		for j, card := range col.Up {
			if card != head {
				continue
			}
			run := col.Up[j:]
			if len(run) != len(cards) {
				return located{}, errCode(ErrNotWholeRun)
			}
			for k := range run {
				if run[k] != cards[k] {
					return located{}, errCode(ErrNotWholeRun)
				}
			}
			return located{kind: srcCol, idx: c, from: j}, nil
		}
	}
	return located{}, errCode(ErrNoSuchCard)
}

// checkMove is the whole legality test for a move, shared by Apply and by the
// offer list so the two cannot disagree.
func checkMove(s *GameState, cards []string, target string) (located, error) {
	src, err := locate(s, cards)
	if err != nil {
		return src, err
	}
	if fi := foundIndex(target); fi >= 0 {
		if len(cards) != 1 {
			return src, errCode(ErrFoundationOrder)
		}
		c := cards[0]
		if suitIndex(suitOf(c)) != fi || rankOf(c) != len(s.Found[fi])+1 {
			return src, errCode(ErrFoundationOrder)
		}
		return src, nil
	}
	ci := colIndex(target)
	if ci < 0 {
		return src, errCode(ErrBadTarget)
	}
	if src.kind == srcFound && !s.TakeBack {
		return src, errCode(ErrTakeBackOff)
	}
	col := s.Cols[ci]
	if len(col.Up) == 0 {
		if rankOf(cards[0]) != rankKing {
			return src, errCode(ErrKingOnly)
		}
		return src, nil
	}
	if !buildsOn(cards[0], col.Up[len(col.Up)-1]) {
		return src, errCode(ErrDoesNotBuild)
	}
	return src, nil
}

func applyMove(s *GameState, cards []string, target string) ([]module.Event, error) {
	src, err := checkMove(s, cards, target)
	if err != nil {
		return nil, err
	}
	before := step{Cards: cloneList(cards), Score: s.Score, Moves: s.Moves, Idle: s.Idle, Moved: s.Moved, To: target}
	cards = cloneList(cards)

	// Lift.
	switch src.kind {
	case srcWaste:
		before.From = zoneWaste
	case srcFound:
		before.From = foundID(src.idx)
	case srcCol:
		before.From = colID(src.idx)
	}
	s.takeOff(before.From, len(cards))

	// Lay.
	s.putOn(target, cards)
	toFoundation := foundIndex(target) >= 0

	progress := false
	switch {
	case toFoundation:
		if n := s.foundationCount(); n > s.BestFound {
			s.BestFound = n
			progress = true
		}
		if s.Scoring == scoreVegas {
			s.addScore(5)
		} else if s.Scoring == scoreStandard {
			s.addScore(10)
		}
	case src.kind == srcWaste:
		progress = true
		if s.Scoring == scoreStandard {
			s.addScore(5)
		}
	case src.kind == srcFound:
		switch s.Scoring {
		case scoreStandard:
			s.addScore(-15)
		case scoreVegas:
			s.addScore(-5)
		}
	}

	events := []module.Event{{Type: "moved", Data: map[string]any{"cards": cards, "to": target}}}

	// Turn up whatever the lift exposed.
	revealed := false
	if src.kind == srcCol {
		col := &s.Cols[src.idx]
		if len(col.Up) == 0 && len(col.Down) > 0 {
			c := col.Down[len(col.Down)-1]
			col.Down = col.Down[:len(col.Down)-1]
			col.Up = []string{c}
			revealed = true
			progress = true
			if s.Scoring == scoreStandard {
				s.addScore(5)
			}
			events = append(events, module.Event{Type: "revealed", Data: map[string]any{"card": c}})
		}
	}

	s.Moves++
	s.Moved = true
	if progress {
		s.Idle = 0
	} else {
		s.Idle++
	}

	if revealed {
		s.Undo = nil
	} else {
		prev := append(s.Undo, before)
		if len(prev) > undoDepth {
			prev = prev[len(prev)-undoDepth:]
		}
		s.Undo = prev
	}
	return events, nil
}

// autoFinishable is when every remaining card is face up in the tableau, which
// is when the rest of the game is only a matter of carrying it out.
func autoFinishable(s *GameState) bool {
	if len(s.Stock) > 0 || len(s.Waste) > 0 {
		return false
	}
	for _, c := range s.Cols {
		if len(c.Down) > 0 {
			return false
		}
	}
	return true
}

func applyAutoFinish(s *GameState) ([]module.Event, error) {
	if why := finishRefusal(s); why != "" {
		return nil, errCode(why)
	}
	var events []module.Event
	for s.foundationCount() < 52 {
		moved := false
		for c := range s.Cols {
			col := &s.Cols[c]
			if len(col.Up) == 0 {
				continue
			}
			top := col.Up[len(col.Up)-1]
			fi := suitIndex(suitOf(top))
			if rankOf(top) != len(s.Found[fi])+1 {
				continue
			}
			col.Up = col.Up[:len(col.Up)-1]
			s.Found[fi] = append(s.Found[fi], top)
			switch s.Scoring {
			case scoreVegas:
				s.addScore(5)
			case scoreStandard:
				s.addScore(10)
			}
			s.Moves++
			events = append(events, module.Event{Type: "moved", Data: map[string]any{"cards": []string{top}, "to": foundID(fi)}})
			moved = true
		}
		if !moved {
			break
		}
	}
	s.Idle = 0
	s.Undo = nil
	return events, nil
}

// --- the end of the game -----------------------------------------------

// settle ends the game when it has been won, or can no longer go anywhere.
func settle(s *GameState) []module.Event {
	if s.Status != statusActive {
		return nil
	}
	if s.foundationCount() == 52 {
		s.Status = statusWon
		return []module.Event{{Type: "won", Data: map[string]any{"score": s.Score, "moves": s.Moves}}}
	}
	if s.Idle >= idleLimit {
		s.Status = statusLost
		return []module.Event{{Type: "lost", Data: map[string]any{"reason": "stuck"}}}
	}
	if !hasProgressiveMove(s) {
		s.Status = statusLost
		return []module.Event{{Type: "lost", Data: map[string]any{"reason": "stuck"}}}
	}
	return nil
}

// hasProgressiveMove is whether anything is left to do that could change the
// position: a card to move, a card to turn, or a stock to go round again after
// something has moved.
func hasProgressiveMove(s *GameState) bool {
	if len(s.Stock) > 0 {
		return true
	}
	if s.canRecycle() && s.Moved {
		return true
	}
	return len(legalMoves(s)) > 0
}
