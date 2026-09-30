package gamemcp

import (
	"fmt"
	"strings"

	"zolik/server/internal/module"
)

// Observation is what one seat may know, and what it may do.
type Observation struct {
	Table     string `json:"table"`
	Seat      int    `json:"seat"`
	Name      string `json:"name"`
	Game      string `json:"game"`
	Variation string `json:"variation,omitempty"`
	// Text is the board rendered for a model to read: own hand, the table,
	// the pile, what every other seat holds (a count), the scores, whose turn
	// it is, and the numbered legal moves.
	Text string `json:"text"`
	// ToAct is every seat the table is waiting on.
	ToAct    []int  `json:"toAct"`
	YourTurn bool   `json:"yourTurn"`
	Finished bool   `json:"finished"`
	Winners  []int  `json:"winners,omitempty"`
	Stalled  string `json:"stalled,omitempty"`
	// Moves is the numbered list play takes an index into. Empty when the
	// seat is not on turn.
	Moves []Move `json:"moves"`
	// View is the module's own board for this seat (module.View) — the only
	// place hidden information is filtered, so the only source of this.
	View module.ViewModel `json:"view"`
}

func (t *Table) observe(seatIdx int) (Observation, error) {
	s, err := t.seat(seatIdx)
	if err != nil {
		return Observation{}, err
	}
	vm, err := t.g.m.View(t.State, s.Player)
	if err != nil {
		return Observation{}, err
	}
	moves, err := t.moves(s.Player)
	if err != nil {
		return Observation{}, err
	}
	if moves == nil {
		moves = []Move{}
	}
	o := Observation{Table: t.ID, Seat: s.Index, Name: s.Name, Game: t.Game, Variation: t.Config.Variation,
		Moves: moves, View: vm, Stalled: t.Stalled, ToAct: []int{}}
	for _, p := range t.awaited() {
		i := t.seatOf(p).Index
		o.ToAct = append(o.ToAct, i)
		if i == s.Index {
			o.YourTurn = true
		}
	}
	if done, winners := t.finished(); done {
		o.Finished = true
		for _, w := range winners {
			if ws := t.seatOf(w); ws != nil {
				o.Winners = append(o.Winners, ws.Index)
			}
		}
	}
	o.Text = t.render(s, vm, o)
	return o, nil
}

// render is the board as text. It is built from the view alone, so it can
// say no more than the view does.
func (t *Table) render(me *SeatInfo, vm module.ViewModel, o Observation) string {
	n := t.names()
	var b strings.Builder
	d := t.g.m.Descriptor()
	title := d.Label
	if v := d.Variation(t.Config.Variation); v != nil {
		title += " (" + v.Label + ")"
	}
	fmt.Fprintf(&b, "%s · table %s · you are seat %d (%s)\n", title, t.ID, me.Index, me.Name)
	if h := n.facts(vm.Header); len(h) > 0 {
		b.WriteString(strings.Join(h, " · ") + "\n")
	}

	b.WriteString("\nSeats:\n")
	for _, s := range t.Seats {
		who := s.Name
		switch {
		case s.Index == me.Index:
			who += " (you)"
		case s.Claude:
			who += " (claude)"
		default:
			who += " (" + s.Spec + ")"
		}
		line := fmt.Sprintf("  seat %d %s", s.Index, who)
		if vs := vm.SeatOf(s.Player); vs != nil {
			var bits []string
			if vs.Active {
				bits = append(bits, "TO ACT")
			}
			for _, k := range vs.LabelKeys {
				bits = append(bits, label(k, nil))
			}
			bits = append(bits, n.facts(vs.Facts)...)
			if vs.Side != "" {
				bits = append(bits, "side "+vs.Side)
			}
			if len(bits) > 0 {
				line += ": " + strings.Join(bits, " · ")
			}
		}
		b.WriteString(line + "\n")
	}

	b.WriteString("\nTable:\n")
	for _, z := range vm.Zones {
		b.WriteString(t.zoneText(z, me, n))
	}
	if st := n.facts(vm.Status); len(st) > 0 {
		b.WriteString("\nStatus: " + strings.Join(st, " · ") + "\n")
	}
	if rl := module.RoundsFor(t.g.m, t.State); rl != nil && len(rl.Rounds) > 0 {
		b.WriteString("\nLast round: " + t.roundLine(rl.Rounds[len(rl.Rounds)-1]) + "\n")
	}
	if pr := n.facts(vm.Prompts); len(pr) > 0 {
		b.WriteString("\nPrompts: " + strings.Join(pr, " · ") + "\n")
	}

	b.WriteString("\n")
	switch {
	case o.Finished:
		var ws []string
		for _, w := range o.Winners {
			ws = append(ws, t.Seats[w].Player)
		}
		b.WriteString("The match is over. " + t.winnersText(ws) + "\n")
		for _, st := range module.StandingsFor(t.g.m, t.State) {
			score := st.Score
			if st.Shown != nil {
				score = *st.Shown
			}
			fmt.Fprintf(&b, "  %d. %s %d\n", st.Rank, n.token(st.PlayerID), score)
		}
	case o.Stalled != "":
		b.WriteString("The table has stopped: " + o.Stalled + "\n")
	case o.YourTurn:
		b.WriteString("Your move. Legal moves (play one by its number):\n")
		for _, m := range o.Moves {
			fmt.Fprintf(&b, "  [%d] %s\n", m.Index, m.Text)
		}
		if len(o.Moves) == 0 {
			b.WriteString("  (none listed: send an explicit action)\n")
		}
	default:
		var who []string
		for _, i := range o.ToAct {
			who = append(who, t.Seats[i].Name)
		}
		b.WriteString("Waiting for " + strings.Join(who, ", ") + ".\n")
	}
	return b.String()
}

func (t *Table) zoneText(z module.Zone, me *SeatInfo, n namer) string {
	name := label(z.LabelKey, nil)
	if z.LabelKey == "" {
		name = humanise(z.ID)
	}
	if z.OwnerID != "" && z.OwnerID != me.Player {
		name = n.token(z.OwnerID) + ": " + name
	}
	cards := viewCards(z.Cards)
	var b strings.Builder
	switch {
	case z.Kind == module.ZoneHand && z.OwnerID == me.Player:
		fmt.Fprintf(&b, "  %s (%d): %s\n", name, len(cards), cardsText(cards))
		if len(cards) >= 5 {
			fmt.Fprintf(&b, "    by suit: %s\n", bySuit(cards))
		}
	case z.Kind == module.ZonePile:
		if len(cards) == 0 {
			fmt.Fprintf(&b, "  %s: %d cards\n", name, z.Count)
		} else {
			fmt.Fprintf(&b, "  %s (%d, bottom to top): %s — top %s\n", name, z.Count, cardsText(cards), cardText(cards[len(cards)-1]))
		}
	case z.Kind == module.ZoneSpread || len(z.Groups) > 0:
		if len(z.Groups) == 0 && len(cards) == 0 {
			fmt.Fprintf(&b, "  %s: none\n", name)
			break
		}
		fmt.Fprintf(&b, "  %s:\n", name)
		for _, g := range z.Groups {
			kind := ""
			if g.Kind != "" {
				kind = " " + g.Kind
			}
			extra := ""
			if g.Complete {
				extra = " (complete)"
			}
			var badges []string
			for _, k := range g.BadgeKeys {
				badges = append(badges, label(k, nil))
			}
			if len(badges) > 0 {
				extra += " (" + strings.Join(badges, ", ") + ")"
			}
			fmt.Fprintf(&b, "    [%s]%s: %s%s\n", g.ID, kind, cardsText(g.Cards), extra)
		}
		if len(cards) > 0 {
			fmt.Fprintf(&b, "    loose: %s\n", cardsText(cards))
		}
	case len(cards) > 0:
		fmt.Fprintf(&b, "  %s (%d): %s\n", name, z.Count, cardsText(cards))
	default:
		fmt.Fprintf(&b, "  %s: %d cards\n", name, z.Count)
	}
	return b.String()
}
