// Package gh is the little goscaffold needs from the GitHub CLI: who the
// user is, which organisations they belong to, and making a repository.
package gh

import (
	"errors"
	"fmt"
	"strings"

	"goscaffold/internal/sh"
)

// CurrentUser is the login of the account gh is authenticated as.
func CurrentUser(r sh.Runner) (string, error) {
	out, err := r.Exec("", "gh", "api", "user", "--jq", ".login")
	if err != nil {
		return "", fmt.Errorf("asking gh who you are (is it logged in? try `gh auth login`): %w", err)
	}
	login := strings.TrimSpace(out)
	if login == "" {
		return "", errors.New("gh didn't say who you are (is it logged in? try `gh auth login`)")
	}
	return login, nil
}

// Orgs lists the organisations the user belongs to.
func Orgs(r sh.Runner) ([]string, error) {
	out, err := r.Exec("", "gh", "org", "list")
	if err != nil {
		return nil, fmt.Errorf("listing your organisations with gh: %w", err)
	}
	return sh.Lines(out), nil
}

// visibility is the gh flag for a repository's visibility.
func visibility(private bool) string {
	if private {
		return "--private"
	}
	return "--public"
}

// CreateRepo makes an empty repository at slug ("owner/name").
func CreateRepo(r sh.Runner, slug string, private bool) error {
	_, err := r.Exec("", "gh", "repo", "create", slug, visibility(private))
	return err
}

// CreateRepoFromSource makes a repository at slug from the git repository in
// dir, adding it there as the remote "origin". Nothing is pushed.
func CreateRepoFromSource(r sh.Runner, dir, slug string, private bool) error {
	_, err := r.Exec(dir, "gh", "repo", "create", slug, visibility(private), "--source", ".", "--remote", "origin")
	return err
}

// Clone checks slug out into dir, which is created if it doesn't exist and
// otherwise must be empty.
func Clone(r sh.Runner, slug, dir string) error {
	_, err := r.Exec("", "gh", "repo", "clone", slug, dir)
	return err
}
