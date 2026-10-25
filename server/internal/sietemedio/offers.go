package sietemedio

import (
	"strconv"

	"zolik/server/internal/module"
)

// Offer IDs. Three decisions in the whole game, and not one of them takes a
// card: everything is dealt to you, so the only input any offer needs is a
// number, and only the stake needs that.
const (
	OfferStake = "stake"
	OfferHit   = "hit"
	OfferStand = "stand"
)

// LegalActions answers "what may this player do right now?".
//
// Every enabled or disabled decision comes from probing the real engine and
// reading back its code, so the list is Apply's own answer asked in advance and
// cannot drift from it. What it adds on top is the arithmetic a player would
// otherwise do by eye — in this game, the stake range, which depends on what
// the bank can still answer for and is therefore not something a client could
// work out at all.
func (m *Module) LegalActions(raw module.State, playerID string) ([]module.ActionOffer, error) {
	offers, err := m.legalActions(raw, playerID)
	if err != nil {
		return nil, err
	}
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}
	// Getting up, to anybody still seated — last, so it is never the first
	// control a hand falls on.
	if seat := s.seat(playerID); seat != nil && !seat.Out && !seat.Leaving && s.Status == "active" {
		offers = append(offers, module.LeaveOffer(true, ""))
	}
	return offers, nil
}

func (m *Module) legalActions(raw module.State, playerID string) ([]module.ActionOffer, error) {
	s, err := decode(raw)
	if err != nil {
		return nil, err
	}

	if s.Break.Open {
		return s.Break.Offers(order(s), playerID), nil
	}
	if s.Status != "active" {
		return m.explained(s, nil, placeholders(ErrGameNotActive)), nil
	}

	seat := s.seat(playerID)
	if seat == nil {
		return m.explained(s, nil, placeholders(module.ErrNotSeated)), nil
	}

	switch s.Phase {
	case phaseStakes:
		return m.explained(s, seat, m.stakeOffers(raw, s, seat)), nil
	case phasePlay, phaseBank:
		if s.Current < 0 || s.Seats[s.Current].PlayerID != playerID {
			return m.explained(s, seat, placeholders(ErrNotYourTurn)), nil
		}
		return m.explained(s, seat, m.playOffers(raw, s, seat)), nil
	}
	return m.explained(s, seat, placeholders(ErrGameNotActive)), nil
}

// explained puts the rule and the way out on every disabled offer before it
// leaves this package — see remedy.go. Wrapped around each branch rather than
// applied at one exit because every branch is its own return, and a phase added
// later that forgot the call would ship bare codes silently.
func (m *Module) explained(s *GameState, seat *Seat, offers []module.ActionOffer) []module.ActionOffer {
	m.annotate(s, seat, offers)
	return offers
}

// placeholders is the whole control set, off, with the one reason that applies.
// A disabled offer stays on screen with its reason — an omitted one is
// indistinguishable from a client bug.
func placeholders(why string) []module.ActionOffer {
	return []module.ActionOffer{
		{ID: OfferStake, Verb: VerbStake, LabelKey: "sietemedio.offer.stake", WhyNot: why},
		{ID: OfferHit, Verb: VerbHit, LabelKey: "sietemedio.offer.hit", WhyNot: why},
		{ID: OfferStand, Verb: VerbStand, LabelKey: "sietemedio.offer.stand", WhyNot: why},
	}
}

func (m *Module) probe(raw module.State, playerID string, a module.Action) (bool, string) {
	if _, _, err := m.Apply(raw, playerID, a); err != nil {
		return false, module.CodeOf(err)
	}
	return true, ""
}

// stakeOffers is the one place this game asks for a number.
//
// The range is declared rather than enumerated, exactly as poker's raise is.
// What is particular to this game is the top of it: a stake is any figure from
// the table minimum up to whichever is smaller of what this seat holds and
// what the bank can still answer for. The second of those is another player's
// stack, which is why no client could compute this range and why it is pushed
// rather than left to be derived.
//
// The default is the table minimum — a real move, and not a reckless one, so a
// client that presses the control unchanged has not shoved.
func (m *Module) stakeOffers(raw module.State, s *GameState, seat *Seat) []module.ActionOffer {
	i := s.seatIndex(seat.PlayerID)
	stake := module.ActionOffer{ID: OfferStake, Verb: VerbStake, LabelKey: "sietemedio.offer.stake"}
	stake.Enabled, stake.WhyNot = m.probe(raw, seat.PlayerID, module.Action{
		OfferID: OfferStake, Verb: VerbStake,
		Params: map[string]string{ParamAmount: strconv.Itoa(s.MinStake)},
	})
	if stake.Enabled {
		top := maxStakeFor(s, i)
		stake.Params = []module.ParamSpec{{
			Name:     ParamAmount,
			Kind:     module.ParamKindInt,
			LabelKey: "sietemedio.prompt.stakeAmount",
			Min:      s.MinStake,
			Max:      top,
			Step:     1,
			Default:  s.MinStake,
			Headline: true,
			Choices:  stakeQuickChoices(s.MinStake, top),
		}}
		stake.Facts = []module.Fact{
			{LabelKey: "sietemedio.fact.tableMinimum", Value: strconv.Itoa(s.MinStake),
				Params: map[string]any{"n": s.MinStake}},
			{LabelKey: "sietemedio.fact.bankCovers", Value: strconv.Itoa(top),
				Params: map[string]any{"n": top}},
		}
	}
	stake.RuleIDs = ruleIDsFor(s, stake.WhyNot)

	offers := []module.ActionOffer{stake}
	for _, o := range placeholders(ErrWrongPhase) {
		if o.ID != OfferStake {
			offers = append(offers, o)
		}
	}
	return offers
}

// stakeQuickChoices names a couple of stakes a player reasons about without
// doing the multiplication themselves — twice the minimum, and everything the
// bank will answer for — alongside the bare range ParamKindInt already carries.
//
// Every LabelKey is a literal string in its own module.ParamChoice{...}, since
// dump-keys finds label keys by reading the source rather than by running it.
func stakeQuickChoices(minStake, top int) []module.ParamChoice {
	if top < minStake {
		return nil
	}
	clamp := func(n int) int {
		if n < minStake {
			return minStake
		}
		if n > top {
			return top
		}
		return n
	}
	doubleMin := clamp(minStake * 2)

	choices := []module.ParamChoice{{Value: strconv.Itoa(top), LabelKey: "sietemedio.quick.theLot"}}
	if doubleMin != top {
		choices = append([]module.ParamChoice{
			{Value: strconv.Itoa(doubleMin), LabelKey: "sietemedio.quick.doubleMin"},
		}, choices...)
	}
	return choices
}

// playOffers is the hand in front of this seat: ask for a card, or stand.
func (m *Module) playOffers(raw module.State, s *GameState, seat *Seat) []module.ActionOffer {
	offers := make([]module.ActionOffer, 0, 3)
	for _, spec := range []struct {
		id, verb, label string
	}{
		{OfferHit, VerbHit, "sietemedio.offer.hit"},
		{OfferStand, VerbStand, "sietemedio.offer.stand"},
	} {
		o := module.ActionOffer{ID: spec.id, Verb: spec.verb, LabelKey: spec.label}
		o.Enabled, o.WhyNot = m.probe(raw, seat.PlayerID, module.Action{OfferID: spec.id, Verb: spec.verb})
		o.RuleIDs = ruleIDsFor(s, o.WhyNot)
		offers = append(offers, o)
	}

	// What the seat is sitting on, and the most a card could add without going
	// over. Pushed rather than derived: both are the halves arithmetic this
	// module exists to own, and a client working out "a figure is the only card
	// that does not bust me" would be re-implementing the rule.
	if len(seat.Cards) > 0 {
		total := Total(seat.Cards)
		facts := []module.Fact{{
			LabelKey: "sietemedio.fact.yourTotal", Value: halvesText(total),
			Params: map[string]any{"total": halvesText(total)},
		}}
		if room := Target - total; room > 0 {
			facts = append(facts, module.Fact{
				LabelKey: "sietemedio.fact.roomLeft", Value: halvesText(room),
				Params: map[string]any{"total": halvesText(room)},
			})
		}
		for i := range offers {
			if offers[i].ID == OfferHit {
				offers[i].Facts = facts
			}
		}
	}

	for _, o := range placeholders(ErrWrongPhase) {
		if o.ID == OfferStake {
			offers = append(offers, o)
		}
	}
	return offers
}

// ruleIDsFor points a refusal at the written rules that justify it at this
// table.
//
// It states no rule of its own: every id here is the label key of a sentence
// Rules() builds for the same table, under the same condition, so a refusal
// gets a config-accurate explanation in whatever locale the reader has.
func ruleIDsFor(s *GameState, code string) []string {
	switch code {
	// Where in a round you are, which is what WRONG_PHASE is always about and
	// what a code on its own can never say: the staking has closed, or the
	// cards are not out yet, or the bank is still drawing.
	case ErrNotYourTurn, ErrWrongPhase:
		return []string{"sietemedio.rules.roundOrder"}
	case ErrNotInRound:
		return []string{"sietemedio.rules.roundOrder", "sietemedio.rules.bankCovers", "sietemedio.rules.brokeOut"}
	case ErrStakeTooSmall:
		return []string{"sietemedio.rules.minStake"}
	case ErrAlreadyStaked:
		return []string{"sietemedio.rules.oneStakePerRound"}
	case ErrNotEnoughChips:
		return []string{"sietemedio.rules.stakeFromStack"}
	case ErrAmountRequired:
		return []string{"sietemedio.rules.minStake", "sietemedio.rules.stakeFromStack"}
	case ErrBankCannotCover:
		return []string{"sietemedio.rules.bankCovers"}
	case ErrBankerDoesNotStake:
		return []string{"sietemedio.rules.bankDoesNotStake"}
	case ErrAlreadyLeft:
		return []string{"sietemedio.rules.leave"}
	default:
		return nil
	}
}
