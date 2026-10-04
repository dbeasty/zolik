package gamemcp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// rpc is a JSON-RPC client over a line-delimited stream, written by hand so
// the test checks the wire format an MCP client actually sends and reads,
// not the SDK talking to itself.
type rpc struct {
	t   *testing.T
	w   io.Writer
	r   *bufio.Scanner
	nid int
}

func (c *rpc) send(method string, params any, notify bool) {
	c.t.Helper()
	msg := map[string]any{"jsonrpc": "2.0", "method": method}
	if params != nil {
		msg["params"] = params
	}
	if !notify {
		c.nid++
		msg["id"] = c.nid
	}
	b, _ := json.Marshal(msg)
	if _, err := c.w.Write(append(b, '\n')); err != nil {
		c.t.Fatal(err)
	}
}

func (c *rpc) call(method string, params any) map[string]any {
	c.t.Helper()
	c.send(method, params, false)
	for c.r.Scan() {
		var msg map[string]any
		if err := json.Unmarshal(c.r.Bytes(), &msg); err != nil {
			c.t.Fatalf("not JSON-RPC: %s", c.r.Text())
		}
		if msg["id"] == nil {
			continue // a notification from the server
		}
		if msg["id"].(float64) != float64(c.nid) {
			c.t.Fatalf("reply to %v, want %d", msg["id"], c.nid)
		}
		if msg["error"] != nil {
			c.t.Fatalf("%s: %v", method, msg["error"])
		}
		return msg["result"].(map[string]any)
	}
	c.t.Fatalf("%s: stream ended: %v", method, c.r.Err())
	return nil
}

// roundTrip speaks MCP to a server over a pair of pipes, the way a client
// speaks to `game-mcp` over its stdin and stdout.
func roundTrip(t *testing.T, w io.Writer, r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	c := &rpc{t: t, w: w, r: sc}

	init := c.call("initialize", map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test", "version": "0"},
	})
	if init["serverInfo"].(map[string]any)["name"] != "zolik-games" {
		t.Fatalf("initialize: %v", init)
	}
	c.send("notifications/initialized", map[string]any{}, true)

	list := c.call("tools/list", map[string]any{})
	var names []string
	for _, tl := range list["tools"].([]any) {
		tool := tl.(map[string]any)
		names = append(names, tool["name"].(string))
		if tool["inputSchema"] == nil || tool["description"] == "" {
			t.Fatalf("tool %v has no schema or description", tool["name"])
		}
	}
	sort.Strings(names)
	if strings.Join(names, ",") != strings.Join(ToolNames(), ",") {
		t.Fatalf("tools/list: %v", names)
	}

	res := c.call("tools/call", map[string]any{"name": "new_table", "arguments": map[string]any{
		"game": "zolik", "seats": 2, "claude_seats": []int{0}, "opponents": []string{"hard"}, "seed": 3,
	}})
	if res["isError"] == true {
		t.Fatalf("new_table: %v", res["content"])
	}
	text := res["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "Legal moves") {
		t.Fatalf("new_table text:\n%s", text)
	}
	sc2 := res["structuredContent"].(map[string]any)
	if sc2["table"] != "t1" {
		t.Fatalf("structured: %v", sc2["table"])
	}

	res = c.call("tools/call", map[string]any{"name": "play", "arguments": map[string]any{"table": "t1", "seat": 0, "move": 0}})
	if res["isError"] == true {
		t.Fatalf("play: %v", res["content"])
	}
	// A tool error is a result the model can read, not a protocol error.
	res = c.call("tools/call", map[string]any{"name": "play", "arguments": map[string]any{"table": "t1", "seat": 0, "move": 500}})
	if res["isError"] != true || !strings.Contains(res["content"].([]any)[0].(map[string]any)["text"].(string), "no move 500") {
		t.Fatalf("illegal move: %v", res)
	}
}

func TestMCPRoundTrip(t *testing.T) {
	cr, sw := io.Pipe() // server -> client
	sr, cw := io.Pipe() // client -> server
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- NewMCPServer(NewService()).Run(ctx, &mcp.IOTransport{Reader: sr, Writer: sw})
	}()
	roundTrip(t, cw, cr)
	cw.Close()
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop")
	}
}
