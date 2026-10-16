package zolikmod

import (
	"sync"

	"zolik/server/internal/module"
)

var _ module.Projector = (*Module)(nil)

// Project decodes a state once for a broadcast to read per seat.
//
// A broadcast renders every seat's board, offers and the scoreboard, and each
// of those used to unmarshal the whole state again: three decodes per seat per
// move, which a profile of a loaded server put at a fifth of all CPU. The
// decoded state is only ever read from here — View filters by viewer, and the
// offers are probed against clones — so one copy serves every viewer.
func (m *Module) Project(raw module.State) (module.Projection, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	return &projection{m: m, s: s}, nil
}

type projection struct {
	m *Module
	s *matchState

	// offers is each viewer's offer list, worked out the first time anyone
	// asks. One state is asked for the same seat's offers more than once:
	// the broadcast renders every seat's, and the bot loop then asks for
	// the seat it is about to play, of the very same state (see
	// Manager.currentProjected). Enumerating offers is the dearest thing a
	// projection does, so the second answer is the first one again.
	//
	// The lists are shared by everybody who asks, so nobody may write to one.
	offersMu sync.Mutex
	offers   map[string]cachedOffers
}

type cachedOffers struct {
	list []module.ActionOffer
	err  error
}

func (p *projection) View(viewerID string) (module.ViewModel, error) {
	return p.m.viewOf(p.s, viewerID, false)
}

func (p *projection) LegalActions(viewerID string) ([]module.ActionOffer, error) {
	p.offersMu.Lock()
	defer p.offersMu.Unlock()
	if c, ok := p.offers[viewerID]; ok {
		return c.list, c.err
	}
	list, err := p.m.legalActions(p.s, viewerID)
	if p.offers == nil {
		p.offers = make(map[string]cachedOffers, len(p.s.Rules.TurnOrder))
	}
	p.offers[viewerID] = cachedOffers{list: list, err: err}
	return list, err
}

func (p *projection) Standings() []module.Standing {
	out, err := p.m.standingsOf(p.s)
	if err != nil {
		return nil
	}
	return out
}

func (p *projection) Rounds() *module.RoundLog {
	out := p.m.roundsOf(p.s)
	if out.Rounds == nil {
		out.Rounds = []module.RoundResult{}
	}
	return &out
}

func (p *projection) Finished() (bool, []string) { return finishedOf(p.s) }
