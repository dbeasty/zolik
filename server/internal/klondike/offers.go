package klondike

import (
	"strconv"

	"zolik/server/internal/module"
)

// move is one legal lift-and-lay: a run, where it comes from, where it goes.
type move struct {
	cards []string
	from  string // the group or zone id it is lifted from
	to    string // a column or foundation group id
}

// legalMoves is every move the rules allow, best first.
//
// The order is advice, not rule: foundations, then moves that turn a card up,
// then the waste, then rearranging, then taking a card back down. A driver
// that takes the first offer it is given therefore always prefers progress,
// and a player scanning the list finds the useful moves at the top.
//
// One legal move is left out: a king that already stands at the base of its
// column, sent to an empty column. It is legal and changes nothing but which
// column the king is in, and listing it only adds noise.
func legalMoves(s *GameState) []move {
	var toFound, reveals, fromWaste, shuffles, takeBacks []move

	try := func(bucket *[]move, cards []string, from, to string) {
		if _, err := checkMove(s, cards, to); err == nil {
			*bucket = append(*bucket, move{cards: cloneList(cards), from: from, to: to})
		}
	}

	if n := len(s.Waste); n > 0 {
		top := []string{s.Waste[n-1]}
		for f := range suits {
			try(&toFound, top, zoneWaste, foundID(f))
		}
		for c := range s.Cols {
			try(&fromWaste, top, zoneWaste, colID(c))
		}
	}

	for c, col := range s.Cols {
		for j := range col.Up {
			run := col.Up[j:]
			if j == len(col.Up)-1 {
				for f := range suits {
					try(&toFound, run, colID(c), foundID(f))
				}
			}
			for t := range s.Cols {
				if t == c {
					continue
				}
				if len(s.Cols[t].Up) == 0 && j == 0 && len(col.Down) == 0 {
					continue // a king shuffled along the bottom of the table
				}
				bucket := &shuffles
				if j == 0 && len(col.Down) > 0 {
					bucket = &reveals
				}
				try(bucket, run, colID(c), colID(t))
			}
		}
	}

	if s.TakeBack {
		for f, cards := range s.Found {
			if n := len(cards); n > 0 {
				top := []string{cards[n-1]}
				for c := range s.Cols {
					try(&takeBacks, top, foundID(f), colID(c))
				}
			}
		}
	}

	var out []move
	for _, b := range [][]move{toFound, reveals, fromWaste, shuffles, takeBacks} {
		out = append(out, b...)
	}
	return out
}

// LegalActions answers "what may this player do right now?". The moves are
// listed only when they exist; the controls that are always on screen — draw,
// recycle, undo, give up — are listed always, with the reason when they are off.
func (m *Module) LegalActions(raw module.State, playerID string) ([]module.ActionOffer, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	why := ""
	switch {
	case s.Status != statusActive:
		why = ErrGameNotActive
	case playerID != s.Player:
		why = ErrNotYourTurn
	}

	var offers []module.ActionOffer
	if why == "" {
		for _, mv := range legalMoves(s) {
			offers = append(offers, moveOffer(s, mv))
		}
	}

	draw := module.ActionOffer{
		ID: OfferDraw, Verb: VerbDraw, LabelKey: "klondike.offer.draw",
		Source: &module.Selector{Zone: module.FromDeck, ZoneID: zoneStock},
		Target: &module.Selector{Zone: module.ToTable, ZoneID: zoneWaste},
	}
	recycle := module.ActionOffer{
		ID: OfferRecycle, Verb: VerbRecycle, LabelKey: "klondike.offer.recycle",
		Source: &module.Selector{Zone: module.FromDiscardPile, ZoneID: zoneWaste},
		Target: &module.Selector{Zone: module.ToTable, ZoneID: zoneStock},
	}
	undo := module.ActionOffer{ID: OfferUndo, Verb: VerbUndo, LabelKey: "klondike.offer.undo", Undo: true}
	giveUp := module.ActionOffer{ID: OfferGiveUp, Verb: VerbGiveUp, LabelKey: "klondike.offer.giveUp"}
	finish := module.ActionOffer{ID: OfferAutoFinish, Verb: VerbAutoFinish, LabelKey: "klondike.offer.autoFinish"}

	gate := func(o *module.ActionOffer, refusal string) {
		switch {
		case why != "":
			o.WhyNot = why
		case refusal != "":
			o.WhyNot = refusal
		default:
			o.Enabled = true
		}
	}
	gate(&draw, drawRefusal(s))
	gate(&recycle, recycleRefusal(s))
	gate(&undo, undoRefusal(s))
	gate(&giveUp, "")
	if s.AutoFinish {
		gate(&finish, finishRefusal(s))
	}

	if n := len(s.Stock); n > 0 {
		draw.Facts = []module.Fact{{LabelKey: "klondike.fact.count", Value: strconv.Itoa(n)}}
	}
	if left := s.redealsLeft(); left >= 0 {
		recycle.Facts = []module.Fact{{LabelKey: "klondike.fact.redealsLeft", Value: strconv.Itoa(left)}}
	}

	offers = append(offers, draw, recycle)
	if s.AutoFinish {
		offers = append(offers, finish)
	}
	offers = append(offers, undo, giveUp)
	m.annotate(s, offers)
	return offers, nil
}

func moveOffer(s *GameState, mv move) module.ActionOffer {
	src := &module.Selector{
		Cards: mv.cards, Submit: mv.cards, MinCards: len(mv.cards), MaxCards: len(mv.cards),
	}
	switch {
	case mv.from == zoneWaste:
		src.Zone, src.ZoneID = module.FromDiscardPile, zoneWaste
	default:
		src.Zone, src.MeldID = module.FromMeld, mv.from
	}
	zone := zoneTableau
	if foundIndex(mv.to) >= 0 {
		zone = zoneFoundations
	}
	return module.ActionOffer{
		ID: "move:" + mv.cards[0] + ":" + mv.to, Verb: VerbMove, Enabled: true,
		LabelKey: "klondike.offer.move",
		Source:   src,
		Target:   &module.Selector{Zone: module.ToMeld, MeldID: mv.to, ZoneID: zone},
		Facts:    []module.Fact{{LabelKey: "klondike.fact.card", Value: mv.cards[0]}, destinationFact(mv.to)},
	}
}

// destinationFact names where a move lands in words a player reads: "to the
// foundations", "to column 3" — never the group's id.
func destinationFact(to string) module.Fact {
	if c := colIndex(to); c >= 0 {
		return module.Fact{LabelKey: "klondike.fact.toColumn", Value: strconv.Itoa(c + 1)}
	}
	return module.Fact{LabelKey: "klondike.fact.toFoundation"}
}

// ruleIDsFor names the written rules behind a refusal at this table.
func ruleIDsFor(s *GameState, code string) []string {
	switch code {
	case ErrNotYourTurn:
		return []string{"klondike.rules.solo"}
	case ErrNotWholeRun, ErrBadTarget:
		return []string{"klondike.rules.runs"}
	case ErrDoesNotBuild:
		return []string{"klondike.rules.build"}
	case ErrKingOnly:
		return []string{"klondike.rules.kingOnly"}
	case ErrFoundationOrder:
		return []string{"klondike.rules.foundation"}
	case ErrTakeBackOff:
		return []string{"klondike.rules.noTakeBack"}
	case ErrStockEmpty:
		return []string{"klondike.rules.stock", redealRule(s)}
	case ErrStockNotEmpty, ErrWasteEmpty:
		return []string{"klondike.rules.stock"}
	case ErrNoRedeals:
		return []string{redealRule(s)}
	case ErrNotFinishable:
		if s.AutoFinish {
			return []string{"klondike.rules.autoFinish"}
		}
		return []string{"klondike.rules.noAutoFinish"}
	case ErrNothingToUndo:
		return []string{"klondike.rules.undo"}
	}
	return nil
}

func redealRule(s *GameState) string {
	switch {
	case s.unlimitedRedeals():
		return "klondike.rules.redealsUnlimited"
	case s.Redeals == 0:
		return "klondike.rules.redealsNone"
	}
	return "klondike.rules.redealsLimited"
}
