package game

import (
	"testing"

	"zolik/server/internal/models"
)

func TestBuildGameStateMsg_OfferRedactionAndHands(t *testing.T) {
	g := models.Game{
		Status:         "active",
		Round:          1,
		Phase:          "draw",
		CurrentTurn:    "p1",
		TurnOrder:      []string{"p1", "p2"},
		DiscardPile:    []string{"7H"},
		ReshuffleCount: 2,
		InitialMeldMinimum: 35,
		Melds:          map[string][][]string{},
		RoundReqMet:    map[string]bool{"p1": false, "p2": false},
		TotalScores:    map[string]int{"p1": 10, "p2": 5},
		Players: []models.Player{
			{ID: "p1", Name: "A"},
			{ID: "p2", Name: "B"},
		},
		Hands: map[string][]string{
			"p1": {"2H", "3H"},
			"p2": {"4H"},
		},
		Offer: &models.DiscardOffer{Card: "KH", OfferedTo: "p2"},
	}

	msg1 := BuildGameStateMsg(g, "p1")
	if msg1.Offer != nil {
		t.Fatalf("expected offer redacted for non-offeree")
	}
	if len(msg1.MyHand) != 2 {
		t.Fatalf("expected my hand size 2, got %d", len(msg1.MyHand))
	}
	if msg1.CardCounts["p2"] != 1 {
		t.Fatalf("expected p2 count 1, got %d", msg1.CardCounts["p2"])
	}

	msg2 := BuildGameStateMsg(g, "p2")
	if msg2.Offer == nil || msg2.Offer.Card != "KH" {
		t.Fatalf("expected offer present for offeree")
	}
	if len(msg2.MyHand) != 1 {
		t.Fatalf("expected my hand size 1, got %d", len(msg2.MyHand))
	}
	if msg2.CardCounts["p1"] != 2 {
		t.Fatalf("expected p1 count 2, got %d", msg2.CardCounts["p1"])
	}
	if msg1.Status != "active" {
		t.Fatalf("expected status active, got %q", msg1.Status)
	}
	if len(msg1.Players) != 2 {
		t.Fatalf("expected 2 players in snapshot, got %d", len(msg1.Players))
	}
}

