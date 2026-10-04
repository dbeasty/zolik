package learn

import (
	"encoding/json"
	"fmt"
	"math"

	"zolik/server/internal/module"
)

// Training-only extras the environment offers a trainer that asks for them:
// a privileged observation for a perfect-information critic, and a reward
// that prices a deal by what it did to the chance of winning the match.
// Neither reaches a bot: NetBot, the bench and the server read Encode and
// Candidates only, so a network trained with them plays exactly as one
// trained without.

// Privileged is implemented by a game that can describe what a seat cannot
// see — the other hands, the stock — as a fixed-length vector. Optional.
//
// It exists for the critic alone (PerfectDou's perfect-training,
// imperfect-execution): the trainer feeds it to the value function it uses
// for advantages, never to the policy, and the exported model carries no
// weight that reads it. A game must never call it from Encode.
type Privileged interface {
	PrivDim() int
	PrivilegedFor(p Position, seat string) ([]float32, error)
}

// MatchScore is a match to a target score as one seat's side stands, between
// episodes (deals).
type MatchScore struct {
	Own    int `json:"own"`    // this side's running score
	Best   int `json:"best"`   // the best other side's running score
	Target int `json:"target"` // the score that ends the match
	Seats  int `json:"seats"`
	Sides  int `json:"sides"`
	Deal   int `json:"deal"` // deals already scored
	// Top is the highest running score at the table, for a game whose match
	// ends when any one seat reaches the target and whose best side is not
	// the one that gets there (Žolíky: penalty points, lowest wins). Zero
	// leaves the closeness to the end as max(Own, Best), which is what it is
	// when the best side is the one racing to the target (Canasta).
	Top int `json:"top,omitempty"`
	// Over is a finished match, and Won its result for this side: 1 a win,
	// 0 a loss.
	Over bool    `json:"over,omitempty"`
	Won  float64 `json:"won,omitempty"`
}

// MatchScored is implemented by a game whose matches run over several
// episodes to a target. Optional; the match reward needs it.
type MatchScored interface {
	// MatchScoreFor is the match as seat's side stands at p, counting only
	// the episodes scored by then.
	MatchScoreFor(p Position, seat string) (MatchScore, error)
	// MatchHistory is the match as seat's side stood before each deal of a
	// finished (or abandoned) match, then after the last one: what a
	// win-probability model is fitted on.
	MatchHistory(final module.State, seat string) ([]MatchScore, error)
}

// WinFeatures is what WinModel reads: the score state from the side's point
// of view, scaled by the target. ml/winmodel.py fits on vectors cmd/matchstates
// writes with this same function, so the two cannot drift.
func WinFeatures(m MatchScore) []float64 {
	t := float64(m.Target)
	if t <= 0 {
		t = 1
	}
	clip := func(x float64) float64 { return math.Max(-1, math.Min(1.5, x)) }
	own, best := clip(float64(m.Own)/t), clip(float64(m.Best)/t)
	end := math.Max(own, best)
	if m.Top != 0 {
		end = math.Max(end, clip(float64(m.Top)/t))
	}
	sides3, four := 0.0, 0.0
	if m.Sides >= 3 {
		sides3 = 1
	}
	if m.Seats >= 4 {
		four = 1
	}
	return []float64{own, best, own - best, end, float64(m.Deal) / 10, sides3, four}
}

// WinModel is a small fitted MLP: P(this side wins the match | MatchScore).
// Hidden layers are tanh, the output a sigmoid.
type WinModel struct {
	Features int           `json:"features"`
	Layers   []WinModelLay `json:"layers"`
}

// WinModelLay is one dense layer, W out × in.
type WinModelLay struct {
	W [][]float64 `json:"w"`
	B []float64   `json:"b"`
}

// Validate checks the layers chain from Features to one output.
func (w *WinModel) Validate() error {
	in := w.Features
	if in != len(WinFeatures(MatchScore{})) {
		return fmt.Errorf("learn: win model reads %d features, WinFeatures makes %d", in, len(WinFeatures(MatchScore{})))
	}
	if len(w.Layers) == 0 {
		return fmt.Errorf("learn: win model has no layers")
	}
	for i, l := range w.Layers {
		if len(l.W) != len(l.B) || len(l.W) == 0 {
			return fmt.Errorf("learn: win model layer %d: %d rows, %d biases", i, len(l.W), len(l.B))
		}
		for _, row := range l.W {
			if len(row) != in {
				return fmt.Errorf("learn: win model layer %d reads %d, gets %d", i, len(row), in)
			}
		}
		in = len(l.W)
	}
	if in != 1 {
		return fmt.Errorf("learn: win model ends in %d outputs", in)
	}
	return nil
}

// P is the modelled chance of winning; a finished match is its result.
func (w *WinModel) P(m MatchScore) float64 {
	if m.Over {
		return m.Won
	}
	x := WinFeatures(m)
	for i, l := range w.Layers {
		y := make([]float64, len(l.W))
		for o, row := range l.W {
			s := l.B[o]
			for j, v := range row {
				s += v * x[j]
			}
			if i < len(w.Layers)-1 {
				s = math.Tanh(s)
			}
			y[o] = s
		}
		x = y
	}
	return 1 / (1 + math.Exp(-x[0]))
}

// MatchReward mixes a game's episode reward with the change in the modelled
// chance of winning the match (Suphx's global reward prediction):
//
//	reward = Alpha * episode reward + (1 - Alpha) * K * (P(after) - P(before))
//
// paid at the end of every episode, where before is the score when the
// episode began and after when it ended (the result itself once the match is
// over). Summed over a match the second term is the result less P at 0-0, so
// it prices a deal by what it did for the match and nothing else; K brings it
// to the episode reward's scale.
type MatchReward struct {
	Alpha float64   `json:"alpha"`
	K     float64   `json:"k"`
	Model *WinModel `json:"model"`
}

func (r *MatchReward) validate() error {
	if r.Alpha < 0 || r.Alpha > 1 {
		return fmt.Errorf("learn: match reward alpha %g outside [0,1]", r.Alpha)
	}
	if r.Model == nil {
		return fmt.Errorf("learn: match reward without a model")
	}
	return r.Model.Validate()
}

// mix is the reward for an episode that ended between before and after.
func (r *MatchReward) mix(g MatchScored, before, after Position, seat string, episode float32) (float32, error) {
	b, err := g.MatchScoreFor(before, seat)
	if err != nil {
		return 0, err
	}
	a, err := g.MatchScoreFor(after, seat)
	if err != nil {
		return 0, err
	}
	d := r.Model.P(a) - r.Model.P(b)
	return float32(r.Alpha*float64(episode) + (1-r.Alpha)*r.K*d), nil
}

// LoadWinModel reads a WinModel from JSON.
func LoadWinModel(b []byte) (*WinModel, error) {
	var w WinModel
	if err := json.Unmarshal(b, &w); err != nil {
		return nil, err
	}
	if err := w.Validate(); err != nil {
		return nil, err
	}
	return &w, nil
}
