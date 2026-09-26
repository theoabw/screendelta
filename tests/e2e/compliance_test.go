// Package e2e also holds the compliance checks behind FR-016: the engine runs without network access and
// writes nothing outside a path it was given.
//
// The checks are two kinds. The static ones walk the module's syntax trees and refuse a network import, a
// process execution, or a filesystem write in a file that has no business writing. The dynamic one runs the
// real command and compares the filesystem before and after. The static half is the one that keeps the
// guarantee as the code grows: a future change that reaches for the network fails a test rather than
// quietly breaking a promise in the specification.
package e2e

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/theoabw/screendelta/internal/corpus"
)

// runIn runs the command with a chosen working directory.
func runIn(t *testing.T, dir string, env []string, args ...string) result {
	t.Helper()
	command := exec.Command(binary(t), args...)
	command.Dir = dir
	command.Env = append(os.Environ(), env...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()

	code := 0
	var exit *exec.ExitError
	if err != nil {
		if !asExitError(err, &exit) {
			t.Fatalf("cannot run %v: %v", args, err)
		}
		code = exit.ExitCode()
	}
	return result{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// moduleRoot walks up from the test's directory to the directory holding go.mod.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("cannot read the working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("cannot find the module root")
		}
		dir = parent
	}
}

// sourceFile is one parsed file, with the path used in messages.
type sourceFile struct {
	path string
	file *ast.File
}

// engineSources parses every non-test Go file in the repository's own packages.
//
// Test files are excluded because a test may legitimately start a process, and the promise is about the
// engine rather than about its tests.
func engineSources(t *testing.T) []sourceFile {
	t.Helper()
	root := moduleRoot(t)

	var sources []sourceFile
	for _, top := range []string{"internal", "cmd", "tools"} {
		err := filepath.Walk(filepath.Join(root, top), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			sources = append(sources, sourceFile{path: relative, file: parsed})
			return nil
		})
		if err != nil {
			t.Fatalf("cannot walk %s: %v", top, err)
		}
	}
	if len(sources) == 0 {
		t.Fatal("the walk found no source files, so the check would pass vacuously")
	}
	return sources
}

// TestEngineImportsNoNetworkPackage is the static half of the no-network guarantee.
func TestEngineImportsNoNetworkPackage(t *testing.T) {
	forbidden := []string{"net", "net/http", "net/rpc", "net/smtp", "net/mail", "net/url", "plugin"}
	for _, source := range engineSources(t) {
		for _, imported := range source.file.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				continue
			}
			for _, banned := range forbidden {
				if path == banned || strings.HasPrefix(path, banned+"/") {
					t.Errorf("%s imports %q, and the engine must run without network access", source.path, path)
				}
			}
			if path == "C" {
				t.Errorf("%s imports cgo, which the build disables", source.path)
			}
		}
	}
}

// TestEngineStartsNoProcess is the other half: an engine that shells out could hide a network call inside
// another program, and the guarantee would be worth nothing.
func TestEngineStartsNoProcess(t *testing.T) {
	for _, source := range engineSources(t) {
		for _, imported := range source.file.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				continue
			}
			if path == "os/exec" || path == "syscall" {
				t.Errorf("%s imports %q, so the engine could start another program", source.path, path)
			}
		}
	}
}

// TestWritesHappenOnlyWhereDeclared checks FR-016's other half statically: every filesystem write in the
// module happens in a file that is allowed to write, which is the command writing its declared output and
// the corpus generator writing frames for a person to inspect.
func TestWritesHappenOnlyWhereDeclared(t *testing.T) {
	allowed := map[string]bool{
		"cmd/screendelta/main.go": true,
	}
	allowedPrefixes := []string{"internal/corpus/"}

	writers := map[string]bool{
		"os.Create":     true,
		"os.WriteFile":  true,
		"os.OpenFile":   true,
		"os.MkdirAll":   true,
		"os.Mkdir":      true,
		"os.Remove":     true,
		"os.RemoveAll":  true,
		"os.Rename":     true,
		"os.Chmod":      true,
		"os.Truncate":   true,
		"os.Symlink":    true,
		"os.CreateTemp": true,
	}

	for _, source := range engineSources(t) {
		parsed, err := parser.ParseFile(token.NewFileSet(), filepath.Join(moduleRoot(t), source.path), nil, 0)
		if err != nil {
			t.Fatalf("cannot parse %s: %v", source.path, err)
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			name := selectorName(call.Fun)
			if name == "" || !writers[name] {
				return true
			}
			if allowed[source.path] || hasPrefix(source.path, allowedPrefixes) {
				return true
			}
			t.Errorf("%s calls %s, and only the command's declared output and the corpus generator may write",
				source.path, name)
			return true
		})
	}
}

// selectorName renders a call's callee as "package.Function" when it is one.
func selectorName(expression ast.Expr) string {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	identifier, ok := selector.X.(*ast.Ident)
	if !ok {
		return ""
	}
	return identifier.Name + "." + selector.Sel.Name
}

func hasPrefix(value string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

// TestARunWritesOnlyItsDeclaredOutput is the dynamic half: the filesystem is compared before and after a
// successful run, and the only difference allowed is the file the command was told to write.
func TestARunWritesOnlyItsDeclaredOutput(t *testing.T) {
	dir, manifest := frames(t, "changed-label", corpus.DefaultOptions())
	before, after := manifest.Paths[0], manifest.Paths[1]

	// A working directory of its own, so anything the command writes relative to itself shows up.
	working := t.TempDir()
	output := filepath.Join(working, "delta.json")

	beforeTree := treeOf(t, dir)
	beforeWorking := treeOf(t, working)

	got := runIn(t, working, nil, "diff", "--previous", before, "--current", after, "--out", output)
	if got.code != 0 {
		t.Fatalf("exit code %d: %s", got.code, got.stderr)
	}
	if strings.TrimSpace(got.stdout) != "" {
		t.Fatalf("a command with --out also wrote to standard output: %q", got.stdout)
	}

	if changed := diffTrees(beforeTree, treeOf(t, dir)); len(changed) != 0 {
		t.Fatalf("the run changed the input directory: %v", changed)
	}
	changed := diffTrees(beforeWorking, treeOf(t, working))
	if len(changed) != 1 || changed[0] != "delta.json" {
		t.Fatalf("the run left %v in its working directory, and only delta.json may appear", changed)
	}
}

// treeOf lists a directory tree as relative paths with their sizes.
func treeOf(t *testing.T, root string) map[string]int64 {
	t.Helper()
	tree := map[string]int64{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		if info.IsDir() {
			tree[relative+"/"] = 0
			return nil
		}
		tree[relative] = info.Size()
		return nil
	})
	if err != nil {
		t.Fatalf("cannot walk %s: %v", root, err)
	}
	return tree
}

// diffTrees returns the entries that differ between two trees.
func diffTrees(before, after map[string]int64) []string {
	var changed []string
	for path, size := range after {
		if previous, ok := before[path]; !ok || previous != size {
			changed = append(changed, path)
		}
	}
	for path := range before {
		if _, ok := after[path]; !ok {
			changed = append(changed, path+" (removed)")
		}
	}
	return changed
}
