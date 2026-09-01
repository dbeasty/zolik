// Package rummytiles implements the tile-rearrangement rummy Phase B of
// docs/rummy-games-plan.md cites as "Rummy Tiles" — the trademarked name
// appears nowhere a player, URL, message key or database document can see.
//
// It is the first module whose whole character is not "cards leave a hand
// and land somewhere" but "the table is rearranged, and only the end state
// has to be legal." See state.go's package doc for the workspace design that
// makes that turn shape work within the same offer/Apply contract every
// other module already uses.
package rummytiles

import (
	"math/rand"
	"strconv"
)

// Tile notation: "<number>-<colour>" — 1 to 13, colour one of R/B/O/K (red,
// blue, orange, black) — or "JOKER1"/"JOKER2", the shared spelling every
// joker-bearing module in this server already uses. The hyphen is deliberate:
// it is what keeps a tile from ever being mistaken for a two-character card
// code, the same way isCardCode's strict length check works the other
// direction on the client.
var colours = []string{"R", "B", "O", "K"}

func isJoker(tile string) bool {
	return tile == "JOKER1" || tile == "JOKER2"
}

func tileCode(number int, colour string) string {
	return strconv.Itoa(number) + "-" + colour
}

// parseTile splits a real (non-joker) tile into its number and colour. Called
// only where isJoker has already been ruled out.
func parseTile(tile string) (number int, colour string) {
	for i := len(tile) - 1; i >= 0; i-- {
		if tile[i] == '-' {
			n, _ := strconv.Atoi(tile[:i])
			return n, tile[i+1:]
		}
	}
	return 0, ""
}

func colourOf(tile string) string {
	_, c := parseTile(tile)
	return c
}

func numberOf(tile string) int {
	n, _ := parseTile(tile)
	return n
}

// buildTiles returns the 106-tile set: two of each number 1-13 in each of
// four colours, plus two jokers.
func buildTiles() []string {
	out := make([]string, 0, 106)
	for _, c := range colours {
		for n := 1; n <= 13; n++ {
			t := tileCode(n, c)
			out = append(out, t, t)
		}
	}
	out = append(out, "JOKER1", "JOKER2")
	return out
}

func shuffle(tiles []string, seed int64) []string {
	out := append([]string(nil), tiles...)
	r := rand.New(rand.NewSource(seed))
	r.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

// tileValue is a tile's face value: its number, or 30 for a joker — the
// scoring value the rules assign a joker left in hand at round end. A joker
// resolved *in a set* counts as the tile it stands for instead; see
// setValueOf in sets.go for that case.
func tileValue(tile string) int {
	if isJoker(tile) {
		return 30
	}
	n, _ := parseTile(tile)
	return n
}

func handValue(tiles []string) int {
	total := 0
	for _, t := range tiles {
		total += tileValue(t)
	}
	return total
}

// removeTile takes one copy of a tile out of a hand — one copy, not every
// matching one, since every number exists twice and a player may hold two
// identical tiles and mean only one of them.
func removeTile(hand []string, tile string) ([]string, bool) {
	out := append([]string(nil), hand...)
	for i, t := range out {
		if t == tile {
			return append(out[:i:i], out[i+1:]...), true
		}
	}
	return hand, false
}

// removeTiles takes one copy of each named tile out of hand, atomically: if
// any one of them is not there, hand comes back unchanged.
func removeTiles(hand []string, tiles []string) ([]string, bool) {
	out := hand
	for _, t := range tiles {
		next, ok := removeTile(out, t)
		if !ok {
			return hand, false
		}
		out = next
	}
	return out, true
}

func hasTile(hand []string, tile string) bool {
	for _, t := range hand {
		if t == tile {
			return true
		}
	}
	return false
}

func hasTiles(hand []string, tiles []string) bool {
	_, ok := removeTiles(hand, tiles)
	return ok
}

func sortedUnique(tiles []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(tiles))
	for _, t := range tiles {
		if seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}
