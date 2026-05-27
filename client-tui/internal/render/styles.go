package render

import "github.com/charmbracelet/lipgloss"

var (
	CardNormal = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#555555")).
			Padding(0, 1)

	CardSelected = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("#FFD700")).
			Padding(0, 1)

	CardBack = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#444488")).
			Foreground(lipgloss.Color("#444488")).
			Padding(0, 1)

	RedSuit   = lipgloss.NewStyle().Foreground(lipgloss.Color("#E05252"))
	BlackSuit = lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))

	HeaderBar = lipgloss.NewStyle().
			Background(lipgloss.Color("#1A3A6B")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 2).
			Bold(true)

	SectionLabel = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Underline(true)

	StatusOK   = lipgloss.NewStyle().Foreground(lipgloss.Color("#44CC44"))
	StatusErr  = lipgloss.NewStyle().Foreground(lipgloss.Color("#E05252"))
	StatusInfo = lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))

	AIBadge = lipgloss.NewStyle().
			Background(lipgloss.Color("#334455")).
			Foreground(lipgloss.Color("#88CCFF")).
			Padding(0, 1)

	Box = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#444444"))
)

var suitSymbol = map[byte]string{
	'H': "♥",
	'D': "♦",
	'S': "♠",
	'C': "♣",
}
