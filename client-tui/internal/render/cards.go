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

// germanDeck is whether the match on screen is dealt from the German-suited
// pack (the server's MatchState.deck). Package state rather than a parameter
// because every card on the screen belongs to the one match being shown, and
// threading it through each render call would teach all of them about decks.
var germanDeck bool

// SetDeck says which pack the cards being drawn belong to: "german", or
// anything else for the French one.
func SetDeck(deck string) { germanDeck = deck == "german" }

// germanColors are the German pack's own suit colours. The symbols stay the
// French ones, which is where those came from — spades are leaves, clubs
// acorns, diamonds bells — and are all one cell wide, which an emoji bell is
// not.
var germanColors = map[byte]lipgloss.Style{
	'H': lipgloss.NewStyle().Foreground(lipgloss.Color("#E05252")),
	'D': lipgloss.NewStyle().Foreground(lipgloss.Color("#E0A526")),
	'C': lipgloss.NewStyle().Foreground(lipgloss.Color("#B07A45")),
	'S': lipgloss.NewStyle().Foreground(lipgloss.Color("#4CB043")),
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
	if germanDeck {
		// The spodek and svršek are the Unter and the Ober.
		switch card[0] {
		case 'J':
			return "U"
		case 'Q':
			return "O"
		}
	}
	return string(card[0])
}

func suitStyle(suit byte) lipgloss.Style {
	if germanDeck {
		if st, ok := germanColors[suit]; ok {
			return st
		}
	}
	if suit == 'H' || suit == 'D' {
		return RedSuit
	}
	return BlackSuit
}

// CardToken is a card as a short coloured token, "[O♠]", for a line of text
// rather than a drawn card.
func CardToken(card string) string { return compactToken(card) }

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
	return suitStyle(suit).Render(sym)
}

func centerSuit(symStyled, rank string) string {
	if rank == "J" || rank == "Q" || rank == "K" || rank == "U" || rank == "O" {
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
	return suitStyle(s).Render(tok)
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
