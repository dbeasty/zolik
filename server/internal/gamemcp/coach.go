package gamemcp

import (
	"fmt"
	"strings"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
)

// CoachIn asks for advice on a claude seat's current decision.
type CoachIn struct {
	Table string `json:"table" jsonschema:"table id"`
	Seat  int    `json:"seat" jsonschema:"0-based claude seat whose decision to score"`
	Model string `json:"model,omitempty" jsonschema:"trained model file; defaults to ZOLIK_LEARNED_MODEL_<GAME>"`
}

// CoachOut scores every legal move.
type CoachOut struct {
	Model string `json:"model,omitempty"`
	// Moves are the observation's moves, in the same order, each with the
	// model's probability at temperature 1.
	Moves []CoachMove `json:"moves"`
	// Value is the model's value head for the position: its estimate of what
	// the rest of the episode is worth to this seat, in the reward's unit.
	Value *float64 `json:"value,omitempty"`
	// Hard is what the hand-written Hard bot would play here.
	Hard CoachPick `json:"hard"`
	Note string    `json:"note,omitempty"`
	Text string    `json:"text"`
}

// CoachMove is one legal move and what the model thinks of it.
type CoachMove struct {
	Index int      `json:"index"`
	Text  string   `json:"text"`
	Prob  *float64 `json:"prob,omitempty"`
	Logit *float64 `json:"logit,omitempty"`
}

// CoachPick is a bot's choice, matched to the move list where it can be.
type CoachPick struct {
	Text string `json:"text"`
	// Indexes are the listed moves that begin with the bot's action. A bot
	// answers one action at a time, so its first meld of a Žolíky opening
	// matches every listed opening that starts with that meld.
	Indexes []int `json:"indexes"`
}

func (svc *Service) Coach(in CoachIn) (CoachOut, error) {
	t, err := svc.table(in.Table)
	if err != nil {
		return CoachOut{}, err
	}
	s, err := t.seat(in.Seat)
	if err != nil {
		return CoachOut{}, err
	}
	if !t.isAwaited(s.Player) {
		return CoachOut{}, fmt.Errorf("seat %d has no decision to make now", s.Index)
	}
	moves, err := t.moves(s.Player)
	if err != nil {
		return CoachOut{}, err
	}
	out := CoachOut{Moves: make([]CoachMove, len(moves)), Hard: CoachPick{Indexes: []int{}}}
	for i, m := range moves {
		out.Moves[i] = CoachMove{Index: m.Index, Text: m.Text}
	}
	offers, err := t.g.m.LegalActions(t.State, s.Player)
	if err != nil {
		return CoachOut{}, err
	}

	// The model.
	path := in.Model
	if path == "" && t.g.g != nil {
		path = defaultModel(t.Game)
	}
	switch {
	case t.g.g == nil:
		out.Note = t.Game + " has no learn adapter, so no network can score its moves; only the heuristic's pick is shown."
	case path == "":
		out.Note = "no model: pass model, or set " + modelEnv(t.Game) + "; only the heuristic's pick is shown."
	default:
		p, lg, err := svc.policy(t.g, path)
		if err != nil {
			return CoachOut{}, err
		}
		out.Model = path
		if err := scoreMoves(t, p, lg, s.Player, moves, &out); err != nil {
			return CoachOut{}, err
		}
	}

	// The heuristic at Hard.
	seat := module.BotSeat{PlayerID: s.Player, Skill: module.SkillHard, Seed: module.SeatSeed(t.Seed, s.Player, "bot")}
	if a, ok := t.g.heuristic().Act(t.State, seat, offers); ok {
		vm, _ := t.g.m.View(t.State, s.Player)
		out.Hard.Text = describer{names: t.names(), vm: vm, offers: offers}.action(a)
		for _, m := range moves {
			if sameAction(m.Steps[0], a) {
				out.Hard.Indexes = append(out.Hard.Indexes, m.Index)
			}
		}
	} else {
		out.Hard.Text = "(the heuristic declined to choose)"
	}
	out.Text = coachText(out)
	return out, nil
}

// scoreMoves scores the moves with the network as it sees the game (lg: the
// encoder it was trained on, which for an older model is a prefix of the one
// that built the moves' features).
func scoreMoves(t *Table, p *learn.Policy, lg learn.Game, player string, moves []Move, out *CoachOut) error {
	if len(moves) == 0 {
		return nil
	}
	view := learn.Positions(lg)
	pos, err := view.Position(t.State)
	if err != nil {
		return err
	}
	obs, err := view.EncodeFor(pos, player)
	if err != nil {
		return err
	}
	feats := make([][]float32, len(moves))
	for i, m := range moves {
		if m.features == nil {
			out.Note = "these moves come from the offer list, not the learn adapter, so the model cannot score them"
			return nil
		}
		feats[i] = m.features[:lg.CandDim()]
	}
	logits := p.Logits(obs, feats)
	probs := softmax(logits)
	for i := range moves {
		pr, lg := probs[i], float64(logits[i])
		out.Moves[i].Prob, out.Moves[i].Logit = &pr, &lg
	}
	net := p.Net()
	v := float64(net.ValueOf(net.Embed(obs)))
	out.Value = &v
	return nil
}

func coachText(c CoachOut) string {
	var b strings.Builder
	if c.Model != "" {
		fmt.Fprintf(&b, "Model %s", c.Model)
		if c.Value != nil {
			fmt.Fprintf(&b, " (value estimate %+.3f)", *c.Value)
		}
		b.WriteString(":\n")
		order := make([]float64, len(c.Moves))
		for i, m := range c.Moves {
			if m.Prob != nil {
				order[i] = *m.Prob
			}
		}
		for _, i := range topK(order, len(order)) {
			m := c.Moves[i]
			if m.Prob == nil {
				continue
			}
			fmt.Fprintf(&b, "  [%d] %5.1f%%  %s\n", m.Index, 100**m.Prob, m.Text)
		}
	}
	fmt.Fprintf(&b, "Hard heuristic would play: %s", c.Hard.Text)
	if len(c.Hard.Indexes) > 0 {
		fmt.Fprintf(&b, " (move %v)", c.Hard.Indexes)
	}
	b.WriteString("\n")
	if c.Note != "" {
		b.WriteString("Note: " + c.Note + "\n")
	}
	return b.String()
}
