package rules

// Hooks for the external tests in this directory, which play whole games with
// internal/ai and so cannot live inside the package.

// ScratchForTest lays out a probe scratch for s: reset puts it back and hands
// it out, release returns it to the pool.
func ScratchForTest(s GameState) (reset func() GameState, release func()) {
	p := newProbeScratch(s)
	return p.reset, p.release
}

// CloneStateForTest is cloneState.
var CloneStateForTest = cloneState

// ApplyOwnedForTest is applyOwned.
var ApplyOwnedForTest = applyOwned
