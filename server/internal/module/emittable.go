package module

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// Reading a package's refusal vocabulary out of its own source.
//
// A guardrail that checks "every code has an explanation" is only worth
// running if the list of codes cannot drift from the validators. A
// hand-maintained list drifts the first time somebody adds a refusal in a
// hurry — which is the exact failure the rule index exists to fix, reproduced
// one level up. So the list is read from the source: every error code
// *constructed* somewhere in these packages, which is by definition every
// code a player can be refused with.
//
// Over-inclusion is harmless (one more mapping to write). Under-inclusion is
// the thing to avoid, so this errs towards collecting too much: any constant
// assigned to a field named Code, and any bare string literal in that
// position.

// EmittedCodes walks the Go sources under each dir and returns the error
// codes constructed in them, plus the codes declared as constants but never
// constructed anywhere.
//
// Both halves are useful. The first is what a rule index must cover. The
// second is a question rather than a failure: a declared code nothing returns
// is either an unimplemented rule or a leftover, and the difference is not
// something a test can decide.
func EmittedCodes(dirs ...string) (emitted, declaredOnly []string) {
	emit := map[string]bool{}
	unused := map[string]bool{}

	// Package by package, because "emitted" means *this* package returns it.
	// A code named somewhere else — in a rule-index mapping, in a test — is a
	// reference to a refusal, not a place one happens, and counting those
	// would make every mapped code look reachable including the ones that
	// are not.
	for _, dir := range dirs {
		declared, uses := scanDir(dir)
		for ident, value := range declared {
			if uses[ident] > 0 || uses[value] > 0 {
				emit[value] = true
			} else if !emit[value] {
				unused[value] = true
			}
		}
		// Codes written as bare literals, with no constant behind them.
		for token, n := range uses {
			if isCodeShaped(token) && n > 0 {
				emit[token] = true
			}
		}
	}

	for k := range emit {
		emitted = append(emitted, k)
		delete(unused, k)
	}
	for k := range unused {
		declaredOnly = append(declaredOnly, k)
	}
	sortStrings(emitted)
	sortStrings(declaredOnly)
	return emitted, declaredOnly
}

// scanDir returns the code constants a package declares, and how often each
// name is mentioned in it (the declaration itself counted once).
func scanDir(dir string) (declared map[string]string, uses map[string]int) {
	declared, uses = map[string]string{}, map[string]int{}
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return declared, uses
	}
	files := []*ast.File{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			continue
		}
		files = append(files, f)
		collectDeclared(f, declared)
	}
	// Only *construction* positions count as emitting a code: the value of a
	// Code: field, an argument to a call (errCode(ErrPileFrozen)), or a bare
	// return. A code named in a switch that classifies refusals is a
	// reference to one, not a place one happens — counting those would make
	// every code the engine merely knows about look reachable.
	note := func(e ast.Expr) {
		switch v := e.(type) {
		case *ast.Ident:
			uses[v.Name]++
		case *ast.SelectorExpr:
			uses[v.Sel.Name]++
		case *ast.BasicLit:
			if v.Kind == token.STRING {
				if s := strings.Trim(v.Value, `"`); isCodeShaped(s) {
					uses[s]++
				}
			}
		}
	}
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.KeyValueExpr:
				if key, ok := v.Key.(*ast.Ident); ok && key.Name == "Code" {
					note(v.Value)
				}
			case *ast.CallExpr:
				for _, a := range v.Args {
					note(a)
				}
			case *ast.ReturnStmt:
				for _, e := range v.Results {
					note(e)
				}
			}
			return true
		})
	}
	return declared, uses
}

// collectDeclared records `ErrX SomeCode = "X"` style constants.
func collectDeclared(file *ast.File, into map[string]string) {
	for _, d := range file.Decls {
		gen, ok := d.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 {
				continue
			}
			lit, ok := vs.Values[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				continue
			}
			value := strings.Trim(lit.Value, `"`)
			// A code is SCREAMING_SNAKE. Every other string constant in these
			// packages — zone ids, offer ids, profile names — is not, which
			// is enough to tell them apart without a type check.
			if isCodeShaped(value) {
				into[vs.Names[0].Name] = value
			}
		}
	}
}

func isCodeShaped(s string) bool {
	if len(s) < 4 {
		return false
	}
	for _, r := range s {
		if (r < 'A' || r > 'Z') && r != '_' && (r < '0' || r > '9') {
			return false
		}
	}
	return strings.ContainsRune(s, '_')
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// EmittableKeys is every message key a client can be sent: the error codes
// refusals travel as, and the label keys everything else does.
//
// Two sources, because neither alone is complete. Rule ids come from actually
// asking each module for its rules across its whole option space — the only
// way to catch a sentence that exists on one setting and not another. The
// rest are read from the source, because a fact built in a view is only
// emitted when a game reaches that state, and a test cannot play every game.
type EmittableKeys struct {
	// ErrorCodes are what a refusal travels as; a client words them under
	// "err.<CODE>".
	ErrorCodes []string `json:"errorCodes"`
	// LabelKeys are everything else — rules, prompts, facts, offer labels.
	LabelKeys []string `json:"labelKeys"`
	// DeclaredOnly are codes declared but never returned. Reported so the
	// list is not silently incomplete; nothing has to word them.
	DeclaredOnly []string `json:"declaredOnly"`
}

// CollectKeys gathers the keys for every module in reg, reading source from
// dirs (relative to the server module root).
func CollectKeys(reg *Registry, dirs ...string) (EmittableKeys, error) {
	codes, declaredOnly := EmittedCodes(dirs...)
	labels := map[string]bool{}

	for _, dir := range dirs {
		for k := range labelKeysIn(dir) {
			labels[k] = true
		}
	}

	for _, id := range reg.IDs() {
		m := reg.Get(id)
		d := m.Descriptor()
		configs := []MatchConfig{{}}
		for _, v := range d.Variations {
			configs = append(configs, MatchConfig{Variation: v.ID})
			for _, opt := range d.Options {
				for _, choice := range opt.Choices {
					configs = append(configs, MatchConfig{
						Variation: v.ID,
						Options:   Options{opt.Name: choice.Value},
					})
				}
			}
		}
		for _, cfg := range configs {
			for _, s := range RulesFor(m, cfg) {
				labels[s.TitleKey] = true
				for _, it := range s.Items {
					labels[it.LabelKey] = true
				}
			}
		}
	}

	out := EmittableKeys{ErrorCodes: codes, DeclaredOnly: declaredOnly}
	for k := range labels {
		if k != "" {
			out.LabelKeys = append(out.LabelKeys, k)
		}
	}
	sortStrings(out.LabelKeys)
	return out, nil
}

// labelKeysIn reads string literals assigned to LabelKey or TitleKey.
func labelKeysIn(dir string) map[string]bool {
	out := map[string]bool{}
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	consts := map[string]string{}
	files := []*ast.File{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			continue
		}
		files = append(files, f)
		collectStringConsts(f, consts)
	}
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			kv, ok := n.(*ast.KeyValueExpr)
			if !ok {
				return true
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok || (key.Name != "LabelKey" && key.Name != "TitleKey") {
				return true
			}
			switch v := kv.Value.(type) {
			case *ast.BasicLit:
				if v.Kind == token.STRING {
					out[strings.Trim(v.Value, `"`)] = true
				}
			case *ast.Ident:
				if s, ok := consts[v.Name]; ok {
					out[s] = true
				}
			}
			return true
		})
		// Calls that take a key as their first argument: Rule("x", …),
		// Section("x", …), and the t("x") style helpers.
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			name := ""
			switch fn := call.Fun.(type) {
			case *ast.Ident:
				name = fn.Name
			case *ast.SelectorExpr:
				name = fn.Sel.Name
			}
			if name != "Rule" && name != "Section" && name != "SectionOf" {
				return true
			}
			switch v := call.Args[0].(type) {
			case *ast.BasicLit:
				if v.Kind == token.STRING {
					out[strings.Trim(v.Value, `"`)] = true
				}
			case *ast.Ident:
				if s, ok := consts[v.Name]; ok {
					out[s] = true
				}
			}
			return true
		})
	}
	return out
}

func collectStringConsts(file *ast.File, into map[string]string) {
	for _, d := range file.Decls {
		gen, ok := d.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 {
				continue
			}
			if lit, ok := vs.Values[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
				into[vs.Names[0].Name] = strings.Trim(lit.Value, `"`)
			}
		}
	}
}
