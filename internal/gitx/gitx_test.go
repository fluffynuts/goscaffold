package gitx

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"goscaffold/internal/sh"
)

func TestGitHubRepo(t *testing.T) {
	good := map[string][2]string{
		"https://github.com/fluffynuts/vibe":         {"fluffynuts", "vibe"},
		"https://github.com/fluffynuts/vibe.git":     {"fluffynuts", "vibe"},
		"https://github.com/fluffynuts/vibe/":        {"fluffynuts", "vibe"},
		"git@github.com:fluffynuts/vibe.git":         {"fluffynuts", "vibe"},
		"git@github.com:fluffynuts/vibe":             {"fluffynuts", "vibe"},
		"ssh://git@github.com/fluffynuts/vibe.git":   {"fluffynuts", "vibe"},
		"https://user:token@github.com/some-org/a.b": {"some-org", "a.b"},
		"https://GitHub.com/Org/Repo.git":            {"Org", "Repo"},
	}
	for remote, want := range good {
		owner, repo, ok := GitHubRepo(remote)
		if !ok || owner != want[0] || repo != want[1] {
			t.Errorf("GitHubRepo(%q) = %q, %q, %v; want %v", remote, owner, repo, ok, want)
		}
	}
	for _, remote := range []string{"", "https://gitlab.com/a/b", "git@gitlab.com:a/b.git", "https://github.com/onlyowner", "https://github.com/a/b/c", "/some/local/path"} {
		if _, _, ok := GitHubRepo(remote); ok {
			t.Errorf("GitHubRepo(%q) was accepted", remote)
		}
	}
}

func answering(answers map[string]string) *sh.Fake {
	return &sh.Fake{Handler: func(dir, name string, args []string) (string, error) {
		key := name
		for _, a := range args {
			key += " " + a
		}
		if out, ok := answers[key]; ok {
			return out + "\n", nil
		}
		return "", errors.New("no such thing")
	}}
}

func TestDefaultBranchPrefersTheRemotesDefault(t *testing.T) {
	f := answering(map[string]string{
		"git symbolic-ref --short refs/remotes/origin/HEAD": "origin/trunk",
		"git symbolic-ref --short HEAD":                     "some-feature",
		"git config --get init.defaultBranch":               "main",
	})
	if got := DefaultBranch(f, "."); got != "trunk" {
		t.Errorf("got %q, want trunk", got)
	}
}

func TestDefaultBranchFallsBackToTheCheckedOutBranch(t *testing.T) {
	f := answering(map[string]string{
		"git symbolic-ref --short HEAD":       "develop",
		"git config --get init.defaultBranch": "main",
	})
	if got := DefaultBranch(f, "."); got != "develop" {
		t.Errorf("got %q, want develop", got)
	}
}

func TestDefaultBranchFallsBackToTheUsersPreferenceThenMain(t *testing.T) {
	if got := DefaultBranch(answering(map[string]string{"git config --get init.defaultBranch": "master"}), "."); got != "master" {
		t.Errorf("got %q, want master", got)
	}
	if got := DefaultBranch(answering(nil), "."); got != "main" {
		t.Errorf("got %q, want main", got)
	}
}

func TestRemote(t *testing.T) {
	f := answering(map[string]string{"git remote get-url origin": "git@github.com:a/b.git"})
	if got := Remote(f, ".", "origin"); got != "git@github.com:a/b.git" {
		t.Errorf("got %q", got)
	}
	if got := Remote(f, ".", "upstream"); got != "" {
		t.Errorf("a missing remote gave %q", got)
	}
}

func TestIsRepoOnlyForTheRootOfAWorkingTree(t *testing.T) {
	dir := t.TempDir()
	if IsRepo(dir) {
		t.Error("an empty folder is a repo")
	}
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !IsRepo(dir) {
		t.Error("a folder with .git isn't a repo")
	}
	sub := filepath.Join(dir, "sub")
	os.Mkdir(sub, 0o755)
	if IsRepo(sub) {
		t.Error("a folder inside a repo, but not its root, counts as a repo")
	}
}
