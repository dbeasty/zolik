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
		Declarer: s.Declarer,
		Game:     s.Game,
		Deltas:   map[string]int{},
		Totals:   map[string]int{},
	}
	if t := s.trump(); t != tricks.NoTrump {
		rec.Trump = string(t)
	}
	rec.Parts = s.results()

	for _, pr := range rec.Parts {
		for _, p := range s.Players {
			if p == s.Declarer {
				continue
			}
			// Each defender settles with the declarer on their own.
			if pr.ForDeclarer {
				rec.Deltas[s.Declarer] += pr.Units
				rec.Deltas[p] -= pr.Units
			} else {
				rec.Deltas[s.Declarer] -= pr.Units
				rec.Deltas[p] += pr.Units
			}
		}
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
	case gameBetl:
		won := s.TricksWon[s.Declarer] == 0
		out = append(out, PartResult{Part: partGame, ForDeclarer: won, Units: t.Betl * multiplier(s.Fleks[partGame], false)})
		return out
	case gameDurch:
		won := !s.decidedEarly()
		out = append(out, PartResult{Part: partGame, ForDeclarer: won, Units: t.Durch * multiplier(s.Fleks[partGame], false)})
		return out
	}

	decl, def := s.sideTotals()
	declHundred, defHundred := s.hundredCount(true), s.hundredCount(false)

	if s.Game == gameSto {
		won := declHundred >= hundred
		out = append(out, PartResult{Part: partGame, ForDeclarer: won, Units: t.stoValue(won, declHundred) * mult(partGame)})
	} else {
		// Hra: more than the defenders together; a tie is theirs. A side
		// that reached a hundred without announcing it doubles the game —
		// but not the defenders when they announced sto proti, which is
		// settled as a part of its own.
		won := decl > def
		count := declHundred
		if !won {
			count = defHundred
			if s.ProtiSto != "" {
				count = 0
			}
		}
		out = append(out, PartResult{Part: partGame, ForDeclarer: won, Units: t.hraValue(count) * mult(partGame)})
	}

	if s.ProtiSto != "" {
		won := defHundred >= hundred
		out = append(out, PartResult{Part: partProtiSto, ForDeclarer: !won, Units: t.stoValue(won, defHundred) * mult(partProtiSto)})
	}

	seven, by, took := s.lastTrickSeven()
	switch {
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

// lastTrickSeven reports whether the trump seven was played to the last
// trick, by whom, and whether it took the trick.
func (s *GameState) lastTrickSeven() (played bool, by string, took bool) {
	if len(s.Hands[s.Players[0]]) != 0 || len(s.LastTrick) == 0 {
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
// (docs/marias-rules.md, settled question 2).
func (s *GameState) sideTotals() (declarer, defenders int) {
	for _, p := range s.Players {
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

// hundredCount is a side's count toward a hundred: its card points and its
// single best marriage (ČSM A; Pagat).
func (s *GameState) hundredCount(declarerSide bool) int {
	n, best := 0, 0
	for _, p := range s.Players {
		if (p == s.Declarer) != declarerSide {
			continue
		}
		n += s.Points[p]
		for _, suit := range s.Marriages[p] {
			if v := s.marriageValue(suit); v > best {
				best = v
			}
		}
	}
	return n + best
}
