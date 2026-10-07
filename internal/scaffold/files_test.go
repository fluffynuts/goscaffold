package scaffold

import (
	"bytes"
	"go/format"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"goscaffold"
	"goscaffold/internal/ui"
)

var testParams = Params{App: "my-tool", Module: "github.com/o/my-tool", Repo: "o/my-tool", Branch: "trunk", Pkg: "./src"}

func TestFilesRendersEverythingWithNoHolesLeft(t *testing.T) {
	files, err := Files(goscaffold.Assets, testParams, Plan{Main: true, Readme: true, Gitignore: true})
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]File{}
	for _, f := range files {
		have[f.Path] = f
		if strings.Contains(string(f.Content), "@@") {
			t.Errorf("%s still has an unfilled hole:\n%s", f.Path, grep(string(f.Content), "@@"))
		}
		if strings.Contains(strings.ToLower(string(f.Content)), "vibe") {
			t.Errorf("%s still mentions vibe:\n%s", f.Path, grep(strings.ToLower(string(f.Content)), "vibe"))
		}
	}
	for _, want := range []string{
		"VERSION", "Makefile", "make.sh", "make.ps1", "install.sh", "install.ps1", ".github/workflows/build.yml",
		"src/main.go", "README.md", ".gitignore",
		"internal/appcli/config.go", "internal/appcli/version.go", "internal/appcli/handle.go", "internal/appcli/install.go",
		"internal/appcli/upgrade.go", "internal/appcli/path_other.go", "internal/appcli/path_windows.go", "internal/appcli/appcli_test.go",
	} {
		if _, ok := have[want]; !ok {
			t.Errorf("no %s in the plan", want)
		}
	}
	if !strings.HasPrefix(string(have["README.md"].Content), "my-tool\n---\n") {
		t.Errorf("README.md starts %q, want the project name underlined", have["README.md"].Content)
	}
	if !strings.Contains(string(have["README.md"].Content), "https://raw.githubusercontent.com/o/my-tool/trunk/install.sh | sh") {
		t.Errorf("README.md has no install instructions for the right repo and branch:\n%s", have["README.md"].Content)
	}
	wf := string(have[".github/workflows/build.yml"].Content)
	if !strings.Contains(wf, "github.ref == 'refs/heads/trunk'") || !strings.Contains(wf, "my-tool-linux-amd64.zip") {
		t.Errorf("the workflow doesn't release trunk, or doesn't name zips after the app:\n%s", grep(wf, "refs/heads"))
	}
	if !strings.Contains(string(have["src/main.go"].Content), `"github.com/o/my-tool/internal/appcli"`) {
		t.Errorf("main doesn't import appcli by the module path:\n%s", have["src/main.go"].Content)
	}
	if !strings.Contains(string(have["Makefile"].Content), "PKG    ?= ./src") {
		t.Errorf("the Makefile doesn't build ./src:\n%s", have["Makefile"].Content)
	}
	if !strings.Contains(string(have["internal/appcli/version.go"].Content), "-X github.com/o/my-tool/internal/appcli.Version") {
		t.Error("version.go doesn't say how to set the version by the module path")
	}
	ignore := string(have[".gitignore"].Content)
	for _, line := range []string{"/my-tool\n", "/dist/\n", "*.exe\n"} {
		if !strings.Contains(ignore, line) {
			t.Errorf(".gitignore lacks %q", line)
		}
	}
	for _, script := range []string{"make.sh", "install.sh"} {
		if have[script].Mode&0o111 == 0 {
			t.Errorf("%s isn't executable", script)
		}
	}
}

func TestGeneratedGoCodeIsGofmtClean(t *testing.T) {
	files, err := Files(goscaffold.Assets, testParams, Plan{Main: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".go") {
			continue
		}
		formatted, err := format.Source(f.Content)
		if err != nil {
			t.Errorf("%s doesn't parse: %v", f.Path, err)
		} else if !bytes.Equal(formatted, f.Content) {
			t.Errorf("%s isn't gofmt-clean", f.Path)
		}
	}
}

func grep(text, needle string) string {
	var out []string
	for _, l := range strings.Split(text, "\n") {
		if strings.Contains(l, needle) {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

func TestFilesLeavesOutWhatThePlanDoesNot(t *testing.T) {
	files, err := Files(goscaffold.Assets, testParams, Plan{})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		switch f.Path {
		case "src/main.go", "README.md", ".gitignore":
			t.Errorf("%s was planned", f.Path)
		}
	}
}

// answers is a ui.UI that answers confirmations from a list, and records them.
type answers struct {
	ui.Defaults
	confirms []bool
	asked    []string
}

func (a *answers) Confirm(q string, def bool) (bool, error) {
	a.asked = append(a.asked, q)
	if len(a.confirms) == 0 {
		return def, nil
	}
	yes := a.confirms[0]
	a.confirms = a.confirms[1:]
	return yes, nil
}

func TestWriteCreatesAndAsksBeforeReplacing(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "Makefile", "mine\n")
	put(t, dir, "VERSION", "0.3\n")
	put(t, dir, "same.txt", "same\n")
	files := []File{
		{Path: "Makefile", Content: []byte("theirs\n"), Mode: 0o644},
		{Path: "VERSION", Content: []byte("0.1\n"), Mode: 0o644},
		{Path: "same.txt", Content: []byte("same\n"), Mode: 0o644},
		{Path: "sub/new.sh", Content: []byte("#!/bin/sh\n"), Mode: 0o755},
	}
	a := &answers{confirms: []bool{true, false}}
	var out bytes.Buffer
	outcomes, err := Write(dir, files, a, false, &out)
	if err != nil {
		t.Fatal(err)
	}
	if outcomes["Makefile"] != Replaced || outcomes["VERSION"] != Kept || outcomes["same.txt"] != Unchanged || outcomes["sub/new.sh"] != Created {
		t.Errorf("outcomes %v\n%s", outcomes, out.String())
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "Makefile")); string(got) != "theirs\n" {
		t.Errorf("Makefile is %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "VERSION")); string(got) != "0.3\n" {
		t.Errorf("VERSION was changed to %q after the user said no", got)
	}
	wantAsked := []string{"Makefile already exists - replace it?", "VERSION already exists - replace it?"}
	if strings.Join(a.asked, "|") != strings.Join(wantAsked, "|") {
		t.Errorf("asked %q, want %q (same.txt needs no asking)", a.asked, wantAsked)
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(filepath.Join(dir, "sub/new.sh")); info == nil || info.Mode().Perm()&0o100 == 0 {
			t.Error("sub/new.sh isn't executable")
		}
	}
}

func TestWriteWithNoOneToAskLeavesFilesAlone(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "Makefile", "mine\n")
	outcomes, err := Write(dir, []File{{Path: "Makefile", Content: []byte("theirs\n"), Mode: 0o644}}, ui.Defaults{}, false, &bytes.Buffer{})
	if err != nil || outcomes["Makefile"] != Kept {
		t.Fatalf("outcomes %v, err %v", outcomes, err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "Makefile")); string(got) != "mine\n" {
		t.Errorf("Makefile is %q", got)
	}
}

func TestWriteForceReplacesWithoutAsking(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "Makefile", "mine\n")
	a := &answers{}
	outcomes, err := Write(dir, []File{{Path: "Makefile", Content: []byte("theirs\n"), Mode: 0o644}}, a, true, &bytes.Buffer{})
	if err != nil || outcomes["Makefile"] != Replaced || len(a.asked) != 0 {
		t.Fatalf("outcomes %v, asked %v, err %v", outcomes, a.asked, err)
	}
}

func TestReplacedScriptStaysRunnable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no executable bit")
	}
	dir := t.TempDir()
	put(t, dir, "make.sh", "old\n") // 0644
	if _, err := Write(dir, []File{{Path: "make.sh", Content: []byte("new\n"), Mode: 0o755}}, ui.Defaults{}, true, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(filepath.Join(dir, "make.sh")); info.Mode().Perm()&0o100 == 0 {
		t.Error("a replaced make.sh lost its executable bit")
	}
}

func TestMissingIgnores(t *testing.T) {
	cases := []struct {
		ignore string
		want   []string
	}{
		{"", []string{"/tool", "/dist/"}},
		{"/tool\n/dist/\n", nil},
		{"tool\ndist\n", nil},
		{"# build\n/tool/\ndist/\n", nil},
		{"/dist/\n", []string{"/tool"}},
	}
	for _, c := range cases {
		got := MissingIgnores(c.ignore, "tool")
		if strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("MissingIgnores(%q) = %q, want %q", c.ignore, got, c.want)
		}
	}
}
