package scaffold

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"goscaffold"
	"goscaffold/internal/sh"
	"goscaffold/internal/ui"
)

// script is a ui.UI that answers from lists, and records what it was asked.
type script struct {
	interactive bool
	selects     []int
	inputs      []string
	confirms    []bool

	asked   []string // every question, in order
	choices [][]string
}

func (s *script) Interactive() bool { return s.interactive }

func (s *script) Select(q string, options []string, def int) (int, error) {
	s.asked = append(s.asked, q)
	s.choices = append(s.choices, options)
	if len(s.selects) == 0 {
		return def, nil
	}
	i := s.selects[0]
	s.selects = s.selects[1:]
	return i, nil
}

func (s *script) Input(q, def string) (string, error) {
	s.asked = append(s.asked, q)
	if len(s.inputs) == 0 {
		return def, nil
	}
	a := s.inputs[0]
	s.inputs = s.inputs[1:]
	if a == "" {
		return def, nil
	}
	return a, nil
}

func (s *script) Confirm(q string, def bool) (bool, error) {
	s.asked = append(s.asked, q)
	if len(s.confirms) == 0 {
		return def, nil
	}
	a := s.confirms[0]
	s.confirms = s.confirms[1:]
	return a, nil
}

func (s *script) askedAbout(fragment string) bool {
	for _, q := range s.asked {
		if strings.Contains(q, fragment) {
			return true
		}
	}
	return false
}

// world fakes gh, git and go, with real folders for the commands that make
// them: a clone makes the folder and its .git, `go mod init` writes a go.mod.
type world struct {
	user       string
	orgs       []string
	branch     string // what a clone lands on; "" means the command fails
	origin     string // the project's origin remote, if any
	tidyFails  bool
	createFail error
	existing   []string // repositories already on GitHub, as owner/name
	protocol   string   // gh's git_protocol setting
}

func (w *world) runner(missing ...string) *sh.Fake {
	f := &sh.Fake{Missing: map[string]bool{}}
	for _, m := range missing {
		f.Missing[m] = true
	}
	f.Handler = func(dir, name string, args []string) (string, error) {
		cmd := name + " " + strings.Join(args, " ")
		switch {
		case cmd == "gh api user --jq .login":
			return w.user + "\n", nil
		case cmd == "gh org list":
			return strings.Join(w.orgs, "\n") + "\n", nil
		case strings.HasPrefix(cmd, "gh repo view "):
			if slices.Contains(w.existing, args[2]) {
				return "{\"name\":\"x\"}\n", nil
			}
			return "", errors.New("gh repo view: exit status 1: GraphQL: Could not resolve to a Repository with the name '" + args[2] + "'. (repository)")
		case cmd == "gh config get git_protocol":
			return w.protocol + "\n", nil
		case strings.HasPrefix(cmd, "gh repo create"):
			return "", w.createFail
		case strings.HasPrefix(cmd, "gh repo clone"):
			target := args[len(args)-1]
			return "", os.MkdirAll(filepath.Join(target, ".git"), 0o755)
		case strings.HasPrefix(cmd, "git remote add "):
			w.origin = args[3]
			return "", nil
		case cmd == "git init":
			return "", os.MkdirAll(filepath.Join(dir, ".git"), 0o755)
		case cmd == "git remote get-url origin":
			if w.origin == "" {
				return "", errors.New("No such remote 'origin'")
			}
			return w.origin + "\n", nil
		case cmd == "git symbolic-ref --short HEAD":
			if w.branch == "" {
				return "", errors.New("not on a branch")
			}
			return w.branch + "\n", nil
		case name == "git":
			return "", errors.New("unexpected git " + cmd)
		case strings.HasPrefix(cmd, "go mod init "):
			return "", os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module "+args[2]+"\n\ngo 1.25\n"), 0o644)
		case cmd == "go mod tidy" && w.tidyFails:
			return "", errors.New("no network")
		}
		return "", nil
	}
	return f
}

func read(t *testing.T, dir, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func exists(dir, rel string) bool {
	_, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel)))
	return err == nil
}

func doRun(t *testing.T, o Options, u ui.UI, f *sh.Fake) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := Run(o, Env{UI: u, Sh: f, Assets: goscaffold.Assets, Out: &out})
	return out.String(), err
}

func TestNewProjectAsksWhereAndHowThenCreatesAndClonesTheRepository(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "some-project")
	w := &world{user: "bobsaget", orgs: []string{"org-a", "org-b"}, branch: "trunk"}
	f := w.runner()
	u := &script{interactive: true, selects: []int{2, 1}} // org-b, private

	out, err := doRun(t, Options{Path: dir}, u, f)
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}

	if !u.askedAbout("Project name") || !u.askedAbout("Where should I create this repository?") || !u.askedAbout("public or private") {
		t.Errorf("asked %q", u.asked)
	}
	if got := strings.Join(u.choices[0], "|"); got != "bobsaget|org-a|org-b" {
		t.Errorf("owners offered: %s (the user's own account goes first)", got)
	}
	if !f.Ran("gh repo create org-b/some-project --private") {
		t.Errorf("ran %q", f.Calls)
	}
	if !f.Ran("gh repo clone org-b/some-project " + dir) {
		t.Errorf("ran %q", f.Calls)
	}
	if !f.Ran("go mod init github.com/org-b/some-project") {
		t.Errorf("ran %q", f.Calls)
	}
	if !f.Ran("go mod tidy") {
		t.Errorf("a new go.mod isn't tidied: %q", f.Calls)
	}

	if got := read(t, dir, "README.md"); !strings.HasPrefix(got, "some-project\n---\n") {
		t.Errorf("README.md: %q", got)
	}
	if got := read(t, dir, ".github/workflows/build.yml"); !strings.Contains(got, "refs/heads/trunk") {
		t.Error("the workflow doesn't release from the branch the clone landed on")
	}
	if !exists(dir, "src/main.go") || !exists(dir, "internal/appcli/handle.go") || !exists(dir, "make.ps1") || !exists(dir, "install.ps1") {
		t.Error("files are missing")
	}
	if f.Ran("git commit") || f.Ran("git push") || f.Ran("git add") {
		t.Errorf("committed or pushed: %q", f.Calls)
	}
}

func TestNewProjectUsesARepositoryThatIsAlreadyOnGitHub(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "some-project")
	f := (&world{user: "bob", branch: "main", existing: []string{"bob/some-project"}}).runner()
	u := &script{interactive: true}
	out, err := doRun(t, Options{Path: dir}, u, f)
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if f.Ran("gh repo create") {
		t.Errorf("tried to create a repository that's already there: %q", f.Calls)
	}
	if u.askedAbout("public or private") {
		t.Error("asked about the visibility of a repository that already has one")
	}
	if !f.Ran("gh repo clone bob/some-project " + dir) {
		t.Errorf("ran %q", f.Calls)
	}
	if !strings.Contains(out, "already on GitHub") {
		t.Errorf("didn't say it's using the existing repository:\n%s", out)
	}
	if !exists(dir, "src/main.go") {
		t.Error("files are missing")
	}
}

func TestNewProjectStopsWhenGitHubCantSayWhetherTheRepositoryExists(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "p")
	f := (&world{user: "bob", branch: "main"}).runner()
	handler := f.Handler
	f.Handler = func(dir, name string, args []string) (string, error) {
		if name == "gh" && args[0] == "repo" && args[1] == "view" {
			return "", errors.New("HTTP 401: Bad credentials")
		}
		return handler(dir, name, args)
	}
	if _, err := doRun(t, Options{Path: dir}, ui.Defaults{}, f); err == nil || !strings.Contains(err.Error(), "Bad credentials") {
		t.Errorf("got %v", err)
	}
	if f.Ran("gh repo create") {
		t.Errorf("ran %q", f.Calls)
	}
}

func TestNewProjectProjectNameDefaultsToTheFolderAndCanBeChanged(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "some-project")
	f := (&world{user: "bob", branch: "main"}).runner()
	u := &script{interactive: true, inputs: []string{"better-name"}}
	if out, err := doRun(t, Options{Path: dir}, u, f); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !f.Ran("gh repo create bob/better-name --public") {
		t.Errorf("ran %q", f.Calls)
	}
	if !strings.HasPrefix(read(t, dir, "README.md"), "better-name\n---") {
		t.Error("the readme doesn't use the name that was typed")
	}

	f = (&world{user: "bob", branch: "main"}).runner()
	dir2 := filepath.Join(parent, "another-project")
	if out, err := doRun(t, Options{Path: dir2}, &script{interactive: true}, f); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !f.Ran("gh repo create bob/another-project --public") {
		t.Errorf("with no answer given, ran %q (the folder's name is the default, and public is)", f.Calls)
	}
}

func TestNewProjectWorksWithNoOneToAskWhenTheFlagsSayEverything(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "p")
	f := (&world{user: "bob", branch: "main"}).runner()
	out, err := doRun(t, Options{Path: dir, Name: "tool", Org: "acme", Visibility: "private"}, ui.Defaults{}, f)
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !f.Ran("gh repo create acme/tool --private") {
		t.Errorf("ran %q", f.Calls)
	}
	if f.Ran("gh api user") || f.Ran("gh org list") {
		t.Errorf("asked gh about the user though --org was given: %q", f.Calls)
	}
}

func TestNewProjectWithNoFlagsAndNoOneToAskUsesTheUsersAccountAndPublic(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "p")
	f := (&world{user: "bob", orgs: []string{"acme"}, branch: "main"}).runner()
	if out, err := doRun(t, Options{Path: dir}, ui.Defaults{}, f); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !f.Ran("gh repo create bob/p --public") {
		t.Errorf("ran %q", f.Calls)
	}
}

func TestNewProjectInAnEmptyExistingFolder(t *testing.T) {
	dir := t.TempDir()
	f := (&world{user: "bob", branch: "main"}).runner()
	if out, err := doRun(t, Options{Path: dir, Name: "tool"}, ui.Defaults{}, f); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !f.Ran("gh repo clone bob/tool " + dir) {
		t.Errorf("ran %q", f.Calls)
	}
	if !exists(dir, "Makefile") {
		t.Error("no Makefile")
	}
}

func TestNewProjectUsesTheBranchTheRepositoryComesWith(t *testing.T) {
	for _, branch := range []string{"main", "master", "trunk"} {
		dir := filepath.Join(t.TempDir(), "p")
		f := (&world{user: "bob", branch: branch}).runner()
		if out, err := doRun(t, Options{Path: dir}, ui.Defaults{}, f); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		wf := read(t, dir, ".github/workflows/build.yml")
		if !strings.Contains(wf, "refs/heads/"+branch+"'") || !strings.Contains(read(t, dir, "README.md"), "/bob/p/"+branch+"/install.sh") {
			t.Errorf("%s: the workflow or readme doesn't use it", branch)
		}
	}
}

func TestNewProjectWithoutAGitHubRepository(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "p")
	f := (&world{}).runner()
	out, err := doRun(t, Options{Path: dir, NoRepo: true}, ui.Defaults{}, f)
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if f.Ran("gh") {
		t.Errorf("ran gh: %q", f.Calls)
	}
	if !exists(dir, ".git") || !f.Ran("go mod init p") {
		t.Errorf("not a git repo with a module: %q", f.Calls)
	}
	if !strings.Contains(out, "OWNER/REPO") {
		t.Errorf("no warning about the placeholder repository:\n%s", out)
	}
	if !strings.Contains(read(t, dir, "README.md"), "OWNER/REPO") {
		t.Error("the readme doesn't have the placeholder to fill in")
	}
}

func TestNewProjectNeedsGhUnlessToldNotTo(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "p")
	_, err := doRun(t, Options{Path: dir}, ui.Defaults{}, (&world{}).runner("gh"))
	if err == nil || !strings.Contains(err.Error(), "--no-repo") {
		t.Fatalf("got %v, want a pointer to --no-repo", err)
	}
	if exists(dir, "Makefile") {
		t.Error("wrote files anyway")
	}
}

func TestNewProjectStopsWhenTheRepositoryCantBeCreated(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "p")
	w := &world{user: "bob", createFail: errors.New("name already exists")}
	_, err := doRun(t, Options{Path: dir}, ui.Defaults{}, w.runner())
	if err == nil || !strings.Contains(err.Error(), "name already exists") {
		t.Fatalf("got %v", err)
	}
	if exists(dir, "Makefile") {
		t.Error("wrote files anyway")
	}
}

func TestNewProjectRefusesAnUnusableOwnerBeforeCreatingAnything(t *testing.T) {
	f := (&world{user: "bob"}).runner()
	_, err := doRun(t, Options{Path: filepath.Join(t.TempDir(), "p"), Org: "not an org"}, ui.Defaults{}, f)
	if err == nil || !strings.Contains(err.Error(), "not an org") {
		t.Fatalf("got %v", err)
	}
	if f.Ran("gh repo create") {
		t.Errorf("created a repository for an unusable owner: %q", f.Calls)
	}
}

func TestNewProjectRefusesAnUnusableName(t *testing.T) {
	_, err := doRun(t, Options{Path: filepath.Join(t.TempDir(), "p"), Name: "my tool"}, ui.Defaults{}, (&world{user: "bob"}).runner())
	if err == nil {
		t.Fatal("accepted a name with a space in it")
	}
}

func TestTidyFailingIsAWarningNotAFailure(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "p")
	out, err := doRun(t, Options{Path: dir, NoRepo: true}, ui.Defaults{}, (&world{tidyFails: true}).runner())
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !strings.Contains(out, "go mod tidy") || !strings.Contains(out, "no network") {
		t.Errorf("no word about it:\n%s", out)
	}
}

func existingProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	put(t, dir, "go.mod", "module example.com/legacy\n\ngo 1.22\n")
	put(t, dir, "cmd/legacy/main.go", "package main\n\nfunc main() {}\n")
	put(t, dir, "README.md", "legacy\n===\n\nmy own words\n")
	put(t, dir, ".gitignore", "*.log\n")
	return dir
}

func TestExistingProjectWithAGitHubRemoteIsNotGivenAnotherRepository(t *testing.T) {
	dir := existingProject(t)
	os.Mkdir(filepath.Join(dir, ".git"), 0o755)
	w := &world{origin: "git@github.com:acme/legacy.git", branch: "master"}
	f := w.runner()
	u := &script{interactive: true}
	out, err := doRun(t, Options{Path: dir}, u, f)
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if f.Ran("gh") || u.askedAbout("GitHub") {
		t.Errorf("went near GitHub: asked %q, ran %q", u.asked, f.Calls)
	}
	if exists(dir, "src/main.go") {
		t.Error("wrote a main though the project has code")
	}
	if got := read(t, dir, "Makefile"); !strings.Contains(got, "PKG    ?= ./cmd/legacy") || !strings.Contains(got, "MODULE  := example.com/legacy") {
		t.Errorf("the Makefile doesn't build the main package it found, or doesn't use the module path:\n%s", got)
	}
	if got := read(t, dir, ".github/workflows/build.yml"); !strings.Contains(got, "refs/heads/master'") {
		t.Error("the workflow doesn't use the repo's branch")
	}
	if !strings.Contains(read(t, dir, "install.sh"), "github.com/acme/legacy/releases") {
		t.Error("install.sh doesn't use the origin's repo")
	}
	if !exists(dir, "internal/appcli/upgrade.go") {
		t.Error("no appcli")
	}
	if !strings.Contains(out, "appcli.Handle(os.Args[1:])") || !strings.Contains(out, "example.com/legacy/internal/appcli") {
		t.Errorf("didn't tell the user how to use the module:\n%s", out)
	}
	put(t, dir, "cmd/legacy/main.go", "package main\n\nimport \"example.com/legacy/internal/appcli\"\n\nfunc main() { appcli.Handle(nil) }\n")
	out, err = doRun(t, Options{Path: dir}, ui.Defaults{}, f)
	if err != nil || strings.Contains(out, "NOTE: this project already has Go code") {
		t.Errorf("explained how to use appcli to a project that already does (err %v):\n%s", err, out)
	}
	if got := read(t, dir, "go.mod"); !strings.HasPrefix(got, "module example.com/legacy") {
		t.Errorf("go.mod was rewritten: %q", got)
	}
	if f.Ran("go mod init") || f.Ran("go mod tidy") || !f.Ran("go get golang.org/x/sys") {
		t.Errorf("should only add x/sys to an existing go.mod: %q", f.Calls)
	}
	if !strings.Contains(out, "/legacy and /dist/") {
		t.Errorf("didn't point out what the .gitignore lacks:\n%s", out)
	}
}

func TestExistingProjectKeepsItsReadmeAndGitignoreUnlessAsked(t *testing.T) {
	dir := existingProject(t)
	os.Mkdir(filepath.Join(dir, ".git"), 0o755)
	f := (&world{origin: "https://github.com/acme/legacy", branch: "main"}).runner()
	if out, err := doRun(t, Options{Path: dir}, ui.Defaults{}, f); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if got := read(t, dir, "README.md"); got != "legacy\n===\n\nmy own words\n" {
		t.Errorf("README.md was changed:\n%s", got)
	}
	if got := read(t, dir, ".gitignore"); got != "*.log\n" {
		t.Errorf(".gitignore was changed:\n%s", got)
	}

	u := &script{interactive: true, confirms: []bool{true}}
	if out, err := doRun(t, Options{Path: dir}, u, f); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	got := read(t, dir, "README.md")
	if !strings.HasPrefix(got, "legacy\n===\n\nmy own words\n") || !strings.Contains(got, "## Install") || !strings.Contains(got, "acme/legacy/main/install.sh") {
		t.Errorf("README.md after agreeing to the install section:\n%s", got)
	}
	if out, err := doRun(t, Options{Path: dir}, &script{interactive: true, confirms: []bool{true}}, f); err != nil || strings.Contains(out, "install instructions to the end") {
		t.Errorf("offered the install section again: %v\n%s", err, out)
	}
	if strings.Count(read(t, dir, "README.md"), "## Install") != 1 {
		t.Error("the install section was added twice")
	}
}

func TestExistingProjectAsksBeforeReplacingItsFiles(t *testing.T) {
	dir := existingProject(t)
	put(t, dir, "Makefile", "all:\n\techo mine\n")
	os.Mkdir(filepath.Join(dir, ".git"), 0o755)
	f := (&world{origin: "https://github.com/acme/legacy", branch: "main"}).runner()
	u := &script{interactive: true, confirms: []bool{false}}
	if out, err := doRun(t, Options{Path: dir}, u, f); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !u.askedAbout("Makefile already exists - replace it?") {
		t.Errorf("asked %q", u.asked)
	}
	if got := read(t, dir, "Makefile"); got != "all:\n\techo mine\n" {
		t.Errorf("Makefile was replaced:\n%s", got)
	}
}

func TestExistingGitProjectWithNoRemoteIsOfferedARepository(t *testing.T) {
	dir := existingProject(t)
	os.Mkdir(filepath.Join(dir, ".git"), 0o755)

	// Declined: no repository, and the user is asked for the one to link to.
	f := (&world{branch: "main"}).runner()
	u := &script{interactive: true, confirms: []bool{false}, inputs: []string{"acme/legacy"}}
	if out, err := doRun(t, Options{Path: dir}, u, f); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if f.Ran("gh repo create") {
		t.Errorf("created a repository after being told no: %q", f.Calls)
	}
	if !u.askedAbout("has no GitHub remote - create a GitHub repository") {
		t.Errorf("asked %q", u.asked)
	}
	if !strings.Contains(read(t, dir, "install.sh"), "github.com/acme/legacy/releases") {
		t.Error("the typed repository isn't used")
	}

	// Accepted: it's created from the folder, as origin.
	dir = existingProject(t)
	os.Mkdir(filepath.Join(dir, ".git"), 0o755)
	f = (&world{user: "bob", branch: "main"}).runner()
	u = &script{interactive: true, confirms: []bool{true}, selects: []int{0, 1}}
	if out, err := doRun(t, Options{Path: dir, Name: "legacy"}, u, f); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !f.Ran("gh repo create bob/legacy --private --source . --remote origin") {
		t.Errorf("ran %q", f.Calls)
	}
	if f.Ran("git init") {
		t.Error("re-initialised an existing repository")
	}
	if !strings.Contains(read(t, dir, "install.sh"), "github.com/bob/legacy/releases") {
		t.Error("the new repository isn't used in install.sh")
	}
}

func TestExistingProjectNotUnderGitCanBeMadeARepository(t *testing.T) {
	dir := existingProject(t)
	f := (&world{user: "bob", branch: "main"}).runner()
	out, err := doRun(t, Options{Path: dir, Name: "legacy", CreateRepo: true, Org: "acme", Visibility: "public"}, ui.Defaults{}, f)
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !f.Ran("git init") || !f.Ran("gh repo create acme/legacy --public --source . --remote origin") {
		t.Errorf("ran %q", f.Calls)
	}
}

func TestExistingProjectIsLinkedToARepositoryThatIsAlreadyOnGitHub(t *testing.T) {
	for _, c := range []struct{ protocol, url string }{
		{"", "https://github.com/acme/legacy.git"},
		{"https", "https://github.com/acme/legacy.git"},
		{"ssh", "git@github.com:acme/legacy.git"},
	} {
		dir := existingProject(t)
		w := &world{branch: "main", existing: []string{"acme/legacy"}, protocol: c.protocol}
		f := w.runner()
		out, err := doRun(t, Options{Path: dir, Name: "legacy", CreateRepo: true, Org: "acme"}, ui.Defaults{}, f)
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		if f.Ran("gh repo create") {
			t.Errorf("tried to create a repository that's already there: %q", f.Calls)
		}
		if !f.Ran("git init") || !f.Ran("git remote add origin "+c.url) {
			t.Errorf("with git_protocol %q, ran %q", c.protocol, f.Calls)
		}
		if !strings.Contains(read(t, dir, "install.sh"), "github.com/acme/legacy/releases") {
			t.Error("the existing repository isn't used in install.sh")
		}
	}
}

func TestExistingProjectWithNoOneToAskGetsNoRepositoryAndAPlaceholder(t *testing.T) {
	dir := existingProject(t)
	f := (&world{}).runner()
	out, err := doRun(t, Options{Path: dir}, ui.Defaults{}, f)
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if f.Ran("gh") || f.Ran("git init") {
		t.Errorf("did things nobody asked for: %q", f.Calls)
	}
	if !strings.Contains(out, "OWNER/REPO") {
		t.Errorf("no warning about the placeholder:\n%s", out)
	}
}

func TestExistingProjectRepoFlagStandsInForARemote(t *testing.T) {
	dir := existingProject(t)
	if out, err := doRun(t, Options{Path: dir, Repo: "acme/legacy"}, ui.Defaults{}, (&world{}).runner()); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !strings.Contains(read(t, dir, "install.sh"), "github.com/acme/legacy/releases") {
		t.Error("--repo isn't used")
	}
}

func TestExistingProjectWithoutGoCodeGetsAMain(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "README.md", "# docs only\n")
	if out, err := doRun(t, Options{Path: dir, Repo: "acme/x", Module: "github.com/acme/x"}, ui.Defaults{}, (&world{}).runner()); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !exists(dir, "src/main.go") {
		t.Error("no src/main.go")
	}
	if got := read(t, dir, "go.mod"); !strings.HasPrefix(got, "module github.com/acme/x") {
		t.Errorf("go.mod: %q", got)
	}
}

func TestSeveralMainPackagesAreChosenBetween(t *testing.T) {
	dir := existingProject(t)
	put(t, dir, "cmd/other/main.go", "package main\n")
	u := &script{interactive: true, selects: []int{1}}
	if out, err := doRun(t, Options{Path: dir, Repo: "acme/x"}, u, (&world{}).runner()); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !u.askedAbout("Which main package") || strings.Join(u.choices[0], "|") != "./cmd/legacy|./cmd/other" {
		t.Errorf("asked %q with %q", u.asked, u.choices)
	}
	if !strings.Contains(read(t, dir, "Makefile"), "PKG    ?= ./cmd/other") {
		t.Error("didn't use the chosen main package")
	}
}

func TestRunRefusesAFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "f")
	os.WriteFile(file, []byte("x"), 0o644)
	if _, err := doRun(t, Options{Path: file}, ui.Defaults{}, (&world{}).runner()); err == nil {
		t.Error("scaffolded into a file")
	}
}

// The generated project has to be one that really builds, vets and passes
// its own tests. golang.org/x/sys is only used by the Windows files, so none
// of this needs the network on any other platform.
func TestScaffoldedProjectBuildsAndPassesItsTests(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the go tool")
	}
	if runtime.GOOS == "windows" {
		t.Skip("the generated Windows code needs golang.org/x/sys fetched")
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go tool")
	}
	dir := filepath.Join(t.TempDir(), "my-tool")
	if out, err := doRun(t, Options{Path: dir, NoRepo: true}, ui.Defaults{}, (&world{}).runner()); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	for _, args := range [][]string{{"vet", "./..."}, {"test", "./..."}, {"build", "-buildvcs=false", "-o", "my-tool", "./src"}} {
		cmd := exec.Command(goTool, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOPROXY=off")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	out, err := exec.Command(filepath.Join(dir, "my-tool"), "--version").CombinedOutput()
	if err != nil || !strings.HasPrefix(string(out), "my-tool dev") {
		t.Errorf("--version: %q, %v", out, err)
	}
	out, err = exec.Command(filepath.Join(dir, "my-tool")).CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "scaffolded with goscaffold" {
		t.Errorf("running it: %q, %v", out, err)
	}
}
