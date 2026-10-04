package canasta

import (
	"math"

	"zolik/server/internal/cardinfer"
)

// Canasta's side of the card inference (internal/cardinfer): every other
// seat's public record — what it captured, threw and declined (GameState.Seen),
// its side's melds, its hand size — handed to the game-agnostic model. Reads
// no hand but the observer's; TestInferDoesNotPeek pins it.
//
// Partners are seats too. Their hands are as unknown as an opponent's, and
// leaving them out would share their cards between the opponents and the
// stock. The bot reads the next seat's row.

// inferSpec is the variation's deck and meld shapes.
func inferSpec(r ruleset) cardinfer.Spec {
	sp := cardinfer.Spec{
		Copies: r.Decks, Jokers: r.Decks * r.JokersPerDeck,
		MinSet: 3, WildDeuces: true,
	}
	if sp.Copies <= 0 {
		sp.Copies = 2
	}
	if r.Sequences {
		// Samba's sequences: three or more of a suit, four to ace.
		sp.MinRun, sp.RunFrom = 3, 2
	}
	sp.Unmeldable[1] = true // threes
	return sp
}

// inference is one seat's view, worked out: the seats it describes, in play
// order after the observer, and the estimate.
type inference struct {
	seats []string
	est   cardinfer.Estimate
}

// infer works out what seat can know about every other hand.
func infer(s *GameState, seat string) *inference {
	r := s.rules()
	seen := s.seenThisDeal()
	o := cardinfer.Observation{Spec: inferSpec(r), Mine: cardinfer.Slots(s.Hands[seat])}
	o.Seen = cardinfer.Slots(s.DiscardPile)
	for i := range s.Teams {
		t := &s.Teams[i]
		o.Seen = append(o.Seen, cardinfer.Slots(t.RedThrees)...)
		for _, m := range t.Melds {
			o.Seen = append(o.Seen, cardinfer.Slots(m.Cards)...)
		}
	}
	inf := &inference{}
	for i := 1; i < len(s.TurnOrder); i++ {
		id := s.TurnOrder[(indexOf(s.TurnOrder, seat)+i)%len(s.TurnOrder)]
		inf.seats = append(inf.seats, id)
		st := cardinfer.Seat{HandSize: len(s.Hands[id]), Held: cardinfer.Slots(seen.Held[id])}
		for _, d := range seen.Discards {
			if d.Player == id {
				st.Discards = append(st.Discards, cardinfer.Slot(d.Card))
			}
		}
		for _, p := range seen.Passes {
			if p.Player == id {
				st.Passes = append(st.Passes, cardinfer.Pass{Slot: cardinfer.Slot(p.Card), Strong: p.Frozen})
			}
		}
		if t := s.team(id); t != nil && t.HasMelded {
			st.Down = true
			st.Extends = extendersOf(r, t)
		}
		o.Seats = append(o.Seats, st)
	}
	cardinfer.Infer(&o, &inf.est)
	return inf
}

func indexOf(xs []string, x string) int {
	for i, v := range xs {
		if v == x {
			return i
		}
	}
	return 0
}

// extendersOf marks the naturals a side could lay straight onto its own melds:
// every natural of a rank with an open group, and a sequence's two ends.
func extendersOf(r ruleset, t *Team) [cardinfer.NumSlots]bool {
	var out [cardinfer.NumSlots]bool
	for i := range t.Melds {
		m := &t.Melds[i]
		if m.closed(r) {
			continue
		}
		switch m.kind() {
		case meldSet:
			for _, su := range suits {
				if k := cardinfer.Slot(m.Rank + su); k >= 0 {
					out[k] = true
				}
			}
		case meldRun:
			lo, hi := runSpan(m.Cards)
			for _, p := range []int{lo - 1, hi + 1} {
				if p >= 0 && p < len(runRanks) {
					if k := cardinfer.Slot(runRanks[p] + m.Suit); k >= 0 {
						out[k] = true
					}
				}
			}
		}
	}
	return out
}

// captureRisk is the chance the seat after this one takes the pile with card
// as its top card, worked out from what that seat probably holds.
//
//	onto a meld   a side with an open group of the rank, a pile not frozen
//	              against it and a variation that allows the capture takes the
//	              pile for nothing: certain.
//	from hand     otherwise it needs two more of the rank, naturals only when
//	              the pile is frozen against it (a buried wild, or a side yet
//	              to open — which also has an opening to find, so it is
//	              discounted), or a natural and a wild when it is not.
//
// Wild cards and threes cannot be captured at all.
func (inf *inference) captureRisk(s *GameState, card string) float64 {
	if len(inf.seats) == 0 || isWild(card) || rankOf(card) == rankThree {
		return 0
	}
	r := s.rules()
	next := inf.seats[0]
	t := s.team(next)
	if t == nil {
		return 0
	}
	frozen := s.Frozen || !t.HasMelded || r.PileAlwaysFrozen
	if !frozen && r.PileMeldCapture {
		if m := t.openGroup(r, rankOf(card)); m != nil {
			return 1
		}
	}
	if !frozen && t.rankIsFull(r, rankOf(card)) && t.openGroup(r, rankOf(card)) == nil {
		return 0
	}
	hold, known := &inf.est.Hold[0], &inf.est.Known[0]
	// Naturals of the rank: the ones it was seen to take, certainly, and a
	// Poisson count with the estimate's mean for the rest.
	sure, lambda := 0, 0.0
	for _, su := range suits {
		if k := cardinfer.Slot(rankOf(card) + su); k >= 0 {
			sure += int(known[k])
			lambda += hold[k] - known[k]
		}
	}
	e := math.Exp(-lambda)
	atLeast := func(n int) float64 { // P(sure + Poisson >= n)
		switch need := n - sure; {
		case need <= 0:
			return 1
		case need == 1:
			return 1 - e
		default:
			return 1 - e - lambda*e
		}
	}
	pair := atLeast(2)
	if frozen {
		if !t.HasMelded {
			pair *= 0.6
		}
		return pair
	}
	wild := hold[cardinfer.JokerSlot]
	for _, su := range suits {
		wild += hold[cardinfer.Slot(rankTwo+su)]
	}
	one := atLeast(1) - pair
	return math.Min(1, pair+one*(1-math.Exp(-wild)))
}
