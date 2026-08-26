package module

// A match's rounds, in a shape no game owns.
//
// The runtime had no concept of a round, deliberately: the statistics model
// once counted deals played, deals won, go-outs and penalty points, and every
// one of those words described rummy. A game with no deals had nothing to put
// in them, which is why statistics existed for Žolíky and for nothing else.
//
// So this is not a deal, and not a hand. It is a *round*: the unit a match is
// made of, scored, and then either followed by another or not. Žolíky deals
// seven, Canasta deals until somebody passes the target, Hold'em plays hands,
// and Prší has one and therefore keeps none of this. What they share is that a
// round ends, moves everyone's score, and is wiped off the table by the next
// one — which is the whole reason a player needs it written down.

// RoundLog is a match's round-by-round history and, when the table is sitting
// between rounds, the fact that it is.
//
// Public by construction. It takes no viewer, and the same values are written
// into the permanent match record, so a module may put nothing in here that
// every seat at the table may not already see. Hold'em is what makes that a
// rule rather than a note: a hand everybody folded out of is never shown, on
// purpose, and a round log that replayed it would leak it twice over — once on
// the wire and once into a row that outlives the match.
type RoundLog struct {
	// Rounds are the completed ones, oldest first. The round in progress is not
	// here; it is the board.
	Rounds []RoundResult `json:"rounds"`
	// Paused is the table sitting between rounds.
	//
	// The module's own answer to "should a results screen be up", rather than
	// something a client works out from the offers it can see. A client that
	// inferred a pause from the presence of a continue offer would be deriving
	// a rule, which is the one thing this protocol exists to stop.
	Paused bool `json:"paused,omitempty"`
	// LabelKey names what a round is called in this game — a deal, a hand, a
	// leg. A key, never a word, like every other label here.
	LabelKey string `json:"labelKey"`
	// WaitingFor are the seats that have not yet said to go on, in turn order.
	// Empty unless Paused.
	WaitingFor []string `json:"waitingFor,omitempty"`
}

// RoundResult is one completed round.
type RoundResult struct {
	// Number is 1-based and counts this match's rounds, so it is the number the
	// header showed while the round was being played. A module whose own
	// counter starts at zero adds one here rather than letting the table and
	// the header disagree.
	Number int `json:"number"`
	// Winners is who took the round — a shedder, a partnership, whoever was
	// pushed the chips. Possibly nobody: a Canasta deal can end exhausted, and
	// a rummy deal can end with two players level.
	Winners []string `json:"winners,omitempty"`
	// Scores is one row per seat, in turn order rather than rank order, so a
	// table reads down the same column from round to round.
	Scores []RoundScore `json:"scores"`
	// Facts are true of the round rather than of any one seat: which contract
	// it required, that the deck ran out, that the go-out was concealed.
	Facts []Fact `json:"facts,omitempty"`
}

// RoundScore is one seat's row in one round.
//
// Delta and Total are both higher-is-better, the same orientation
// Standing.Score keeps, so one component renders a rummy penalty and a chip
// stack and puts the arrow the right way round for both. Žolíky negates its
// penalty; poker does not have to. Standing.Shown is how a module says what to
// print when the negated figure is not it.
type RoundScore struct {
	PlayerID string `json:"playerId"`
	// Delta is what this round changed the score by; Total the running score
	// after it. Total is snapshotted rather than derived, because a game where
	// the score *is* the state — a poker stack — cannot reconstruct "what was I
	// on after round 12" once round 13 has happened.
	Delta int `json:"delta"`
	Total int `json:"total"`
	// Shown and ShownTotal print in place of Delta and Total, for the same
	// reason Standing.Shown exists: a rummy penalty is carried negated and
	// nobody should be shown the negation.
	Shown      *int `json:"shown,omitempty"`
	ShownTotal *int `json:"shownTotal,omitempty"`
	// Facts break the delta into the parts a player argues about: melded cards,
	// canastas, red threes, what was left in hand.
	Facts []Fact `json:"facts,omitempty"`
}

// Rounded is implemented by a module whose match is made of rounds.
//
// Optional, in the same way Ranked is, and one of the four games declines: Prší
// is a single deal that ends the moment somebody sheds their last card. A
// one-row history is a worse answer than no history, so it keeps none.
type Rounded interface {
	Rounds(s State) (RoundLog, error)
}

// RoundsFor returns a module's round history, or nil if it keeps none.
//
// Nil and empty mean different things and both are real: nil is "this game has
// no rounds", an empty Rounds slice is "it does, and none has finished yet".
func RoundsFor(m GameModule, s State) *RoundLog {
	r, ok := m.(Rounded)
	if !ok {
		return nil
	}
	out, err := r.Rounds(s)
	if err != nil {
		return nil
	}
	if out.Rounds == nil {
		// Never nil on the wire: a nil slice serialises to `null`, which every
		// client then has to guard before indexing.
		out.Rounds = []RoundResult{}
	}
	return &out
}

// Outcome is everything a module can say about a match, in one value.
//
// One value rather than a longer argument list because the recorder is already
// handed "the module's own standings" and is now handed "the module's own
// rounds"; a third fact would be a third parameter and a fourth signature
// change. The runtime still learns nothing from either — it carries them.
type Outcome struct {
	Standings []Standing
	Rounds    *RoundLog
}

// OutcomeOf asks a module for everything it will say about a match.
func OutcomeOf(m GameModule, s State) Outcome {
	return Outcome{Standings: StandingsFor(m, s), Rounds: RoundsFor(m, s)}
}
