package klondike

import "strconv"

// Zone and group ids. The view draws them and the offers name them, so they
// live in one place.
const (
	zoneStock       = "stock"
	zoneWaste       = "waste"
	zoneFoundations = "foundations"
	zoneTableau     = "tableau"
)

func colID(i int) string   { return "t" + strconv.Itoa(i+1) }
func foundID(i int) string { return "f-" + suits[i] }

// colIndex parses "t3" to 2, or -1.
func colIndex(id string) int {
	if len(id) < 2 || id[0] != 't' {
		return -1
	}
	n, err := strconv.Atoi(id[1:])
	if err != nil || n < 1 || n > numColumns {
		return -1
	}
	return n - 1
}

// foundIndex parses "f-H" to 1, or -1.
func foundIndex(id string) int {
	if len(id) != 3 || id[:2] != "f-" {
		return -1
	}
	return suitIndex(id[2:])
}
