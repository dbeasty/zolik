package module

// The pause between rounds, written once.
//
// Three of the four games want the same thing at the end of a round: stop,
// show what happened, go on when the table says so. Written three times that is
// three chances to disagree about what happens when a seat presses the button
// twice, or when somebody who is not at the table presses it at all.
//
// It is deliberately not a runtime primitive. The runtime has exactly one way
// for a seat to say something — an action against an offer it was given — and a
// second would be a second write path, with its own version check, its own
// broadcast and its own bugs. So an intermission is an ordinary turn: the
// module offers `continue`, seats take it, and the module deals when they all
// have. The proof that it really is ordinary is that SubmissionFor builds a
// complete action from the offer with no special case, so every bot and the
// conformance driver press it without knowing intermissions exist.

const (
	// VerbContinue is the verb a seat sends to go on to the next round;
	// OfferContinue is the offer's id.
	VerbContinue  = "continue"
	OfferContinue = "continue"

	// ErrAlreadyReady, ErrNotPaused and ErrNotSeated are refusals, as codes.
	// Codes, never sentences — a client renders them from its locale bundle.
	ErrAlreadyReady = "ALREADY_READY"
	ErrNotPaused    = "NOT_BETWEEN_ROUNDS"
	ErrNotSeated    = "NOT_AT_THIS_TABLE"
)

// Intermission is the between-rounds state a module embeds in its own.
//
// Embedded rather than held by the runtime because it *is* game state: which
// round has just ended, and who has seen it, are facts about the match, and the
// runtime does not read match state. Kept inside the module's own State it
// marshals, persists, replicates across instances and survives a reload for
// free — which the events a client might otherwise have accumulated do not.
type Intermission struct {
	Open  bool            `json:"open,omitempty"`
	Round int             `json:"round,omitempty"`
	Ready map[string]bool `json:"ready,omitempty"`
}

// Begin opens an intermission after the given round number.
func (i *Intermission) Begin(round int) {
	i.Open, i.Round, i.Ready = true, round, map[string]bool{}
}

// Close ends it, and forgets who was ready — the next one starts empty.
func (i *Intermission) Close() { i.Open, i.Round, i.Ready = false, 0, nil }

// Mark records one seat as ready to go on, or refuses with a code.
//
// order is passed in so that a continue from somebody who is not at this table
// is refused here rather than in three modules. During an intermission there is
// no player on turn to check the actor against, which is the one thing the
// ordinary turn path does for free and this does not.
func (i *Intermission) Mark(order []string, playerID string) error {
	if !i.Open {
		return Error{Code: ErrNotPaused}
	}
	if !seated(order, playerID) {
		return Error{Code: ErrNotSeated, Message: playerID}
	}
	if i.Ready[playerID] {
		return Error{Code: ErrAlreadyReady, Message: playerID}
	}
	if i.Ready == nil {
		i.Ready = map[string]bool{}
	}
	i.Ready[playerID] = true
	return nil
}

// Waiting lists the seats that have not readied, in the order given.
func (i *Intermission) Waiting(order []string) []string {
	if !i.Open {
		return nil
	}
	out := make([]string, 0, len(order))
	for _, id := range order {
		if !i.Ready[id] {
			out = append(out, id)
		}
	}
	return out
}

// Settled reports every seat having readied.
func (i *Intermission) Settled(order []string) bool {
	return i.Open && len(i.Waiting(order)) == 0
}

// Seats is the seat list a paused table shows.
//
// Every seat still waited on is Active, because that is what Active means: the
// seats the module is waiting on. An intermission is the first state where
// there is more than one, which is why the runtime learned to await a set
// rather than a single player — see AwaitedSeats.
func (i *Intermission) Seats(order []string) []Seat {
	out := make([]Seat, 0, len(order))
	for _, id := range order {
		s := Seat{PlayerID: id}
		if i.Open && !i.Ready[id] {
			s.Active = true
		} else if i.Open {
			s.LabelKeys = append(s.LabelKeys, "seat.ready")
		}
		out = append(out, s)
	}
	return out
}

// Offers is the whole offer list a seat gets during an intermission: one
// control, enabled while that seat has not readied and disabled with a reason
// once it has.
//
// Disabled rather than absent, like every other offer here — an omitted offer
// is indistinguishable from a client bug, and "greyed out, and here is why" is
// what an interface actually needs.
func (i *Intermission) Offers(order []string, playerID string) []ActionOffer {
	waiting := i.Waiting(order)
	o := ActionOffer{
		ID:       OfferContinue,
		Verb:     VerbContinue,
		LabelKey: "round.continue",
		Enabled:  i.Open && seated(order, playerID) && !i.Ready[playerID],
		Facts: []Fact{{
			LabelKey: "round.waitingFor",
			Params:   map[string]any{"n": len(waiting), "players": waiting},
		}},
	}
	if !o.Enabled {
		o.WhyNot = ErrAlreadyReady
		if !seated(order, playerID) {
			o.WhyNot = ErrNotSeated
		}
	}
	return []ActionOffer{o}
}

func seated(order []string, playerID string) bool {
	for _, id := range order {
		if id == playerID {
			return true
		}
	}
	return false
}
