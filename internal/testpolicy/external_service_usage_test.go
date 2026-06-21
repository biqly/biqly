package testpolicy_test

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

// liveConnectCallees are functions that open real network connections when invoked.
var liveConnectCallees = map[string]struct{}{
	"ConnectNATS": {},
	"Connect":     {}, // nats.Connect
}

func TestUnitTestsDoNotRequireLiveExternalServices(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()
	var failures []string

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if skip, err := shouldSkipPath(entry, err); skip || err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if strings.Contains(path, filepath.Join("internal", "testpolicy", "testdata")) {
			return nil
		}
		if strings.HasSuffix(path, "integration_test.go") {
			return nil
		}

		fileFailures, err := liveConnectRequireNoError(root, fset, path)
		if err != nil {
			return err
		}
		failures = append(failures, fileFailures...)
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir(%q) error = %v, want nil", root, err)
	}
	if len(failures) > 0 {
		t.Fatalf(
			"unit tests require live external services (mock/skip or move to integration_test.go):\n%s",
			strings.Join(failures, "\n"),
		)
	}
}

func liveConnectRequireNoError(root string, fset *token.FileSet, path string) ([]string, error) {
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}
	if hasIntegrationBuildTag(file) {
		return nil, nil
	}

	var failures []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || !strings.HasPrefix(fn.Name.Name, "Test") {
			continue
		}
		failures = append(failures, scanTestFunctionForLiveConnect(root, fset, fn)...)
	}

	return failures, nil
}

type liveConnectScan struct {
	liveErrVars map[string]struct{}
	hasSkip     bool
}

func scanTestFunctionForLiveConnect(root string, fset *token.FileSet, fn *ast.FuncDecl) []string {
	scan := collectLiveConnectScan(fn.Body)
	if scan.hasSkip {
		return nil
	}
	return liveConnectRequireFailures(root, fset, fn.Name.Name, fn.Body, scan.liveErrVars)
}

func collectLiveConnectScan(body *ast.BlockStmt) liveConnectScan {
	scan := liveConnectScan{liveErrVars: map[string]struct{}{}}
	ast.Inspect(body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.CallExpr:
			if isSkipCall(n) {
				scan.hasSkip = true
			}
		case *ast.AssignStmt:
			for i, rhs := range n.Rhs {
				call, ok := rhs.(*ast.CallExpr)
				if !ok || !isLiveConnectCall(call) {
					continue
				}
				for _, name := range errNameFromAssignLHS(n.Lhs, i, len(n.Rhs)) {
					scan.liveErrVars[name] = struct{}{}
				}
			}
		}
		return true
	})
	return scan
}

func liveConnectRequireFailures(root string, fset *token.FileSet, testName string, body *ast.BlockStmt, liveErrVars map[string]struct{}) []string {
	var failures []string
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || !isRequireOrAssertNoError(call) {
			return true
		}
		for _, arg := range call.Args {
			ident, ok := arg.(*ast.Ident)
			if !ok {
				continue
			}
			if _, live := liveErrVars[ident.Name]; !live {
				continue
			}
			pos := fset.Position(call.Pos())
			rel, relErr := filepath.Rel(root, pos.Filename)
			if relErr != nil {
				rel = pos.Filename
			}
			failures = append(failures, rel+":"+strconv.Itoa(pos.Line)+":"+testName+
				" must not require.NoError/assert.NoError on "+ident.Name+
				" from a live ConnectNATS/nats.Connect call; mock, extract defaults, or t.Skip when unavailable")
		}
		return true
	})
	return failures
}

func errNameFromAssignLHS(lhs []ast.Expr, rhsIndex, rhsCount int) []string {
	if rhsCount == 1 {
		if len(lhs) == 2 {
			if ident, ok := lhs[1].(*ast.Ident); ok && strings.HasPrefix(ident.Name, "err") {
				return []string{ident.Name}
			}
		}
		return nil
	}
	if rhsIndex >= len(lhs) {
		return nil
	}
	if ident, ok := lhs[rhsIndex].(*ast.Ident); ok && strings.HasPrefix(ident.Name, "err") {
		return []string{ident.Name}
	}
	if rhsIndex+1 < len(lhs) {
		if ident, ok := lhs[rhsIndex+1].(*ast.Ident); ok && strings.HasPrefix(ident.Name, "err") {
			return []string{ident.Name}
		}
	}
	return nil
}

func TestLiveConnectPolicyDetectsForbiddenFixture(t *testing.T) {
	root := repoRoot(t)
	fixture := filepath.Join(root, "internal", "testpolicy", "testdata", "forbidden", "forbidden_test.go")
	fset := token.NewFileSet()
	failures, err := liveConnectRequireNoError(root, fset, fixture)
	if err != nil {
		t.Fatalf("liveConnectRequireNoError() error = %v, want nil", err)
	}
	if len(failures) == 0 {
		t.Fatal("expected fixture to trigger live-connect policy violation")
	}
}

func hasIntegrationBuildTag(file *ast.File) bool {
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			text := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
			if strings.HasPrefix(text, "go:build integration") || strings.HasPrefix(text, "+build integration") {
				return true
			}
		}
	}
	return false
}

func isLiveConnectCall(call *ast.CallExpr) bool {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		_, ok := liveConnectCallees[fun.Name]
		return ok
	case *ast.SelectorExpr:
		if fun.Sel.Name != "Connect" {
			return false
		}
		if ident, ok := fun.X.(*ast.Ident); ok {
			return ident.Name == "nats"
		}
	}
	return false
}

func isRequireOrAssertNoError(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	switch pkg.Name + "." + sel.Sel.Name {
	case "require.NoError", "assert.NoError":
		return true
	default:
		return false
	}
}

func isSkipCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	recv, ok := sel.X.(*ast.Ident)
	return ok && recv.Name == "t" && sel.Sel.Name == "Skip"
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
