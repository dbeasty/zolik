package marias

import (
	"zolik/server/internal/module"
	"zolik/server/internal/tricks"
)

// settle scores the finished deal part by part, records it, and moves the
// match on: the next deal, a pause before it, or the end.
func (s *GameState) settle() []module.Event {
	rec := DealRecord{
		Number:   s.Deal + 1,
		Sitter:   s.Sitter,
		Declarer: s.Declarer,
		Game:     s.Game,
		Deltas:   map[string]int{},
		Totals:   map[string]int{},
	}
	if t := s.trump(); t != tricks.NoTrump {
		rec.Trump = string(t)
	}
	rec.Parts = s.results()

	for _, p := range s.active() {
		if p == s.Declarer {
			continue
		}
		// Each defender settles with the declarer on their own, and no deal
		// moves more than the limit between the two of them. A sitter is
		// no defender: they neither pay nor are paid.
		owed := 0
		for _, pr := range rec.Parts {
			if pr.ForDeclarer {
				owed += pr.Units
			} else {
				owed -= pr.Units
			}
		}
		owed = max(-payLimit, min(payLimit, owed))
		rec.Deltas[s.Declarer] += owed
		rec.Deltas[p] -= owed
	}
	for _, p := range s.Players {
		s.Scores[p] += rec.Deltas[p]
		rec.Totals[p] = s.Scores[p]
	}
	s.Rounds = append(s.Rounds, rec)

	events := []module.Event{{Type: "deal_settled", Data: map[string]any{"deal": rec.Number, "deltas": rec.Deltas}}}
	s.Deal++
	s.Current = ""
	switch {
	case s.Deal >= s.Deals:
		s.Status = "completed"
		events = append(events, module.Event{Type: "game_ended"})
	case s.Pause:
		s.Intermission.Begin(s.Deal)
	default:
		startDeal(s)
	}
	return events
}

// results is every part of the deal, won or lost, in units per defender.
func (s *GameState) results() []PartResult {
	t := s.Tariff
	red := s.RedDoubles && s.trump() == suitRed
	mult := func(part string) int { return multiplier(s.Fleks[part], red) }
	var out []PartResult

	switch s.Game {
	case gameOmyl:
		// Folded before play: the flekked hra and sedma a defence would have
		// doubled, as one figure (ČSM licitovaný I and II/17).
		return []PartResult{{Part: partOmyl, ForDeclarer: false, Units: t.Omyl}}
	case gameBetl:
		won := s.TricksWon[s.Declarer] == 0
		return []PartResult{{Part: partGame, ForDeclarer: won, Units: t.Betl * multiplier(s.Fleks[partGame], false)}}
	case gameDurch:
		won := !s.decidedEarly()
		return []PartResult{{Part: partGame, ForDeclarer: won, Units: t.Durch * multiplier(s.Fleks[partGame], false)}}
	}

	declTotal, defTotal := s.sideTotals()
	declMarriages, defMarriages := s.sideMarriages(true), s.sideMarriages(false)

	switch s.Game {
	case gameSto:
		count := s.stoCount(true)
		won := count >= hundred
		out = append(out, PartResult{Part: partGame, ForDeclarer: won, Units: t.stoValue(won, count, defMarriages) * mult(partGame)})
	case gameDveSedmy:
		won := s.dveSedmyMade()
		out = append(out, PartResult{Part: partGame, ForDeclarer: won, Units: t.DveSedmy * mult(partGame)})
		if s.WithSto {
			count := s.stoCount(true)
			made := count >= hundred
			out = append(out, PartResult{Part: partSto, ForDeclarer: made, Units: t.stoValue(made, count, defMarriages) * mult(partSto)})
		}
	default:
		// Hra: more than the defenders together, marriages and all. A side
		// reaching a hundred unannounced (tiché sto) raises the game — but
		// not the defenders when they announced sto proti, which is settled
		// as a part of its own.
		won := declTotal > defTotal
		winner := declTotal
		if !won {
			winner = defTotal
			if s.ProtiSto != "" {
				winner = 0
			}
		}
		out = append(out, PartResult{Part: partGame, ForDeclarer: won, Units: t.hraValue(winner) * mult(partGame)})
	}

	if s.ProtiSto != "" {
		count := s.stoCount(false)
		won := count >= hundred
		out = append(out, PartResult{Part: partProtiSto, ForDeclarer: !won, Units: t.stoValue(won, count, declMarriages) * mult(partProtiSto)})
	}

	seven, by, took := s.lastTrickSeven()
	switch {
	case s.Game == gameDveSedmy:
		// Both sevens are the game itself.
	case s.Sedma:
		won := by == s.Declarer && took
		out = append(out, PartResult{Part: partSedma, ForDeclarer: won, Units: t.Sedma * mult(partSedma)})
	case s.ProtiSedma != "":
		won := by == s.ProtiSedma && took
		out = append(out, PartResult{Part: partProtiSedma, ForDeclarer: !won, Units: t.Sedma * mult(partProtiSedma)})
	case seven:
		// Tichá sedma, or one killed in the last trick: half a seven to
		// whichever side it went well for. Doubled by hearts, not by flek —
		// nobody announced it to double.
		forDeclarer := (by == s.Declarer) == took
		out = append(out, PartResult{Part: partQuietSeven, ForDeclarer: forDeclarer, Units: t.quietSeven() * multiplier(0, red)})
	}
	return out
}

// dveSedmyMade is whether the declarer took the second-to-last trick with
// the helper seven and the last with the trump seven (ČSM general IV/9).
func (s *GameState) dveSedmyMade() bool {
	took := func(trick []tricks.Play, card string) bool {
		if len(trick) == 0 {
			return false
		}
		win, _ := tricks.Trick{Plays: trick}.Winning(s.trump(), s.order())
		return win.Card == card && s.Players[win.Seat] == s.Declarer
	}
	return took(s.PrevTrick, s.helperSeven()) && took(s.LastTrick, s.trumpSeven())
}

// lastTrickSeven reports whether the trump seven was played to the last
// trick, by whom, and whether it took the trick.
func (s *GameState) lastTrickSeven() (played bool, by string, took bool) {
	if len(s.Hands[s.Declarer]) != 0 || len(s.LastTrick) == 0 {
		return false, "", false // the deal did not reach its last trick
	}
	seven := s.trumpSeven()
	win, _ := tricks.Trick{Plays: s.LastTrick}.Winning(s.trump(), s.order())
	for _, pl := range s.LastTrick {
		if pl.Card == seven {
			return true, s.Players[pl.Seat], pl.Card == win.Card
		}
	}
	return false, "", false
}

// marriageValue is what one announced marriage is worth.
func (s *GameState) marriageValue(suit string) int {
	if s.trump() != tricks.NoTrump && suit == string(s.trump()) {
		return marriageTrump
	}
	return marriagePlain
}

// sideTotals are each side's points in hra: card points and every marriage.
// The defenders' marriages count whether or not they took a trick
// (docs/marias-rules.md, settled question 2), and every marriage counts
// toward a quiet hundred (ČSM general V/7).
func (s *GameState) sideTotals() (declarer, defenders int) {
	for _, p := range s.active() {
		n := s.Points[p]
		for _, suit := range s.Marriages[p] {
			n += s.marriageValue(suit)
		}
		if p == s.Declarer {
			declarer += n
		} else {
			defenders += n
		}
	}
	return declarer, defenders
}

// sideMarriages is everything one side announced in marriages.
func (s *GameState) sideMarriages(declarerSide bool) int {
	n := 0
	for _, m := range s.Announced {
		if (m.Player == s.Declarer) == declarerSide {
			n += s.marriageValue(m.Suit)
		}
	}
	return n
}

// stoCount is a side's count toward an announced hundred: its card points
// and the first marriage it announced, and no other (ČSM).
func (s *GameState) stoCount(declarerSide bool) int {
	n := 0
	for _, p := range s.active() {
		if (p == s.Declarer) == declarerSide {
			n += s.Points[p]
		}
	}
	for _, m := range s.Announced {
		if (m.Player == s.Declarer) == declarerSide {
			return n + s.marriageValue(m.Suit)
		}
	}
	return n
}
