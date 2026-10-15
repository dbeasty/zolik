package zolikmod

import "zolik/server/internal/module"

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
}

func (p *projection) View(viewerID string) (module.ViewModel, error) {
	return p.m.viewOf(p.s, viewerID, false)
}

func (p *projection) LegalActions(viewerID string) ([]module.ActionOffer, error) {
	return p.m.legalActions(p.s, viewerID)
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
