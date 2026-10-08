package canasta

import (
	"fmt"

	"zolik/server/internal/module"
)

// Canasta's rounds are its deals, and its scores are a partnership's.
//
// This is the game the history is most for. A deal's settlement has six parts —
// melded cards, canastas, red threes, going out, what was caught in hand, and
// the running total they make — the swing is routinely in the thousands, and
// dealNew clears every meld off the table the moment the next deal starts. Left
// unwritten it is simply gone, which is why "how did we get here" was an
// argument rather than a question.
var _ module.Rounded = (*Module)(nil)

func (m *Module) Rounds(raw module.State) (module.RoundLog, error) {
	s, err := decode(raw)
	if err != nil {
		return module.RoundLog{}, err
	}

	log := module.RoundLog{
		LabelKey:   "canasta.round.deal",
		Rounds:     []module.RoundResult{},
		Paused:     s.Break.Open,
		WaitingFor: s.Break.Waiting(s.TurnOrder),
	}
	for _, d := range s.Deals {
		// DealNumber counts from zero and scoreDeal stamps it before endDeal
		// increments it, while the header renders it plus one. Add the same one
		// here, or the round table and the header disagree by a deal.
		r := module.RoundResult{Number: d.DealNumber + 1}

		byTeam := map[int]TeamResult{}
		for _, t := range d.Teams {
			byTeam[t.TeamID] = t
		}

		// The deal is taken by the side that scored more in it, not by the
		// side that closed it: going out is worth a hundred, and a partnership
		// caught holding a hand of wilds while the other closes can still win
		// the deal by a thousand. Crediting the closer is what the table used
		// to do, and it named the wrong side exactly when the deal was closest
		// to an argument.
		if top, ok := dealLeader(d); ok {
			for _, pid := range s.TurnOrder {
				if s.TeamOf[pid] == top {
					r.Winners = append(r.Winners, pid)
				}
			}
		}
		if d.WentOut != "" {
			if t, ok := byTeam[s.TeamOf[d.WentOut]]; ok {
				r.Headline = &module.Fact{LabelKey: "canasta.round.closed", Params: map[string]any{
					"player": d.WentOut, "diff": signed(t.Total - bestOther(d, t.TeamID)),
				}}
			}
		}
		r.Facts = dealFacts(d)

		// A partnership's score is every member's score: the row is per seat,
		// because a scoreboard is read by people and people sit in seats.
		for _, pid := range s.TurnOrder {
			t, ok := byTeam[s.TeamOf[pid]]
			if !ok {
				continue
			}
			r.Scores = append(r.Scores, module.RoundScore{
				PlayerID: pid,
				// Already higher-is-better: canasta is scored upwards, so
				// nothing is negated and nothing needs a Shown override.
				Delta: t.Total,
				Total: t.Running,
				Facts: teamFacts(t),
				Lines: teamLines(s.rules(), d, t),
			})
		}
		log.Rounds = append(log.Rounds, r)
	}
	return log, nil
}

// dealFacts is what was true of the deal rather than of any one side.
func dealFacts(d DealResult) []module.Fact {
	var out []module.Fact
	if d.Concealed {
		out = append(out, module.Fact{LabelKey: "canasta.round.concealed"})
	}
	if d.Exhausted {
		out = append(out, module.Fact{LabelKey: "canasta.round.exhausted"})
	}
	return out
}

// teamFacts breaks a partnership's deal into the parts a player argues about.
//
// Only the parts that actually moved: a row of five zeroes is noise, and the
// components that scored are the ones worth reading.
func teamFacts(t TeamResult) []module.Fact {
	// One literal per key rather than a table of them: the manifest is read
	// from this source, and keys in a table of anonymous structs were never in
	// it.
	var out []module.Fact
	if t.MeldCards != 0 {
		out = append(out, module.Fact{LabelKey: "canasta.round.meldCards", Params: map[string]any{"n": t.MeldCards}})
	}
	if t.Canastas != 0 {
		out = append(out, module.Fact{LabelKey: "canasta.round.canastas", Params: map[string]any{"n": t.Canastas}})
	}
	if t.RedThrees != 0 {
		out = append(out, module.Fact{LabelKey: "canasta.round.redThrees", Params: map[string]any{"n": t.RedThrees}})
	}
	if t.GoingOut != 0 {
		out = append(out, module.Fact{LabelKey: "canasta.round.goingOut", Params: map[string]any{"n": t.GoingOut}})
	}
	if t.InHand != 0 {
		out = append(out, module.Fact{LabelKey: "canasta.round.inHand", Params: map[string]any{"n": t.InHand}})
	}
	return out
}

// teamLines is a partnership's deal as an account: each part with its points
// and, where the deal kept them, the melds, canastas and hands behind it.
//
// Every key is written out in a literal of its own, including the ones picked
// by a branch, because the manifest is built by reading this source and a key
// passed through a variable is a key it never sees.
func teamLines(r ruleset, d DealResult, t TeamResult) []module.ScoreLine {
	var out []module.ScoreLine

	if t.MeldCards != 0 || len(t.Melds) > 0 {
		line := module.ScoreLine{LabelKey: "canasta.line.meldCards", Points: t.MeldCards}
		for _, m := range t.Melds {
			line.Sub = append(line.Sub, meldLine(m))
		}
		out = append(out, line)
	}

	if t.Canastas != 0 {
		line := module.ScoreLine{LabelKey: "canasta.line.canastas", Points: t.Canastas}
		if t.Naturals > 0 {
			line.Sub = append(line.Sub, module.ScoreLine{
				LabelKey: "canasta.line.natural",
				Params:   map[string]any{"n": t.Naturals, "each": r.NaturalCanastaBonus},
				Points:   t.Naturals * r.NaturalCanastaBonus,
			})
		}
		if t.Mixed > 0 {
			line.Sub = append(line.Sub, module.ScoreLine{
				LabelKey: "canasta.line.mixed",
				Params:   map[string]any{"n": t.Mixed, "each": r.MixedCanastaBonus},
				Points:   t.Mixed * r.MixedCanastaBonus,
			})
		}
		if t.Sambas > 0 {
			line.Sub = append(line.Sub, module.ScoreLine{
				LabelKey: "canasta.line.samba",
				Params:   map[string]any{"n": t.Sambas, "each": r.SambaBonus},
				Points:   t.Sambas * r.SambaBonus,
			})
		}
		if t.DirtySambas > 0 {
			line.Sub = append(line.Sub, module.ScoreLine{
				LabelKey: "canasta.line.dirtySamba",
				Params:   map[string]any{"n": t.DirtySambas, "each": r.DirtySambaBonus},
				Points:   t.DirtySambas * r.DirtySambaBonus,
			})
		}
		if t.WildCanastas > 0 {
			line.Sub = append(line.Sub, module.ScoreLine{
				LabelKey: "canasta.line.wildCanasta",
				Params:   map[string]any{"n": t.WildCanastas, "each": r.WildCanastaBonus},
				Points:   t.WildCanastas * r.WildCanastaBonus,
			})
		}
		if t.WildMixedCanastas > 0 {
			line.Sub = append(line.Sub, module.ScoreLine{
				LabelKey: "canasta.line.wildMixedCanasta",
				Params:   map[string]any{"n": t.WildMixedCanastas, "each": r.WildMixedCanastaBonus},
				Points:   t.WildMixedCanastas * r.WildMixedCanastaBonus,
			})
		}
		// A deal kept before the counts were is shown as its sum alone; so is
		// one whose counts no longer price out under the ruleset, rather than
		// an account that does not add up.
		if sum := linePoints(line.Sub); sum != t.Canastas {
			line.Sub = nil
		}
		out = append(out, line)
	}

	switch {
	case t.RedThrees == 0:
	case t.RedThreeCount == 0:
		out = append(out, module.ScoreLine{LabelKey: "canasta.line.redThreesSum", Points: t.RedThrees})
	case t.RedThreeShort:
		out = append(out, module.ScoreLine{
			LabelKey: "canasta.line.redThreesShort",
			Params:   map[string]any{"n": t.RedThreeCount, "need": r.RedThreesNeed, "made": t.Naturals + t.Mixed + t.Sambas + t.DirtySambas + t.WildCanastas + t.WildMixedCanastas},
			Points:   t.RedThrees,
		})
	case t.RedThreesAll:
		out = append(out, module.ScoreLine{
			LabelKey: "canasta.line.redThreesAll",
			Params:   map[string]any{"n": t.RedThreeCount},
			Points:   t.RedThrees,
		})
	default:
		out = append(out, module.ScoreLine{
			LabelKey: "canasta.line.redThrees",
			Params:   map[string]any{"n": t.RedThreeCount, "each": redThreeValue},
			Points:   t.RedThrees,
		})
	}

	if t.GoingOut != 0 {
		// Named concealed only when it paid as concealed: Samba has no such
		// bonus and pays a hand melded in one turn the ordinary hundred, and
		// the line should say what was paid, not what was done.
		if d.Concealed && r.ConcealedBonus > 0 && t.GoingOut == r.ConcealedBonus {
			out = append(out, module.ScoreLine{
				LabelKey: "canasta.line.goingOutConcealed",
				Params:   map[string]any{"player": d.WentOut},
				Points:   t.GoingOut,
			})
		} else {
			out = append(out, module.ScoreLine{
				LabelKey: "canasta.line.goingOut",
				Params:   map[string]any{"player": d.WentOut},
				Points:   t.GoingOut,
			})
		}
	}

	if t.InHand != 0 {
		line := module.ScoreLine{LabelKey: "canasta.line.inHand", Points: -t.InHand}
		for _, h := range t.Hands {
			if h.Cards == 0 {
				continue
			}
			hand := module.ScoreLine{
				LabelKey: "canasta.line.hand",
				Params:   map[string]any{"player": h.PlayerID, "n": h.Cards},
				Points:   -h.Points,
			}
			other := h.Points - h.BlackThrees*blackThreeValue - h.WildPoints
			if h.BlackThrees > 0 {
				hand.Sub = append(hand.Sub, module.ScoreLine{
					LabelKey: "canasta.line.handBlackThrees",
					Params:   map[string]any{"n": h.BlackThrees, "each": blackThreeValue},
					Points:   -h.BlackThrees * blackThreeValue,
				})
			}
			if h.Wilds > 0 {
				hand.Sub = append(hand.Sub, module.ScoreLine{
					LabelKey: "canasta.line.handWilds",
					Params:   map[string]any{"n": h.Wilds},
					Points:   -h.WildPoints,
				})
			}
			if n := h.Cards - h.BlackThrees - h.Wilds; n > 0 {
				hand.Sub = append(hand.Sub, module.ScoreLine{
					LabelKey: "canasta.line.handOther",
					Params:   map[string]any{"n": n},
					Points:   -other,
				})
			}
			// One category is the hand itself said twice.
			if len(hand.Sub) == 1 {
				hand.Sub = nil
			}
			line.Sub = append(line.Sub, hand)
		}
		if linePoints(line.Sub) != line.Points {
			line.Sub = nil
		}
		out = append(out, line)
	}
	return out
}

// meldLine names a meld by what it is — "K × 5 + wild × 1", "Hearts: sequence
// × 7" — rather than by its cards, which the round log may not carry.
func meldLine(m MeldTally) module.ScoreLine {
	if m.Kind == meldRun {
		return module.ScoreLine{
			LabelKey: "canasta.line.meldRun",
			Params:   map[string]any{"suit": "suit." + m.Suit, "n": m.Cards},
			Points:   m.Points,
		}
	}
	if m.Wilds > 0 {
		return module.ScoreLine{
			LabelKey: "canasta.line.meldSetWild",
			Params:   map[string]any{"rank": rankShown(m.Rank), "naturals": m.Cards - m.Wilds, "wilds": m.Wilds},
			Points:   m.Points,
		}
	}
	return module.ScoreLine{
		LabelKey: "canasta.line.meldSet",
		Params:   map[string]any{"rank": rankShown(m.Rank), "n": m.Cards},
		Points:   m.Points,
	}
}

// rankShown is a rank as the card faces print it: the ten is a "10" there,
// and "T" is only this package's spelling of it.
func rankShown(rank string) string {
	if rank == "T" {
		return "10"
	}
	return rank
}

func linePoints(ls []module.ScoreLine) int {
	total := 0
	for _, l := range ls {
		total += l.Points
	}
	return total
}

// dealLeader is the partnership that scored the most in one deal, or false when
// two or more share the top: a level deal was taken by nobody.
func dealLeader(d DealResult) (int, bool) {
	best, id, tied := 0, -1, false
	for _, t := range d.Teams {
		switch {
		case id < 0 || t.Total > best:
			best, id, tied = t.Total, t.TeamID, false
		case t.Total == best:
			tied = true
		}
	}
	return id, id >= 0 && !tied
}

// bestOther is the highest deal score among the partnerships other than team:
// what the closing side's score is measured against.
func bestOther(d DealResult, team int) int {
	best, seen := 0, false
	for _, t := range d.Teams {
		if t.TeamID == team {
			continue
		}
		if !seen || t.Total > best {
			best, seen = t.Total, true
		}
	}
	return best
}

// signed prints a differential the way it is said: "+220", "-240", and a level
// deal as plain "0" rather than "+0".
func signed(n int) string {
	if n == 0 {
		return "0"
	}
	return fmt.Sprintf("%+d", n)
}
