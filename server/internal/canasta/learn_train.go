package canasta

import (
	"encoding/json"
	"fmt"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
)

// The training-only views of a position (learn.Privileged, learn.MatchScored).
// Nothing here is read by Encode, Candidates or any bot: the privileged vector
// is the critic's alone, and the match score feeds only the trainer's reward.

var (
	_ learn.Privileged  = learnGame{}
	_ learn.MatchScored = learnGame{}
)

// The privileged vector: what the seat cannot see.
//
//	[0,75)    the other seats in the order they play after me (othersFrom),
//	          fifteen each: their actual hands, count per card category (/4);
//	          an absent seat is zero
//	75        stock size (/100)
//	[76,91)   the stock's composition, count per card category (/8)
//	[91,121)  the next two cards to be drawn, category one-hot each; zero past
//	          the end of the stock
const (
	privOthers = 0
	privStock  = privOthers + maxOthers*numCats
	privComp   = privStock + 1
	privNext   = privComp + numCats
	privDim    = privNext + 2*numCats
)

func (learnGame) PrivDim() int { return privDim }

// PrivilegedFor is seat's privileged vector at p.
func (learnGame) PrivilegedFor(at learn.Position, seat string) ([]float32, error) {
	p, err := positionOf(at)
	if err != nil {
		return nil, err
	}
	s, err := p.state()
	if err != nil {
		return nil, err
	}
	v := make([]float32, privDim)
	if s.Break.Open || s.Status != "active" {
		return v, nil
	}
	for i, id := range othersFrom(s, seat) {
		for _, c := range s.Hands[id] {
			if k := cardCat(c); k >= 0 {
				v[privOthers+i*numCats+k] += 0.25
			}
		}
	}
	v[privStock] = float32(len(s.DrawPile)) / 100
	for _, c := range s.DrawPile {
		if k := cardCat(c); k >= 0 {
			v[privComp+k] += 1.0 / 8
		}
	}
	for j := 0; j < 2 && j < len(s.DrawPile); j++ {
		if k := cardCat(s.DrawPile[len(s.DrawPile)-1-j]); k >= 0 {
			v[privNext+j*numCats+k] = 1
		}
	}
	return v, nil
}

// matchOnly is the part of a state the match score reads.
type matchOnly struct {
	Status      string         `json:"status"`
	WinnerTeam  int            `json:"winnerTeam"`
	TargetScore int            `json:"targetScore"`
	TeamOf      map[string]int `json:"teamOf"`
	Teams       []struct {
		ID int `json:"id"`
	} `json:"teams"`
	Deals []DealResult `json:"deals"`
}

func decodeMatch(raw module.State) (*matchOnly, error) {
	var m matchOnly
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("canasta: decode state: %w", err)
	}
	return &m, nil
}

// scoreAfter is the match as seat's side stood after the first n deals.
func (m *matchOnly) scoreAfter(n int, seat string) (learn.MatchScore, error) {
	mine, ok := m.TeamOf[seat]
	if !ok {
		return learn.MatchScore{}, fmt.Errorf("canasta: no team for %q", seat)
	}
	out := learn.MatchScore{Target: m.TargetScore, Seats: len(m.TeamOf), Sides: len(m.Teams), Deal: n}
	if n > 0 {
		found := false
		for _, tr := range m.Deals[n-1].Teams {
			if tr.TeamID == mine {
				out.Own = tr.Running
				continue
			}
			if !found || tr.Running > out.Best {
				out.Best, found = tr.Running, true
			}
		}
	}
	if n == len(m.Deals) && m.Status == "completed" {
		out.Over = true
		if m.WinnerTeam == mine {
			out.Won = 1
		}
	}
	return out, nil
}

// MatchScoreFor is the match as seat's side stands at p.
func (learnGame) MatchScoreFor(at learn.Position, seat string) (learn.MatchScore, error) {
	p, err := positionOf(at)
	if err != nil {
		return learn.MatchScore{}, err
	}
	m, err := p.match.Get(func() (*matchOnly, error) { return decodeMatch(p.raw) })
	if err != nil {
		return learn.MatchScore{}, err
	}
	return m.scoreAfter(len(m.Deals), seat)
}

// MatchHistory is the score before every deal of the match, then after the
// last.
func (learnGame) MatchHistory(final module.State, seat string) ([]learn.MatchScore, error) {
	m, err := decodeMatch(final)
	if err != nil {
		return nil, err
	}
	out := make([]learn.MatchScore, 0, len(m.Deals)+1)
	for n := 0; n <= len(m.Deals); n++ {
		s, err := m.scoreAfter(n, seat)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

var _ learn.Appending = learnGame{}

// EncoderPrefixes are the earlier encoders this one appended to: 383/40,
// before the card inference blocks (the canasta-v2 and -v3 models).
func (learnGame) EncoderPrefixes() [][2]int { return [][2]int{{offInfer, fCapture}} }
