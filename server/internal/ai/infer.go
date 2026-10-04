package ai

import (
	"sort"

	"zolik/server/internal/cardinfer"
	"zolik/server/internal/rules"
)

// Žolíky's side of the card inference (internal/cardinfer): the public record
// of every other seat, read off VisibleState and nothing else, and handed to
// the game-agnostic model. TestInferDoesNotPeek pins that permuting the hidden
// hands and the stock changes none of it.

// InferSpec is Žolíky's deck and meld shapes for the inference.
func InferSpec(v VisibleState) cardinfer.Spec {
	packs := v.DeckCount
	if packs <= 0 {
		packs = defaultPacks
	}
	minSet, minRun := meldSizes(v.Rules)
	return cardinfer.Spec{
		Copies: packs, Jokers: 2 * packs,
		MinSet: minSet, MinRun: minRun,
		AceLow:           true,
		DistinctSuitSets: true,
	}
}

// Opponents is every other seat, starting with the one who plays next. A
// snapshot without a turn order (a hand-built test position) falls back to
// the seats it knows of, sorted.
func Opponents(v VisibleState, actor string) []string {
	order := v.TurnOrder
	if len(order) == 0 {
		for id := range v.HandCounts {
			order = append(order, id)
		}
		sort.Strings(order)
	}
	at := -1
	for i, id := range order {
		if id == actor {
			at = i
		}
	}
	var out []string
	for i := 1; i <= len(order); i++ {
		id := order[(at+i+len(order))%len(order)]
		if id != actor {
			out = append(out, id)
		}
	}
	return out
}

// InferObservation is actor's view of the table, for cardinfer.Infer. The
// seats are Opponents(v, actor), in that order.
func InferObservation(v VisibleState, hand []string, actor string) cardinfer.Observation {
	o := cardinfer.Observation{Spec: InferSpec(v), Mine: cardinfer.Slots(hand)}
	o.Seen = cardinfer.Slots(v.DiscardPile)
	for _, owner := range sortedMeldOwners(v.Melds) {
		for _, m := range v.Melds[owner] {
			o.Seen = append(o.Seen, cardinfer.Slots(m)...)
		}
	}
	ext := TableExtenders(v.Melds, v.Rules)
	for _, id := range Opponents(v, actor) {
		s := cardinfer.Seat{HandSize: v.HandCounts[id], Held: cardinfer.Slots(v.KnownHeld[id])}
		for _, d := range v.DealDiscards {
			if d.Player == id {
				if k := cardinfer.Slot(d.Card); k >= 0 {
					s.Discards = append(s.Discards, k)
				}
			}
		}
		for _, d := range v.DealPasses {
			if d.Player == id {
				if k := cardinfer.Slot(d.Card); k >= 0 {
					s.Passes = append(s.Passes, cardinfer.Pass{Slot: k})
				}
			}
		}
		// Anybody who is down may lay off onto any meld on the table, so for
		// them the table's extenders are wanted outright.
		if v.RoundReqMet[id] {
			s.Down = true
			s.Extends = ext
		}
		o.Seats = append(o.Seats, s)
	}
	return o
}

// Infer is cardinfer.Infer on actor's view; e's seats are Opponents(v, actor).
func Infer(v VisibleState, hand []string, actor string, e *cardinfer.Estimate) {
	o := InferObservation(v, hand, actor)
	cardinfer.Infer(&o, e)
}

// TableExtenders marks every card that would lay off onto some meld on the
// table as it stands, in cardinfer slots. Asked of the validator, and only for
// the cards that could possibly answer yes — a set's missing suits, a run's
// two ends, a joker — which is a handful of questions a meld rather than
// fifty-three.
func TableExtenders(melds map[string][][]string, cfg rules.RulesConfig) [cardinfer.NumSlots]bool {
	var out [cardinfer.NumSlots]bool
	for _, owner := range sortedMeldOwners(melds) {
		for _, m := range melds[owner] {
			mv, err := rules.ValidateMeld(m, cfg)
			if err != nil {
				continue
			}
			var try []string
			if mv.Type == rules.MeldSet {
				rank := byte(0)
				for _, c := range m {
					if !rules.IsJoker(c) {
						rank = c[0]
						break
					}
				}
				for _, s := range suits {
					try = append(try, string(rank)+s)
				}
			} else if len(mv.ResolvedRun) > 0 {
				lo, hi := mv.ResolvedRun[0], mv.ResolvedRun[len(mv.ResolvedRun)-1]
				for _, r := range []int{lo - 1, hi + 1} {
					if c := runPositionCard(r, mv.ResolvedSuit); c != "" {
						try = append(try, c)
					}
				}
			}
			try = append(try, "JOKER1")
			for _, c := range try {
				k := cardinfer.Slot(c)
				if k < 0 || out[k] {
					continue
				}
				if _, err := rules.ValidateMeld(append(append([]string(nil), m...), c), cfg); err == nil {
					out[k] = true
				}
			}
		}
	}
	return out
}

// runPositionCard is the card at a run position: 1 and 14 are the ace, 2..13
// the ranks between.
func runPositionCard(r int, suit string) string {
	switch {
	case suit == "":
		return ""
	case r == 1 || r == 14:
		return "A" + suit
	case r >= 2 && r <= 13:
		return string(rules.RankOrder[r-2]) + suit
	}
	return ""
}
