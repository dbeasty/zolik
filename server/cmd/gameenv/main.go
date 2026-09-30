// Command gameenv is the environment a trainer (ml/) drives: JSON lines on
// stdin, JSON lines on stdout, every table played by the real engine. See
// internal/learn/env.go for the protocol.
//
//	echo '{"op":"info","game":"holdem"}' | go run ./cmd/gameenv
package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"zolik/server/internal/learn"

	_ "zolik/server/internal/canasta"
	_ "zolik/server/internal/holdem"
	_ "zolik/server/internal/zolikmod"
)

func main() {
	// The engines are JSON in and JSON out, so a step is mostly garbage, and
	// at the default the collector ran often enough to cost more than half of
	// Canasta's throughput. A trainer's process is not a server sharing a box
	// with anything; trading memory for speed is the right way round here.
	// GOGC still wins when it is set.
	if os.Getenv("GOGC") == "" {
		debug.SetGCPercent(400)
	}
	if err := learn.Serve(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "gameenv:", err)
		os.Exit(1)
	}
}
