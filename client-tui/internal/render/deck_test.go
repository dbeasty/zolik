package render

import (
	"strings"
	"testing"
)

// On a German-suited table the spodek and svršek read as U and O, and
// nothing else about a card changes shape.
func TestGermanDeckRenamesTheLowerCourts(t *testing.T) {
	SetDeck("german")
	defer SetDeck("")
	if got := CardToken("QS"); !strings.Contains(got, "[O♠]") {
		t.Errorf("svršek of leaves = %q", got)
	}
	if got := CardToken("JD"); !strings.Contains(got, "[U♦]") {
		t.Errorf("spodek of bells = %q", got)
	}
	SetDeck("")
	if got := CardToken("QS"); !strings.Contains(got, "[Q♠]") {
		t.Errorf("French queen of spades = %q", got)
	}
}
