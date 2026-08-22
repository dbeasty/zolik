package module

import (
	"fmt"
	"sort"
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
	WinnerID string
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
		done, winner, err := m.Finished(state)
		if err != nil {
			return state, res, fmt.Errorf("step %d: Finished: %w", step, err)
		}
		if done {
			res.Finished, res.WinnerID = true, winner
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

		a, ok := chooseAction(offers, opts.Prefer)
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

	done, winner, err := m.Finished(state)
	if err == nil {
		res.Finished, res.WinnerID = done, winner
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

// chooseAction picks a move from the offers alone.
//
// Preference order is a UI choice, not a rule: a client may reasonably prefer
// playing to drawing. Every branch is gated on an offer the module marked
// enabled, and the concrete input comes from that offer's own selector and
// parameter declarations.
func chooseAction(offers []ActionOffer, prefer []string) (Action, bool) {
	try := func(o *ActionOffer) (Action, bool) {
		if o == nil || !o.Enabled {
			return Action{}, false
		}
		a := Action{OfferID: o.ID, Verb: o.Verb}
		// If the offer wants cards, take the first it says it will accept.
		if o.Source != nil && o.Source.MinCards > 0 {
			if len(o.Source.Cards) == 0 {
				return Action{}, false // enabled but nothing concrete to send
			}
			a.Cards = []string{o.Source.Cards[0]}
		}
		if o.Target != nil && o.Target.MeldID != "" {
			a.Target = o.Target.MeldID
		}
		// Fill any declared parameter from its own first choice. The driver
		// has no idea what "suit" means; it only knows the offer declared a
		// parameter and listed what is allowed.
		for _, p := range o.Params {
			if len(p.Choices) == 0 {
				return Action{}, false
			}
			if a.Params == nil {
				a.Params = map[string]string{}
			}
			a.Params[p.Name] = p.Choices[0].Value
		}
		return a, true
	}

	for _, verb := range prefer {
		for i := range offers {
			if offers[i].Verb != verb {
				continue
			}
			if a, ok := try(&offers[i]); ok {
				return a, true
			}
		}
	}
	for i := range offers {
		if a, ok := try(&offers[i]); ok {
			return a, true
		}
	}
	return Action{}, false
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
