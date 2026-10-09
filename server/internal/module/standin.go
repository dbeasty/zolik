package module

import "time"

// Stand-in bots: who plays a person's seat while they are away.
//
// A game that needs everyone — every game that is not DropIn — used to have one
// answer to a dropped phone: pause, and after a couple of minutes with nobody
// coming back, abandon. That is right for a table of two who both walked away,
// and wrong for the three people still sitting at it. So the table waits for a
// while, as it always has, and then a bot plays the seat until its player is
// back. The seat stays theirs throughout; the bot is a stand-in, not a
// replacement.
//
// Declared here, like OptPauseBetweenRounds and OptBotSkill, because it is
// about how a table is staffed rather than about any game's rules: one name on
// the wire, rendered off the descriptor by a client that does not know which
// game it is configuring. A drop-in game does not declare it — it already
// plays on past an absent seat (see DropIn) — and neither does a one-seat
// game, which has nobody to keep waiting.

// OptStandInAfter is how many seconds a table waits for a missing player
// before a bot plays for them. Zero is never: the table waits, as it did
// before this option existed.
const OptStandInAfter = "standInAfter"

// StandInAfterDefault is the wait when a lobby has not chosen one. Long enough
// for a page reload, a lift, a tunnel; short enough that the people still at
// the table are not left staring at a paused board.
const StandInAfterDefault = 60

// StandInOption is the ready-made spec a module drops into its descriptor.
func StandInOption() OptionSpec {
	return OptionSpec{
		Name:  OptStandInAfter,
		Type:  OptionEnumInt,
		Label: "When a player drops",
		Help:  "How long the table waits for a player whose connection dropped before a bot plays for them. They take their seat back as soon as they return.",
		Choices: []OptionChoice{
			{Value: 30, Label: "Bot after 30 seconds"},
			{Value: 60, Label: "Bot after 1 minute"},
			{Value: 120, Label: "Bot after 2 minutes"},
			{Value: 300, Label: "Bot after 5 minutes"},
			{Value: 0, Label: "Wait for them"},
		},
	}
}

// StandInAfter reads the option: how long to wait, or zero for never.
func (c MatchConfig) StandInAfter() time.Duration {
	return time.Duration(c.Opt(OptStandInAfter, StandInAfterDefault)) * time.Second
}

// DeclaresStandIn reports whether a module offers stand-in bots at all.
func DeclaresStandIn(m GameModule) bool {
	return m != nil && m.Descriptor().Option(OptStandInAfter) != nil
}

// StandInRules is the section a module appends to its written rules, so the
// rules screen says what this table does when somebody's connection drops.
func StandInRules(c MatchConfig) RuleSection {
	secs := int(c.StandInAfter() / time.Second)
	if secs == 0 {
		return Section("rules.standIn.section",
			Fact{LabelKey: "rules.standIn.wait"},
		)
	}
	return Section("rules.standIn.section",
		Fact{LabelKey: "rules.standIn.after", Params: map[string]any{"seconds": secs}},
		Fact{LabelKey: "rules.standIn.return"},
		Fact{LabelKey: "rules.standIn.record"},
	)
}
