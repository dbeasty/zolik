package learn

import (
	"sync"

	"zolik/server/internal/module"
)

// Position is a game's state decoded once, to be read as any seat.
//
// Opaque here: each adapter decides what it holds. It is immutable once made
// — every *For method only reads it — so one Position may be asked about by
// every seat at a table, and by several goroutines at once. An adapter whose
// candidate search plays moves through the engine does it on copies, never on
// what the Position holds.
type Position interface{}

// Positional is implemented by a game that can decode a state once and then
// answer Encode, Candidates and Reward for any seat from the decoded form.
// Optional: the raw-state methods on Game stay the contract, and for every
// adapter that has both they are thin wrappers — Position, then the *For
// method — so the two can never disagree.
//
// It exists because the decode is most of what those three cost (a state is a
// JSON document), and every caller asks more than one question of the same
// state: a bot move asks for the candidates and the encoding, the environment
// asks for a reward for every learner seat after every action, and the
// previous action's "after" is the next one's "before".
type Positional interface {
	Position(s module.State) (Position, error)
	EncodeFor(p Position, seat string) ([]float32, error)
	CandidatesFor(p Position, seat string, offers []module.ActionOffer) ([]Candidate, error)
	RewardFor(before, after Position, seat string) (float32, bool, error)
}

// Positions is g's Positional, or, for a game without one, a stand-in whose
// Position is the raw state and whose *For methods are g's own. Callers use it
// unconditionally.
func Positions(g Game) Positional {
	if p, ok := g.(Positional); ok {
		return p
	}
	return rawPositions{g}
}

type rawPositions struct{ g Game }

func (r rawPositions) Position(s module.State) (Position, error) { return s, nil }

func (r rawPositions) EncodeFor(p Position, seat string) ([]float32, error) {
	return r.g.Encode(p.(module.State), seat)
}

func (r rawPositions) CandidatesFor(p Position, seat string, offers []module.ActionOffer) ([]Candidate, error) {
	return r.g.Candidates(p.(module.State), seat, offers)
}

func (r rawPositions) RewardFor(before, after Position, seat string) (float32, bool, error) {
	return r.g.Reward(before.(module.State), after.(module.State), seat)
}

// Memo is a value worked out at most once, on first use, and safe to ask for
// from several goroutines. Adapters build their Positions from these: a
// Position decodes only what it is asked for — the environment makes one for
// every state it passes through, and most are only ever asked for a reward,
// which reads a sliver of the document.
type Memo[T any] struct {
	once sync.Once
	v    T
	err  error
}

// Get is the value, computing it with f the first time.
func (m *Memo[T]) Get(f func() (T, error)) (T, error) {
	m.once.Do(func() { m.v, m.err = f() })
	return m.v, m.err
}
