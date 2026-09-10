package canasta

import "zolik/server/internal/module"

// redThreeValue is the one bonus that is the same in every variation. The rest
// belong to the ruleset, because Samba disagrees with Canasta about all of them.
const redThreeValue = 100

func errCode(code string) error { return module.Error{Code: code} }

// redThreeScore is the partnership's red threes, signed.
//
// The sign is the whole point: red threes are a gift that turns into a
// liability if the partnership never got where it needed to, which is what stops
// them being free points for doing nothing. What "where it needed to" means is
// the variation's, and so is whether the all-of-them bonus is charged back in
// full when it goes the other way.
func redThreeScore(r ruleset, t *Team) int {
	n := len(t.RedThrees)
	if n == 0 {
		return 0
	}
	value := n * redThreeValue
	if n >= r.redThrees() {
		value = r.redThreeAllBonus()
	}
	if t.canastas() < r.RedThreesNeed {
		if r.RedThreePenaltyFlat {
			return -(n * redThreeValue)
		}
		return -value
	}
	return value
}

// canastaScore is the bonus for everything the side has completed — 500 for a
// canasta with no wilds in it, 300 for one built with them, and in Samba 1500
// for a seven-card sequence.
func canastaScore(r ruleset, t *Team) int {
	total := 0
	for _, m := range t.Melds {
		if !m.isCanasta() {
			continue
		}
		switch {
		case m.kind() == meldRun:
			total += r.SambaBonus
		case m.isNatural():
			total += r.NaturalCanastaBonus
		default:
			total += r.MixedCanastaBonus
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
	r := s.rules()

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
			Canastas:  canastaScore(r, t),
			RedThrees: redThreeScore(r, t),
		}
		if t.ID == outTeam {
			tr.GoingOut = r.GoingOutBonus
			// A variation with no concealed bonus — Samba — pays the ordinary
			// one for a hand melded in a single turn, rather than nothing.
			if concealed && r.ConcealedBonus > 0 {
				tr.GoingOut = r.ConcealedBonus
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
