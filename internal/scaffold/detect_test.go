package scaffold

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func put(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestModulePath(t *testing.T) {
	cases := map[string]string{
		"module github.com/o/r\n\ngo 1.25\n":     "github.com/o/r",
		"module   example.com/x  // the thing\n": "example.com/x",
		"// header\nmodule \"quoted/path\"\n":    "quoted/path",
		"modulefoo\n":                            "",
		"go 1.25\n":                              "",
	}
	for content, want := range cases {
		dir := t.TempDir()
		put(t, dir, "go.mod", content)
		if got := ModulePath(dir); got != want {
			t.Errorf("ModulePath(%q) = %q, want %q", content, got, want)
		}
	}
	if got := ModulePath(t.TempDir()); got != "" {
		t.Errorf("no go.mod gave %q", got)
	}
}

func TestMainPackagesShallowestFirst(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "cmd/tool/main.go", "package main\nfunc main(){}\n")
	put(t, dir, "src/app/main.go", "package main\nfunc main(){}\n")
	put(t, dir, "main.go", "package main\nfunc main(){}\n")
	put(t, dir, "lib/lib.go", "package lib\n")
	put(t, dir, "lib/lib_test.go", "package main\n") // a test file doesn't make a program
	got := MainPackages(dir)
	want := []string{".", "./cmd/tool", "./src/app"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGoSourcesIgnoreWhatIsNotTheProjects(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "internal/appcli/config.go", "package appcli\n")
	put(t, dir, "vendor/x/x.go", "package x\n")
	put(t, dir, "node_modules/y/y.go", "package y\n")
	put(t, dir, ".hidden/z.go", "package z\n")
	put(t, dir, "pkg/testdata/t.go", "package t\n")
	put(t, dir, "nested/go.mod", "module nested\n")
	put(t, dir, "nested/main.go", "package main\n")
	put(t, dir, "README.md", "x")
	if HasGoSources(dir) {
		t.Errorf("found Go sources in %v", goFiles(dir))
	}
	if got := MainPackages(dir); len(got) != 0 {
		t.Errorf("found main packages %v in a nested module", got)
	}
	put(t, dir, "pkg/real.go", "package pkg\n")
	if !HasGoSources(dir) {
		t.Error("missed pkg/real.go")
	}
}

func TestIsEmptyDir(t *testing.T) {
	dir := t.TempDir()
	if !IsEmptyDir(dir) {
		t.Error("an empty folder isn't empty")
	}
	if IsEmptyDir(filepath.Join(dir, "missing")) {
		t.Error("a missing folder is empty")
	}
	put(t, dir, "f", "x")
	if IsEmptyDir(dir) {
		t.Error("a folder with a file is empty")
	}
}

func TestUsesAppcli(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "main.go", "package main\n\nimport \"example.com/x\"\n")
	put(t, dir, "internal/appcli/handle.go", "package appcli\n\n// mentions example.com/x/internal/appcli\"\n")
	if UsesAppcli(dir) {
		t.Error("counted the package's own files, or an unrelated import")
	}
	put(t, dir, "main.go", "package main\n\nimport \"example.com/x/internal/appcli\"\n")
	if !UsesAppcli(dir) {
		t.Error("missed the import")
	}
}
