package marias

import (
	"zolik/server/internal/module"
	"zolik/server/internal/tricks"
)

// The ids this module draws its zones under, shared with the offers that
// point at them.
const (
	talonZoneID = "talon"
	trickZoneID = "trick"
)

func handZoneID(playerID string) string { return "hand:" + playerID }

var (
	_ module.Ranked     = (*Module)(nil)
	_ module.OpenViewer = (*Module)(nil)
	_ module.Rounded    = (*Module)(nil)
	_ module.Botted     = (*Module)(nil)
	_ module.GameModule = (*Module)(nil)
)

// View is the board as one seat sees it.
//
// What is secret in Mariáš, and so filtered here: every other hand, the
// chooser's unseen five, the talon (known to the declarer who laid it), and
// the trump suit until the game is agreed — the chooser's trump card lies
// face down until then, so the others bid against a game whose trumps they
// do not know.
func (m *Module) View(raw module.State, viewerID string) (module.ViewModel, error) {
	return m.view(raw, viewerID, false)
}

// OpenView is the board with everything face up, for replaying a finished
// match.
func (m *Module) OpenView(raw module.State) (module.ViewModel, error) {
	return m.view(raw, "", true)
}

func (m *Module) view(raw module.State, viewerID string, reveal bool) (module.ViewModel, error) {
	s, err := decode(raw)
	if err != nil {
		return module.ViewModel{}, err
	}
	vm := module.ViewModel{}
	playing := !s.Intermission.Open && s.Status == "active"
	trumpsPublic := reveal || s.Phase == phaseFlek || s.Phase == phasePlay

	// The viewer's own hand, and the trump card marked in it for the
	// chooser while it still lies face down.
	if viewerID != "" && s.seat(viewerID) >= 0 {
		z := module.Zone{
			ID: handZoneID(viewerID), Kind: module.ZoneHand, OwnerID: viewerID,
			LabelKey: "zone.yourHand", Count: len(s.Hands[viewerID]),
		}
		for _, c := range s.Hands[viewerID] {
			cv := module.CardView{Card: c}
			if playing && c == s.TrumpCard && viewerID == s.chooser() && s.Phase != phasePlay {
				cv.BadgeKeys = []string{"marias.badge.trumpCard"}
			}
			z.Cards = append(z.Cards, cv)
		}
		vm.Zones = append(vm.Zones, z)
	}
	for _, p := range s.Players {
		if p == viewerID {
			continue
		}
		z := module.Zone{
			ID: handZoneID(p), Kind: module.ZoneHand, OwnerID: p,
			LabelKey: "zone.opponentHand", Count: len(s.Hands[p]),
		}
		if reveal {
			for _, c := range s.Hands[p] {
				z.Cards = append(z.Cards, module.CardView{Card: c})
			}
		}
		vm.Zones = append(vm.Zones, z)
	}

	// The trick on the table, each card toward whoever played it. Between
	// tricks the one just taken stays there until the next lead, so the
	// third card is not snatched away the moment it lands.
	if s.Phase == phasePlay || len(s.LastTrick) > 0 {
		z := module.Zone{
			ID: trickZoneID, Kind: module.ZoneSpread, Shared: true, Arrange: module.ArrangeBySeat,
			LabelKey: "marias.zone.trick",
		}
		shown := s.Trick
		if len(shown) == 0 {
			shown = s.LastTrick
			z.LabelKey = "marias.zone.lastTrick"
		}
		z.Count = len(shown)
		for _, pl := range shown {
			z.Cards = append(z.Cards, module.CardView{Card: pl.Card, By: s.Players[pl.Seat]})
		}
		vm.Zones = append(vm.Zones, z)
	}

	// The talon: two cards face down, which the declarer who laid them may
	// look at.
	if len(s.Talon) > 0 || s.Phase == phaseTalon {
		z := module.Zone{ID: talonZoneID, Kind: module.ZoneStack, LabelKey: "marias.zone.talon", Count: len(s.Talon)}
		if reveal || viewerID == s.Declarer {
			for _, c := range s.Talon {
				z.Cards = append(z.Cards, module.CardView{Card: c})
			}
		}
		vm.Zones = append(vm.Zones, z)
	}

	// The seats.
	for _, p := range s.Players {
		seat := module.Seat{
			PlayerID: p,
			Active:   (s.Intermission.Open && !s.Intermission.Ready[p]) || (playing && s.Current == p),
		}
		if s.Intermission.Open && s.Intermission.Ready[p] {
			seat.LabelKeys = append(seat.LabelKeys, "seat.ready")
		}
		if playing {
			switch {
			case s.Phase == phaseTrump && p == s.Declarer:
				seat.LabelKeys = append(seat.LabelKeys, "marias.seat.chooser")
			case p == s.Declarer:
				seat.LabelKeys = append(seat.LabelKeys, "marias.seat.declarer")
			}
			if s.Phase == phasePlay {
				seat.Facts = append(seat.Facts, module.Fact{
					LabelKey: "marias.seat.tricks", Params: map[string]any{"n": s.TricksWon[p]},
				})
				if s.ShowCardPoints || reveal {
					seat.Facts = append(seat.Facts, module.Fact{
						LabelKey: "marias.seat.points", Params: map[string]any{"n": s.Points[p]},
					})
				}
			}
			for _, suit := range s.Marriages[p] {
				seat.Facts = append(seat.Facts, module.Fact{
					LabelKey: "marias.seat.marriage",
					Params:   map[string]any{"suit": suitKey(suit), "n": s.marriageValue(suit)},
				})
			}
			if p == s.ProtiSedma {
				seat.Facts = append(seat.Facts, module.Fact{LabelKey: "marias.seat.protiSedma"})
			}
			if p == s.ProtiSto {
				seat.Facts = append(seat.Facts, module.Fact{LabelKey: "marias.seat.protiSto"})
			}
		}
		vm.Seats = append(vm.Seats, seat)
	}

	// The header: which deal, and the contract as it stands.
	vm.Header = []module.Fact{{
		LabelKey: "marias.header.deal",
		Params:   map[string]any{"n": min(s.Deal+1, s.Deals), "of": s.Deals},
	}}
	if playing && s.Game != "" && (s.Phase != phaseAnnounce) {
		vm.Header = append(vm.Header, module.Fact{
			LabelKey: "marias.header.game",
			Params:   map[string]any{"game": gameKey(s.Game), "player": s.Declarer},
		})
		if s.Sedma {
			vm.Header = append(vm.Header, module.Fact{LabelKey: "marias.header.sedma"})
		}
	}
	if playing && s.TrumpCard != "" && (trumpsPublic || viewerID == s.chooser()) && (s.Game == "" || s.trumpGame()) {
		vm.Header = append(vm.Header, module.Fact{
			LabelKey: "marias.header.trumps",
			Params:   map[string]any{"suit": suitKey(string(tricks.Suit(s.TrumpCard)))},
		})
	}
	if playing {
		for _, part := range s.parts() {
			if n := s.Fleks[part]; n > 0 {
				vm.Header = append(vm.Header, module.Fact{
					LabelKey: "marias.header.doubled",
					Params:   map[string]any{"part": partKey(part), "n": 1 << n},
				})
			}
		}
	}

	// What the seat being waited on is being asked.
	if playing && viewerID == s.Current {
		switch s.Phase {
		case phaseTrump:
			vm.Prompts = append(vm.Prompts, module.Fact{LabelKey: "marias.prompt.trump"})
		case phaseAnnounce:
			vm.Prompts = append(vm.Prompts, module.Fact{LabelKey: "marias.prompt.announce"})
		case phaseTalon:
			vm.Prompts = append(vm.Prompts, module.Fact{LabelKey: "marias.prompt.talon"})
		case phaseAnswer:
			vm.Prompts = append(vm.Prompts, module.Fact{
				LabelKey: "marias.prompt.answer",
				Params:   map[string]any{"game": gameKey(s.Game), "player": s.Declarer},
			})
		case phaseFlek:
			vm.Prompts = append(vm.Prompts, module.Fact{LabelKey: "marias.prompt.flek"})
		case phasePlay:
			if viewerID == s.sevenHolder() && hasCard(s.Hands[viewerID], s.trumpSeven()) && len(s.Hands[viewerID]) > 1 {
				vm.Prompts = append(vm.Prompts, module.Fact{LabelKey: "marias.prompt.keepSeven"})
			}
		}
	}
	return vm, nil
}

// Keys for the words a fact names rather than says. Written out so each is
// a literal the key manifest can find.
func suitKey(suit string) string { return module.GermanSuitKey(suit) }

func gameKey(game string) string {
	switch game {
	case gameSto:
		return "marias.game.sto"
	case gameBetl:
		return "marias.game.betl"
	case gameDurch:
		return "marias.game.durch"
	}
	return "marias.game.hra"
}

func partKey(part string) string {
	switch part {
	case partSedma:
		return "marias.part.sedma"
	case partProtiSedma:
		return "marias.part.protiSedma"
	case partProtiSto:
		return "marias.part.protiSto"
	case partQuietSeven:
		return "marias.part.quietSeven"
	}
	return "marias.part.game"
}

// Standings ranks by units won, higher first.
func (m *Module) Standings(raw module.State) ([]module.Standing, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	return module.RankByScore(s.Players, func(id string) int { return s.Scores[id] }, "marias.unit.units"), nil
}

// Rounds is the deal-by-deal history: who declared what, and how each part
// went. It names games, suits and players, never a card.
func (m *Module) Rounds(raw module.State) (module.RoundLog, error) {
	s, err := decode(raw)
	if err != nil {
		return module.RoundLog{}, err
	}
	log := module.RoundLog{
		LabelKey:   "marias.round.deal",
		Rounds:     []module.RoundResult{},
		Paused:     s.Intermission.Open,
		WaitingFor: s.Intermission.Waiting(s.Players),
	}
	for _, d := range s.Rounds {
		r := module.RoundResult{Number: d.Number}
		r.Headline = &module.Fact{
			LabelKey: "marias.round.headline",
			Params:   map[string]any{"player": d.Declarer, "game": gameKey(d.Game)},
		}
		if d.Trump != "" {
			r.Facts = append(r.Facts, module.Fact{
				LabelKey: "marias.header.trumps", Params: map[string]any{"suit": suitKey(d.Trump)},
			})
		}
		for _, pr := range d.Parts {
			if pr.ForDeclarer {
				r.Facts = append(r.Facts, module.Fact{
					LabelKey: "marias.round.partWon",
					Params:   map[string]any{"part": partKey(pr.Part), "n": pr.Units},
				})
			} else {
				r.Facts = append(r.Facts, module.Fact{
					LabelKey: "marias.round.partLost",
					Params:   map[string]any{"part": partKey(pr.Part), "n": pr.Units},
				})
			}
		}
		for _, p := range s.Players {
			if d.Deltas[p] > 0 {
				r.Winners = append(r.Winners, p)
			}
			r.Scores = append(r.Scores, module.RoundScore{PlayerID: p, Delta: d.Deltas[p], Total: d.Totals[p]})
		}
		log.Rounds = append(log.Rounds, r)
	}
	return log, nil
}

// Bot is Mariáš's own bot — see bot.go.
func (m *Module) Bot() module.Bot { return bot{} }
