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
	// SentenceKeys are the subset that are whole sentences rather than names
	// of things: every written rule, and every remedy. The shape fallback can
	// turn `zone.drawPile` into "Draw pile" but it can never turn
	// `zolik.rules.pickup.obligation` into a rule, so these are the other
	// half of what wording is not optional for.
	SentenceKeys []string `json:"sentenceKeys"`
	// LabelKeysWithData are the subset ever emitted carrying params or a
	// value of their own. The distinction matters: a client turns an
	// unworded key into readable English by its own shape, so `zone.drawPile`
	// reads "Draw pile" with no bundle entry at all — but that fallback has
	// nowhere to put a number, so `prompt.pickupMustBeMelded` reads "Pickup
	// must be melded 7H" and `holdem.seat.stack` reads "Stack" with the
	// figure missing. These are the keys wording is not optional for.
	LabelKeysWithData []string `json:"labelKeysWithData"`
	// DeclaredOnly are codes declared but never returned. Reported so the
	// list is not silently incomplete; nothing has to word them.
	DeclaredOnly []string `json:"declaredOnly"`
}

// CollectKeys gathers the keys for every module in reg, reading source from
// dirs (relative to the server module root).
func CollectKeys(reg *Registry, dirs ...string) (EmittableKeys, error) {
	codes, declaredOnly := EmittedCodes(dirs...)
	labels := map[string]bool{}
	withData := map[string]bool{}
	// A rule or a remedy is a sentence. Rules are known exactly, from asking
	// each module below; remedies are named by convention — a `.remedy.`
	// segment — because a module emits one only when a player is actually
	// refused, which no test can provoke for every code.
	sentences := map[string]bool{}

	for _, dir := range dirs {
		for k, carries := range labelKeysIn(dir) {
			labels[k] = true
			if carries {
				withData[k] = true
			}
			if strings.Contains(k, ".remedy.") {
				sentences[k] = true
			}
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
				sentences[s.TitleKey] = true
				for _, it := range s.Items {
					labels[it.LabelKey] = true
					sentences[it.LabelKey] = true
					if len(it.Params) > 0 || it.Value != "" {
						withData[it.LabelKey] = true
					}
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
	for k := range withData {
		if k != "" {
			out.LabelKeysWithData = append(out.LabelKeysWithData, k)
		}
	}
	for k := range sentences {
		if k != "" {
			out.SentenceKeys = append(out.SentenceKeys, k)
		}
	}
	sortStrings(out.LabelKeys)
	sortStrings(out.LabelKeysWithData)
	sortStrings(out.SentenceKeys)
	return out, nil
}

// labelKeysIn reads message keys out of a package's source, and reports for
// each whether it is ever built alongside params or a value — see
// EmittableKeys.LabelKeysWithData.
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

	// A key resolved from a literal or from a string constant; anything else
	// (a variable chosen at runtime) is not statically knowable and is caught
	// instead by the dynamic pass over each module's Rules().
	keyOf := func(e ast.Expr) (string, bool) {
		switch v := e.(type) {
		case *ast.BasicLit:
			if v.Kind == token.STRING {
				return strings.Trim(v.Value, `"`), true
			}
		case *ast.Ident:
			if s, ok := consts[v.Name]; ok {
				return s, true
			}
		}
		return "", false
	}
	record := func(key string, carriesData bool) {
		if key == "" {
			return
		}
		out[key] = out[key] || carriesData
	}

	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch node := n.(type) {

			// A Fact built as a struct literal: the key is one field and the
			// data it carries is another, so both are read from the same
			// literal rather than guessed at.
			case *ast.CompositeLit:
				key, carries := "", false
				for _, elt := range node.Elts {
					kv, ok := elt.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					name, ok := kv.Key.(*ast.Ident)
					if !ok {
						continue
					}
					switch name.Name {
					case "LabelKey", "TitleKey":
						if k, ok := keyOf(kv.Value); ok {
							key = k
						}
					case "Params", "Value":
						carries = true
					case "BadgeKeys":
						// A list of keys rather than one, and each is a
						// standalone mark with nothing else in the literal
						// belonging to it — so they are recorded here rather
						// than through `key` below.
						if list, ok := kv.Value.(*ast.CompositeLit); ok {
							for _, elt := range list.Elts {
								if k, ok := keyOf(elt); ok {
									record(k, false)
								}
							}
						}
					}
				}
				record(key, carries)

			// The same fields set by assignment rather than in a literal —
			// `cv.BadgeKeys = []string{…}`, which is how a mark added
			// conditionally is written.
			case *ast.AssignStmt:
				for i, lhs := range node.Lhs {
					sel, ok := lhs.(*ast.SelectorExpr)
					if !ok || i >= len(node.Rhs) {
						continue
					}
					switch sel.Sel.Name {
					case "LabelKey", "TitleKey":
						if k, ok := keyOf(node.Rhs[i]); ok {
							record(k, false)
						}
					case "BadgeKeys":
						if list, ok := node.Rhs[i].(*ast.CompositeLit); ok {
							for _, elt := range list.Elts {
								if k, ok := keyOf(elt); ok {
									record(k, false)
								}
							}
						}
					}
				}

			// A rule built through the helpers, where the params are the
			// second argument: Rule("x", map[...]{…}) carries data,
			// Rule("x", nil) does not.
			case *ast.CallExpr:
				name := ""
				switch fn := node.Fun.(type) {
				case *ast.Ident:
					name = fn.Name
				case *ast.SelectorExpr:
					name = fn.Sel.Name
				}
				if name != "Rule" && name != "Section" && name != "SectionOf" {
					return true
				}
				if len(node.Args) == 0 {
					return true
				}
				key, ok := keyOf(node.Args[0])
				if !ok {
					return true
				}
				carries := false
				if name == "Rule" && len(node.Args) > 1 {
					ident, isIdent := node.Args[1].(*ast.Ident)
					carries = !isIdent || ident.Name != "nil"
				}
				record(key, carries)
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
