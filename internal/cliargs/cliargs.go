// Package cliargs parses goscaffold's command line: every option has both a
// long and a short form.
package cliargs

import (
	"fmt"
	"strings"
)

// Args holds the parsed command line.
type Args struct {
	Path       string
	Name       string
	Org        string
	Visibility string // "", "public" or "private"
	Repo       string
	Module     string
	NoRepo     bool
	CreateRepo bool
	Force      bool
	Yes        bool
	Help       bool
}

// Options is the --help text for these options, in the style the generated
// appcli package's help uses for its own.
const Options = `  -n, --name <name>      the project's name (default: asked for, offering the folder's name)
  -o, --org <owner>      create the GitHub repository under this user or organisation
  -P, --public           make the new GitHub repository public (the default)
  -p, --private          make the new GitHub repository private
  -r, --repo <o/name>    the project's GitHub repository, if it isn't a git remote yet
  -m, --module <path>    the Go module path, if there is no go.mod (default: github.com/<owner>/<name>)
  -g, --no-repo          don't create a GitHub repository
  -c, --create-repo      for an existing project with no GitHub remote, create one without asking
  -f, --force            replace files that already exist without asking
  -y, --yes              never ask: take the default answer to every question`

// Parse parses argv (excluding the program name).
func Parse(argv []string) (Args, error) {
	var a Args
	var positional []string
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		// --name=value is the same as --name value.
		var inline *string
		if strings.HasPrefix(arg, "--") {
			if k, v, ok := strings.Cut(arg, "="); ok {
				arg, inline = k, &v
			}
		}
		value := func() (string, error) {
			if inline != nil {
				return *inline, nil
			}
			if i+1 >= len(argv) {
				return "", fmt.Errorf("%s needs a value", arg)
			}
			i++
			return argv[i], nil
		}
		flag := func(set *bool) error {
			if inline != nil {
				return fmt.Errorf("%s doesn't take a value", arg)
			}
			*set = true
			return nil
		}
		var err error
		switch arg {
		case "-h", "--help":
			err = flag(&a.Help)
		case "-n", "--name":
			a.Name, err = value()
		case "-o", "--org":
			a.Org, err = value()
		case "-r", "--repo":
			a.Repo, err = value()
		case "-m", "--module":
			a.Module, err = value()
		case "-P", "--public":
			var set bool
			if err = flag(&set); err == nil {
				a.Visibility = "public"
			}
		case "-p", "--private":
			var set bool
			if err = flag(&set); err == nil {
				a.Visibility = "private"
			}
		case "-g", "--no-repo":
			err = flag(&a.NoRepo)
		case "-c", "--create-repo":
			err = flag(&a.CreateRepo)
		case "-f", "--force":
			err = flag(&a.Force)
		case "-y", "--yes":
			err = flag(&a.Yes)
		default:
			if strings.HasPrefix(arg, "-") && arg != "-" {
				return a, fmt.Errorf("unknown option %s", arg)
			}
			positional = append(positional, arg)
		}
		if err != nil {
			return a, err
		}
	}
	if a.Help {
		return a, nil
	}
	switch len(positional) {
	case 0:
		return a, fmt.Errorf("which folder? usage: goscaffold <path to folder>")
	case 1:
		a.Path = positional[0]
	default:
		return a, fmt.Errorf("one folder at a time (got %s)", strings.Join(positional, ", "))
	}
	return a, nil
}
