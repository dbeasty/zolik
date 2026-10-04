// Command pushfold regenerates internal/holdem/pushfold_table.go, the heads-up
// jam-or-fold chart Hold'em Hard plays at short stacks.
//
//	go run ./cmd/pushfold                 # writes the table, prints the solve
//	go run ./cmd/pushfold -out /dev/stdout
//
// The solve is deterministic (internal/holdem/pushfold_solve.go), and
// TestPushFoldTableIsGenerated holds the committed file to it.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"zolik/server/internal/holdem"
)

func main() {
	out := flag.String("out", "internal/holdem/pushfold_table.go", "where to write the table")
	flag.Parse()

	start := time.Now()
	source, summary := holdem.PushFoldSource()
	if err := os.WriteFile(*out, []byte(source), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "pushfold:", err)
		os.Exit(1)
	}
	fmt.Fprint(os.Stderr, summary)
	fmt.Fprintf(os.Stderr, "wrote %s in %s\n", *out, time.Since(start).Round(time.Millisecond))
}
