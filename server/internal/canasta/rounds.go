package canasta

import "zolik/server/internal/module"

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

	log := module.RoundLog{LabelKey: "canasta.round.deal", Rounds: []module.RoundResult{}}
	for _, d := range s.Deals {
		// DealNumber counts from zero and scoreDeal stamps it before endDeal
		// increments it, while the header renders it plus one. Add the same one
		// here, or the round table and the header disagree by a deal.
		r := module.RoundResult{Number: d.DealNumber + 1}

		if d.WentOut != "" {
			r.Winners = []string{d.WentOut}
		}
		r.Facts = dealFacts(d)

		// A partnership's score is every member's score: the row is per seat,
		// because a scoreboard is read by people and people sit in seats.
		byTeam := map[int]TeamResult{}
		for _, t := range d.Teams {
			byTeam[t.TeamID] = t
		}
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
