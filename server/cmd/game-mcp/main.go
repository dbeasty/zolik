// Command game-mcp lets Claude — or any MCP client, or a shell — play,
// inspect and coach games on the real rules engine, against the heuristic
// bots or trained networks. See README.md beside this file.
//
//	game-mcp                          MCP server over stdio (what .mcp.json runs)
//	game-mcp cli [--script file]      one JSON command per line in, one JSON reply per line out
//	game-mcp call [flags] TOOL [ARGS] one tool call; tables persist in a state file between calls
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"zolik/server/internal/gamemcp"
)

func main() {
	args := os.Args[1:]
	cmd := "serve"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	var err error
	switch cmd {
	case "serve":
		err = serve(args)
	case "cli":
		err = cli(args)
	case "call":
		err = call(args)
	case "help", "-h", "--help":
		usage(os.Stdout)
	default:
		usage(os.Stderr)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "game-mcp:", err)
		os.Exit(1)
	}
}

func usage(w io.Writer) {
	fmt.Fprintf(w, `usage:
  game-mcp [serve]                     MCP server on stdin/stdout
  game-mcp cli [--script FILE]         JSON lines: {"tool":"...","args":{...}} -> {"ok":...,"result":...,"text":...}
  game-mcp call [-state F] [-json] TOOL [ARGS-JSON]
                                       one call; tables kept in F (default $GAME_MCP_STATE or %s)
tools: %s
`, gamemcp.DefaultStatePath(), strings.Join(gamemcp.ToolNames(), ", "))
}

func serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	_ = fs.Parse(args)
	s := gamemcp.NewMCPServer(gamemcp.NewService())
	return s.Run(context.Background(), &mcp.StdioTransport{})
}

func cli(args []string) error {
	fs := flag.NewFlagSet("cli", flag.ExitOnError)
	script := fs.String("script", "", "read commands from this file instead of stdin")
	_ = fs.Parse(args)
	in := io.Reader(os.Stdin)
	if *script != "" {
		f, err := os.Open(*script)
		if err != nil {
			return err
		}
		defer f.Close()
		in = f
	}
	return gamemcp.RunCLI(gamemcp.NewService(), in, os.Stdout)
}

func call(args []string) error {
	fs := flag.NewFlagSet("call", flag.ExitOnError)
	state := fs.String("state", gamemcp.DefaultStatePath(), "file the tables are kept in between calls")
	asJSON := fs.Bool("json", false, "print the whole reply as JSON rather than its text")
	_ = fs.Parse(args)
	if fs.NArg() < 1 || fs.NArg() > 2 {
		usage(os.Stderr)
		os.Exit(2)
	}
	var raw json.RawMessage
	if fs.NArg() == 2 {
		raw = json.RawMessage(fs.Arg(1))
	}
	r, err := gamemcp.CallOnce(*state, fs.Arg(0), raw)
	if err != nil {
		return err
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", " ")
		return enc.Encode(r)
	}
	if !r.OK {
		fmt.Println("error:", r.Error)
		os.Exit(1)
	}
	if r.Text != "" {
		fmt.Print(r.Text)
		return nil
	}
	b, _ := json.MarshalIndent(r.Result, "", " ")
	fmt.Println(string(b))
	return nil
}
