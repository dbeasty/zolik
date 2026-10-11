package sedma

import "zolik/server/internal/module"

var _ module.LessonProvider = (*Module)(nil)

// Lessons is Sedma's guided walk-through. The match target is passed in, and
// the card-points step appears only where the table shows every side's points.
func (m *Module) Lessons(cfg module.MatchConfig) ([]module.Lesson, error) {
	c := resolve(cfg)
	steps := []module.LessonStep{
		module.Say("goal", "guide.sedma.goal", map[string]any{"n": pointsTotal}, module.AnchorHand).
			About("sedma.rules.goal", "sedma.rules.points"),
		module.Say("setup", "guide.sedma.setup", map[string]any{"n": handSize}, module.AnchorSeats).
			About("sedma.rules.teams", "sedma.rules.deal"),
		module.Do("lead", "guide.sedma.lead", nil, []string{VerbPlay},
			module.AnchorHand, module.AnchorActionBar).
			About("sedma.rules.play"),
		module.Say("capture", "guide.sedma.capture", nil, module.AnchorTable).
			About("sedma.rules.capture"),
		module.Say("sevens", "guide.sedma.sevens", nil, module.AnchorHand).
			About("sedma.rules.capture"),
		module.Say("continue", "guide.sedma.continue", nil, module.AnchorActionBar).
			About("sedma.rules.continue"),
		module.Say("draw", "guide.sedma.draw", map[string]any{"n": handSize}, module.AnchorDrawPile).
			About("sedma.rules.draw"),
	}
	if c.showCardPoints {
		steps = append(steps, module.Say("points", "guide.sedma.points", nil, module.AnchorScore).
			About("sedma.rules.points"))
	}
	steps = append(steps,
		module.Say("scoring", "guide.sedma.scoring", nil, module.AnchorScore).
			About("sedma.rules.score"),
		module.Say("target", "guide.sedma.target", map[string]any{"n": c.target}, module.AnchorScore).
			About("sedma.rules.end"),
		module.Do("freeplay", "guide.sedma.freeplay", nil, []string{VerbPlay, VerbStop},
			module.AnchorHand, module.AnchorActionBar).Times(4),
		module.Say("hint", "guide.sedma.hint", nil, module.AnchorHint),
		module.Say("done", "guide.sedma.done", nil),
	)
	return []module.Lesson{{
		ID:         "basics",
		TitleKey:   "guide.sedma.basics.title",
		SummaryKey: "guide.sedma.basics.summary",
		Seed:       1,
		Bots:       3,
		Steps:      steps,
	}}, nil
}
