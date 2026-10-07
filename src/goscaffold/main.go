// goscaffold scaffolds Go projects: a GitHub repository, build scripts, a
// release workflow, and --help/--version/--install/--upgrade support.
package main

import (
	"fmt"
	"os"

	"goscaffold"
	"goscaffold/internal/appcli"
	"goscaffold/internal/cliargs"
	"goscaffold/internal/scaffold"
	"goscaffold/internal/sh"
	"goscaffold/internal/ui"
)

func main() {
	// --help, --version, --install and --upgrade live in internal/appcli;
	// goscaffold's own options are described in its --help too.
	appcli.Usage = "goscaffold [options] <path to folder>"
	appcli.ExtraHelp = "\nScaffolding options:\n" + cliargs.Options
	appcli.Examples = []string{
		"goscaffold ~/code/my-new-tool                     create a GitHub repository and scaffold it",
		"goscaffold -o my-org -p -y ~/code/my-new-tool     the same, asking nothing: private, under my-org",
		"goscaffold -g ~/code/my-new-tool                  scaffold, but don't create a GitHub repository",
		"goscaffold .                                      scaffold the project in the current folder",
		"goscaffold --upgrade                              upgrade goscaffold itself",
	}
	if handled, exitCode := appcli.Handle(os.Args[1:]); handled {
		os.Exit(exitCode)
	}
	os.Exit(run(os.Args[1:]))
}

func run(argv []string) int {
	args, err := cliargs.Parse(argv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "goscaffold: %s (see goscaffold --help)\n", err)
		return 2
	}
	if args.Help {
		appcli.WriteHelp(os.Stdout)
		return 0
	}

	u, closeUI := ui.Auto(!args.Yes)
	defer closeUI()
	err = scaffold.Run(scaffold.Options{
		Path:       args.Path,
		Name:       args.Name,
		Org:        args.Org,
		Visibility: args.Visibility,
		Repo:       args.Repo,
		Module:     args.Module,
		NoRepo:     args.NoRepo,
		CreateRepo: args.CreateRepo,
		Force:      args.Force,
	}, scaffold.Env{UI: u, Sh: sh.OS{}, Assets: goscaffold.Assets, Out: os.Stdout})
	if err != nil {
		if err == ui.ErrCancelled {
			fmt.Fprintln(os.Stderr, "goscaffold: cancelled")
			return 130
		}
		fmt.Fprintf(os.Stderr, "goscaffold: %s\n", err)
		return 1
	}
	return 0
}
