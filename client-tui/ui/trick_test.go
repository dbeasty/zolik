package ui

import (
	"testing"

	"zolik/client-tui/api"
)

func TestTrickCardsNameTheirPlayers(t *testing.T) {
	players := []api.Player{{ID: "a", Name: "Anna"}, {ID: "b", Name: "Petr"}, {ID: "me", Name: "Me"}}
	got := trickCards([]api.CardView{{Card: "KH", By: "a"}, {Card: "AH", By: "b"}, {Card: "7H", By: "me"}, {Card: "9S"}}, players, "me")
	if want := "Anna KH  Petr AH  you 7H  9S"; got != want {
		t.Errorf("trickCards = %q, want %q", got, want)
	}
}
