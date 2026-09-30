package rummytiles

import "zolik/server/internal/module"

// What to do *instead* — the third thing a refused player needs, after the
// reason and the rule.
//
// This game refuses a turn rather than a move: the workspace is edited freely
// and only `commit` judges it, so "Done" is the button that says no, and it
// says no for four different reasons that all word as something vague. "You
// still have loose tiles to place", "the table isn't valid yet" and "your
// first lay needs to be worth 30 points or more" are each one press away from
// being actionable — put the tray back, reset the turn, lay this much more —
// and that press is what this names.
//
// It decides nothing about legality: it reads the refusal that already
// happened and the offers that were already built.
func (m *Module) annotate(s *GameState, offers []module.ActionOffer) {
	stated := module.StatedRuleIDs(m, configOf(s))

	enabled := func(ids ...string) string {
		for _, want := range ids {
			for _, o := range offers {
				if o.Enabled && o.ID == want {
					return o.ID
				}
			}
		}
		return ""
	}

	for i := range offers {
		o := &offers[i]
		if o.Enabled || o.WhyNot == "" {
			continue
		}
		o.RuleIDs = explainWith(stated, o.WhyNot)

		switch o.WhyNot {

		case ErrTrayNotEmpty:
			o.Remedy = &module.Fact{
				LabelKey: "rummytiles.remedy.emptyTheTray",
				Params:   map[string]any{"n": trayCount(s)},
			}
			o.RemedyOfferID = enabled(OfferPlace, OfferResetTurn)

		case ErrNothingPlayed:
			// The rule players trip over on the turn they only rearranged the
			// table: moving tiles about is not a turn, and if nothing comes
			// out of the hand the move is a draw.
			o.Remedy = &module.Fact{LabelKey: "rummytiles.remedy.playFromHandOrDraw"}
			o.RemedyOfferID = enabled(OfferPlace, OfferDraw)

		case ErrTableNotValid:
			o.Remedy = &module.Fact{LabelKey: "rummytiles.remedy.fixOrReset"}
			o.RemedyOfferID = enabled(OfferResetTurn)

		case ErrInitialMeldLow:
			// The gap, not the floor. A player who has laid 22 against 30 is
			// eight points away; telling them the floor makes them do the
			// arithmetic the server already did.
			short := initialMeldFloor - newSetsValue(s)
			if short < 1 {
				short = 1
			}
			o.Remedy = &module.Fact{
				LabelKey: "rummytiles.remedy.needMorePoints",
				Params:   map[string]any{"n": short, "floor": initialMeldFloor},
			}
			o.RemedyOfferID = enabled(OfferPlace)

		case ErrInitialMeldOnly:
			o.Remedy = &module.Fact{
				LabelKey: "rummytiles.remedy.ownNewSetsOnly",
				Params:   map[string]any{"n": initialMeldFloor},
			}
			o.RemedyOfferID = enabled(OfferPlace)

		case ErrTileDoesNotFit:
			o.Remedy = &module.Fact{LabelKey: "rummytiles.remedy.startANewSet"}
			o.RemedyOfferID = enabled(OfferPlace)

		case ErrBadSplitPosition:
			o.Remedy = &module.Fact{
				LabelKey: "rummytiles.remedy.splitLeavesThree",
				Params:   map[string]any{"n": minSetSize},
			}

		case ErrJokerSwapMismatch:
			o.Remedy = &module.Fact{LabelKey: "rummytiles.remedy.matchTheJoker"}
		}
	}
}

// trayCount is how many tiles are still loose, for a sentence that says how
// far from finished the turn is rather than only that it is not.
func trayCount(s *GameState) int {
	if s.Workspace == nil {
		return 0
	}
	return len(s.Workspace.Tray)
}

// newSetsValue is what this turn's own new sets are worth, computed the way
// applyCommit computes it — same validator, same scorer, so the number under
// the refusal is the number that caused it.
//
// An invalid set contributes nothing rather than aborting: this is arithmetic
// for a sentence, not a second opinion about legality, and a workspace mid-edit
// is routinely invalid.
func newSetsValue(s *GameState) int {
	if s.Workspace == nil {
		return 0
	}
	total := 0
	for _, set := range s.Workspace.Sets {
		if !s.Workspace.NewSetIDs[set.ID] {
			continue
		}
		kind, canonical, ok := validateSet(set.Cards)
		if !ok {
			continue
		}
		total += setValueOf(kind, canonical)
	}
	return total
}

// configOf is the table a round is actually being played at, rebuilt from the
// state's own resolved options rather than read back from the lobby — a match
// keeps the rules it was dealt under, so the rules a refusal points at have to
// be those same ones.
func configOf(s *GameState) module.MatchConfig {
	return module.MatchConfig{
		Variation: s.Variation,
		Options: module.Options{
			OptTargetScore:    s.TargetScore,
			OptRoundLimit:     s.RoundLimit,
			OptPoolExhaustion: module.BoolOpt(s.PoolExhaustionLowestWins),
		},
	}
}
