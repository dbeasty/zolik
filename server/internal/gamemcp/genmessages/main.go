// Command genmessages copies the client's English message bundle into the
// game MCP server, which renders the engine's message keys as English for a
// model to read.
//
// The server ships keys, never sentences (the client owns every word), and
// the client bundle lives outside this Go module, where go:embed cannot reach.
// So the words are copied, not shared, and the copy is checked:
// gamemcp's TestMessagesMatchClientBundle fails when en.ts has moved on.
//
//	go generate ./internal/gamemcp
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"zolik/server/internal/gamemcp/tsbundle"
)

func main() {
	in := flag.String("in", "../../../client-react-native/src/lib/locales/en.ts", "the client's English bundle")
	out := flag.String("out", "messages_en.json", "where to write the copy")
	flag.Parse()
	src, err := os.ReadFile(*in)
	if err != nil {
		fail(err)
	}
	msgs, err := tsbundle.Parse(string(src))
	if err != nil {
		fail(err)
	}
	b, err := json.MarshalIndent(msgs, "", " ")
	if err != nil {
		fail(err)
	}
	if err := os.WriteFile(*out, append(b, '\n'), 0o644); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "genmessages:", err)
	os.Exit(1)
}
