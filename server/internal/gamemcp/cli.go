package gamemcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// The command line: the same tools, one JSON command per line.
//
//	{"tool":"new_table","args":{"game":"zolik","seats":2,"claude_seats":[0]}}
//
// answered by one JSON line each:
//
//	{"tool":"new_table","ok":true,"result":{...},"text":"..."}
//	{"tool":"play","ok":false,"error":"no move 40: ..."}
//
// A command may carry an "id", which its answer repeats. Blank lines and lines
// starting with # are skipped, so a script can be commented.

// Command is one line of input.
type Command struct {
	ID   any             `json:"id,omitempty"`
	Tool string          `json:"tool"`
	Args json.RawMessage `json:"args,omitempty"`
}

// Reply is one line of output.
type Reply struct {
	ID     any    `json:"id,omitempty"`
	Tool   string `json:"tool"`
	OK     bool   `json:"ok"`
	Result any    `json:"result,omitempty"`
	// Text is the result rendered for a reader: the observation for the
	// game tools, the rules text for rules.
	Text  string `json:"text,omitempty"`
	Error string `json:"error,omitempty"`
}

// Do runs one command.
func (svc *Service) Do(c Command) Reply {
	r := Reply{ID: c.ID, Tool: c.Tool}
	out, err := svc.Call(c.Tool, c.Args)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	r.OK, r.Result, r.Text = true, out, textOf(out)
	return r
}

// RunCLI answers every command on in, in order, on out. Tables persist for
// the whole run. It stops at the end of input; a line that is not a command
// is answered with an error and the run goes on.
func RunCLI(svc *Service, in io.Reader, out io.Writer) error {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var c Command
		var r Reply
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			r = Reply{Error: "not a command: " + err.Error()}
		} else {
			r = svc.Do(c)
		}
		if err := enc.Encode(r); err != nil {
			return err
		}
		if f, ok := out.(interface{ Sync() error }); ok {
			_ = f.Sync()
		}
	}
	return sc.Err()
}

// DefaultStatePath is where `game-mcp call` keeps its tables between calls
// unless told otherwise.
func DefaultStatePath() string {
	if p := os.Getenv("GAME_MCP_STATE"); p != "" {
		return p
	}
	return filepath.Join(os.TempDir(), "zolik-game-mcp-state.json")
}

// CallOnce is `game-mcp call`: load the tables from path (none if it does not
// exist yet), run one tool, save them back, and return the reply. Each call
// is a fresh process, so separate shell commands — a Claude Code session's
// separate Bash calls — can carry one game between them with nothing left
// running.
func CallOnce(path, tool string, args json.RawMessage) (Reply, error) {
	svc := NewService()
	f, err := os.Open(path)
	switch {
	case err == nil:
		err = svc.Load(f)
		f.Close()
		if err != nil {
			return Reply{}, fmt.Errorf("reading %s: %w", path, err)
		}
	case !errors.Is(err, os.ErrNotExist):
		return Reply{}, err
	}
	r := svc.Do(Command{Tool: tool, Args: args})
	var buf bytes.Buffer
	if err := svc.Save(&buf); err != nil {
		return r, err
	}
	// Written whole and renamed into place, so a call interrupted halfway
	// leaves the previous tables rather than half a file.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o600); err != nil {
		return r, err
	}
	return r, os.Rename(tmp, path)
}
