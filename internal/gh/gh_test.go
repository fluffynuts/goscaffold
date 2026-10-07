package gh

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"goscaffold/internal/sh"
)

func TestCurrentUser(t *testing.T) {
	f := &sh.Fake{Handler: func(dir, name string, args []string) (string, error) { return "bobsaget\n", nil }}
	user, err := CurrentUser(f)
	if err != nil || user != "bobsaget" {
		t.Fatalf("got %q, %v", user, err)
	}
	if !f.Ran("gh api user --jq .login") {
		t.Errorf("asked gh with %v", f.Calls)
	}
}

func TestCurrentUserWhenNotLoggedIn(t *testing.T) {
	f := &sh.Fake{Handler: func(dir, name string, args []string) (string, error) { return "", errors.New("exit 4") }}
	if _, err := CurrentUser(f); err == nil || !strings.Contains(err.Error(), "gh auth login") {
		t.Errorf("got %v, want a hint about gh auth login", err)
	}
}

func TestOrgs(t *testing.T) {
	f := &sh.Fake{Handler: func(dir, name string, args []string) (string, error) {
		return "Organisation A\n\n  Organisation B  \n", nil
	}}
	orgs, err := Orgs(f)
	if err != nil || !reflect.DeepEqual(orgs, []string{"Organisation A", "Organisation B"}) {
		t.Fatalf("got %q, %v", orgs, err)
	}
}

func TestRepoCommands(t *testing.T) {
	f := &sh.Fake{}
	CreateRepo(f, "o/r", false)
	CreateRepo(f, "o/p", true)
	CreateRepoFromSource(f, "/some/dir", "o/s", true)
	Clone(f, "o/r", "/some/dir")
	want := []string{
		"gh repo create o/r --public",
		"gh repo create o/p --private",
		"gh repo create o/s --private --source . --remote origin",
		"gh repo clone o/r /some/dir",
	}
	if !reflect.DeepEqual(f.Calls, want) {
		t.Errorf("ran %q, want %q", f.Calls, want)
	}
}
