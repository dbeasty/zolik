package learn

import (
	"hash/fnv"
	"math/rand"

	"zolik/server/internal/module"
)

// NetBot plays a seat with a trained network.
//
// It holds the same two disciplines as every hand-written bot here. It never
// decides a move is legal — every candidate is built from an enabled offer, so
// the worst a bad network can do is choose a bad legal move. And it never sees
// more than the seat may: what reaches the network is Encode's output, which
// each adapter's no-peek test pins.
//
// It is a small value around a shared Policy, and one NetBot plays every seat
// it is handed: the seat, the position and the offers are Act's arguments,
// and nothing about any of them is kept between calls. A table of four bots
// on one model is one Policy asked four times, not four agents.
//
// Anything it cannot do, the heuristic does: no model, a model for another
// encoder, an adapter with nothing to offer, an encoding error. A trained bot
// that cannot play falls back to the bot that already could, rather than to
// the first offer in the list.
type NetBot struct {
	Game     Game
	Policy   *Policy
	Fallback module.Bot
	// Temperature is how far from its favourite move it will stray. Zero is
	// always the favourite — the strongest play and the most predictable one.
	Temperature float64
}

var _ module.Bot = NetBot{}

func (b NetBot) Act(s module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	a, _, ok := b.choose(s, seat, offers)
	if !ok {
		return b.fallback(s, seat, offers)
	}
	return a, true
}

// usable reports whether the network can play this game at all.
func (b NetBot) usable() bool { return b.Policy.Fits(b.Game) }

// choose is Act without the fallback, and with the candidate index, so the
// environment can tell a network move from a heuristic one.
//
// The state is decoded once (Positional) and both the candidates and the
// encoding are read off that one decode.
//
// A candidate with more than one step (Candidate.Then) is played one step per
// call: this returns its first action, and the next call — from the position
// that action left — is answered by the adapter's candidates for finishing it.
// Nothing is remembered between calls, so two bots sharing a network, or a
// bench running seeds in parallel, cannot see each other's plans.
func (b NetBot) choose(s module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, int, bool) {
	if !b.usable() {
		return module.Action{}, -1, false
	}
	view := Positions(b.Game)
	pos, err := view.Position(s)
	if err != nil {
		return module.Action{}, -1, false
	}
	cands, err := view.CandidatesFor(pos, seat.PlayerID, offers)
	if err != nil || len(cands) == 0 {
		return module.Action{}, -1, false
	}
	if len(cands) == 1 {
		return cands[0].Action, 0, true
	}
	obs, err := view.EncodeFor(pos, seat.PlayerID)
	if err != nil {
		return module.Action{}, -1, false
	}
	feats := make([][]float32, len(cands))
	for i, c := range cands {
		feats[i] = c.Features
	}
	i := pick(b.Policy.Logits(obs, feats), b.Temperature, turnSeed(s, seat))
	return cands[i].Action, i, true
}

func (b NetBot) fallback(s module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	if b.Fallback != nil {
		return b.Fallback.Act(s, seat, offers)
	}
	return module.ChooseAction(offers, nil)
}

// pick is the argmax at zero temperature, and a draw from the softmax above it.
func pick(logits []float32, temperature float64, seed int64) int {
	if temperature <= 0 {
		best := 0
		for i, l := range logits {
			if l > logits[best] {
				best = i
			}
		}
		return best
	}
	p := softmax(logits, temperature)
	x := rand.New(rand.NewSource(seed)).Float64()
	for i, pi := range p {
		if x < pi {
			return i
		}
		x -= pi
	}
	return len(p) - 1
}

// turnSeed mixes the position into the seat's seed. BotSeat.Seed is fixed for
// the whole match; the position is what makes this turn's coin differ from the
// last one's while keeping the same match, seat and position reproducible.
func turnSeed(s module.State, seat module.BotSeat) int64 {
	h := fnv.New64a()
	_, _ = h.Write(s)
	return seat.Seed ^ int64(h.Sum64())
}
