package canasta

import "zolik/server/internal/module"

// What everybody at the table watched happen this deal.
//
// The rest of GameState is position: the pile, the melds, the hands. Three
// things a player at the table knows are history instead, and nothing in the
// position remembers them a moment later:
//
//	Held      the cards a seat took into its hand off the pile. A capture
//	          puts the whole pile in a hand in front of everybody, and a
//	          player who watched knows where those cards are until they are
//	          melded or thrown again.
//	Discards  who threw what, in order.
//	Passes    the top card each time a seat drew from the stock instead of
//	          taking the pile, and whether it was frozen against that seat
//	          (so the only way to take it was a natural pair it plainly did
//	          not have).
//
// Recorded by Apply from the before and after of every action, so no rule is
// restated here; read only by the card inference (infer.go) for the bot.
// Nothing in it is hidden: every card in it was face up when it moved. It is never shown to a client, because the client already saw it
// happen.
type Seen struct {
	Deal     int                 `json:"deal"`
	Held     map[string][]string `json:"held,omitempty"`
	Discards []SeenCard          `json:"discards,omitempty"`
	Passes   []SeenCard          `json:"passes,omitempty"`
	// heldAtDraw is Held as it stood after this turn's draw or capture, so the
	// undos (which never reach behind the draw) can put it back.
	HeldAtDraw map[string][]string `json:"heldAtDraw,omitempty"`
	// Capture is the record as it stood before this turn's pile capture, so
	// undoing the capture restores it exactly (nil Prior: there was none).
	Capture *SeenUndo `json:"capture,omitempty"`
}

// SeenUndo is a record to go back to.
type SeenUndo struct {
	Prior *Seen `json:"prior,omitempty"`
}

func (v *Seen) clone() *Seen {
	if v == nil {
		return nil
	}
	out := *v
	out.Held = cloneHeld(v.Held)
	out.HeldAtDraw = cloneHeld(v.HeldAtDraw)
	out.Discards = append([]SeenCard(nil), v.Discards...)
	out.Passes = append([]SeenCard(nil), v.Passes...)
	out.Capture = nil
	return &out
}

// SeenCard is one card and the seat it belongs to in the record. Frozen is a
// pass made with the pile frozen against that seat.
type SeenCard struct {
	Player string `json:"p"`
	Card   string `json:"c"`
	Frozen bool   `json:"f,omitempty"`
}

// maxSeen bounds each list: several times a long deal's discards, so a deal
// that ran away cannot grow the document without limit.
const maxSeen = 160

// seenBefore is the little of a state the record needs from before an action.
type seenBefore struct {
	pile   []string
	frozen bool // the pile was frozen against the actor
}

func beforeSeen(s *GameState, playerID string) seenBefore {
	b := seenBefore{pile: append([]string(nil), s.DiscardPile...)}
	if t := s.team(playerID); t != nil {
		b.frozen = s.Frozen || !t.HasMelded || s.rules().PileAlwaysFrozen
	}
	return b
}

// observe records what was public about an action that has been applied.
//
// The record is created only by an action that has something to put in it,
// so a lay-off and its undo on a table with nothing recorded leave the state
// exactly as it was — which the undo tests pin, byte for byte.
func (s *GameState) observe(b seenBefore, playerID string, a module.Action) {
	if s.Seen != nil && s.Seen.Deal != s.DealNumber {
		s.Seen = nil
	}
	if s.Status != "active" || s.Break.Open {
		return
	}
	rec := func() *Seen {
		if s.Seen == nil {
			s.Seen = &Seen{Deal: s.DealNumber}
		}
		if s.Seen.Held == nil {
			s.Seen.Held = map[string][]string{}
		}
		return s.Seen
	}
	v := s.Seen
	switch a.Verb {
	case VerbDraw:
		if n := len(b.pile); n > 0 {
			top := b.pile[n-1]
			if !isWild(top) && !isRedThree(top) && !isBlackThree(top) {
				v = rec()
				v.Passes = capSeen(append(v.Passes, SeenCard{Player: playerID, Card: top, Frozen: b.frozen}))
			}
		}
		if v != nil {
			v.HeldAtDraw = cloneHeld(v.Held)
			v.Capture = nil
		}
	case VerbTakeTop:
		if v != nil {
			v.HeldAtDraw = cloneHeld(v.Held)
			v.Capture = nil
		}
	case VerbTakePile:
		prior := s.Seen.clone()
		v = rec()
		v.Capture = &SeenUndo{Prior: prior}
		// The capture's own cards from hand went onto the meld in front of
		// everybody; the rest of the pile under the top card came into the
		// hand, less any red three, which went to the row.
		v.drop(playerID, a.Cards)
		if n := len(b.pile); n > 0 {
			for _, c := range b.pile[:n-1] {
				if !isRedThree(c) {
					v.Held[playerID] = append(v.Held[playerID], c)
				}
			}
		}
		v.HeldAtDraw = cloneHeld(v.Held)
	case VerbLayMeld, VerbLayOff:
		if v != nil {
			v.drop(playerID, a.Cards)
		}
	case VerbDiscard:
		if len(a.Cards) == 1 {
			v = rec()
			v.Discards = capSeen(append(v.Discards, SeenCard{Player: playerID, Card: a.Cards[0]}))
			v.Capture, v.HeldAtDraw = nil, nil
			v.drop(playerID, a.Cards)
		}
	case VerbUndoTakePile:
		if v != nil && v.Capture != nil {
			s.Seen = v.Capture.Prior
		}
	case VerbUndoLayOff, VerbUndoLayMeld:
		// Back to the draw, less whatever this turn still has on the table.
		if v != nil && v.HeldAtDraw != nil {
			v.Held = cloneHeld(v.HeldAtDraw)
			for _, l := range s.LaidOff {
				v.drop(playerID, l.Cards)
			}
			for _, m := range s.MeldsLaid {
				v.drop(playerID, m.Cards)
			}
		}
	}
}

// drop removes one copy of each card from what a seat is known to hold.
func (v *Seen) drop(playerID string, cards []string) {
	held := v.Held[playerID]
	if len(held) == 0 {
		return
	}
	for _, c := range cards {
		for i, h := range held {
			if h == c {
				held = append(held[:i:i], held[i+1:]...)
				break
			}
		}
	}
	if len(held) == 0 {
		delete(v.Held, playerID)
		return
	}
	v.Held[playerID] = held
}

func capSeen(xs []SeenCard) []SeenCard {
	if over := len(xs) - maxSeen; over > 0 {
		return append(xs[:0:0], xs[over:]...)
	}
	return xs
}

func cloneHeld(in map[string][]string) map[string][]string {
	out := make(map[string][]string, len(in))
	for k, v := range in {
		out[k] = append([]string(nil), v...)
	}
	return out
}

// seenThisDeal is the record, or an empty one when it belongs to another deal
// (or to a match dealt before it existed).
func (s *GameState) seenThisDeal() *Seen {
	if s.Seen == nil || s.Seen.Deal != s.DealNumber {
		return &Seen{Deal: s.DealNumber}
	}
	return s.Seen
}
