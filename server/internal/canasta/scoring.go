package canasta

import "zolik/server/internal/module"

// Bonuses, all of them. The numbers live here and nowhere else.
const (
	redThreeValue       = 100
	allRedThreesBonus   = 800 // all four, instead of 4×100
	naturalCanastaBonus = 500
	mixedCanastaBonus   = 300
	goingOutBonus       = 100
	concealedBonus      = 200
)

func errCode(code string) error { return module.Error{Code: code} }

// initialMeldMinimum is the value a partnership must lay in one turn to get on
// the table, and it rises with that partnership's own accumulated score — so a
// team that is ahead has to work harder to open, which is the mechanism that
// keeps a match from running away.
func initialMeldMinimum(score int) int {
	switch {
	case score < 0:
		return 15
	case score < 1500:
		return 50
	case score < 3000:
		return 90
	default:
		return 120
	}
}

// redThreeScore is the partnership's red threes, signed.
//
// The sign is the whole point: red threes are a gift that turns into a
// liability if the partnership never completes a canasta, which is what stops
// them being free points for doing nothing.
func redThreeScore(t *Team) int {
	n := len(t.RedThrees)
	if n == 0 {
		return 0
	}
	value := n * redThreeValue
	if n == 4 {
		value = allRedThreesBonus
	}
	if t.canastas() == 0 {
		return -value
	}
	return value
}

// canastaScore is the bonus for completed canastas — 500 for one with no
// wilds in it, 300 for one built with them.
func canastaScore(t *Team) int {
	total := 0
	for _, m := range t.Melds {
		if !m.isCanasta() {
			continue
		}
		if m.isNatural() {
			total += naturalCanastaBonus
		} else {
			total += mixedCanastaBonus
		}
	}
	return total
}

// meldCardScore is the face value of everything the partnership has on the
// table.
func meldCardScore(t *Team) int {
	total := 0
	for _, m := range t.Melds {
		total += handValue(m.Cards)
	}
	return total
}

// scoreDeal computes both partnerships' arithmetic for the deal that just
// ended and folds it into their running totals.
//
// wentOut is the player who shed their last card, or "" when the stock ran out
// and nobody did.
func scoreDeal(s *GameState, wentOut string, concealed bool, exhausted bool) DealResult {
	res := DealResult{DealNumber: s.DealNumber, WentOut: wentOut, Concealed: concealed, Exhausted: exhausted}

	outTeam := -1
	if wentOut != "" {
		if t := s.team(wentOut); t != nil {
			outTeam = t.ID
		}
	}

	for i := range s.Teams {
		t := &s.Teams[i]
		tr := TeamResult{
			TeamID:    t.ID,
			MeldCards: meldCardScore(t),
			Canastas:  canastaScore(t),
			RedThrees: redThreeScore(t),
		}
		if t.ID == outTeam {
			tr.GoingOut = goingOutBonus
			if concealed {
				tr.GoingOut = concealedBonus
			}
		}
		// Everything still in either partner's hand counts against them —
		// including the hand of the partner of whoever went out.
		for _, p := range t.Players {
			tr.InHand += handValue(s.Hands[p])
		}
		tr.Total = tr.MeldCards + tr.Canastas + tr.RedThrees + tr.GoingOut - tr.InHand
		t.Score += tr.Total
		tr.Running = t.Score
		res.Teams = append(res.Teams, tr)
	}
	return res
}

// matchWinner reports the partnership that has won the match, or -1.
//
// A target reached by both partnerships in the same deal is settled on the
// higher total; a tie there is not a win, and another deal is played. Refusing
// to break a tie arbitrarily is the only answer that cannot be wrong.
func matchWinner(s *GameState) int {
	best, bestID, tied := 0, -1, false
	for i := range s.Teams {
		t := &s.Teams[i]
		if t.Score < s.TargetScore {
			continue
		}
		switch {
		case bestID < 0 || t.Score > best:
			best, bestID, tied = t.Score, t.ID, false
		case t.Score == best:
			tied = true
		}
	}
	if tied {
		return -1
	}
	return bestID
}
