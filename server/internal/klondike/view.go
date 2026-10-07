package klondike

import (
	"strconv"

	"zolik/server/internal/module"
)

var (
	_ module.Ranked     = (*Module)(nil)
	_ module.OpenViewer = (*Module)(nil)
)

// Descriptor is Solitaire's self-description.
func (m *Module) Descriptor() module.ModuleDescriptor {
	return module.ModuleDescriptor{
		ID: "klondike", Label: "Solitaire", MinPlayers: 1, MaxPlayers: 1,
		Variations: []module.VariationSpec{
			{
				ID: "classic", Label: "Classic",
				Summary: []module.Fact{
					{LabelKey: "klondike.rules.draw1"},
					{LabelKey: "klondike.rules.redealsUnlimited"},
					{LabelKey: "klondike.rules.scoreStandard"},
				},
				Defaults: defaultsFor("classic"),
			},
			{
				ID: "draw3", Label: "Draw Three",
				Summary: []module.Fact{
					{LabelKey: "klondike.rules.draw3"},
					{LabelKey: "klondike.rules.redealsUnlimited"},
					{LabelKey: "klondike.rules.scoreStandard"},
				},
				Defaults: defaultsFor("draw3"),
			},
			{
				ID: "vegas", Label: "Vegas",
				Summary: []module.Fact{
					{LabelKey: "klondike.rules.draw3"},
					{LabelKey: "klondike.rules.redealsNone"},
					{LabelKey: "klondike.rules.scoreVegas"},
				},
				Defaults: defaultsFor("vegas"),
			},
		},
		Options: []module.OptionSpec{
			{
				Name: OptDraw, Type: module.OptionEnumInt,
				Label: "Cards turned", Help: "How many cards the stock turns over at a time.",
				Choices: []module.OptionChoice{{Value: 1, Label: "One"}, {Value: 3, Label: "Three"}},
			},
			{
				Name: OptRedeals, Type: module.OptionEnumInt,
				Label: "Turning the waste over", Help: "How many times the waste may be turned back into a stock.",
				Choices: []module.OptionChoice{
					{Value: redealsUnlimited, Label: "Unlimited"},
					{Value: 2, Label: "Twice"},
					{Value: 0, Label: "Never"},
				},
			},
			{
				Name: OptScoring, Type: module.OptionEnumInt,
				Label: "Scoring", Help: "How the game is scored.",
				Choices: []module.OptionChoice{
					{Value: scoreStandard, Label: "Standard"},
					{Value: scoreVegas, Label: "Vegas"},
					{Value: scoreNone, Label: "None"},
				},
			},
			{
				Name: OptTakeBack, Type: module.OptionEnumInt,
				Label: "Cards back from the foundations", Help: "Whether a card may be moved down from a foundation to the tableau.",
				Choices: []module.OptionChoice{
					{Value: module.OptOn, Label: "Allowed"}, {Value: module.OptOff, Label: "Not allowed"},
				},
			},
			{
				Name: OptAutoFinish, Type: module.OptionEnumInt,
				Label: "Finish for me", Help: "Offer to play out the last cards once every card is face up.",
				Choices: []module.OptionChoice{
					{Value: module.OptOn, Label: "Offered"}, {Value: module.OptOff, Label: "Not offered"},
				},
			},
		},
	}
}

// View renders the table. The viewer makes no difference: there is one seat,
// and what it may not see — the face-down cards — is hidden from everyone.
// The only place a face-down card is turned into a number is here.
func (m *Module) View(raw module.State, viewerID string) (module.ViewModel, error) {
	s, err := decode(raw)
	if err != nil {
		return module.ViewModel{}, err
	}
	return viewOf(s, false), nil
}

// OpenView shows every card, for a finished game being watched back.
func (m *Module) OpenView(raw module.State) (module.ViewModel, error) {
	s, err := decode(raw)
	if err != nil {
		return module.ViewModel{}, err
	}
	return viewOf(s, true), nil
}

func viewOf(s *GameState, reveal bool) module.ViewModel {
	stock := module.Zone{ID: zoneStock, Kind: module.ZoneStack, LabelKey: "klondike.zone.stock", Shared: true, Count: len(s.Stock)}
	if reveal {
		for i := len(s.Stock) - 1; i >= 0; i-- {
			stock.Cards = append(stock.Cards, module.CardView{Card: s.Stock[i]})
		}
	}

	shown := s.Draw
	if shown < 1 {
		shown = 1
	}
	if shown > len(s.Waste) {
		shown = len(s.Waste)
	}
	waste := module.Zone{ID: zoneWaste, Kind: module.ZonePile, LabelKey: "klondike.zone.waste", Shared: true, Count: len(s.Waste)}
	if s.Draw > 1 {
		waste.Fan = s.Draw
	}
	for _, c := range s.Waste[len(s.Waste)-shown:] {
		waste.Cards = append(waste.Cards, module.CardView{Card: c})
	}

	found := module.Zone{ID: zoneFoundations, Kind: module.ZoneSpread, LabelKey: "klondike.zone.foundations", Shared: true}
	for i, cards := range s.Found {
		found.Groups = append(found.Groups, module.Group{
			ID: foundID(i), Kind: "foundation", Cards: cloneList(cards), Complete: len(cards) == rankKing,
		})
		found.Count += len(cards)
	}

	tableau := module.Zone{ID: zoneTableau, Kind: module.ZoneSpread, LabelKey: "klondike.zone.tableau", Shared: true}
	for i, col := range s.Cols {
		g := module.Group{ID: colID(i), Kind: "column"}
		if reveal {
			g.Cards = append(cloneList(col.Down), col.Up...)
		} else {
			g.Cards = cloneList(col.Up)
			g.Hidden = len(col.Down)
		}
		tableau.Groups = append(tableau.Groups, g)
		tableau.Count += len(col.Down) + len(col.Up)
	}

	seat := module.Seat{
		PlayerID: s.Player, Active: s.Status == statusActive,
		Facts: []module.Fact{{LabelKey: "klondike.fact.moves", Value: strconv.Itoa(s.Moves)}},
	}
	if s.Scoring != scoreNone {
		seat.Facts = append([]module.Fact{{LabelKey: "klondike.fact.score", Value: strconv.Itoa(s.Score)}}, seat.Facts...)
	}
	if left := s.redealsLeft(); left >= 0 {
		seat.Facts = append(seat.Facts, module.Fact{LabelKey: "klondike.fact.redealsLeft", Value: strconv.Itoa(left)})
	}

	vm := module.ViewModel{
		Zones: []module.Zone{stock, waste, found, tableau},
		Seats: []module.Seat{seat},
	}
	switch s.Status {
	case statusWon:
		vm.Status = []module.Fact{{LabelKey: "klondike.status.won"}}
	case statusLost:
		vm.Status = []module.Fact{{LabelKey: "klondike.status.lost"}}
	}
	return vm
}

// Standings is the single-line scoreboard. Won is set here and not by
// RankByScores, which would hand a lone player rank 1 and a win on a game
// they lost.
func (m *Module) Standings(raw module.State) ([]module.Standing, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	return []module.Standing{{
		PlayerID: s.Player, Rank: 1, Score: s.foundationCount(), Won: s.Status == statusWon,
		LabelKey: "klondike.unit.cards",
	}}, nil
}
