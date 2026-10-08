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

// At a Last Card table its codes are drawn as its own cards — face and shape —
// and nowhere else is "W" anything but a word.
func TestLastCardDeckDrawsItsOwnCards(t *testing.T) {
	SetDeck("lastcard")
	defer SetDeck("")
	for code, want := range map[string]string{"C-7": "[7●]", "T-S": "[SK◆]", "V-D": "[+2▲]", "A-R": "[RV■]", "W": "[W✦]", "W4": "[+4✦]"} {
		if got := CardToken(code); !strings.Contains(got, want) {
			t.Errorf("%s = %q, want %s", code, got, want)
		}
	}
	card := RenderCard("C-7", false, false)
	if !strings.Contains(card, "7") || !strings.Contains(card, "●") {
		t.Errorf("drawn coral 7 = %q", card)
	}
	SetDeck("")
	if _, ok := parseLastCard("W"); ok {
		t.Error("W read as a card away from a Last Card table")
	}
}
