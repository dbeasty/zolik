package module

import "time"

// Getting up from the table.
//
// At a cash table — a poker game played for the chips in front of you rather
// than to the last one standing, and every Blackjack table — a player may
// leave whenever they like and take their chips with them: what they leave
// with is their result. The table plays on without them while two seats (or,
// at Blackjack, one) are still dealt in.
//
// Declared here rather than per game because it is one verb with one name on
// the wire, and one option, rendered by a client that does not know which
// game it is configuring.

// VerbLeave is getting up from the table with your chips.
const VerbLeave = "leave"

// OfferLeave is the id of the one leave offer a module lists.
const OfferLeave = "leave"

// LeaveOffer is the offer to get up from the table. Manual, so nothing but
// the person themselves ever chooses it, and listed last by every module so a
// control for it is never the first one a hand falls on.
func LeaveOffer(enabled bool, whyNot string) ActionOffer {
	return ActionOffer{
		ID:       OfferLeave,
		Verb:     VerbLeave,
		Enabled:  enabled,
		WhyNot:   whyNot,
		LabelKey: "offer.leave",
		Manual:   true,
	}
}

// OptLeaveAfterAway is how many seconds a seat at a cash table may be away —
// its connection dropped, and sat out since — before it is cashed out for
// its player. Zero is never.
const OptLeaveAfterAway = "leaveAfterAway"

// LeaveAfterAwayDefault is the wait when a lobby has not chosen one: long
// enough that a lost connection is back, short enough that a phone left on
// the train does not post blinds all evening.
const LeaveAfterAwayDefault = 300

// LeaveAfterAwayOption is the ready-made spec a module drops into its
// descriptor.
func LeaveAfterAwayOption() OptionSpec {
	return OptionSpec{
		Name:  OptLeaveAfterAway,
		Type:  OptionEnumInt,
		Label: "Leave if away for",
		Help:  "At a cash table, how long a player whose connection dropped is sat out before they leave the table with their chips.",
		Choices: []OptionChoice{
			{Value: 120, Label: "2 minutes"},
			{Value: 300, Label: "5 minutes"},
			{Value: 600, Label: "10 minutes"},
			{Value: 0, Label: "Never"},
		},
	}
}

// LeaveAfterAway reads the option: how long to wait, or zero for never.
func (c MatchConfig) LeaveAfterAway() time.Duration {
	return time.Duration(c.Opt(OptLeaveAfterAway, LeaveAfterAwayDefault)) * time.Second
}

// DeclaresLeave reports whether a module lets a player get up from the table.
func DeclaresLeave(m GameModule) bool {
	return m != nil && m.Descriptor().Option(OptLeaveAfterAway) != nil
}

// Live reports whether an offer is a move this seat is being asked to make
// now: enabled, and not a manual one like getting up, which is open to a
// seated player all game and so says nothing about whose turn it is.
func (o ActionOffer) Live() bool { return o.Enabled && !o.Manual }
