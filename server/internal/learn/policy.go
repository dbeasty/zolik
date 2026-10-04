package learn

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"zolik/server/internal/module"
)

// Policy is a trained network as the bots share it: loaded once, never
// written again, and asked by any number of seats, tables and goroutines at
// once.
//
// That is the whole of the "one agent serves every seat" design. Nothing about
// a seat lives in the agent: what a move depends on is the position, the seat
// it is asked for, and the offers, and all three arrive with the call
// (module.Bot's Act). What a seat remembers lives where it always has — in the
// game state (Žolíky's ledger, Hold'em's reads) — so switching the agent from
// one player's move to another's is switching the arguments, not building
// another agent.
//
// Concurrency: the weights are read-only after construction, and every buffer
// a forward pass writes is allocated by that pass. There is no scratch space
// on the Policy to share, so no lock and no pool.
type Policy struct {
	net *Net
}

// NewPolicy wraps a loaded network. The Net must not be changed afterwards.
func NewPolicy(n *Net) *Policy {
	if n == nil {
		return nil
	}
	return &Policy{net: n}
}

// Net is the network behind the policy, for the tools that read its header or
// write it out. Read it; never change it.
func (p *Policy) Net() *Net { return p.net }

// Fits reports whether the policy was trained for this game's encoder.
func (p *Policy) Fits(g Game) bool {
	return p != nil && p.net != nil && g != nil &&
		p.net.StateDim == g.StateDim() && p.net.CandDim == g.CandDim()
}

// Logits scores every candidate against one observation.
//
// The same numbers as Net.Logits(Net.Embed(obs), cands), bit for bit, and a
// good deal cheaper when there are many candidates: the first scorer layer
// reads the embedding and the candidate side by side, and the embedding's
// half of every row's sum is the same for every candidate. It is summed once,
// in the order Dense.forward sums it, and each candidate's half is added on
// from there — the same additions in the same order, so the same float32.
func (p *Policy) Logits(obs []float32, cands [][]float32) []float32 {
	n := p.net
	emb := n.Embed(obs)
	out := make([]float32, len(cands))
	if len(cands) == 0 {
		return out
	}
	first := &n.Scorer[0]
	partial := make([]float32, first.Out)
	for o := range partial {
		sum := first.B[o]
		row := first.W[o*first.In : o*first.In+len(emb)]
		emb := emb[:len(row)]
		for i, w := range row {
			sum += w * emb[i]
		}
		partial[o] = sum
	}
	h := make([]float32, first.Out)
	for c, cand := range cands {
		if short := n.CandDim - len(cand); short > 0 {
			// A short candidate reads as zeros past its end.
			cand = append(append(make([]float32, 0, n.CandDim), cand...), make([]float32, short)...)
		}
		for o := range h {
			sum := partial[o]
			row := first.W[o*first.In+len(emb) : (o+1)*first.In]
			cand := cand[:len(row)]
			for i, w := range row {
				sum += w * cand[i]
			}
			if len(n.Scorer) > 1 && sum < 0 {
				sum = 0
			}
			h[o] = sum
		}
		out[c] = run(n.Scorer[1:], h, false)[0]
	}
	return out
}

// --- one load per process ------------------------------------------------------

// embedded is a model loaded from bytes compiled into the binary, keyed by
// what those bytes are: the name, and where and how long the slice is. A
// go:embed slice never moves, so a bot constructed for every move (as
// module.BotFor constructs them) finds the same entry every time and parses
// nothing; different bytes under the same name are a different entry, not a
// silent reuse of the first model. The entry keeps the slice it was made
// from, so the memory behind a key is never freed and handed to other bytes
// that could then be mistaken for it.
type embeddedKey struct {
	name string
	data uintptr
	size int
}

type embeddedEntry struct {
	src  []byte
	once sync.Once
	p    *Policy
	err  error
}

var (
	embeddedMu     sync.RWMutex
	embeddedModels = map[embeddedKey]*embeddedEntry{}
)

// LoadEmbedded parses a model once per process and hands every caller the
// same Policy. A model that does not parse is remembered as not parsing, so
// a broken embed costs one attempt, not one per move. A repeat call is a map
// read under a read lock, and allocates nothing. The bytes must not change
// after the first call: they are meant to be a go:embed variable.
func LoadEmbedded(name string, b []byte) (*Policy, error) {
	if len(b) == 0 {
		return nil, errors.New("learn: no model bytes")
	}
	key := embeddedKey{name: name, data: uintptr(unsafe.Pointer(unsafe.SliceData(b))), size: len(b)}
	embeddedMu.RLock()
	e, ok := embeddedModels[key]
	embeddedMu.RUnlock()
	if !ok {
		embeddedMu.Lock()
		if e, ok = embeddedModels[key]; !ok {
			e = &embeddedEntry{src: b}
			embeddedModels[key] = e
		}
		embeddedMu.Unlock()
	}
	e.once.Do(func() {
		n, err := LoadNet(e.src)
		if err != nil {
			e.err = fmt.Errorf("learn: model %s: %w", name, err)
			return
		}
		e.p = NewPolicy(n)
	})
	return e.p, e.err
}

// HardBot is a NetBot on the shared policy for these bytes, or the fallback
// when there is no model, it does not load, or it was trained for another
// game or for an encoder this game did not grow from. A model trained for an
// earlier encoder that today's only appended to plays on the prefix it was
// trained on (Policy.GameFor). Modules reach it through HardModel, which
// decides whether a Hard seat should be playing a model at all.
func HardBot(game Game, modelBytes []byte, fallback module.Bot) module.Bot {
	if game == nil || len(modelBytes) == 0 {
		return fallback
	}
	p, err := LoadEmbedded(game.Name(), modelBytes)
	if err != nil {
		return fallback
	}
	g, ok := p.GameFor(game)
	if !ok {
		return fallback
	}
	return NetBot{Game: g, Policy: p, Fallback: fallback}
}
