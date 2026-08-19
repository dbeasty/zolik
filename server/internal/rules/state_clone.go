package rules

// cloneState returns a deep copy of game state for dry-run validation.
func cloneState(s GameState) GameState {
	out := s
	out.TurnOrder = append([]string(nil), s.TurnOrder...)
	out.DrawPile = append([]string(nil), s.DrawPile...)
	out.DiscardPile = append([]string(nil), s.DiscardPile...)
	out.Hands = map[string][]string{}
	for k, v := range s.Hands {
		out.Hands[k] = append([]string(nil), v...)
	}
	out.Melds = map[string][][]string{}
	for k, melds := range s.Melds {
		for _, m := range melds {
			out.Melds[k] = append(out.Melds[k], append([]string(nil), m...))
		}
	}
	out.MeldMeta = map[string][]MeldInfo{}
	for k, metas := range s.MeldMeta {
		for _, mi := range metas {
			out.MeldMeta[k] = append(out.MeldMeta[k], mi)
		}
	}
	out.RoundReqMet = map[string]bool{}
	for k, v := range s.RoundReqMet {
		out.RoundReqMet[k] = v
	}
	out.GameScores = map[string][]int{}
	for k, v := range s.GameScores {
		out.GameScores[k] = append([]int(nil), v...)
	}
	out.TotalScores = map[string]int{}
	for k, v := range s.TotalScores {
		out.TotalScores[k] = v
	}
	return out
}
