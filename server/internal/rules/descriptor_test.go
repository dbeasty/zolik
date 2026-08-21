package rules

import "testing"

// The descriptor's whole value is that a client can trust it instead of
// carrying its own copy of the option space. These tests hold it to that:
// it must describe every profile the engine actually ships, its declared
// defaults must be reachable, and the values it declares must be exactly the
// values the server accepts.

func TestDescriptor_CoversEveryShippedProfile(t *testing.T) {
	d := Descriptor()
	// ResolveProfile is the engine's own list. A profile it knows but the
	// descriptor omits is invisible to every lobby — the exact failure this
	// test exists to catch when a third variation is added.
	for _, name := range []string{"continental", "zolik_classic"} {
		spec := d.Profile(name)
		if spec == nil {
			t.Fatalf("descriptor omits profile %q", name)
		}
		want := ResolveProfile(name)
		if spec.Rules != want {
			t.Errorf("profile %q describes a different ruleset than ResolveProfile returns:\n got %+v\nwant %+v",
				name, spec.Rules, want)
		}
		if spec.Label == "" {
			t.Errorf("profile %q has no label", name)
		}
	}
}

func TestDescriptor_ProfileDefaultsAreThemselvesLegalOptions(t *testing.T) {
	// A lobby starts on a profile's own defaults. If a profile ships a value
	// the option schema does not allow, the first settings write the host
	// makes would be rejected — a lobby that cannot save what it is already
	// showing.
	d := Descriptor()
	for _, p := range d.Profiles {
		for _, tc := range []struct {
			option string
			value  int
		}{
			{OptInitialMeldMinimum, p.Rules.InitialMeldMinimum},
			{OptDiscardDrawMinRound, p.Rules.DiscardDrawMinRound},
		} {
			spec := d.Option(tc.option)
			if spec == nil {
				t.Fatalf("descriptor declares no option %q", tc.option)
			}
			if !spec.Allows(tc.value) {
				t.Errorf("profile %q defaults %s=%d, which the option schema does not allow (choices: %+v)",
					p.ID, tc.option, tc.value, spec.Choices)
			}
		}
	}
}

func TestDescriptor_EveryOptionIsRenderable(t *testing.T) {
	// A client renders the form straight from this, so anything missing here
	// is a blank or unlabelled control on screen.
	for _, o := range Descriptor().Options {
		if o.Name == "" || o.Label == "" {
			t.Errorf("option %+v is missing a name or label", o)
		}
		if o.Type != OptionEnumInt {
			t.Errorf("option %q has unrenderable type %q", o.Name, o.Type)
		}
		if len(o.Choices) == 0 {
			t.Errorf("option %q offers no choices", o.Name)
		}
		seenLabel := map[string]int{}
		seenValue := map[int]bool{}
		for _, c := range o.Choices {
			if c.Label == "" {
				t.Errorf("option %q has an unlabelled choice %d", o.Name, c.Value)
			}
			// The client renders these as one chip that cycles, so two
			// choices sharing a label make tapping look like a no-op.
			if prev, dup := seenLabel[c.Label]; dup {
				t.Errorf("option %q labels both %d and %d %q — tapping between them looks broken",
					o.Name, prev, c.Value, c.Label)
			}
			seenLabel[c.Label] = c.Value
			if seenValue[c.Value] {
				t.Errorf("option %q offers value %d twice", o.Name, c.Value)
			}
			seenValue[c.Value] = true
		}
	}
}

func TestValidateOptions_AcceptsExactlyWhatTheSchemaDeclares(t *testing.T) {
	d := Descriptor()
	for _, o := range d.Options {
		for _, c := range o.Choices {
			v := c.Value
			if err := ValidateOptions(map[string]*int{o.Name: &v}); err != nil {
				t.Errorf("declared choice %s=%d was rejected: %v", o.Name, v, err)
			}
		}
	}

	// ...and nothing else. 45 is a plausible-looking meld floor that is not
	// on the list; accepting it would mean the advertised option space is a
	// suggestion rather than the contract.
	bad := 45
	if err := ValidateOptions(map[string]*int{OptInitialMeldMinimum: &bad}); err == nil {
		t.Error("an undeclared value was accepted")
	}

	unknown := 1
	if err := ValidateOptions(map[string]*int{"noSuchOption": &unknown}); err == nil {
		t.Error("an unknown option name was accepted")
	}
}

func TestValidateOptions_NilMeansNotBeingSet(t *testing.T) {
	// The lobby sends only the knob it is changing, so a nil must never be
	// mistaken for "0".
	if err := ValidateOptions(map[string]*int{
		OptInitialMeldMinimum:  nil,
		OptDiscardDrawMinRound: nil,
	}); err != nil {
		t.Errorf("unset options were rejected: %v", err)
	}
}

func TestDescriptor_PlayerRangeMatchesTheDeckBuilder(t *testing.T) {
	// The range is only meaningful if the deck builder actually has a rule
	// for those sizes; outside 2..8 it falls back to a 2-deck guess, which
	// is tolerated rather than supported.
	d := Descriptor()
	if d.MinPlayers != 2 || d.MaxPlayers != 8 {
		t.Fatalf("player range = %d..%d, want 2..8", d.MinPlayers, d.MaxPlayers)
	}
	for n := d.MinPlayers; n <= d.MaxPlayers; n++ {
		if decks := DeckCountForPlayers(n); decks < 2 {
			t.Errorf("%d players resolves to %d decks", n, decks)
		}
		// Every advertised size must be able to deal every profile's hand.
		for _, p := range d.Profiles {
			need := n * p.Rules.DealSize
			if have := len(BuildDeck(n)); have <= need {
				t.Errorf("%d players under %q: deck of %d cannot deal %d cards and still leave a draw pile",
					n, p.ID, have, need)
			}
		}
	}
}
