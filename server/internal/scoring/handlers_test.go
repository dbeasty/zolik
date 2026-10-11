package scoring

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
)

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

// memRepo is the repository with nothing behind it.
type memRepo struct {
	s map[bson.ObjectID]ScoringSession
}

func (m *memRepo) Insert(_ context.Context, s ScoringSession) error {
	m.s[s.ID] = s
	return nil
}
func (m *memRepo) FindByID(_ context.Context, id bson.ObjectID) (ScoringSession, error) {
	s, ok := m.s[id]
	if !ok {
		return ScoringSession{}, errors.New("not found")
	}
	return s, nil
}
func (m *memRepo) Replace(_ context.Context, s ScoringSession) error {
	m.s[s.ID] = s
	return nil
}

func scorepad(t *testing.T) (http.Handler, *memRepo) {
	t.Helper()
	repo := &memRepo{s: map[bson.ObjectID]ScoringSession{}}
	r := chi.NewRouter()
	NewHandlers(repo).RegisterRoutes(r)
	return r, repo
}

func call(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}

// A scorepad is an unauthenticated write: the one thing that bounds what it
// stores is this file.
func TestCreateCleansAndBoundsPlayerNames(t *testing.T) {
	h, repo := scorepad(t)
	long := strings.Repeat("x", 10_000)
	body, _ := json.Marshal(map[string]any{"players": []string{"  Ann\u202e ", "   ", long}})
	rec := call(h, http.MethodPost, "/scoring-sessions", string(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if len(repo.s) != 1 {
		t.Fatalf("stored %d sessions, want 1", len(repo.s))
	}
	for _, s := range repo.s {
		got := []string{s.Players[0].Name, s.Players[1].Name, s.Players[2].Name}
		want := []string{"Ann", "Player 2", strings.Repeat("x", 24)}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("player %d = %q, want %q", i+1, got[i], want[i])
			}
		}
	}
}

func TestCreateRefusesABodyBiggerThanAScorepadCouldBe(t *testing.T) {
	h, repo := scorepad(t)
	body, _ := json.Marshal(map[string]any{"players": []string{"A", "B"}, "pad": strings.Repeat("y", 2<<20)})
	rec := call(h, http.MethodPost, "/scoring-sessions", string(body))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
	if len(repo.s) != 0 {
		t.Fatal("an oversized request was stored")
	}
}

func TestAScoreCannotOverflowAWinnerAway(t *testing.T) {
	h, repo := scorepad(t)
	rec := call(h, http.MethodPost, "/scoring-sessions", `{"players":["A","B"]}`)
	var created struct {
		SessionID string `json:"sessionId"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	// Two rounds of the largest int wrap a total to -2, making the player who
	// scored most the winner of a lowest-wins game.
	huge := strconv.Itoa(math.MaxInt64)
	for round := 1; round <= 2; round++ {
		call(h, http.MethodPatch, "/scoring-sessions/"+created.SessionID,
			`{"round":`+strconv.Itoa(round)+`,"scoresArr":[`+huge+`,5]}`)
	}
	for _, s := range repo.s {
		resp := buildGetResp(s)
		if resp.Winner == nil || *resp.Winner != "B" {
			t.Fatalf("winner = %v, want B (A scored the most)", resp.Winner)
		}
		if resp.Players[0].Total < 0 {
			t.Fatalf("A's total wrapped negative: %d", resp.Players[0].Total)
		}
	}
}

func TestScoresByNameStillFindAPlayerWhoseNameWasCleaned(t *testing.T) {
	h, repo := scorepad(t)
	rec := call(h, http.MethodPost, "/scoring-sessions", `{"players":["Ann ","Bob"]}`)
	var created struct {
		SessionID string `json:"sessionId"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	// The client still calls her "Ann " — the name it typed.
	call(h, http.MethodPatch, "/scoring-sessions/"+created.SessionID, `{"round":1,"scores":{"Ann ":7,"Bob":3}}`)
	for _, s := range repo.s {
		if s.Players[0].Name != "Ann" || s.Players[0].Scores[0] != 7 || s.Players[1].Scores[0] != 3 {
			t.Fatalf("scores did not land: %+v", s.Players)
		}
	}
}
