package module

import (
	"fmt"
	"sort"
	"strconv"
)

// A game-agnostic driver that plays any module using only its offer list.
//
// This is the falsification apparatus for the whole abstraction. It lives in
// the module package, not in a test file, because both games' test suites run
// it — and because a driver that has to import a specific game to work would
// prove nothing.
//
// The rule it obeys: it may read the offer list and the ViewModel, and nothing
// else. It never decodes State, never names a card rank, never mentions melds
// or suits or sevens. If it can play a game to completion, that game's offers
// are sufficient for a UI shell to play it too.

// DriverResult reports what a play-through did.
type DriverResult struct {
	Actions  int
	Finished bool
	// Winners is every player the module named — one for a shedding or rummy
	// game, a whole partnership for Canasta, and possibly several for a poker
	// match that ended level.
	Winners []string
	// Verbs counts how many times each verb was played, so a caller can
	// assert that a run actually exercised the game rather than drawing in
	// circles.
	Verbs map[string]int
}

// DriverOptions tunes a play-through.
type DriverOptions struct {
	// MaxActions bounds the run so a stalled game fails rather than hangs.
	MaxActions int
	// Prefer lists verbs to try first, most-preferred first. A driver that
	// always drew would technically "play" a shedding game forever without
	// ever finishing it.
	Prefer []string
	// OnAction, if set, is called after each accepted action.
	OnAction func(playerID string, a Action)
}

// PlayWithOffers drives a match to completion (or to MaxActions) using only
// what LegalActions and View report.
//
// Every refusal is an error: the offer list said the action was available, so
// the engine accepting it is the contract. The one exception is an offer whose
// concrete submission the module documents as un-enumerable (a rummy meld
// shape), which callers exclude via Prefer.
func PlayWithOffers(m GameModule, state State, players []PlayerRef, opts DriverOptions) (State, DriverResult, error) {
	res := DriverResult{Verbs: map[string]int{}}
	if opts.MaxActions <= 0 {
		opts.MaxActions = 2000
	}

	for step := 0; step < opts.MaxActions; step++ {
		done, winners, err := m.Finished(state)
		if err != nil {
			return state, res, fmt.Errorf("step %d: Finished: %w", step, err)
		}
		if done {
			res.Finished, res.Winners = true, winners
			return state, res, nil
		}

		actor, err := whoseTurn(m, state, players)
		if err != nil {
			return state, res, fmt.Errorf("step %d: %w", step, err)
		}

		offers, err := m.LegalActions(state, actor)
		if err != nil {
			return state, res, fmt.Errorf("step %d: LegalActions: %w", step, err)
		}
		if len(offers) == 0 {
			return state, res, fmt.Errorf("step %d: no offers at all for the player on turn (%s)", step, actor)
		}

		a, ok := ChooseAction(offers, opts.Prefer)
		if !ok {
			return state, res, fmt.Errorf("step %d: offers left %s with nothing to do:\n%s",
				step, actor, DescribeOffers(offers))
		}

		next, _, err := m.Apply(state, actor, a)
		if err != nil {
			return state, res, fmt.Errorf(
				"step %d: %s was offered %+v but the engine refused it: %v\n%s",
				step, actor, a, err, DescribeOffers(offers))
		}
		state = next
		res.Actions++
		res.Verbs[a.Verb]++
		if opts.OnAction != nil {
			opts.OnAction(actor, a)
		}
	}

	done, winners, err := m.Finished(state)
	if err == nil {
		res.Finished, res.Winners = done, winners
	}
	return state, res, nil
}

// whoseTurn finds the player the module is currently offering something to.
//
// The runtime has no "current turn" field to read — that is game state, and
// game state is opaque. So it asks: exactly one player should have an enabled
// offer. That this works for both games without either declaring a turn model
// is itself a small result.
func whoseTurn(m GameModule, state State, players []PlayerRef) (string, error) {
	var active []string
	for _, p := range players {
		offers, err := m.LegalActions(state, p.ID)
		if err != nil {
			return "", fmt.Errorf("LegalActions(%s): %w", p.ID, err)
		}
		for _, o := range offers {
			if o.Enabled {
				active = append(active, p.ID)
				break
			}
		}
	}
	switch len(active) {
	case 1:
		return active[0], nil
	case 0:
		return "", fmt.Errorf("no player has any enabled offer — the game is stuck")
	default:
		sort.Strings(active)
		return "", fmt.Errorf("more than one player has enabled offers at once: %v", active)
	}
}

// ChooseAction picks a move from the offers alone.
//
// Preference order is a UI choice, not a rule: a client may reasonably prefer
// playing to drawing. Every branch is gated on an offer the module marked
// enabled, and the concrete input comes from that offer's own selector and
// parameter declarations.
//
// Exported because it is not only test apparatus: the runtime drives AI seats
// with it, which is how a module gets a playable opponent the day it is
// registered, with no AI of its own.
func ChooseAction(offers []ActionOffer, prefer []string) (Action, bool) {
	for _, verb := range prefer {
		for i := range offers {
			if offers[i].Verb != verb {
				continue
			}
			if a, ok := SubmissionFor(offers[i]); ok {
				return a, true
			}
		}
	}
	for i := range offers {
		if a, ok := SubmissionFor(offers[i]); ok {
			return a, true
		}
	}
	return Action{}, false
}

// SubmissionFor builds the concrete action an offer describes, using only what
// the offer itself declares.
//
// This is the whole discipline a UI shell — or a bot, or this driver — is held
// to, in one function: cards from the offer's own selector, the target it
// names, and a value for every parameter it declares. Nothing here knows what a
// suit, a meld or a raise is.
//
// Reports false when the offer is enabled but describes no submission anything
// could send, which is a module bug and worth surfacing as one.
func SubmissionFor(o ActionOffer) (Action, bool) {
	if !o.Enabled {
		return Action{}, false
	}
	// A composite offer is a combination only a person can compose — a rummy
	// meld shape. The module says so rather than leaving it to be inferred, so
	// this can decline honestly instead of submitting an illegal fragment.
	if o.Composite {
		return Action{}, false
	}
	a := Action{OfferID: o.ID, Verb: o.Verb}

	// If the offer wants cards, take as many as it says it needs, from the
	// front of the list it says it will accept.
	//
	// As many, not one: an offer that ships a concrete combination — Canasta's
	// melds, where a candidate is n cards of a single rank — declares MinCards
	// equal to that combination's size and orders the list so the prefix is the
	// combination. Sending only the first card would submit an illegal fragment
	// of a legal move. Modules whose offers take a single card set MinCards to
	// 1 and are unaffected.
	if o.Source != nil && o.Source.MinCards > 0 {
		if len(o.Source.Cards) < o.Source.MinCards {
			return Action{}, false
		}
		a.Cards = append([]string(nil), o.Source.Cards[:o.Source.MinCards]...)
	}
	if o.Target != nil && o.Target.MeldID != "" {
		a.Target = o.Target.MeldID
	}

	for _, p := range o.Params {
		v, ok := defaultParam(p)
		if !ok {
			return Action{}, false
		}
		if a.Params == nil {
			a.Params = map[string]string{}
		}
		a.Params[p.Name] = v
	}
	return a, true
}

// defaultParam picks a legal value for a declared parameter.
//
// For a choice, the first one; for a number, the offer's own default, or the
// bottom of its range. Deliberately the *smallest* legal number rather than
// anything cleverer: in poker that is the minimum raise, which is a real move
// and not a reckless one, and choosing a value the module did not sanction
// would make this a player with opinions rather than a reader of offers.
func defaultParam(p ParamSpec) (string, bool) {
	switch p.Kind {
	case ParamKindInt:
		v := p.Default
		if v < p.Min || v > p.Max {
			v = p.Min
		}
		return strconv.Itoa(v), true
	default:
		if len(p.Choices) == 0 {
			return "", false
		}
		return p.Choices[0].Value, true
	}
}

// DescribeOffers renders an offer list for a failure message.
func DescribeOffers(offers []ActionOffer) string {
	out := ""
	for _, o := range offers {
		state := "off"
		if o.Enabled {
			state = "ON "
		}
		cards := ""
		if o.Source != nil && len(o.Source.Cards) > 0 {
			cards = fmt.Sprintf(" cards=%v", o.Source.Cards)
		}
		params := ""
		if len(o.Params) > 0 {
			params = fmt.Sprintf(" params=%d", len(o.Params))
		}
		out += fmt.Sprintf("  %s %-22s %-24s%s%s\n", state, o.ID, o.WhyNot, cards, params)
	}
	return out
}
