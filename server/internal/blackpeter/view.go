package blackpeter

import (
	"strconv"

	"zolik/server/internal/module"
)

// The ids this module draws its zones under.
const pairsZoneID = "pairs"

func handZoneID(playerID string) string { return "hand:" + playerID }

var (
	_ module.Ranked     = (*Module)(nil)
	_ module.OpenViewer = (*Module)(nil)
	_ module.Rounded    = (*Module)(nil)
	_ module.Botted     = (*Module)(nil)
)

// View is the board as one seat sees it. Secret: every hand but the viewer's
// own. Every hand is a hand zone, so the one the viewer takes from can be
// drawn as a fan of backs to point at.
func (m *Module) View(raw module.State, viewerID string) (module.ViewModel, error) {
	return m.view(raw, viewerID, false)
}

// OpenView is the board with everything face up, for replaying a match.
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

	for _, st := range s.Seats {
		mine := st.PlayerID == viewerID
		z := module.Zone{
			ID: handZoneID(st.PlayerID), Kind: module.ZoneHand, OwnerID: st.PlayerID,
			LabelKey: "blackpeter.zone.hand", Count: len(st.Hand),
		}
		if mine {
			z.LabelKey = "zone.yourHand"
		}
		if mine || reveal {
			for _, c := range st.Hand {
				cv := module.CardView{Card: c}
				if c == peter {
					cv.BadgeKeys = []string{"blackpeter.badge.peter"}
				}
				z.Cards = append(z.Cards, cv)
			}
		}
		vm.Zones = append(vm.Zones, z)
	}
	// Every pair put down is public, so the whole pile is sent; the board
	// shows the top and lets a player look under it.
	pile := module.Zone{ID: pairsZoneID, Kind: module.ZonePile, LabelKey: "blackpeter.zone.pairs"}
	for _, p := range s.Pairs {
		for _, c := range p {
			pile.Cards = append(pile.Cards, module.CardView{Card: c})
		}
	}
	pile.Count = len(pile.Cards)
	vm.Zones = append(vm.Zones, pile)

	for _, st := range s.Seats {
		seat := module.Seat{
			PlayerID: st.PlayerID,
			Active:   (s.Intermission.Open && !s.Intermission.Ready[st.PlayerID]) || (playing && s.Current == st.PlayerID),
		}
		if s.Intermission.Open && s.Intermission.Ready[st.PlayerID] {
			seat.LabelKeys = append(seat.LabelKeys, "seat.ready")
		}
		if playing && st.Out {
			seat.LabelKeys = append(seat.LabelKeys, "blackpeter.seat.out")
		}
		seat.Facts = append(seat.Facts,
			module.Fact{LabelKey: "blackpeter.seat.cards", Value: strconv.Itoa(len(st.Hand)), Params: map[string]any{"n": len(st.Hand)}},
		)
		vm.Seats = append(vm.Seats, seat)
	}

	vm.Header = []module.Fact{
		{LabelKey: "blackpeter.header.round", Params: map[string]any{"n": min(s.Round+1, s.RoundLimit), "of": s.RoundLimit}},
	}

	if playing && viewerID == s.Current {
		if from := s.source(viewerID); from >= 0 {
			vm.Prompts = append(vm.Prompts, module.Fact{LabelKey: "blackpeter.prompt.draw", Params: map[string]any{"player": s.Seats[from].PlayerID}})
		}
	}
	if playing {
		if st := s.seat(viewerID); st != nil && st.Out {
			vm.Status = append(vm.Status, module.Fact{LabelKey: "blackpeter.status.out"})
		}
	}
	return vm, nil
}

// Standings ranks by fewest marks. Score is negated so the runtime ranks
// higher-is-better; Shown is the figure a player reads.
func (m *Module) Standings(raw module.State) ([]module.Standing, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	marks := func(id string) int { return -s.seat(id).Marks }
	out := module.RankByScore(s.players(), marks, "blackpeter.unit.marks")
	for i := range out {
		shown := -out[i].Score
		out[i].Shown = &shown
	}
	return out, nil
}

// Rounds is the round-by-round history: who was left holding Black Peter,
// and in what order everyone else went out.
func (m *Module) Rounds(raw module.State) (module.RoundLog, error) {
	s, err := decode(raw)
	if err != nil {
		return module.RoundLog{}, err
	}
	log := module.RoundLog{
		LabelKey:   "blackpeter.round.round",
		Rounds:     []module.RoundResult{},
		Paused:     s.Intermission.Open,
		WaitingFor: s.Intermission.Waiting(s.players()),
	}
	for _, r := range s.Rounds {
		rr := module.RoundResult{Number: r.Number}
		if r.Loser != "" {
			rr.Headline = &module.Fact{LabelKey: "blackpeter.round.loser", Params: map[string]any{"player": r.Loser}}
		}
		if len(r.OutOrder) > 0 {
			rr.Facts = append(rr.Facts, module.Fact{LabelKey: "blackpeter.round.first", Params: map[string]any{"player": r.OutOrder[0]}})
		}
		for _, id := range s.players() {
			if id != r.Loser {
				rr.Winners = append(rr.Winners, id)
			}
			delta, total := 0, r.Marks[id]
			if id == r.Loser {
				delta = 1
			}
			rr.Scores = append(rr.Scores, module.RoundScore{
				PlayerID: id, Delta: -delta, Total: -total, Shown: &delta, ShownTotal: &total,
			})
		}
		log.Rounds = append(log.Rounds, rr)
	}
	return log, nil
}

// Bot is Black Peter's own bot — see bot.go.
func (m *Module) Bot() module.Bot { return bot{} }
