// Command gameenv is the environment a trainer (ml/) drives: JSON lines on
// stdin, JSON lines on stdout, every table played by the real engine. See
// internal/learn/env.go for the protocol.
//
//	echo '{"op":"info","game":"holdem"}' | go run ./cmd/gameenv
package main

import (
	"fmt"
	"os"

	"zolik/server/internal/learn"

	_ "zolik/server/internal/canasta"
	_ "zolik/server/internal/holdem"
)

func main() {
	if err := learn.Serve(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "gameenv:", err)
		os.Exit(1)
	}
}
