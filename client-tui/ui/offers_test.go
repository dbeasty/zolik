package ui

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"zolik/client-tui/api"
)

func stateWith(offers ...api.ActionOffer) api.GameState {
	// Only LegalActions is populated: the point of these lookups is that they
	// read nothing else. If one ever starts consulting Phase or RoundReqMet,
	// these tests keep passing but the invariant test at the bottom fails.
	return api.GameState{LegalActions: offers}
}

func TestCan_ReadsTheServersAnswer(t *testing.T) {
	s := stateWith(
		api.ActionOffer{ID: OfferDrawDeck, Verb: "draw", Enabled: true},
		api.ActionOffer{ID: OfferDrawDiscard, Verb: "draw", Enabled: false, WhyNot: "DISCARD_LOCKED"},
	)
	if !can(s, OfferDrawDeck) {
		t.Error("an enabled offer should be reported as available")
	}
	if can(s, OfferDrawDiscard) {
		t.Error("a disabled offer should not be reported as available")
	}
	if got := whyNot(s, OfferDrawDiscard); got != "DISCARD_LOCKED" {
		t.Errorf("whyNot = %q, want DISCARD_LOCKED", got)
	}
	if got := whyNot(s, OfferDrawDeck); got != "" {
		t.Errorf("an enabled offer should carry no reason, got %q", got)
	}
}

func TestCan_UnknownAndMissingOffersAreUnavailable(t *testing.T) {
	// The case that actually happens: this client built after Phase 1,
	// talking to a server built before it. An inert control beats one that
	// sends an action the server will reject.
	if can(api.GameState{}, OfferDiscard) {
		t.Error("a state with no offer list should offer nothing")
	}
	if can(stateWith(), OfferDiscard) {
		t.Error("an offer the server never sent should not be available")
	}
	if got := eligibleCards(stateWith(), OfferDiscard); got != nil {
		t.Errorf("eligibleCards on a missing offer = %v, want nil", got)
	}
}

func TestLayOff_AnsweredPerMeldNotTableWide(t *testing.T) {
	s := stateWith(
		api.ActionOffer{
			ID: layOffOfferID("meld_1"), Verb: "lay_off", Enabled: true,
			Source: &api.Selector{Zone: "hand", Cards: []string{"4H", "9H"}, Placements: []api.Placement{
				{Card: "4H", Positions: []string{"front"}},
				{Card: "9H", Positions: []string{"end"}},
			}},
			Target: &api.Selector{Zone: "meld", MeldID: "meld_1"},
		},
		api.ActionOffer{
			ID: layOffOfferID("meld_2"), Verb: "lay_off", Enabled: false, WhyNot: "INVALID_MELD",
			Target: &api.Selector{Zone: "meld", MeldID: "meld_2"},
		},
	)
	if !canLayOffOnto(s, "meld_1") {
		t.Error("meld_1 accepts a lay-off")
	}
	if canLayOffOnto(s, "meld_2") {
		t.Error("meld_2 accepts nothing in hand and should not be offered")
	}
	if !canLayOffAnywhere(s) {
		t.Error("the table as a whole is still accepting a lay-off")
	}

	if got := positionsForCard(s, "meld_1", "4H"); len(got) != 1 || got[0] != "front" {
		t.Errorf("4H positions = %v, want [front]", got)
	}
	if got := positionsForCard(s, "meld_1", "KS"); got != nil {
		t.Errorf("a card the server did not offer should have no positions, got %v", got)
	}
}

func TestCanLayOffAnywhere_FalseWhenEveryMeldRefuses(t *testing.T) {
	s := stateWith(api.ActionOffer{
		ID: layOffOfferID("meld_1"), Verb: "lay_off", Enabled: false, WhyNot: "ROUND_REQ_NOT_MET",
	})
	if canLayOffAnywhere(s) {
		t.Error("no meld is accepting a lay-off")
	}
}

func TestSwapJoker_OfferedOnlyWhereTheServerOffersIt(t *testing.T) {
	s := stateWith(
		api.ActionOffer{
			ID: swapJokerOfferID("meld_3"), Verb: "swap_joker", Enabled: true,
			Source: &api.Selector{Zone: "hand", Cards: []string{"4S"}},
		},
		api.ActionOffer{
			ID: swapJokerOfferID("meld_1"), Verb: "swap_joker", Enabled: false, WhyNot: "NO_JOKER_IN_MELD",
		},
	)
	if !canSwapJokerOn(s, "meld_3") {
		t.Error("meld_3 has a joker a card in hand can replace")
	}
	if canSwapJokerOn(s, "meld_1") {
		t.Error("meld_1 has no swappable joker")
	}
	if got := eligibleCards(s, swapJokerOfferID("meld_3")); len(got) != 1 || got[0] != "4S" {
		t.Errorf("swap cards = %v, want [4S]", got)
	}
}

func TestReasonText_NeverLeaksARawCode(t *testing.T) {
	if got := reasonText("DISCARD_LOCKED", ""); got != "discard pickup is locked for now" {
		t.Errorf("got %q", got)
	}
	// A code this client has not been taught must fall back, not print
	// SCREAMING_SNAKE at the player.
	if got := reasonText("SOME_FUTURE_CODE", "not available"); got != "not available" {
		t.Errorf("got %q, want the fallback", got)
	}
	if got := reasonText("", "fine"); got != "fine" {
		t.Errorf("got %q, want the fallback", got)
	}
}

func TestAvailableMovesLine(t *testing.T) {
	t.Run("lists what the server offers", func(t *testing.T) {
		s := stateWith(
			api.ActionOffer{ID: OfferDrawDeck, Verb: "draw", Enabled: true},
			api.ActionOffer{ID: OfferDrawDiscard, Verb: "draw", Enabled: false, WhyNot: "DISCARD_LOCKED"},
			api.ActionOffer{ID: OfferLayMeld, Verb: "lay_meld", Enabled: false, WhyNot: "WRONG_PHASE"},
			api.ActionOffer{ID: OfferDiscard, Verb: "discard", Enabled: false, WhyNot: "WRONG_PHASE"},
		)
		got := availableMovesLine(s)
		if !strings.Contains(got, "draw") {
			t.Errorf("expected the available move to be listed: %q", got)
		}
		if strings.Contains(got, "take discard") {
			t.Errorf("a locked pickup should not be listed as available: %q", got)
		}
	})

	t.Run("explains an empty move list", func(t *testing.T) {
		s := stateWith(
			api.ActionOffer{ID: OfferDrawDeck, Verb: "draw", Enabled: false, WhyNot: "NOT_YOUR_TURN"},
			api.ActionOffer{ID: OfferDiscard, Verb: "discard", Enabled: false, WhyNot: "NOT_YOUR_TURN"},
		)
		got := availableMovesLine(s)
		if !strings.Contains(got, "not your turn") {
			t.Errorf("expected the engine's reason to be surfaced: %q", got)
		}
	})

	t.Run("renders nothing at all without an offer list", func(t *testing.T) {
		if got := availableMovesLine(api.GameState{}); got != "" {
			t.Errorf("want an empty line against a pre-Phase-1 server, got %q", got)
		}
	})
}

func TestPreviewLine(t *testing.T) {
	t.Run("renders nothing with no selection", func(t *testing.T) {
		if got := previewLine(nil); got != "" {
			t.Errorf("want empty, got %q", got)
		}
		if got := previewLine(&api.MeldPreview{}); got != "" {
			t.Errorf("want empty for an empty selection, got %q", got)
		}
	})

	t.Run("reports the shape and value the server computed", func(t *testing.T) {
		got := previewLine(&api.MeldPreview{
			Cards: []string{"QH", "QD", "QC"}, Valid: true, Type: "set",
			NaturalValue: 30, Playable: true,
		})
		if !strings.Contains(got, "set") || !strings.Contains(got, "30") {
			t.Errorf("want the shape and value, got %q", got)
		}
	})

	t.Run("flags the floor against the server figure", func(t *testing.T) {
		short := previewLine(&api.MeldPreview{
			Cards: []string{"QH", "QD", "QC"}, Valid: true, Type: "set",
			NaturalValue: 30, InitialMeldMinimum: 50, MeetsMinimum: false, Playable: true,
		})
		if !strings.Contains(short, "min 50") || !strings.Contains(short, "✗") {
			t.Errorf("want a failing floor flag, got %q", short)
		}
		ok := previewLine(&api.MeldPreview{
			Cards: []string{"QH", "QD", "QC"}, Valid: true, Type: "set",
			NaturalValue: 30, InitialMeldMinimum: 25, MeetsMinimum: true, Playable: true,
		})
		if !strings.Contains(ok, "✓") {
			t.Errorf("want a passing floor flag, got %q", ok)
		}
	})

	t.Run("shows the sum when part of the total is already on the table", func(t *testing.T) {
		// 30 against a 50-point floor is short on its own but clears it with
		// 26 already laid. The line has to show the addition, or it reads as
		// a refusal of a legal play.
		got := previewLine(&api.MeldPreview{
			Cards: []string{"QH", "QD", "QC"}, Valid: true, Type: "set",
			NaturalValue: 30, AlreadyLaidValue: 26, InitialMeldMinimum: 50,
			MeetsMinimum: true, Playable: true,
		})
		if !strings.Contains(got, "30 + 26 laid = 56 pts") {
			t.Errorf("want the addition spelled out, got %q", got)
		}
		if !strings.Contains(got, "min 50 ✓") {
			t.Errorf("want the floor still flagged, got %q", got)
		}
	})

	t.Run("omits the addition when nothing is laid yet", func(t *testing.T) {
		got := previewLine(&api.MeldPreview{
			Cards: []string{"QH", "QD", "QC"}, Valid: true, Type: "set",
			NaturalValue: 30, AlreadyLaidValue: 0, InitialMeldMinimum: 50, Playable: true,
		})
		if !strings.Contains(got, "30 pts") || strings.Contains(got, "laid") {
			t.Errorf("want a plain value with nothing on the table, got %q", got)
		}
	})

	t.Run("omits the floor entirely when there is none", func(t *testing.T) {
		got := previewLine(&api.MeldPreview{
			Cards: []string{"QH"}, NaturalValue: 10, InitialMeldMinimum: 0, Playable: false,
			WhyNot: "INVALID_MELD", WhyNotPlayable: "INVALID_MELD",
		})
		if strings.Contains(got, "min") {
			t.Errorf("no floor configured, so none should be shown: %q", got)
		}
	})

	t.Run("does not state the same problem twice", func(t *testing.T) {
		// An invalid selection is unplayable *because* it is invalid. Saying
		// so in both halves of the line reads as two separate faults.
		got := previewLine(&api.MeldPreview{
			Cards: []string{"QH", "5H"}, Valid: false, NaturalValue: 15,
			WhyNot: "INVALID_MELD", Playable: false, WhyNotPlayable: "INVALID_MELD",
		})
		if strings.Count(got, reasonMessages["INVALID_MELD"]) > 1 {
			t.Errorf("reason stated twice: %q", got)
		}
	})

	t.Run("explains a valid meld that still cannot be played", func(t *testing.T) {
		got := previewLine(&api.MeldPreview{
			Cards: []string{"QH", "QD", "QC"}, Valid: true, Type: "set", NaturalValue: 30,
			Playable: false, WhyNotPlayable: "NOT_YOUR_TURN",
		})
		if !strings.Contains(got, reasonMessages["NOT_YOUR_TURN"]) {
			t.Errorf("want the playability reason, got %q", got)
		}
	})
}

// TestOffersFileHoldsNoRuleKnowledge is the acceptance test for this module.
// It exists to end the drift caused by clients re-deriving rules; if a rule
// expression creeps back in, the drift starts over — so assert its absence
// directly rather than trusting review.
func TestOffersFileHoldsNoRuleKnowledge(t *testing.T) {
	src, err := os.ReadFile("offers.go")
	if err != nil {
		t.Fatalf("read offers.go: %v", err)
	}
	// Strip comments: they legitimately discuss the rules this file avoids
	// implementing, and the reasonMessages table legitimately names the
	// engine's error codes.
	var body strings.Builder
	for _, line := range strings.Split(string(src), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		if strings.Contains(line, `":`) && strings.Contains(line, `",`) {
			continue // a reasonMessages entry: a code plus its wording
		}
		body.WriteString(line + "\n")
	}

	for _, forbidden := range []struct {
		what    string
		pattern string
	}{
		{"a phase comparison", `(Phase|phase)\s*==`},
		{"roundReqMet", `RoundReqMet`},
		{"a profile name", `zolik_classic|continental`},
		{"a joker literal", `JOKER`},
		{"a card rank table", `"[2-9TJQKA][HDSC]"`},
	} {
		if regexp.MustCompile(forbidden.pattern).MatchString(body.String()) {
			t.Errorf("offers.go contains %s — rule knowledge belongs on the server, "+
				"as a new field on the offer", forbidden.what)
		}
	}
}
