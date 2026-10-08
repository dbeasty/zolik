package render

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Last Card's own pack: codes "<colour>-<face>" and the two wilds "W" and
// "W4". Drawn in the card's colour, with the colour's shape beside the face —
// the shape carries the colour for a terminal, or a player, without it.

var lastCardDeck bool

type lastCardFace struct {
	face  string // "7", "SK", "RV", "+2", "W", "+4"
	shape string // ● ◆ ▲ ■, or ✦ for a wild
	style lipgloss.Style
}

var lastCardColours = map[string]struct {
	shape string
	hex   string
}{
	"C": {"●", "#E8603C"}, // coral
	"T": {"◆", "#14A193"}, // teal
	"V": {"▲", "#7A5BD8"}, // violet
	"A": {"■", "#E5A019"}, // amber
}

var lastCardActions = map[string]string{"S": "SK", "R": "RV", "D": "+2"}

// wildStyle is the wilds' own: bright on the terminal's background, since a
// wild is every colour and none.
var wildStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F4EBD0"))

// parseLastCard reads a Last Card code, only at a Last Card table: "W" is a
// card there and nothing anywhere else.
func parseLastCard(card string) (lastCardFace, bool) {
	if !lastCardDeck {
		return lastCardFace{}, false
	}
	switch card {
	case "W":
		return lastCardFace{face: "W", shape: "✦", style: wildStyle}, true
	case "W4":
		return lastCardFace{face: "+4", shape: "✦", style: wildStyle}, true
	}
	colour, face, ok := strings.Cut(card, "-")
	c, known := lastCardColours[colour]
	if !ok || !known {
		return lastCardFace{}, false
	}
	if a, action := lastCardActions[face]; action {
		face = a
	} else if len(face) != 1 || face[0] < '0' || face[0] > '9' {
		return lastCardFace{}, false
	}
	return lastCardFace{face: face, shape: c.shape, style: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(c.hex))}, true
}

// renderLastCard draws one card in the same three-row frame as every other.
func renderLastCard(lc lastCardFace, highlighted bool) string {
	lines := []string{
		"│ " + lc.style.Render(padRankLeft(lc.face, 3)) + " │",
		"│ " + centerText(lc.style.Render(lc.shape), 3) + " │",
		"│ " + lc.style.Render(padRankRight(lc.face, 3)) + " │",
	}
	style := CardNormal
	if highlighted {
		style = CardSelected
	}
	return style.Render(strings.Join(lines, "\n"))
}
