package zolikmod

import (
	"encoding/json"
	"fmt"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// The training-only views of a position (learn.Privileged, learn.MatchScored,
// learn.Appending). Nothing here is read by Encode, Candidates or any bot: the
// privileged vector is for the trainer's critic and its auxiliary hidden-card
// head, and the match score feeds only the trainer's reward.

var (
	_ learn.Privileged  = learnGame{}
	_ learn.MatchScored = learnGame{}
	_ learn.Appending   = learnGame{}
)

// The privileged vector: what the seat cannot see.
//
//	[0,159)    the other seats in the order they play after me (seatsFrom), 53
//	           each: their actual hands, copies per card slot (/2); an absent
//	           seat is zero. The first block — the next player's hand — is
//	           also the auxiliary head's target (ml/zolik_ml/model.py).
//	159        stock size (/100)
//	[160,213)  the stock's composition, copies per card slot (/2)
//	[213,266)  the next card to be drawn, one-hot; zero on an empty stock
const (
	privOthers = 0
	privStock  = privOthers + (maxSeats-1)*numCardSlot
	privComp   = privStock + 1
	privNext   = privComp + numCardSlot
	privDim    = privNext + numCardSlot
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
	gs := &s.Rules
	if s.Break.Open || gs.Status != rules.StatusActive {
		return v, nil
	}
	order := seatsFrom(gs.TurnOrder, seat)
	for i, id := range order[1:] {
		for _, c := range gs.Hands[id] {
			if k := cardSlot(c); k >= 0 {
				v[privOthers+i*numCardSlot+k] += 0.5
			}
		}
	}
	v[privStock] = float32(len(gs.DrawPile)) / 100
	for _, c := range gs.DrawPile {
		if k := cardSlot(c); k >= 0 {
			v[privComp+k] += 0.5
		}
	}
	if n := len(gs.DrawPile); n > 0 {
		if k := cardSlot(gs.DrawPile[n-1]); k >= 0 {
			v[privNext+k] = 1
		}
	}
	return v, nil
}

// matchOnly is the part of a state the match score reads.
type matchOnly struct {
	Rules struct {
		Status      rules.GameStatus
		TurnOrder   []string
		GameScores  map[string][]int
		WinnerID    string
		IsDraw      bool
		Rules       struct{ TargetScore int }
		TotalScores map[string]int
	} `json:"rules"`
}

func decodeMatch(raw module.State) (*matchOnly, error) {
	var m matchOnly
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("zolik: decode state: %w", err)
	}
	return &m, nil
}

// deals is how many deals the sheet has scored.
func (m *matchOnly) deals() int {
	n := 0
	for _, id := range m.Rules.TurnOrder {
		n = max(n, len(m.Rules.GameScores[id]))
	}
	return n
}

// scoreAfter is the match as seat stood after the first n deals. Žolíky is
// scored in penalty points, so Own and Best are penalty totals and lower is
// better: Best is the lowest of the other seats', and Top the highest at the
// table — the total that ends the match when it reaches the target. The win
// model learns which way they point.
func (m *matchOnly) scoreAfter(n int, seat string) (learn.MatchScore, error) {
	r := &m.Rules
	if !contains(r.TurnOrder, seat) {
		return learn.MatchScore{}, fmt.Errorf("zolik: no seat %q", seat)
	}
	target := r.Rules.TargetScore
	if target <= 0 {
		target = rules.ProfileZolikClassic.TargetScore
	}
	out := learn.MatchScore{Target: target, Seats: len(r.TurnOrder), Sides: len(r.TurnOrder), Deal: n}
	total := func(id string) int {
		t := 0
		for i, p := range r.GameScores[id] {
			if i >= n {
				break
			}
			t += p
		}
		return t
	}
	found := false
	for _, id := range r.TurnOrder {
		t := total(id)
		out.Top = max(out.Top, t)
		if id == seat {
			out.Own = t
			continue
		}
		if !found || t < out.Best {
			out.Best, found = t, true
		}
	}
	if n == m.deals() && r.Status == rules.StatusCompleted {
		out.Over = true
		switch {
		case r.WinnerID == seat:
			out.Won = 1
		case r.IsDraw || r.WinnerID == "":
			// A draw is shared by the tied seats; anyone not tied for the
			// lowest total lost.
			if out.Own <= out.Best {
				out.Won = 0.5
			}
		}
	}
	return out, nil
}

// MatchScoreFor is the match as seat stands at p.
func (learnGame) MatchScoreFor(at learn.Position, seat string) (learn.MatchScore, error) {
	p, err := positionOf(at)
	if err != nil {
		return learn.MatchScore{}, err
	}
	m, err := p.match.Get(func() (*matchOnly, error) { return decodeMatch(p.raw) })
	if err != nil {
		return learn.MatchScore{}, err
	}
	return m.scoreAfter(m.deals(), seat)
}

// MatchHistory is the score before every deal of the match, then after the
// last.
func (learnGame) MatchHistory(final module.State, seat string) ([]learn.MatchScore, error) {
	m, err := decodeMatch(final)
	if err != nil {
		return nil, err
	}
	n := m.deals()
	out := make([]learn.MatchScore, 0, n+1)
	for i := 0; i <= n; i++ {
		s, err := m.scoreAfter(i, seat)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// EncoderPrefixes lives in learn.go: the 900/62 v2 prefix the shipped model reads.
