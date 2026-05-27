package ui

import (
	"fmt"
	"strings"
)

func roundRequirementLabel(round int) string {
	switch round {
	case 1:
		return "Two Sets of 3"
	case 2:
		return "One Set of 3, One Run of 4"
	case 3:
		return "Two Runs of 4"
	case 4:
		return "Three Sets of 3"
	case 5:
		return "Two Sets of 3, One Run of 4"
	case 6:
		return "One Set of 3, Two Runs of 4"
	case 7:
		return "Three Runs of 4"
	default:
		return ""
	}
}

func selectedCards(hand []string, sel map[int]bool) []string {
	var out []string
	for i, c := range hand {
		if sel[i] {
			out = append(out, c)
		}
	}
	return out
}

func selectedIndexes(sel map[int]bool) []int {
	var out []int
	for i, on := range sel {
		if on {
			out = append(out, i)
		}
	}
	return out
}

func playerName(players []struct {
	ID   string
	Name string
}, id string) string {
	for _, p := range players {
		if p.ID == id {
			return p.Name
		}
	}
	return id
}

func approximateNaturalValue(cards []string) int {
	total := 0
	for _, c := range cards {
		if strings.HasPrefix(c, "JOKER") {
			continue
		}
		if len(c) < 1 {
			continue
		}
		r := c[0]
		switch r {
		case 'A':
			total += 1
		case 'K', 'Q', 'J', 'T':
			total += 10
		case '2', '3', '4', '5', '6', '7', '8', '9':
			total += int(r - '0')
		}
	}
	return total
}

func formatScores(totals map[string]int, players []playerRef) string {
	var parts []string
	for _, p := range players {
		sc := totals[p.ID]
		parts = append(parts, fmt.Sprintf("%s: %d", p.Name, sc))
	}
	return strings.Join(parts, "  │  ")
}

type playerRef struct {
	ID   string
	Name string
	IsAI bool
}
