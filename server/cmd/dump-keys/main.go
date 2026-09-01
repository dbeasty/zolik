// Command dump-keys prints every message key this server can send a client.
//
// The server ships keys, never sentences: that is what makes a Czech UI a
// client-only change. The cost of that split is that nothing connects the Go
// constants to the locale bundles, so a key added on this side can reach a
// player as SCREAMING_SNAKE and nobody finds out until they see a screenshot.
//
// This closes it with a golden file. The output is committed next to the
// bundles; a test on this side asserts the file still matches what the server
// emits, and a test on the client side asserts every key in it has wording in
// every locale. Neither side has to know about the other's language.
//
//	go run ./cmd/dump-keys > ../client-react-native/src/lib/serverKeys.json
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"zolik/server/internal/canasta"
	"zolik/server/internal/ginrummy"
	"zolik/server/internal/holdem"
	"zolik/server/internal/module"
	"zolik/server/internal/prsi"
	"zolik/server/internal/zolikmod"
)

func main() {
	keys, err := module.CollectKeys(
		module.NewRegistry(zolikmod.New(), prsi.New(), canasta.New(), holdem.New(), ginrummy.New()),
		"internal/admission", "internal/rules", "internal/prsi", "internal/canasta", "internal/holdem",
		"internal/zolikmod", "internal/match", "internal/module", "internal/lobby", "internal/ginrummy",
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dump-keys:", err)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(keys); err != nil {
		fmt.Fprintln(os.Stderr, "dump-keys:", err)
		os.Exit(1)
	}
}
