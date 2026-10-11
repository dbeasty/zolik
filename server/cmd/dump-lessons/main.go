// Command dump-lessons prints every locale key the guided lessons can show.
//
// The lessons name their words by key, like every other thing the server
// sends, and nothing else connects those keys to the client's bundles. This is
// the same golden-file lock cmd/dump-keys keeps for labels and error codes: the
// output is committed beside the client, a test here asserts the file is
// current, and a test there asserts each key has wording in every language.
//
//	go run ./cmd/dump-lessons > ../client-react-native/src/guide/lessonKeys.json
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"zolik/server/internal/blackjack"
	"zolik/server/internal/canasta"
	"zolik/server/internal/ferbl"
	"zolik/server/internal/ginrummy"
	"zolik/server/internal/holdem"
	"zolik/server/internal/klondike"
	"zolik/server/internal/lastcard"
	"zolik/server/internal/lessontest"
	"zolik/server/internal/lora"
	"zolik/server/internal/marias"
	"zolik/server/internal/module"
	"zolik/server/internal/okobere"
	"zolik/server/internal/prsi"
	"zolik/server/internal/rummytiles"
	"zolik/server/internal/sedma"
	"zolik/server/internal/snaps"
	"zolik/server/internal/zolikmod"
)

func main() {
	mods := []module.GameModule{
		zolikmod.New(), prsi.New(), canasta.New(), holdem.New(), ginrummy.New(), rummytiles.New(),
		blackjack.New(), marias.New(), klondike.New(), lastcard.New(), snaps.New(), sedma.New(),
		okobere.New(), ferbl.New(), lora.New(),
	}
	all := map[string]bool{}
	for _, m := range mods {
		keys, err := lessontest.Keys(m)
		if err != nil {
			fmt.Fprintln(os.Stderr, "dump-lessons:", err)
			os.Exit(1)
		}
		for _, k := range keys {
			all[k] = true
		}
	}
	out := make([]string, 0, len(all))
	for k := range all {
		out = append(out, k)
	}
	sort.Strings(out)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintln(os.Stderr, "dump-lessons:", err)
		os.Exit(1)
	}
}
