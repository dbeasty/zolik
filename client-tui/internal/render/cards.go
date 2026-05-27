package render

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderCard returns a 5-line string for a single card.
func RenderCard(card string, highlighted, faceDown bool) string {
	if faceDown {
		return RenderCardBack()
	}
	if strings.HasPrefix(card, "JOKER") {
		return renderJoker(highlighted)
	}
	rank := displayRank(card)
	suit := cardSuit(card)
	sym := suitSymbol[suit]
	suitStyled := colorSuit(sym, suit)

	top := padRankLeft(rank, 3)
	mid := centerSuit(suitStyled, rank)
	bot := padRankRight(rank, 3)

	lines := []string{
		"│ " + top + " │",
		"│ " + mid + " │",
		"│ " + bot + " │",
	}
	inner := strings.Join(lines, "\n")
	style := CardNormal
	if highlighted {
		style = CardSelected
	}
	return style.Render(inner)
}

func renderJoker(highlighted bool) string {
	lines := []string{
		"│ ★   │",
		"│ JKR │",
		"│   ★ │",
	}
	style := CardNormal
	if highlighted {
		style = CardSelected
	}
	return style.Render(strings.Join(lines, "\n"))
}

func RenderCardBack() string {
	lines := []string{
		"│ ░░░ │",
		"│ ░░░ │",
		"│ ░░░ │",
	}
	return CardBack.Render(strings.Join(lines, "\n"))
}

func RenderHand(cards []string, selected []int) string {
	return renderHandRow(cards, selected, false, 0)
}

func RenderHandWithNumbers(cards []string, selected []int) string {
	return renderHandRow(cards, selected, true, 0)
}

func RenderHandCompact(cards []string, selected []int) string {
	var b strings.Builder
	for i, c := range cards {
		if i > 0 {
			b.WriteString(" ")
		}
		tok := compactToken(c)
		for _, si := range selected {
			if si == i {
				tok = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFD700")).Render(tok)
				break
			}
		}
		b.WriteString(tok)
	}
	return b.String()
}

func RenderMeld(cards []string, meldID, ownerName string) string {
	label := SectionLabel.Render("[" + ownerName + "] " + meldID)
	row := RenderHand(cards, nil)
	return label + "\n" + row
}

func renderHandRow(cards []string, selected []int, numbers bool, width int) string {
	if width > 0 && width < 80 {
		return RenderHandCompact(cards, selected)
	}
	sel := map[int]bool{}
	for _, i := range selected {
		sel[i] = true
	}
	var parts []string
	for i, c := range cards {
		parts = append(parts, RenderCard(c, sel[i], false))
	}
	row := joinCardsHoriz(parts)
	if !numbers {
		return row
	}
	labels := numberLabels(len(cards))
	return labels + "\n" + row
}

func joinCardsHoriz(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	lines := strings.Split(parts[0], "\n")
	height := len(lines)
	grid := make([][]string, height)
	for i := range grid {
		grid[i] = []string{lines[i]}
	}
	for _, p := range parts[1:] {
		pl := strings.Split(p, "\n")
		for i := 0; i < height && i < len(pl); i++ {
			grid[i] = append(grid[i], " "+pl[i])
		}
	}
	var out strings.Builder
	for i, row := range grid {
		if i > 0 {
			out.WriteByte('\n')
		}
		out.WriteString(strings.Join(row, ""))
	}
	return out.String()
}

func numberLabels(n int) string {
	var parts []string
	for i := 0; i < n; i++ {
		num := i + 1
		label := "  " + itoa(num) + "  "
		parts = append(parts, label)
	}
	return strings.Join(parts, " ")
}

func displayRank(card string) string {
	if strings.HasPrefix(card, "JOKER") {
		return "JKR"
	}
	if len(card) < 1 {
		return "?"
	}
	if card[0] == 'T' {
		return "10"
	}
	return string(card[0])
}

func cardSuit(card string) byte {
	if len(card) < 2 {
		return 'S'
	}
	if card[0] == 'T' {
		return card[1]
	}
	return card[len(card)-1]
}

func colorSuit(sym string, suit byte) string {
	if suit == 'H' || suit == 'D' {
		return RedSuit.Render(sym)
	}
	return BlackSuit.Render(sym)
}

func centerSuit(symStyled, rank string) string {
	if rank == "J" || rank == "Q" || rank == "K" {
		return centerText(rank, 3)
	}
	return centerText(symStyled, 3)
}

func padRankLeft(rank string, w int) string {
	if len(rank) >= w {
		return rank[:w]
	}
	return rank + strings.Repeat(" ", w-len(rank))
}

func padRankRight(rank string, w int) string {
	if len(rank) >= w {
		return rank[:w]
	}
	return strings.Repeat(" ", w-len(rank)) + rank
}

func centerText(s string, w int) string {
	// lipgloss width ignores ANSI; approximate visible width for single-char suits
	vis := visibleLen(s)
	if vis >= w {
		return s
	}
	pad := (w - vis) / 2
	return strings.Repeat(" ", pad) + s + strings.Repeat(" ", w-vis-pad)
}

func visibleLen(s string) int {
	// crude: non-ANSI rune count
	n := 0
	in := false
	for _, r := range s {
		if r == '\x1b' {
			in = true
			continue
		}
		if in {
			if r == 'm' {
				in = false
			}
			continue
		}
		n++
	}
	if n == 0 {
		return len(s)
	}
	return n
}

func compactToken(card string) string {
	if strings.HasPrefix(card, "JOKER") {
		return "[JOKER]"
	}
	r := displayRank(card)
	s := cardSuit(card)
	sym := suitSymbol[s]
	tok := "[" + r + sym + "]"
	if s == 'H' || s == 'D' {
		return RedSuit.Render(tok)
	}
	return BlackSuit.Render(tok)
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
