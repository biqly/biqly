package jsonusage_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

var forbiddenStdlibJSONSelectors = map[string]struct{}{
	"Compact":       {},
	"HTMLEscape":    {},
	"Indent":        {},
	"Marshal":       {},
	"MarshalIndent": {},
	"NewDecoder":    {},
	"NewEncoder":    {},
	"Unmarshal":     {},
	"Valid":         {},
}

func TestDirectStdlibJSONEncodeDecodeUsage(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()
	var failures []string

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if skip, err := shouldSkipPath(entry, err); skip || err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		jsonAliases, err := encodingJSONAliases(fset, path)
		if err != nil {
			return err
		}
		if len(jsonAliases) == 0 {
			return nil
		}

		nextFailures, err := forbiddenStdlibJSONCalls(root, fset, path, jsonAliases)
		if err != nil {
			return err
		}
		failures = append(failures, nextFailures...)
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir(%q) error = %v, want nil", root, err)
	}
	if len(failures) > 0 {
		t.Fatalf("direct encoding/json encode/decode/parser usage remains:\n%s", strings.Join(failures, "\n"))
	}
}

func shouldSkipPath(entry os.DirEntry, err error) (bool, error) {
	if err != nil {
		return false, err
	}
	if !entry.IsDir() {
		return false, nil
	}

	switch entry.Name() {
	case ".git", "frontend", "node_modules", "vendor":
		return true, filepath.SkipDir
	default:
		return false, nil
	}
}

func encodingJSONAliases(fset *token.FileSet, path string) (map[string]struct{}, error) {
	file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}

	aliases := map[string]struct{}{}
	for _, spec := range file.Imports {
		if strings.Trim(spec.Path.Value, `"`) != "encoding/json" {
			continue
		}
		if spec.Name != nil {
			aliases[spec.Name.Name] = struct{}{}
			continue
		}
		aliases["json"] = struct{}{}
	}
	return aliases, nil
}

func forbiddenStdlibJSONCalls(root string, fset *token.FileSet, path string, jsonAliases map[string]struct{}) ([]string, error) {
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}

	var failures []string
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := selector.X.(*ast.Ident)
		if !ok || !isForbiddenStdlibJSONSelector(jsonAliases, ident.Name, selector.Sel.Name) {
			return true
		}
		pos := fset.Position(selector.Pos())
		rel, err := filepath.Rel(root, pos.Filename)
		if err != nil {
			rel = pos.Filename
		}
		failures = append(failures, rel+":"+strconv.Itoa(pos.Line)+": replace encoding/json "+ident.Name+"."+selector.Sel.Name+" with sonic")
		return true
	})
	return failures, nil
}

func isForbiddenStdlibJSONSelector(jsonAliases map[string]struct{}, identName, selectorName string) bool {
	if _, ok := jsonAliases[identName]; !ok {
		return false
	}
	_, ok := forbiddenStdlibJSONSelectors[selectorName]
	return ok
}

func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v, want nil", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("repoRoot() could not find go.mod from %q", dir)
		}
		dir = parent
	}
}
