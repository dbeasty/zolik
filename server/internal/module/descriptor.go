package module

// The module descriptor: what a game is called, how many can play, which
// variations it ships, and what a lobby may configure.
//
// Generalised from the Žolíky-specific version added in Phase 2.1. The shape
// survived contact with a second game unchanged, which is mild evidence it is
// the right shape — the option space, the variations and the player range are
// things every card game has, unlike melds or tricks.

// OptionType is the shape of a configurable option.
type OptionType string

// OptionEnumInt: an integer chosen from a fixed, declared set.
const OptionEnumInt OptionType = "enum_int"

// OptionChoice is one selectable value and how to name it.
type OptionChoice struct {
	Value int    `json:"value"`
	Label string `json:"label"`
}

// OptionSpec declares one knob a lobby may set.
type OptionSpec struct {
	// Name is the key the client sends back, so a client can drive the whole
	// form generically without knowing what any option means.
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

// VariationSpec is one shipped ruleset variation.
//
// Defaults carries that variation's starting value for every option the
// module declares, so a lobby can show what it is about to create without
// asking the server to resolve anything.
type VariationSpec struct {
	ID       string         `json:"id"`
	Label    string         `json:"label"`
	Summary  []Fact         `json:"summary,omitempty"`
	Defaults map[string]int `json:"defaults,omitempty"`
}

// ModuleDescriptor is a game's whole self-description.
type ModuleDescriptor struct {
	ID         string          `json:"id"`
	Label      string          `json:"label"`
	MinPlayers int             `json:"minPlayers"`
	MaxPlayers int             `json:"maxPlayers"`
	Variations []VariationSpec `json:"variations,omitempty"`
	Options    []OptionSpec    `json:"options,omitempty"`
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

// Variation returns the named variation's spec, or nil.
func (d ModuleDescriptor) Variation(id string) *VariationSpec {
	for i := range d.Variations {
		if d.Variations[i].ID == id {
			return &d.Variations[i]
		}
	}
	return nil
}

// ValidateOptions rejects any value the schema does not declare, so a
// descriptor is authoritative rather than decorative. A nil pointer means
// "not being set" and is always fine.
func (d ModuleDescriptor) ValidateOptions(opts map[string]*int) error {
	for name, v := range opts {
		if v == nil {
			continue
		}
		spec := d.Option(name)
		if spec == nil {
			return Error{Code: "UNKNOWN_OPTION", Message: "unknown option: " + name}
		}
		if !spec.Allows(*v) {
			return Error{Code: "OPTION_NOT_ALLOWED", Message: "value not allowed for option " + name}
		}
	}
	return nil
}

// Registry holds the modules this server can host.
//
// In-process and compile-time, per the blueprint's open decision 2: a
// plugin/WASM boundary would let third parties add games at a large cost in
// complexity, and there is no second author yet.
type Registry struct {
	byID map[string]GameModule
	ids  []string
}

func NewRegistry(mods ...GameModule) *Registry {
	r := &Registry{byID: map[string]GameModule{}}
	for _, m := range mods {
		id := m.Descriptor().ID
		if _, dup := r.byID[id]; dup {
			panic("module registered twice: " + id)
		}
		r.byID[id] = m
		r.ids = append(r.ids, id)
	}
	return r
}

// Get returns the module with this id, or nil.
func (r *Registry) Get(id string) GameModule { return r.byID[id] }

// IDs lists registered modules in registration order.
func (r *Registry) IDs() []string { return append([]string(nil), r.ids...) }

// Descriptors lists every module's self-description, for a game-picker.
func (r *Registry) Descriptors() []ModuleDescriptor {
	out := make([]ModuleDescriptor, 0, len(r.ids))
	for _, id := range r.ids {
		out = append(out, r.byID[id].Descriptor())
	}
	return out
}
