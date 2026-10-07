// Package gitx answers the few questions goscaffold has about a folder's git
// repository.
package gitx

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"goscaffold/internal/sh"
)

// FallbackBranch is the branch name used when nothing says otherwise.
const FallbackBranch = "main"

// IsRepo reports whether dir is itself the root of a git working tree (not
// merely somewhere inside one, whose root is higher up).
func IsRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// Init makes dir a git repository.
func Init(r sh.Runner, dir string) error {
	_, err := r.Exec(dir, "git", "init")
	return err
}

// Remote is the URL of the named remote, or "" when there isn't one.
func Remote(r sh.Runner, dir, name string) string {
	out, err := r.Exec(dir, "git", "remote", "get-url", name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// DefaultBranch is the branch a repository's releases should come from: what
// the remote calls its default, else the branch checked out, else the
// user's init.defaultBranch, else "main". For a repository just cloned from
// GitHub, empty or not, the checked-out branch is already GitHub's default
// for it, which is how a new repository gets the user's own preference.
func DefaultBranch(r sh.Runner, dir string) string {
	if out, err := r.Exec(dir, "git", "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		if b := strings.TrimPrefix(strings.TrimSpace(out), "origin/"); b != "" {
			return b
		}
	}
	if out, err := r.Exec(dir, "git", "symbolic-ref", "--short", "HEAD"); err == nil {
		if b := strings.TrimSpace(out); b != "" {
			return b
		}
	}
	if out, err := r.Exec(dir, "git", "config", "--get", "init.defaultBranch"); err == nil {
		if b := strings.TrimSpace(out); b != "" {
			return b
		}
	}
	return FallbackBranch
}

var scpLike = regexp.MustCompile(`^(?:[^@/]+@)?github\.com:([^/]+)/([^/]+?)(?:\.git)?/?$`)

// GitHubRepo reads the owner and repository name from a GitHub remote URL,
// in any of its spellings: https://github.com/o/r(.git), git@github.com:o/r.git,
// ssh://git@github.com/o/r.git. ok is false for anything else.
func GitHubRepo(remote string) (owner, repo string, ok bool) {
	remote = strings.TrimSpace(remote)
	if m := scpLike.FindStringSubmatch(remote); m != nil {
		return m[1], m[2], true
	}
	u, err := url.Parse(remote)
	if err != nil || !strings.EqualFold(u.Hostname(), "github.com") {
		return "", "", false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], strings.TrimSuffix(parts[1], ".git"), true
}
