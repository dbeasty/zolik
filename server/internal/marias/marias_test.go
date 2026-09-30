package marias

import (
	"testing"

	"zolik/server/internal/module"
)

// The association's table (ČSM, bodovaný volený mariáš, 8.5.2007), section A.
func TestTariffMatchesTheAssociationTable(t *testing.T) {
	want := Tariff{Hra: 1, Sedma: 2, Sto: 4, Betl: 15, Durch: 30, DveSedmy: 40, Omyl: 6}
	if got := tariffFor(TariffCSM); got != want {
		t.Errorf("ČSM tariff = %+v, want %+v", got, want)
	}
	if got := tariffFor(TariffPub); got.Betl != 5 || got.Durch != 10 || got.Hra != 1 {
		t.Errorf("pub tariff = %+v", got)
	}
	// "Tichá sedma má poloviční sazbu sedmy hlášené."
	if got := want.quietSeven(); got != 1 {
		t.Errorf("quiet seven = %d, want half of sedma", got)
	}
}

// ČSM general rules V/6-7: a hundred scales linearly — the tariff again for
// every ten points past it, and a failed sto pays for every ten short and
// for every ten of the other side's marriages.
func TestHundredsScaleLinearly(t *testing.T) {
	tr := tariffFor(TariffCSM)
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"hra, no hundred", tr.hraValue(90), 1},
		{"hra with a quiet hundred", tr.hraValue(100), 2},
		{"quiet hundred and ten", tr.hraValue(110), 4},
		{"quiet hundred and twenty", tr.hraValue(120), 6},
		{"announced sto made", tr.stoValue(true, 100, 0), 4},
		{"announced sto at 110", tr.stoValue(true, 110, 0), 8},
		{"announced sto at 130", tr.stoValue(true, 130, 0), 16},
		{"sto ten short", tr.stoValue(false, 90, 0), 4},
		{"sto thirty short", tr.stoValue(false, 70, 0), 12},
		{"sto short, against a marriage", tr.stoValue(false, 90, 20), 12},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: %d, want %d", c.name, c.got, c.want)
		}
	}
}

func TestMultiplier(t *testing.T) {
	cases := []struct {
		doublings int
		red       bool
		want      int
	}{
		{0, false, 1}, {1, false, 2}, {2, false, 4}, {3, true, 16}, {0, true, 2},
	}
	for _, c := range cases {
		if got := multiplier(c.doublings, c.red); got != c.want {
			t.Errorf("multiplier(%d, %v) = %d, want %d", c.doublings, c.red, got, c.want)
		}
	}
}

// Every variation must name a default for every option it declares, so a
// lobby shows exactly the table it is about to create.
func TestVariationDefaultsCoverEveryOption(t *testing.T) {
	d := (&Module{}).Descriptor()
	for _, v := range d.Variations {
		for _, o := range d.Options {
			dv, ok := v.Defaults[o.Name]
			if !ok {
				t.Errorf("%s: no default for %s", v.ID, o.Name)
				continue
			}
			if !o.Allows(dv) {
				t.Errorf("%s: default %d for %s is not one of its choices", v.ID, dv, o.Name)
			}
		}
	}
}

// Options change what the rules say, and only what they are set to.
func TestRulesFollowTheOptions(t *testing.T) {
	m := &Module{}
	ids := func(opts module.Options) map[string]bool {
		secs, err := m.Rules(module.MatchConfig{Variation: variationVoleny, Options: opts})
		if err != nil {
			t.Fatal(err)
		}
		return module.RuleIDsIn(secs)
	}

	def := ids(nil)
	for _, id := range []string{"marias.rules.deal.zLidu", "marias.rules.score.red", "marias.rules.play.follow"} {
		if !def[id] {
			t.Errorf("default table does not state %s", id)
		}
	}
	if def["marias.rules.bid.flekLimit"] {
		t.Error("an unlimited table states a flek limit")
	}

	off := ids(module.Options{OptZLidu: module.OptOff, OptRedDoubles: module.OptOff, OptFlekLimit: 2})
	if off["marias.rules.deal.zLidu"] || off["marias.rules.score.red"] {
		t.Error("rules state an option that is switched off")
	}
	if !off["marias.rules.bid.flekLimit"] {
		t.Error("a limited table does not state its limit")
	}
}

// The tariff sentence carries the table's own numbers.
func TestRulesQuoteTheChosenTariff(t *testing.T) {
	secs, _ := (&Module{}).Rules(module.MatchConfig{Options: module.Options{OptTariff: TariffPub}})
	for _, s := range secs {
		for _, it := range s.Items {
			if it.ID == "marias.rules.score.tariff" {
				if it.Params["betl"] != 5 || it.Params["durch"] != 10 {
					t.Errorf("pub tariff sentence says %v", it.Params)
				}
				return
			}
		}
	}
	t.Fatal("no tariff sentence")
}
