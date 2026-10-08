package lastcard

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// This game shares its mechanics with a trademarked commercial one, and only
// its mechanics: a mechanic is free to use and a brand is not. So the
// commercial name must never appear in anything this game ships — its code,
// its message keys, its wording in any locale, its card art.
//
// Spelt in two halves so this file does not trip its own test, and matched as
// a whole word so "unofficial" is fine. In the locale bundles only Last Card's
// own lines are checked: in Spanish and Italian the word means "one", and a
// sentence elsewhere in those bundles is entitled to it.
var brand = regexp.MustCompile(`(?i)\b` + "u" + "no" + `\b`)

func TestTheCommercialNameAppearsNowhere(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	type place struct {
		glob    string
		ownOnly bool // check only lines that mention lastcard
	}
	places := []place{
		{filepath.Join(root, "server", "internal", "lastcard", "*.go"), false},
		{filepath.Join(root, "client-react-native", "src", "components", "cards", "lastCard*"), false},
		{filepath.Join(root, "client-react-native", "src", "components", "cards", "LastCard*"), false},
		{filepath.Join(root, "client-react-native", "src", "lib", "locales", "*.ts"), true},
		{filepath.Join(root, "client-react-native", "src", "lib", "serverKeys.json"), true},
		{filepath.Join(root, "docs", "lastcard-*.md"), false},
	}
	checked := 0
	for _, pl := range places {
		files, err := filepath.Glob(pl.glob)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			if strings.HasSuffix(f, "brand_test.go") || strings.HasSuffix(f, "lastcard-plan.md") {
				// This file, and the plan that explains why the name is banned.
				continue
			}
			fh, err := os.Open(f)
			if err != nil {
				t.Fatal(err)
			}
			sc := bufio.NewScanner(fh)
			sc.Buffer(make([]byte, 1<<20), 1<<20)
			for n := 1; sc.Scan(); n++ {
				line := sc.Text()
				if pl.ownOnly && !strings.Contains(strings.ToLower(line), "lastcard") {
					continue
				}
				if brand.MatchString(line) {
					t.Errorf("%s:%d names the commercial game: %s", f, n, strings.TrimSpace(line))
				}
			}
			fh.Close()
			checked++
		}
	}
	if checked < 10 {
		t.Fatalf("only %d files checked — the globs no longer find this game's files", checked)
	}
}
