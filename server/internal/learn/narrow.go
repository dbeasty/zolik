package learn

import (
	"fmt"

	"zolik/server/internal/module"
)

// Appending is implemented by a game whose encoder has only ever grown by
// appending: an older network's state and candidate vectors are prefixes of
// today's. Optional.
//
// It is what lets a model trained before a feature was added be measured
// against one trained after, in one binary, and keep playing after the
// encoder grows: Narrowed hands the older model the prefix it was trained on,
// which is the encoding it was trained on. HardBot (and so the admin switch
// and LocalHard) seats such a model through Policy.GameFor.
type Appending interface {
	// EncoderPrefixes lists the (state, candidate) widths of the earlier
	// encoders that today's extends.
	EncoderPrefixes() [][2]int
}

// Narrowed is g as seen by a network trained for an earlier encoder of it
// (Appending): every vector cut to that encoder's width. Anything else is an
// error.
func Narrowed(g Game, stateDim, candDim int) (Game, error) {
	if stateDim == g.StateDim() && candDim == g.CandDim() {
		return g, nil
	}
	if a, ok := g.(Appending); ok {
		for _, p := range a.EncoderPrefixes() {
			if p[0] == stateDim && p[1] == candDim {
				return narrowed{Game: g, view: Positions(g), sd: stateDim, cd: candDim}, nil
			}
		}
	}
	return nil, fmt.Errorf("learn: %s has no %d/%d encoder (today's is %d/%d)", g.Name(), stateDim, candDim, g.StateDim(), g.CandDim())
}

// GameFor is g as this policy's network sees it: g itself when the network
// was trained for g's encoder, or g narrowed to the earlier encoder it was
// trained for (Narrowed), which is only offered by a game whose encoder has
// grown by appending. False for a network trained for another game, or for
// an encoder g did not grow from.
func (p *Policy) GameFor(g Game) (Game, bool) {
	if p == nil || p.net == nil || g == nil {
		return nil, false
	}
	if p.net.Game != "" && p.net.Game != g.Name() {
		return nil, false
	}
	ng, err := Narrowed(g, p.net.StateDim, p.net.CandDim)
	if err != nil {
		return nil, false
	}
	return ng, true
}

type narrowed struct {
	Game
	view   Positional
	sd, cd int
}

var _ Positional = narrowed{}

func (n narrowed) StateDim() int { return n.sd }
func (n narrowed) CandDim() int  { return n.cd }

func (n narrowed) Encode(s module.State, seat string) ([]float32, error) {
	p, err := n.view.Position(s)
	if err != nil {
		return nil, err
	}
	return n.EncodeFor(p, seat)
}

func (n narrowed) Candidates(s module.State, seat string, offers []module.ActionOffer) ([]Candidate, error) {
	p, err := n.view.Position(s)
	if err != nil {
		return nil, err
	}
	return n.CandidatesFor(p, seat, offers)
}

func (n narrowed) Position(s module.State) (Position, error) { return n.view.Position(s) }

func (n narrowed) EncodeFor(p Position, seat string) ([]float32, error) {
	v, err := n.view.EncodeFor(p, seat)
	if err != nil {
		return nil, err
	}
	return v[:n.sd], nil
}

func (n narrowed) CandidatesFor(p Position, seat string, offers []module.ActionOffer) ([]Candidate, error) {
	cs, err := n.view.CandidatesFor(p, seat, offers)
	if err != nil {
		return nil, err
	}
	out := make([]Candidate, len(cs))
	for i, c := range cs {
		c.Features = c.Features[:n.cd]
		out[i] = c
	}
	return out, nil
}

func (n narrowed) RewardFor(before, after Position, seat string) (float32, bool, error) {
	return n.view.RewardFor(before, after, seat)
}
