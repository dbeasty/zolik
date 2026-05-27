package render

// UseCompact returns true when terminal width is below 80 columns.
func UseCompact(width int) bool {
	return width > 0 && width < 80
}
