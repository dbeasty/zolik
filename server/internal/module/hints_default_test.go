package module

import "testing"

func TestWithHintsDefaultStartsHintsOn(t *testing.T) {
	d := ModuleDescriptor{
		ID: "h", Options: []OptionSpec{HintsOption()},
		Variations: []VariationSpec{{ID: "a", Defaults: map[string]int{"x": 1}}, {ID: "b", Defaults: map[string]int{OptHints: OptOff}}, {ID: "c"}},
	}
	got := withHintsDefault(d)
	want := map[string]int{"a": OptOn, "b": OptOff, "c": OptOn}
	for _, v := range got.Variations {
		if v.Defaults[OptHints] != want[v.ID] {
			t.Fatalf("variation %s: hints default %d, want %d", v.ID, v.Defaults[OptHints], want[v.ID])
		}
	}
	if _, ok := d.Variations[0].Defaults[OptHints]; ok {
		t.Fatal("the module's own descriptor was changed")
	}
	if g := withHintsDefault(ModuleDescriptor{ID: "n", Variations: []VariationSpec{{ID: "a"}}}); g.Variations[0].Defaults != nil {
		t.Fatal("a module without hints gained a default")
	}
}
