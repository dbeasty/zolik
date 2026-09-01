package rummytiles

import (
	"reflect"
	"testing"

	"zolik/server/internal/module"
)

func TestValidateGroup_ThreeDistinctColours(t *testing.T) {
	canonical, ok := validateGroup([]string{"7-R", "7-B", "7-O"})
	if !ok {
		t.Fatal("three distinct colours of the same number should be a valid group")
	}
	if len(canonical) != 3 {
		t.Errorf("got %v", canonical)
	}
}

func TestValidateGroup_DuplicateColourIsRefused(t *testing.T) {
	if _, ok := validateGroup([]string{"7-R", "7-R", "7-B"}); ok {
		t.Fatal("two tiles of the same colour must never form a group")
	}
}

func TestValidateGroup_MismatchedNumberIsRefused(t *testing.T) {
	if _, ok := validateGroup([]string{"7-R", "8-B", "7-O"}); ok {
		t.Fatal("a group must be one number")
	}
}

func TestValidateGroup_JokerFillsAMissingColour(t *testing.T) {
	canonical, ok := validateGroup([]string{"7-R", "7-B", "JOKER1"})
	if !ok {
		t.Fatalf("a joker should fill the third colour")
	}
	if len(canonical) != 3 {
		t.Errorf("got %v", canonical)
	}
}

func TestValidateGroup_SizeBounds(t *testing.T) {
	if _, ok := validateGroup([]string{"7-R", "7-B"}); ok {
		t.Error("two tiles is not a group")
	}
	if _, ok := validateGroup([]string{"7-R", "7-B", "7-O", "7-K", "JOKER1"}); ok {
		t.Error("five tiles is not a group — only four colours exist")
	}
}

func TestValidateRun_ThreeConsecutive(t *testing.T) {
	canonical, ok := validateRun([]string{"5-R", "6-R", "7-R"})
	if !ok {
		t.Fatal("5,6,7 of one colour should be a valid run")
	}
	want := []string{"5-R", "6-R", "7-R"}
	if !reflect.DeepEqual(canonical, want) {
		t.Errorf("canonical = %v, want %v", canonical, want)
	}
}

func TestValidateRun_MixedColourIsRefused(t *testing.T) {
	if _, ok := validateRun([]string{"5-R", "6-B", "7-R"}); ok {
		t.Fatal("a run must be one colour")
	}
}

func TestValidateRun_DoesNotWrapPast13(t *testing.T) {
	if _, ok := validateRun([]string{"12-R", "13-R", "1-R"}); ok {
		t.Fatal("13 must never run into 1")
	}
}

func TestValidateRun_JokerFillsAnInternalGap(t *testing.T) {
	canonical, ok := validateRun([]string{"5-R", "JOKER1", "7-R"})
	if !ok {
		t.Fatal("a joker should fill the gap at 6")
	}
	want := []string{"5-R", "JOKER1", "7-R"}
	if !reflect.DeepEqual(canonical, want) {
		t.Errorf("canonical = %v, want %v", canonical, want)
	}
}

func TestValidateRun_JokerExtendsTheLowEnd(t *testing.T) {
	// Two reals, no internal gap: the joker must extend an end. Low end is
	// preferred deterministically.
	canonical, ok := validateRun([]string{"5-R", "6-R", "JOKER1"})
	if !ok {
		t.Fatal("a joker should extend the run")
	}
	want := []string{"JOKER1", "5-R", "6-R"}
	if !reflect.DeepEqual(canonical, want) {
		t.Errorf("canonical = %v, want %v", canonical, want)
	}
}

func TestValidateRun_JokerCannotExtendPast13(t *testing.T) {
	// Twelve consecutive reals plus two jokers would need fourteen slots —
	// one more than the thirteen numbers that exist, regardless of ends.
	reals := []string{"1-R", "2-R", "3-R", "4-R", "5-R", "6-R", "7-R", "8-R", "9-R", "10-R", "11-R", "12-R"}
	cards := append(append([]string(nil), reals...), "JOKER1", "JOKER2")
	if _, ok := validateRun(cards); ok {
		t.Fatal("a fourteen-tile run cannot exist — there are only thirteen numbers")
	}
}

func TestValidateRun_DuplicateNumberIsRefused(t *testing.T) {
	if _, ok := validateRun([]string{"5-R", "5-R", "6-R"}); ok {
		t.Fatal("a run cannot repeat a number")
	}
}

func TestValidateSet_TriesGroupBeforeRun(t *testing.T) {
	// A single real tile plus two jokers is structurally both — the tie is
	// broken toward group, deterministically.
	kind, _, ok := validateSet([]string{"7-R", "JOKER1", "JOKER2"})
	if !ok || kind != "group" {
		t.Errorf("kind = %q, ok = %v, want group", kind, ok)
	}
}

func TestSetValueOf_GroupCountsEachTileAtItsNumber(t *testing.T) {
	if v := setValueOf("group", []string{"7-R", "7-B", "7-O"}); v != 21 {
		t.Errorf("got %d, want 21", v)
	}
}

func TestSetValueOf_RunResolvesAJokersImpliedNumber(t *testing.T) {
	// 5, joker(=6), 7 — the joker counts as 6, not as a flat 30.
	if v := setValueOf("run", []string{"5-R", "JOKER1", "7-R"}); v != 18 {
		t.Errorf("got %d, want 18 (5+6+7)", v)
	}
}

func TestJokerMatching_GroupAcceptsTheMissingColour(t *testing.T) {
	canonical := []string{"7-R", "7-B", "JOKER1"}
	joker, code := jokerMatching("group", canonical, "7-O")
	if code != "" || joker != "JOKER1" {
		t.Errorf("joker=%q code=%q, want JOKER1 and no error", joker, code)
	}
}

func TestJokerMatching_GroupRefusesAnAlreadyPresentColour(t *testing.T) {
	canonical := []string{"7-R", "7-B", "JOKER1"}
	if _, code := jokerMatching("group", canonical, "7-R"); code != ErrJokerSwapMismatch {
		t.Errorf("code = %q, want %q", code, ErrJokerSwapMismatch)
	}
}

func TestJokerMatching_RunAcceptsExactlyTheImpliedNumber(t *testing.T) {
	canonical := []string{"5-R", "JOKER1", "7-R"} // joker implies 6-R
	if joker, code := jokerMatching("run", canonical, "6-R"); code != "" || joker != "JOKER1" {
		t.Errorf("joker=%q code=%q", joker, code)
	}
	if _, code := jokerMatching("run", canonical, "6-B"); code != ErrJokerSwapMismatch {
		t.Error("wrong colour should be refused")
	}
	if _, code := jokerMatching("run", canonical, "8-R"); code != ErrJokerSwapMismatch {
		t.Error("a number the joker does not stand for should be refused")
	}
}

// --- the workspace validator (via applyCommit's own validation path) -------

func TestWorkspace_SplitRunProducesTwoValidRuns(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.InitialMeld["p1"] = true
		s.Sets = []Set{{ID: "s0", Kind: "run", Cards: []string{"3-R", "4-R", "5-R", "6-R", "7-R", "8-R"}}}
		s.Workspace = &Workspace{Sets: cloneSets(s.Sets)}
		s.Hands["p1"] = []string{"9-B"} // needs at least one hand tile played to commit
	})
	next, err := apply(t, raw, "p1", moduleAction(VerbSplit, "s0", nil, map[string]string{"position": "3"}))
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	s := stateOf(t, next)
	if len(s.Workspace.Sets) != 2 {
		t.Fatalf("expected two sets after split, got %d", len(s.Workspace.Sets))
	}
	for _, set := range s.Workspace.Sets {
		if _, _, ok := validateSet(set.Cards); !ok {
			t.Errorf("half %v is not a valid run", set.Cards)
		}
	}
}

func TestWorkspace_StolenMiddleTileMustBeReplacedForTheGroupToValidate(t *testing.T) {
	// Taking the middle of a run leaves two invalid fragments — commit must
	// refuse until the table is repaired.
	raw := withState(t, func(s *GameState) {
		s.InitialMeld["p1"] = true
		s.Sets = []Set{{ID: "s0", Kind: "run", Cards: []string{"3-R", "4-R", "5-R"}}}
		s.Workspace = &Workspace{Sets: cloneSets(s.Sets)}
		s.Hands["p1"] = []string{"4-R"} // the player's own copy, not the table's
	})
	// Take the middle tile out.
	raw, err := apply(t, raw, "p1", moduleAction(VerbTake, "s0", []string{"4-R"}, nil))
	if err != nil {
		t.Fatalf("take: %v", err)
	}
	s := stateOf(t, raw)
	if len(s.Workspace.Sets) != 1 || len(s.Workspace.Sets[0].Cards) != 2 {
		t.Fatalf("expected a two-tile remainder, got %+v", s.Workspace.Sets)
	}
	// Commit must refuse: the remainder is not a valid run and the tray still
	// holds the tile that was taken.
	if _, err := apply(t, raw, "p1", moduleAction(VerbCommit, "", nil, nil)); err == nil {
		t.Fatal("expected commit to refuse an invalid table")
	}
}

func TestWorkspace_JokerSwappedOutJoinsTheTray(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.InitialMeld["p1"] = true
		s.Sets = []Set{{ID: "s0", Kind: "run", Cards: []string{"5-R", "JOKER1", "7-R"}}}
		s.Workspace = &Workspace{Sets: cloneSets(s.Sets)}
		s.Hands["p1"] = []string{"6-R"}
	})
	next, err := apply(t, raw, "p1", moduleAction(VerbSwapJoker, "s0", []string{"6-R"}, nil))
	if err != nil {
		t.Fatalf("swap_joker: %v", err)
	}
	s := stateOf(t, next)
	if len(s.Workspace.Tray) != 1 || s.Workspace.Tray[0] != "JOKER1" {
		t.Fatalf("expected the freed joker in the tray, got %v", s.Workspace.Tray)
	}
	if !hasTile(s.Workspace.Sets[0].Cards, "6-R") {
		t.Error("the replacement tile should be in the set")
	}
}

func TestWorkspace_CommitRefusedWhileTheTrayHoldsALooseTile(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.InitialMeld["p1"] = true
		s.Sets = []Set{{ID: "s0", Kind: "run", Cards: []string{"5-R", "JOKER1", "7-R"}}}
		s.Workspace = &Workspace{Sets: cloneSets(s.Sets)}
		s.Hands["p1"] = []string{"6-R"}
	})
	raw, err := apply(t, raw, "p1", moduleAction(VerbSwapJoker, "s0", []string{"6-R"}, nil))
	if err != nil {
		t.Fatalf("swap_joker: %v", err)
	}
	if _, err := apply(t, raw, "p1", moduleAction(VerbCommit, "", nil, nil)); err == nil {
		t.Fatal("expected commit to refuse while the freed joker sits unplaced in the tray")
	} else if module.CodeOf(err) != ErrTrayNotEmpty {
		t.Errorf("expected %s, got %v", ErrTrayNotEmpty, err)
	}
}
