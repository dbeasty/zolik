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
	parts := []struct {
		key string
		n   int
	}{
		{"canasta.round.meldCards", t.MeldCards},
		{"canasta.round.canastas", t.Canastas},
		{"canasta.round.redThrees", t.RedThrees},
		{"canasta.round.goingOut", t.GoingOut},
		{"canasta.round.inHand", t.InHand},
	}
	var out []module.Fact
	for _, p := range parts {
		if p.n == 0 {
			continue
		}
		out = append(out, module.Fact{LabelKey: p.key, Params: map[string]any{"n": p.n}})
	}
	return out
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
