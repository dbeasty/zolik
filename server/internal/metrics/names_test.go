package metrics_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zolik/server/internal/metrics"
)

// Every counter name that may reach Sink.Add, either as a constant or as the
// result of one of the three helpers that build a name from a run-time value.
var (
	allowedConstants = map[string]bool{
		"MatchesCreated": true, "MatchesStarted": true, "MatchesCompleted": true,
		"MatchesAbandoned": true, "MatchesResumed": true, "MatchesDeleted": true,
		"UsersRegistered": true, "SessionsGuest": true,
		"WSConnected": true, "AdmissionRefusedMatchStart": true,
		"BootsTotal": true, "BootsUnclean": true,
		"AdminLoginOK": true, "AdminLoginDenied": true,
	}
	allowedBuilders = map[string]bool{
		"MatchesCompletedFor": true, "MatchesAbandonedFor": true,
		"MatchesResumedFor": true, "AdmissionRefusedFor": true,
	}
)

// TestCounterNamesAreLiterals walks every Add call in the server and fails on
// one whose name did not come from names.go.
//
// The house already learned this with server message keys: a key passed
// through a variable dropped silently out of the generated manifest, because
// the tool that builds it reads the source rather than running it. A metric
// name assembled at run time is the same bug with a quieter symptom — no
// failing check, just a column that never appears on the operator's screen,
// holding a number nobody realises is missing.
//
// The rule is deliberately syntactic: a name is either metrics.SomeConstant,
// or metrics.SomeBuilder(x), or the test fails. Adding a counter means adding
// a constant here and there, which is the point — it is a two-line tax that
// keeps one file authoritative about what this server measures.
func TestCounterNamesAreLiterals(t *testing.T) {
	root := serverRoot(t)

	var offences []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); name == "node_modules" || name == "dist" || name == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Add" || len(call.Args) != 2 {
				return true
			}
			// Only calls that look like a metrics sink. Any other Add(x, y) in
			// the tree — a set, a duration, a score — is none of this test's
			// business, so the receiver has to name something metric-ish.
			if !looksLikeSink(sel.X) {
				return true
			}
			if !isApprovedName(call.Args[0]) {
				offences = append(offences,
					fset.Position(call.Pos()).String()+": counter name is not a metrics constant or builder")
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	for _, o := range offences {
		t.Error(o)
	}
}

func looksLikeSink(x ast.Expr) bool {
	var name string
	switch e := x.(type) {
	case *ast.Ident:
		name = e.Name
	case *ast.SelectorExpr:
		name = e.Sel.Name
	default:
		return false
	}
	lower := strings.ToLower(name)
	return strings.Contains(lower, "metric") || strings.Contains(lower, "sink") ||
		strings.Contains(lower, "counter") || strings.Contains(lower, "recorder")
}

// isApprovedName accepts metrics.SomeConstant and metrics.SomeBuilder(...),
// and rejects everything else — a bare string, a variable, a concatenation.
func isApprovedName(arg ast.Expr) bool {
	switch e := arg.(type) {
	case *ast.SelectorExpr:
		if pkg, ok := e.X.(*ast.Ident); ok && pkg.Name == "metrics" {
			return allowedConstants[e.Sel.Name]
		}
		// Inside package metrics itself the constant is unqualified, so a
		// selector here is something else entirely.
		return false
	case *ast.Ident:
		// Unqualified: only valid inside package metrics.
		return allowedConstants[e.Name]
	case *ast.CallExpr:
		switch fn := e.Fun.(type) {
		case *ast.SelectorExpr:
			if pkg, ok := fn.X.(*ast.Ident); ok && pkg.Name == "metrics" {
				return allowedBuilders[fn.Sel.Name]
			}
		case *ast.Ident:
			return allowedBuilders[fn.Name]
		}
		return false
	default:
		return false
	}
}

// The constants and the list this test checks against are two copies of the
// same fact, so they have to be tied together: a constant added to names.go
// without being added here would be rejected at its first call site with a
// confusing message, and one removed from names.go would leave a stale
// permission behind.
func TestAllowListMatchesTheConstants(t *testing.T) {
	root := serverRoot(t)
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filepath.Join(root, "internal", "metrics", "names.go"), nil, 0)
	if err != nil {
		t.Fatalf("parse names.go: %v", err)
	}

	declared := map[string]bool{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, name := range vs.Names {
				// The two Prefix constants are ingredients, not names: nothing
				// passes one to Add on its own.
				if strings.HasSuffix(name.Name, "Prefix") {
					continue
				}
				declared[name.Name] = true
			}
		}
	}

	for name := range declared {
		if !allowedConstants[name] {
			t.Errorf("names.go declares %s but this test's allow-list does not", name)
		}
	}
	for name := range allowedConstants {
		if !declared[name] {
			t.Errorf("the allow-list names %s but names.go no longer declares it", name)
		}
	}
}

func TestSanitiseKeepsNamesFlat(t *testing.T) {
	// A module id with a dot in it would otherwise fake a level of hierarchy
	// that is not there, and an empty one would leave a trailing dot that
	// reads as truncation.
	cases := map[string]string{
		"zolik":     "matches.completed.zolik",
		"Gin Rummy": "matches.completed.gin_rummy",
		"a.b":       "matches.completed.a_b",
		"":          "matches.completed.unknown",
		"   ":       "matches.completed.unknown",
	}
	for in, want := range cases {
		if got := metrics.MatchesCompletedFor(in); got != want {
			t.Errorf("MatchesCompletedFor(%q) = %q, want %q", in, got, want)
		}
	}
}

func serverRoot(t *testing.T) string {
	t.Helper()
	// The test binary runs in internal/metrics; the server module root is two
	// levels up.
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve server root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("server root %s has no go.mod: %v", root, err)
	}
	return root
}
