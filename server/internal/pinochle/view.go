package pinochle

import (
	"strconv"

	"zolik/server/internal/module"
)

// The ids this module draws its zones under, shared with the offers that
// point at them.
const (
	trickZoneID  = "trick"
	widowZoneID  = "widow"
	buriedZoneID = "buried"
	stockZoneID  = "stock"
	turnUpZoneID = "turnUp"
)

func handZoneID(playerID string) string { return "hand:" + playerID }
func meldZoneID(playerID string) string { return "meld:" + playerID }

// View is the board as one seat sees it. Secret: the other hands, the stock,
// the widow until it is turned, and what the bidder buried.
func (m *Module) View(raw module.State, viewerID string) (module.ViewModel, error) {
	return m.view(raw, viewerID, false)
}

// OpenView is the board with everything face up, for replaying a finished
// match.
func (m *Module) OpenView(raw module.State) (module.ViewModel, error) {
	return m.view(raw, "", true)
}

// meldShown is whether this hand's three- and four-handed meld is still on
// the table: from the end of the auction to the end of the first trick.
func (s *GameState) meldShown() bool {
	return s.Variation != VarTwoHanded && s.Phase == phasePlay && len(s.History) == 0
}

func (m *Module) view(raw module.State, viewerID string, reveal bool) (module.ViewModel, error) {
	s, err := decode(raw)
	if err != nil {
		return module.ViewModel{}, err
	}
	vm := module.ViewModel{}
	playing := !s.Intermission.Open && s.Status == "active"
	seated := viewerID != "" && s.seat(viewerID) >= 0
	partners := s.table().partners

	if seated {
		z := module.Zone{
			ID: handZoneID(viewerID), Kind: module.ZoneHand, OwnerID: viewerID,
			LabelKey: "zone.yourHand", Count: len(s.Hands[viewerID]), Arrange: module.ArrangeSorted,
		}
		shown := countOf(nil)
		if u := s.Used[viewerID]; u != nil {
			shown = countOf(u.shown())
		}
		for _, c := range s.Hands[viewerID] {
			cv := module.CardView{Card: c}
			if shown[c] > 0 {
				shown[c]--
				cv.BadgeKeys = []string{"pinochle.badge.melded"}
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
		if seated && partners && s.side(p) == s.side(viewerID) {
			z.LabelKey = "pinochle.zone.partnerHand"
		}
		if reveal {
			for _, c := range s.Hands[p] {
				z.Cards = append(z.Cards, module.CardView{Card: c})
			}
		}
		vm.Zones = append(vm.Zones, z)
	}

	// Melds on the table: everyone's at once in the bidding games, while the
	// first trick is played; in two-handed each player's laid melds, while
	// the stock lasts.
	if playing {
		for _, p := range s.Players {
			if z, ok := s.meldZone(p); ok {
				vm.Zones = append(vm.Zones, z)
			}
		}
	}

	// The trick, each card toward whoever played it; between tricks the one
	// just taken stays until the next lead.
	if s.Phase == phasePlay || s.Phase == phaseMeld || len(s.LastTrick) > 0 {
		z := module.Zone{
			ID: trickZoneID, Kind: module.ZoneSpread, Shared: true, Arrange: module.ArrangeBySeat,
			LabelKey: "pinochle.zone.trick",
		}
		shown := s.Trick
		if len(shown) == 0 {
			shown = s.LastTrick
			z.LabelKey = "pinochle.zone.lastTrick"
		}
		z.Count = len(shown)
		for _, pl := range shown {
			z.Cards = append(z.Cards, module.CardView{Card: pl.Card, By: s.Players[pl.Seat]})
		}
		vm.Zones = append(vm.Zones, z)
	}

	// Cutthroat's widow: face down through the auction, face up while the
	// bidder names trumps and buries, then gone into the bidder's hand.
	if playing && len(s.Widow) > 0 {
		switch s.Phase {
		case phaseAuction:
			vm.Zones = append(vm.Zones, module.Zone{ID: widowZoneID, Kind: module.ZoneStack, LabelKey: "pinochle.zone.widow", Count: len(s.Widow)})
		case phaseTrump, phaseBury:
			z := module.Zone{ID: widowZoneID, Kind: module.ZoneSpread, Shared: true, LabelKey: "pinochle.zone.widowTaken", Count: len(s.Widow)}
			for _, c := range s.Widow {
				z.Cards = append(z.Cards, module.CardView{Card: c})
			}
			vm.Zones = append(vm.Zones, z)
		}
	}
	if len(s.Buried) > 0 && playing {
		z := module.Zone{ID: buriedZoneID, Kind: module.ZoneStack, OwnerID: s.Bidder, LabelKey: "pinochle.zone.buried", Count: len(s.Buried)}
		if reveal || viewerID == s.Bidder {
			for _, c := range s.Buried {
				z.Cards = append(z.Cards, module.CardView{Card: c})
			}
		}
		vm.Zones = append(vm.Zones, z)
	}

	// Two-handed: the stock, with the trump card turned across its foot.
	if s.Variation == VarTwoHanded && playing && !s.Late {
		z := module.Zone{ID: stockZoneID, Kind: module.ZoneStack, LabelKey: "pinochle.zone.stock", Count: len(s.Stock)}
		if reveal {
			for _, c := range s.Stock {
				z.Cards = append(z.Cards, module.CardView{Card: c})
			}
		}
		vm.Zones = append(vm.Zones, z)
		if s.TurnUp != "" {
			vm.Zones = append(vm.Zones, module.Zone{
				ID: turnUpZoneID, Kind: module.ZonePile, Shared: true, LabelKey: "pinochle.zone.trumpCard",
				Count: 1, Cards: []module.CardView{{Card: s.TurnUp}},
			})
		}
	}

	winning := ""
	if len(s.Trick) > 0 {
		winning = s.Players[s.winner(s.Trick)]
	}
	for _, p := range s.Players {
		seat := module.Seat{
			PlayerID: p,
			Active:   (s.Intermission.Open && !s.Intermission.Ready[p]) || (playing && s.Current == p),
		}
		if partners {
			seat.Side = s.side(p)
		}
		if s.Intermission.Open && s.Intermission.Ready[p] {
			seat.LabelKeys = append(seat.LabelKeys, "seat.ready")
		}
		if playing {
			if p == s.dealer() {
				seat.LabelKeys = append(seat.LabelKeys, "pinochle.seat.dealer")
			}
			if p == s.Bidder && s.Phase != phaseAuction {
				seat.LabelKeys = append(seat.LabelKeys, "pinochle.seat.bidder")
			}
			if p == winning {
				seat.LabelKeys = append(seat.LabelKeys, "pinochle.seat.winning")
			}
			if s.Phase == phaseAuction {
				switch n, ok := s.LastBid[p]; {
				case ok && n < 0:
					seat.LabelKeys = append(seat.LabelKeys, "pinochle.seat.passed")
				case ok:
					seat.Facts = append(seat.Facts, module.Fact{LabelKey: "pinochle.seat.bid", Value: strconv.Itoa(n), Params: map[string]any{"n": n}})
				}
			}
			// The running score: each seat's meld and its side's points taken,
			// for the whole of the play rather than only the first trick. Both
			// are public — the meld was laid face up, the tricks were played
			// face up — so the option decides only whether the table shows
			// them or leaves them to be remembered.
			running := (!s.HideCardPoints || reveal) && (s.Phase == phasePlay || s.Phase == phaseMeld)
			if s.meldShown() || running && s.Variation != VarTwoHanded {
				n := s.Melds[p].Total
				seat.Facts = append(seat.Facts, module.Fact{LabelKey: "pinochle.seat.meld", Value: strconv.Itoa(n), Params: map[string]any{"n": n}})
			}
			if running {
				side := s.side(p)
				if s.Variation == VarTwoHanded {
					n := s.MeldScore[side]
					seat.Facts = append(seat.Facts, module.Fact{LabelKey: "pinochle.seat.meld", Value: strconv.Itoa(n), Params: map[string]any{"n": n}})
				}
				n := s.Taken[side]
				seat.Facts = append(seat.Facts, module.Fact{LabelKey: "pinochle.seat.taken", Value: strconv.Itoa(n), Params: map[string]any{"n": n}})
			}
		}
		vm.Seats = append(vm.Seats, seat)
	}

	vm.Header = []module.Fact{{LabelKey: "pinochle.header.hand", Params: map[string]any{"n": s.Deal + 1}}}
	if playing && s.Trump != "" {
		// The suit's key is the value too, so a client can draw trumps as the
		// suit itself rather than one more word in the header row.
		vm.Header = append(vm.Header, module.Fact{LabelKey: "pinochle.header.trump", Value: suitKey(s.Trump),
			Params: map[string]any{"suit": suitKey(s.Trump)}})
	}
	if playing && s.Bid > 0 {
		vm.Header = append(vm.Header, module.Fact{LabelKey: "pinochle.header.bid", Value: strconv.Itoa(s.Bid),
			Params: map[string]any{"n": s.Bid, "playerId": s.Bidder}})
	}
	// With the running score on the seats, the header's copy of the viewer's
	// own numbers would only say them twice.
	if playing && seated && s.HideCardPoints && (s.Phase == phasePlay || s.Phase == phaseMeld) {
		side := s.side(viewerID)
		n := s.Taken[side]
		if s.Variation == VarTwoHanded {
			vm.Header = append(vm.Header, module.Fact{LabelKey: "pinochle.header.yourMeld", Value: strconv.Itoa(s.MeldScore[side]),
				Params: map[string]any{"n": s.MeldScore[side]}})
		}
		if partners {
			vm.Header = append(vm.Header, module.Fact{LabelKey: "pinochle.header.sideTaken", Value: strconv.Itoa(n), Params: map[string]any{"n": n}})
		} else {
			vm.Header = append(vm.Header, module.Fact{LabelKey: "pinochle.header.yourTaken", Value: strconv.Itoa(n), Params: map[string]any{"n": n}})
		}
	}

	if playing && viewerID != "" && viewerID == s.Current {
		vm.Prompts = append(vm.Prompts, s.prompt(viewerID))
	}
	return vm, nil
}

// meldZone is one player's meld on the table, as groups, if it is showing.
func (s *GameState) meldZone(p string) (module.Zone, bool) {
	z := module.Zone{ID: meldZoneID(p), Kind: module.ZoneSpread, OwnerID: p, LabelKey: "pinochle.zone.meld"}
	if s.table().partners {
		z.Side = s.side(p)
	}
	var items []MeldItem
	switch {
	case s.meldShown():
		items = s.Melds[p].Items
	case s.Variation == VarTwoHanded && !s.Late:
		for _, lm := range s.MeldLog {
			if lm.Player == p {
				items = append(items, lm.Item)
			}
		}
	default:
		return z, false
	}
	if len(items) == 0 {
		return z, false
	}
	for i, it := range items {
		// The points ride with the name, so a seat's meld total can be read
		// off the table instead of added up from the rules.
		g := module.Group{ID: "m" + strconv.Itoa(i), Kind: it.Kind, Cards: it.Cards,
			BadgeKeys: []string{meldLabelKey(it.Kind)}, Value: strconv.Itoa(it.Points)}
		z.Groups = append(z.Groups, g)
		z.Count += len(it.Cards)
	}
	return z, true
}

func (s *GameState) prompt(p string) module.Fact {
	switch s.Phase {
	case phaseAuction:
		return module.Fact{LabelKey: "pinochle.prompt.bid", Params: map[string]any{"n": s.minNext()}}
	case phaseTrump:
		return module.Fact{LabelKey: "pinochle.prompt.trump"}
	case phaseBury:
		return module.Fact{LabelKey: "pinochle.prompt.bury", Params: map[string]any{"n": s.table().widow}}
	case phasePassIn, phasePassBack:
		return module.Fact{LabelKey: "pinochle.prompt.give", Params: map[string]any{"n": s.PassCount, "playerId": s.partner(p)}}
	case phaseMeld:
		return module.Fact{LabelKey: "pinochle.prompt.meld"}
	}
	switch {
	case len(s.Trick) > 0:
		return module.Fact{LabelKey: "pinochle.prompt.play"}
	case s.table().auction && p == s.Bidder && len(s.History) == 0:
		return module.Fact{LabelKey: "pinochle.prompt.leadOrThrowIn"}
	}
	return module.Fact{LabelKey: "pinochle.prompt.lead"}
}

// suitKey names a suit in a sentence, each key written out so the manifest
// finds it.
func suitKey(s string) string {
	switch s {
	case "H":
		return "suit.H"
	case "C":
		return "suit.C"
	case "D":
		return "suit.D"
	}
	return "suit.S"
}

// meldLabelKey names a meld kind.
func meldLabelKey(kind string) string {
	switch kind {
	case MeldRun:
		return "pinochle.meld.run"
	case MeldDoubleRun:
		return "pinochle.meld.doubleRun"
	case MeldRoyalMarriage:
		return "pinochle.meld.royalMarriage"
	case MeldMarriage:
		return "pinochle.meld.marriage"
	case MeldDix:
		return "pinochle.meld.dix"
	case MeldAces:
		return "pinochle.meld.aces"
	case MeldKings:
		return "pinochle.meld.kings"
	case MeldQueens:
		return "pinochle.meld.queens"
	case MeldJacks:
		return "pinochle.meld.jacks"
	}
	return "pinochle.meld.pinochle"
}

// Standings ranks by score, higher first; partners share theirs.
func (m *Module) Standings(raw module.State) ([]module.Standing, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	score := func(id string) int { return s.Scores[s.side(id)] }
	// The bidding side goes out first: a side over the target can win the
	// match with fewer points than another side also over it. So the first
	// measure is the engine's own verdict, and the points only order the
	// rest — or the scoreboard would crown a side the rules did not.
	won := map[string]bool{}
	for _, w := range s.Winners {
		won[w] = true
	}
	out := module.RankByScores(s.Players, "pinochle.unit.points",
		func(id string) int { return module.BoolOpt(won[id]) }, score)
	for i := range out {
		out[i].Score = score(out[i].PlayerID)
	}
	return out, nil
}

// Rounds is the hand-by-hand history.
func (m *Module) Rounds(raw module.State) (module.RoundLog, error) {
	s, err := decode(raw)
	if err != nil {
		return module.RoundLog{}, err
	}
	log := module.RoundLog{
		LabelKey:   "pinochle.round.hand",
		Rounds:     []module.RoundResult{},
		Paused:     s.Intermission.Open,
		WaitingFor: s.Intermission.Waiting(s.Players),
	}
	partners := s.table().partners
	for _, h := range s.Rounds {
		r := module.RoundResult{Number: h.Number}
		switch {
		case h.Redealt:
			r.Facts = append(r.Facts, module.Fact{LabelKey: "pinochle.round.redealt"})
		case h.Bidder != "":
			r.Facts = append(r.Facts, module.Fact{LabelKey: "pinochle.round.bid", Params: map[string]any{"playerId": h.Bidder, "n": h.Bid}})
			if h.Trump != "" {
				r.Facts = append(r.Facts, module.Fact{LabelKey: "pinochle.round.trump", Params: map[string]any{"suit": suitKey(h.Trump)}})
			}
			switch {
			case h.ThrownIn:
				r.Facts = append(r.Facts, module.Fact{LabelKey: "pinochle.round.thrownIn"})
			case h.Made:
				r.Facts = append(r.Facts, module.Fact{LabelKey: "pinochle.round.made"})
			default:
				r.Facts = append(r.Facts, module.Fact{LabelKey: "pinochle.round.set"})
			}
			if h.Stuck {
				r.Facts = append(r.Facts, module.Fact{LabelKey: "pinochle.round.stuck"})
			}
		}
		best := -1 << 31
		for _, d := range h.Delta {
			best = max(best, d)
		}
		for i, p := range s.Players {
			side := sideOf(i, partners)
			d := h.Delta[side]
			if !h.Redealt && d == best {
				r.Winners = append(r.Winners, p)
			}
			rs := module.RoundScore{PlayerID: p, Delta: d, Total: h.Totals[p]}
			if !h.Redealt {
				rs.Facts = []module.Fact{
					{LabelKey: "pinochle.round.meld", Value: strconv.Itoa(h.Meld[side]), Params: map[string]any{"n": h.Meld[side]}},
					{LabelKey: "pinochle.round.taken", Value: strconv.Itoa(h.Taken[side]), Params: map[string]any{"n": h.Taken[side]}},
				}
			}
			r.Scores = append(r.Scores, rs)
		}
		log.Rounds = append(log.Rounds, r)
	}
	return log, nil
}
