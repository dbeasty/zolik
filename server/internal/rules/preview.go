package rules

// Submission previews: what a set of cards *would* be, if it were played.
//
// This is the answer to the one question the offer list structurally cannot
// answer. An offer describes a shape ("3 or more cards from your hand"),
// because enumerating every legal combination of a 13-card hand is
// combinatorial. But a player assembling a meld wants to know, right now,
// whether the cards they have picked up form a valid run and what they are
// worth — a question about a specific candidate the server has not been sent.
//
// Before this existed, each client answered it with its own copy of the
// rules: the terminal client carried a second card-scoring table
// (approximateNaturalValue) purely to render a live "natural value" readout.
// A preview is a round trip, but it is a cheap one — the validators are pure
// — and it means the number a player sees while choosing is computed by the
// same code that will judge the submission.

// MeldPreview describes what a candidate set of cards would be.
type MeldPreview struct {
	Cards []string `json:"cards"`
	// Valid reports whether these cards form a legal meld under this game's
	// ruleset — which is not the same as being *playable* right now; see
	// Playable below.
	Valid bool `json:"valid"`
	// Type is "set" or "run" when Valid; empty otherwise.
	//
	// Deliberately not `json:"type"`: this struct is embedded in the wire
	// envelope, whose own Type field carries the frame name ("meld_preview").
	// Go resolves that collision by depth, so the shallower envelope field
	// wins and the meld type would vanish from the JSON silently.
	Type MeldType `json:"meldType,omitempty"`
	// NaturalValue is the meld's point total, the figure an initial-meld
	// floor is measured against. A wild counts as the card it stands in for.
	NaturalValue int `json:"naturalValue"`
	WildCount    int `json:"wildCount"`
	// WhyNot is the engine's reason this is not a legal meld — empty when
	// Valid. A stable code, not a sentence, like ActionOffer.WhyNot.
	WhyNot RulesErrorCode `json:"whyNot,omitempty"`

	// Playable reports whether laying exactly this, right now, would be
	// accepted — the full check including turn, phase, contract progress and
	// the must-keep-a-discard rule. Valid answers "is this a meld?";
	// Playable answers "may I play it?".
	Playable bool `json:"playable"`
	// WhyNotPlayable is the reason Playable is false while Valid is true.
	WhyNotPlayable RulesErrorCode `json:"whyNotPlayable,omitempty"`

	// InitialMeldMinimum is the floor in force, and MeetsMinimum whether the
	// player would clear it counting everything they have already laid plus
	// this candidate. 0 means no floor.
	InitialMeldMinimum int  `json:"initialMeldMinimum"`
	MeetsMinimum       bool `json:"meetsMinimum"`
	// AlreadyLaidValue is the natural value the player has already put on the
	// table this deal — the other half of the sum MeetsMinimum reports on.
	// Without it a client can only show the candidate's own value, so a
	// 30-point set under a 35-point floor reads as "30, needs 35" whether the
	// player is 5 points short or already well past it. Always 0 when there
	// is no floor, since nothing is being counted toward one.
	AlreadyLaidValue int `json:"alreadyLaidValue"`
}

// PreviewMeld evaluates a candidate meld for playerID without changing
// anything. Pure: it clones before any dry run, so the caller's state is
// untouched (see the aliasing note on LegalActions' probe).
func PreviewMeld(state GameState, playerID string, cards []string) MeldPreview {
	cfg := effectiveRules(state)
	out := MeldPreview{
		Cards:              append([]string(nil), cards...),
		InitialMeldMinimum: cfg.InitialMeldMinimum,
	}
	// No early return for an empty selection: it falls through the same path
	// as any other invalid one, so the "WhyNotPlayable is set whenever
	// Playable is false" invariant holds for every input.
	mv, err := ValidateMeld(cards, cfg)
	if err == nil {
		out.Valid = true
		out.Type = mv.Type
		out.WildCount = mv.WildCount
		out.NaturalValue = mv.NaturalValue
	} else {
		out.WhyNot = codeOf(err)
		// Even an invalid selection gets a value, so a player assembling one
		// can watch the total climb toward the floor before the shape is
		// complete — which is exactly what the readout is for.
		out.NaturalValue = looseNaturalValue(cards)
	}

	// Would it actually be accepted? Ask the engine rather than reasoning
	// about it, for the same anti-drift reason LegalActions probes.
	//
	// Asked even when the cards are not a valid meld, so that a caller has
	// one field to read: WhyNotPlayable is always populated whenever
	// Playable is false — "because it is not a meld" is a perfectly good
	// answer to "why can't I play this", and making the UI check two
	// different fields depending on validity is how inconsistent messages
	// get written.
	_, applyErr := ApplyAction(cloneState(state), playerID, Action{Type: ActionLayMeld, Cards: cards})
	if applyErr == nil {
		out.Playable = true
	} else {
		out.WhyNotPlayable = codeOf(applyErr)
	}

	// The floor is measured across everything laid toward the initial meld,
	// not this candidate alone — a candidate that falls short by itself can
	// still clear it once what is already on the table counts.
	if cfg.InitialMeldMinimum <= 0 {
		out.MeetsMinimum = true
	} else {
		out.AlreadyLaidValue = PlayerInitialMeldNaturalValue(state, playerID)
		out.MeetsMinimum = out.AlreadyLaidValue+out.NaturalValue >= cfg.InitialMeldMinimum
	}
	return out
}

// looseNaturalValue totals a selection that is not (yet) a valid meld, so the
// running readout still means something while a player is picking cards.
//
// An ace is worth AceMeldValue as a set member or a run's top card, but only
// AceRunLowValue at a run's bottom, and an incomplete selection has not
// committed to any of them. It is guessed from the company the ace keeps:
//
//   - two or more aces can only be heading for a set (a run holds at most one
//     ace per end, and never two of the same suit);
//   - a lone ace alongside a king of its own suit is heading for the top of a
//     run (Q-K-A);
//   - anything else reads as the low endpoint, the conservative guess.
//
// A joker is worth the card it replaces, and an incomplete selection has not
// said which card that is either. It is estimated at the highest value already
// in the selection, on the grounds that a joker is being held to complete
// *these* cards: exactly right for a set, where every card scores the same,
// and within a rank or two for a run, where the joker lands next to what is
// already there. A selection of nothing but jokers has no company to read and
// scores 0.
//
// Guessing wrong only mis-states a readout that is explicitly an estimate —
// the moment the selection becomes a valid meld, ValidateMeld's own figure
// replaces this one.
func looseNaturalValue(cards []string) int {
	aces := 0
	kingSuits := map[string]bool{}
	for _, c := range cards {
		if IsAce(c) {
			aces++
			continue
		}
		if !IsJoker(c) && CardRank(c) == 11 { // king
			kingSuits[CardSuit(c)] = true
		}
	}

	total := 0
	jokers := 0
	best := 0
	for _, c := range cards {
		if IsJoker(c) {
			jokers++
			continue
		}
		v := NaturalCardValue(c, true)
		if IsAce(c) {
			if aces >= 2 || kingSuits[CardSuit(c)] {
				v = AceMeldValue
			} else {
				v = AceRunLowValue
			}
		}
		if v > best {
			best = v
		}
		total += v
	}
	return total + jokers*best
}
