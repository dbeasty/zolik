package canasta

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"zolik/server/internal/module"
)

// Every key this module can actually put on a screen is in the manifest.
//
// `cmd/dump-keys` finds keys by reading the source, and a static scan can only
// find the spellings it was taught. It had not been taught
// `g.BadgeKeys = append(g.BadgeKeys, "badge.x")`, which is how Canasta has
// badged its canastas since it shipped — so `badge.naturalCanasta` and
// `badge.mixedCanasta` were absent from the manifest, absent from all 24
// bundles, and rendering in English to everyone. Nothing failed, because
// nothing was looking.
//
// This looks. It plays real matches under every variation and collects the keys
// the module *emits* — from the board, the offers and the written rules — then
// checks each one against the committed manifest. A key the scanner cannot see
// fails here instead of reaching a player.
func TestEveryEmittedKeyIsInTheManifest(t *testing.T) {
	manifest := loadManifest(t)
	m := New()

	emitted := map[string]bool{}
	note := func(k string) {
		if k != "" {
			emitted[k] = true
		}
	}

	for _, variation := range []string{"classic", "modern_american", "samba"} {
		cfg := module.MatchConfig{
			Variation: variation,
			Options:   module.Options{OptTargetScore: 500},
		}

		sections, err := m.Rules(cfg)
		if err != nil {
			t.Fatalf("%s: Rules: %v", variation, err)
		}
		for _, sec := range sections {
			note(sec.TitleKey)
			for _, f := range sec.Items {
				note(f.LabelKey)
			}
		}

		players := refs("p1", "p2", "p3", "p4")
		for seed := int64(1); seed <= 6; seed++ {
			state, err := m.NewMatch(cfg, players, seed)
			if err != nil {
				t.Fatalf("%s seed %d: NewMatch: %v", variation, seed, err)
			}

			// Every state the match passes through, not just the last: a badge
			// only appears while the meld that earns it is on the table, and
			// the table is swept between deals.
			step := func(s module.State) {
				for _, p := range players {
					vm, err := m.View(s, p.ID)
					if err != nil {
						t.Fatalf("%s: View: %v", variation, err)
					}
					for _, z := range vm.Zones {
						note(z.LabelKey)
						for _, g := range z.Groups {
							for _, b := range g.BadgeKeys {
								note(b)
							}
						}
					}
					for _, f := range vm.Header {
						note(f.LabelKey)
					}
					for _, f := range vm.Status {
						note(f.LabelKey)
					}
					for _, f := range vm.Prompts {
						note(f.LabelKey)
					}
					for _, seat := range vm.Seats {
						for _, f := range seat.Facts {
							note(f.LabelKey)
						}
						for _, k := range seat.LabelKeys {
							note(k)
						}
					}
					offers, err := m.LegalActions(s, p.ID)
					if err != nil {
						t.Fatalf("%s: LegalActions: %v", variation, err)
					}
					for _, o := range offers {
						note(o.LabelKey)
						note(o.WhyNot)
						for _, f := range o.Facts {
							note(f.LabelKey)
						}
					}
				}
			}

			step(state)
			_, _, err = module.PlayWithOffers(m, state, players, module.DriverOptions{
				MaxActions: 4000, Prefer: driverPrefer, OnState: step,
			})
			if err != nil {
				t.Fatalf("%s seed %d: %v", variation, seed, err)
			}
		}
	}

	var missing []string
	for k := range emitted {
		if !manifest[k] {
			missing = append(missing, k)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("these keys reach a screen but are not in serverKeys.json, so no locale words them:\n  %v\n\n"+
			"Regenerate it:\n    cd server && go run ./cmd/dump-keys > ../client-react-native/src/lib/serverKeys.json\n"+
			"If regenerating does not add them, cmd/dump-keys cannot see how they are written.", missing)
	}
	t.Logf("%d distinct keys emitted across three variations", len(emitted))
}

// loadManifest reads every key the committed manifest lists, in any of its
// categories — error codes travel as bare codes and are worded as `err.<CODE>`,
// which is the client's convention rather than this side's.
func loadManifest(t *testing.T) map[string]bool {
	t.Helper()
	path := filepath.Join("..", "..", "..", "client-react-native", "src", "lib", "serverKeys.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("no serverKeys.json to check against: %v", err)
	}
	var manifest struct {
		ErrorCodes   []string `json:"errorCodes"`
		LabelKeys    []string `json:"labelKeys"`
		DeclaredOnly []string `json:"declaredOnly"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("serverKeys.json: %v", err)
	}
	out := map[string]bool{}
	for _, group := range [][]string{manifest.ErrorCodes, manifest.LabelKeys, manifest.DeclaredOnly} {
		for _, k := range group {
			out[k] = true
		}
	}
	return out
}
