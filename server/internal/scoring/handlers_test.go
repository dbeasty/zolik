package scoring

import "testing"

func TestBuildGetResp_WinnerByLowestTotal(t *testing.T) {
	s := ScoringSession{
		Players: []PlayerScore{
			{Name: "A", Scores: []int{10, 0, 0, 0, 0, 0, 0}},
			{Name: "B", Scores: []int{20, 0, 0, 0, 0, 0, 0}},
		},
	}

	resp := buildGetResp(s)
	if resp.Winner == nil || *resp.Winner != "A" {
		t.Fatalf("expected winner A got %#v", resp.Winner)
	}
}

func TestBuildGetResp_TieDrawWhenRoundsWonEqual(t *testing.T) {
	// Both players have equal totals; roundsWon equal -> winner nil.
	s := ScoringSession{
		Players: []PlayerScore{
			{Name: "A", Scores: []int{0, 10, 0, 0, 0, 0, 0}},
			{Name: "B", Scores: []int{10, 0, 0, 0, 0, 0, 0}},
		},
	}
	resp := buildGetResp(s)
	if resp.Winner != nil {
		t.Fatalf("expected draw (nil winner) got %#v", *resp.Winner)
	}
}
