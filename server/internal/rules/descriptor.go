package rules

// The module descriptor: what this game is called, how many can play, which
// variations it ships, and — the part that earns its keep — what a lobby is
// allowed to configure.
//
// Before this existed, the option space lived in the client as
// `MELD_MINS = [0, 35, 50, 70]` and `DISCARD_LOCK_ROUNDS = [0, 1, 2, 3]`,
// next to a hardcoded list of profile names and a hand-written paragraph
// restating one profile's constants. Adding a knob, a choice, or a third
// variation meant editing a client. Declaring it here makes all of that a
// server-only change, which is the same move LegalActions made for legality
// (see docs/extensibility-plan.md Phase 2.1).
//
// The descriptor is also *authoritative*, not decorative: ValidateOptions
// rejects a value the schema does not declare, so a lobby cannot smuggle in
// a setting simply by not rendering the control for it.

// OptionType is the shape of a configurable option. Only one kind is needed
// so far; a future bool/free-int knob adds a case here and a renderer in the
// clients, not a new concept.
type OptionType string

// OptionEnumInt: an integer chosen from a fixed, declared set.
const OptionEnumInt OptionType = "enum_int"

// OptionChoice is one selectable value plus how to name it. Label is a
// rendered string today; Phase 2.2 turns it into a message key, at which
// point only this struct and the locale bundle change.
type OptionChoice struct {
	Value int    `json:"value"`
	Label string `json:"label"`
}

// OptionSpec declares one knob a lobby may set.
type OptionSpec struct {
	// Name matches the JSON field the client sends back on create/settings
	// (CreateGameReq), so a client can drive the whole form generically.
	Name    string         `json:"name"`
	Type    OptionType     `json:"type"`
	Label   string         `json:"label"`
	Help    string         `json:"help,omitempty"`
	Choices []OptionChoice `json:"choices"`
}

// Allows reports whether v is one of this option's declared choices.
func (o OptionSpec) Allows(v int) bool {
	for _, c := range o.Choices {
		if c.Value == v {
			return true
		}
	}
	return false
}

// ProfileSpec is one shipped variation, with the fully-resolved ruleset it
// starts from. Shipping the resolved config (rather than just a name) is
// what lets a lobby show "13-card deal, 3+ card runs, ..." for a profile the
// client has never heard of, reusing the same summary renderer the in-game
// rules panel uses.
type ProfileSpec struct {
	ID    string      `json:"id"`
	Label string      `json:"label"`
	Rules RulesConfig `json:"-"` // mapped to the wire shape by the game package
}

// Contract is what this profile requires to go down on its first deal.
//
// A rotating profile's requirement changes from deal to deal, so a lobby
// cannot state one figure for the whole match — but it can state the one
// players meet first, and taking it from the ruleset is what stops the client
// from carrying the rotation table itself.
func (p ProfileSpec) Contract() ContractRequirement {
	return ResolveConfig(p.Rules).ContractFor(1)
}

// ModuleDescriptor is the whole self-description of this game module. It is
// the thing a lobby screen renders, and in Phase 3 it becomes
// GameModule.Descriptor().
type ModuleDescriptor struct {
	ID         string        `json:"id"`
	Label      string        `json:"label"`
	MinPlayers int           `json:"minPlayers"`
	MaxPlayers int           `json:"maxPlayers"`
	Profiles   []ProfileSpec `json:"profiles"`
	Options    []OptionSpec  `json:"options"`
}

// Option returns the named option's spec, or nil.
func (d ModuleDescriptor) Option(name string) *OptionSpec {
	for i := range d.Options {
		if d.Options[i].Name == name {
			return &d.Options[i]
		}
	}
	return nil
}

// Profile returns the named profile's spec, or nil.
func (d ModuleDescriptor) Profile(id string) *ProfileSpec {
	for i := range d.Profiles {
		if d.Profiles[i].ID == id {
			return &d.Profiles[i]
		}
	}
	return nil
}

// Option names. These are the JSON field names a client sends back, so they
// are part of the wire contract and are referenced rather than retyped.
const (
	OptInitialMeldMinimum  = "initialMeldMinimum"
	OptDiscardDrawMinRound = "discardDrawMinRound"
	OptRequireCleanRun     = "requireCleanRun"
	OptDealStarter         = "dealStarter"
)

// DealStarterOpt/ParseDealStarterOpt carry a DealStarterMode over the
// enum_int wire shape, the same way BoolOpt carries a bool.
func DealStarterOpt(mode DealStarterMode) int {
	if mode == DealStarterWinner {
		return OptOn
	}
	return OptOff
}

func ParseDealStarterOpt(v int) DealStarterMode {
	if v == OptOn {
		return DealStarterWinner
	}
	return DealStarterRotate
}

// OptOff/OptOn are the two values of a yes/no option carried over the
// enum_int wire shape. A dedicated bool OptionType would mean a new renderer
// in every client for a control that is already a two-choice cycle, so a
// declared pair of choices does the job with the machinery that exists.
const (
	OptOff = 0
	OptOn  = 1
)

// BoolOpt maps a rule flag onto the wire values above.
func BoolOpt(on bool) int {
	if on {
		return OptOn
	}
	return OptOff
}

// Descriptor describes this module. Pure and allocation-free to call.
//
// Player range comes from the deck builder: DeckCountForPlayers has explicit
// cases for 2..8 and falls back to a 2-deck guess outside that, so 2..8 is
// the range that is actually supported rather than merely tolerated.
func Descriptor() ModuleDescriptor {
	return ModuleDescriptor{
		ID:         "zolik",
		Label:      "Žolíky",
		MinPlayers: MinPlayers,
		MaxPlayers: MaxPlayers,
		Profiles: []ProfileSpec{
			{ID: "zolik_classic", Label: "Žolík Classic", Rules: ProfileZolikClassic},
			{ID: "continental", Label: "Continental", Rules: ProfileContinental},
		},
		Options: []OptionSpec{
			{
				Name:  OptInitialMeldMinimum,
				Type:  OptionEnumInt,
				Label: "Meld value",
				Help:  "Minimum natural points across your first meld(s) before you are down.",
				Choices: []OptionChoice{
					{Value: 0, Label: "Off"},
					{Value: 35, Label: "35"},
					{Value: 50, Label: "50"},
					{Value: 70, Label: "70"},
				},
			},
			{
				Name:  OptDiscardDrawMinRound,
				Type:  OptionEnumInt,
				Label: "Discard pickup",
				Help:  "The lap from which the discard pile may be drawn from.",
				// 0 and 1 both mean "no restriction" to the engine (see
				// ValidateDraw), but offering both would put two chips
				// labelled "Open" in the cycle, so tapping would look broken.
				// Only the canonical 0 is offered.
				Choices: []OptionChoice{
					{Value: 0, Label: "Open"},
					{Value: 2, Label: "Round 2"},
					{Value: 3, Label: "Round 3"},
				},
			},
			{
				Name:  OptRequireCleanRun,
				Type:  OptionEnumInt,
				Label: "Clean run",
				Help:  "Whether a joker-free run must be on the table before you count as down.",
				Choices: []OptionChoice{
					{Value: OptOn, Label: "Required"},
					{Value: OptOff, Label: "Off"},
				},
			},
			{
				Name:  OptDealStarter,
				Type:  OptionEnumInt,
				Label: "Deal starter",
				Help:  "Who leads the next deal: the table rotating through every seat, or whoever just went out.",
				Choices: []OptionChoice{
					{Value: DealStarterOpt(DealStarterRotate), Label: "Rotate"},
					{Value: DealStarterOpt(DealStarterWinner), Label: "Winner leads"},
				},
			},
		},
	}
}

// ValidateOptions rejects any supplied value the schema does not declare, so
// the descriptor is the single source of truth for the option space rather
// than a hint the server also has to remember to enforce. A nil pointer means
// "not being set" and is always fine.
func ValidateOptions(opts map[string]*int) error {
	d := Descriptor()
	for name, v := range opts {
		if v == nil {
			continue
		}
		spec := d.Option(name)
		if spec == nil {
			return RulesError{Code: ErrInvalidMeld, Message: "unknown option: " + name}
		}
		if !spec.Allows(*v) {
			return RulesError{
				Code:    ErrInvalidMeld,
				Message: "value not allowed for option " + name,
			}
		}
	}
	return nil
}
