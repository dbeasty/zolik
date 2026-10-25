package sietemedio

import "zolik/server/internal/module"

var _ module.LessonProvider = (*Module)(nil)

// Lessons is Siete y Medio's guided walk-through, written against the table's
// own stake, tie rule, bonus and bank rotation.
//
// Four of its steps have more than one spelling, because four of this game's
// rules are settings rather than laws — see docs/sietemedio-rules.md. A guide
// that taught the default while the table played something else would be the
// same split brain the written rules exist to avoid, and `lessontest` fails a
// step that points at a rule this table does not state.
func (m *Module) Lessons(cfg module.MatchConfig) ([]module.Lesson, error) {
	t := tableOf(cfg)

	deal := module.Say("deal", "guide.sietemedio.deal", nil, module.AnchorHand, module.AnchorSeats).
		About("sietemedio.rules.firstCardDown")
	if !t.FirstCardDown {
		deal = module.Say("deal", "guide.sietemedio.dealOpen", nil, module.AnchorHand, module.AnchorSeats).
			About("sietemedio.rules.allFaceUp")
	}

	seven := module.Say("seven", "guide.sietemedio.sevenDouble", nil, module.AnchorHand).
		About("sietemedio.rules.sevenStands", "sietemedio.rules.sevenPaysDouble")
	if !t.SevenDoubles {
		seven = module.Say("seven", "guide.sietemedio.sevenEven", nil, module.AnchorHand).
			About("sietemedio.rules.sevenStands", "sietemedio.rules.sevenPaysEven")
	}

	ties := module.Say("ties", "guide.sietemedio.tiesToBank", nil, module.AnchorScore).
		About("sietemedio.rules.tiesToBank")
	if !t.TiesToBank {
		ties = module.Say("ties", "guide.sietemedio.tiesPush", nil, module.AnchorScore).
			About("sietemedio.rules.tiesPush")
	}

	return []module.Lesson{{
		ID:         "basics",
		TitleKey:   "guide.sietemedio.basics.title",
		SummaryKey: "guide.sietemedio.basics.summary",
		Seed:       1,
		Bots:       2,
		Steps: []module.LessonStep{
			module.Say("goal", "guide.sietemedio.goal", nil, module.AnchorHand).
				About("sietemedio.rules.goal"),
			module.Say("pack", "guide.sietemedio.pack", nil, module.AnchorHand).
				About("sietemedio.rules.pack"),
			module.Say("values", "guide.sietemedio.values", nil, module.AnchorHand).
				About("sietemedio.rules.values"),
			module.Say("bank", "guide.sietemedio.bank", map[string]any{"chips": t.StartingStack}, module.AnchorSeats).
				About("sietemedio.rules.bankDoesNotStake", "sietemedio.rules.bankCovers"),
			deal,
			module.Say("stake", "guide.sietemedio.stake", map[string]any{"n": t.MinStake}, module.AnchorActionBar).
				About("sietemedio.rules.minStake", "sietemedio.rules.bankCovers"),
			module.Do("freeplay", "guide.sietemedio.freeplay", nil, []string{VerbStake, VerbHit, VerbStand},
				module.AnchorHand, module.AnchorActionBar).Times(4).
				About("sietemedio.rules.hitStand"),
			module.Say("bust", "guide.sietemedio.bust", nil, module.AnchorHand).
				About("sietemedio.rules.bust"),
			seven,
			module.Say("bankLast", "guide.sietemedio.bankLast", nil, module.AnchorSeats).
				About("sietemedio.rules.bankLast", "sietemedio.rules.bankBust"),
			ties,
			bankMovesStep(t),
			module.Say("end", "guide.sietemedio.end", map[string]any{"n": t.RoundLimit, "chips": t.StartingStack}, module.AnchorScore).
				About("sietemedio.rules.rounds", "sietemedio.rules.mostChipsWins"),
			module.Say("hint", "guide.sietemedio.hint", nil, module.AnchorHint),
			module.Say("done", "guide.sietemedio.done", nil),
		},
	}}, nil
}

// bankMovesStep is the step with three spellings, pointing at whichever of the
// three rules this table states.
func bankMovesStep(t *GameState) module.LessonStep {
	switch t.BankPass {
	case bankRound:
		return module.Say("bankMoves", "guide.sietemedio.bankRound", nil, module.AnchorSeats).
			About("sietemedio.rules.bankRound")
	case bankFixed:
		return module.Say("bankMoves", "guide.sietemedio.bankFixed", nil, module.AnchorSeats).
			About("sietemedio.rules.bankFixed")
	default:
		return module.Say("bankMoves", "guide.sietemedio.bankOnSeven", nil, module.AnchorSeats).
			About("sietemedio.rules.bankOnSeven")
	}
}
