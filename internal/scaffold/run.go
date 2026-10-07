package scaffold

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"goscaffold/internal/gh"
	"goscaffold/internal/gitx"
	"goscaffold/internal/sh"
	"goscaffold/internal/ui"
)

// Options is what the command line asked for. Whatever is left empty is
// asked about, or defaulted when there is no one to ask.
type Options struct {
	Path       string // the project folder, relative or absolute
	Name       string // the project's name; defaults to the folder's
	Org        string // who owns the new GitHub repository: the user or an organisation
	Visibility string // "public" or "private" for the new GitHub repository
	Repo       string // owner/name of the project's existing GitHub repository
	Module     string // the Go module path, for a project with no go.mod yet
	NoRepo     bool   // never create a GitHub repository
	CreateRepo bool   // create one for an existing project without asking
	Force      bool   // replace existing files without asking
}

// Env is what Run works with, kept apart from Options so tests can fake it.
type Env struct {
	UI     ui.UI
	Sh     sh.Runner
	Assets fs.FS
	Out    io.Writer
}

// Run scaffolds the project at o.Path: a brand new one, with its own GitHub
// repository, when the folder is missing or empty, and otherwise an existing
// one.
func Run(o Options, e Env) error {
	dir, err := filepath.Abs(o.Path)
	if err != nil {
		return err
	}
	info, err := os.Stat(dir)
	switch {
	case err == nil && !info.IsDir():
		return fmt.Errorf("%s exists and isn't a folder", dir)
	case err != nil && !os.IsNotExist(err):
		return err
	}
	fresh := err != nil || IsEmptyDir(dir)

	r := &run{o: o, e: e, dir: dir, fresh: fresh}
	if fresh {
		err = r.newProject()
	} else {
		err = r.existingProject()
	}
	if err != nil {
		return err
	}
	return r.scaffold()
}

type run struct {
	o     Options
	e     Env
	dir   string
	fresh bool

	name   string // the project's name
	repo   string // owner/name on GitHub, if known
	hasGit bool

	createdModule bool // go.mod was made by this run
}

func (r *run) say(format string, args ...any) {
	fmt.Fprintf(r.e.Out, format+"\n", args...)
}

func (r *run) warn(format string, args ...any) {
	fmt.Fprintf(r.e.Out, "WARNING: "+format+"\n", args...)
}

// projectName is asked for when there's someone to ask, and otherwise comes
// from the folder.
func (r *run) projectName() (string, error) {
	if r.o.Name != "" {
		return r.o.Name, ValidateApp(r.o.Name)
	}
	def := filepath.Base(r.dir)
	name, err := r.e.UI.Input("Project name?", def)
	if err != nil {
		return "", err
	}
	return name, ValidateApp(name)
}

// repoDetails asks which account should own a new GitHub repository and
// whether it should be private, unless the command line already said.
func (r *run) repoDetails() (owner string, private bool, err error) {
	if !r.e.Sh.Has("gh") {
		return "", false, errors.New("the GitHub CLI (gh) isn't installed, or isn't on your PATH: see https://cli.github.com, or use --no-repo to skip creating a GitHub repository")
	}
	owner = r.o.Org
	if owner == "" {
		user, err := gh.CurrentUser(r.e.Sh)
		if err != nil {
			return "", false, err
		}
		orgs, err := gh.Orgs(r.e.Sh)
		if err != nil {
			return "", false, err
		}
		choices := append([]string{user}, orgs...)
		i, err := r.e.UI.Select("Where should I create this repository?", choices, 0)
		if err != nil {
			return "", false, err
		}
		owner = choices[i]
	}
	if err := ValidateOwner(owner); err != nil {
		return "", false, err
	}
	switch strings.ToLower(r.o.Visibility) {
	case "public":
		private = false
	case "private":
		private = true
	default:
		i, err := r.e.UI.Select("Should the repository be public or private?", []string{"public", "private"}, 0)
		if err != nil {
			return "", false, err
		}
		private = i == 1
	}
	return owner, private, nil
}

// newProject is Scenario 1: the folder is missing or empty, so there is a
// GitHub repository to make and check out into it.
func (r *run) newProject() error {
	name, err := r.projectName()
	if err != nil {
		return err
	}
	r.name = name
	if r.o.NoRepo {
		if err := os.MkdirAll(r.dir, 0o755); err != nil {
			return err
		}
		if r.e.Sh.Has("git") {
			if err := gitx.Init(r.e.Sh, r.dir); err != nil {
				return err
			}
			r.hasGit = true
		}
		return nil
	}
	owner, private, err := r.repoDetails()
	if err != nil {
		return err
	}
	slug := owner + "/" + name
	r.say("creating %s on GitHub", slug)
	if err := gh.CreateRepo(r.e.Sh, slug, private); err != nil {
		return fmt.Errorf("creating %s: %w", slug, err)
	}
	r.say("checking it out into %s", r.dir)
	if err := gh.Clone(r.e.Sh, slug, r.dir); err != nil {
		return fmt.Errorf("checking %s out (it has been created): %w", slug, err)
	}
	r.repo = slug
	r.hasGit = true
	return nil
}

// existingProject is Scenario 2: find out what the project already has, and
// offer to give it a GitHub repository if it's missing one. Its name, unless
// --name says, is its GitHub repository's, since that is what releases and
// the install scripts are named for; failing that, its folder's.
func (r *run) existingProject() error {
	r.hasGit = gitx.IsRepo(r.dir)
	r.repo = r.o.Repo
	if r.repo == "" && r.hasGit {
		if origin := gitx.Remote(r.e.Sh, r.dir, "origin"); origin != "" {
			if owner, repo, ok := gitx.GitHubRepo(origin); ok {
				r.repo = owner + "/" + repo
			} else {
				r.warn("origin (%s) isn't a GitHub repository, so release links will need filling in", origin)
				return r.nameExisting()
			}
		}
	}
	if r.repo != "" {
		return r.nameExisting()
	}

	if !r.o.NoRepo {
		create := r.o.CreateRepo
		if !create {
			why := "isn't under git"
			if r.hasGit {
				why = "has no GitHub remote"
			}
			var err error
			if create, err = r.e.UI.Confirm(fmt.Sprintf("%s %s - create a GitHub repository for it?", filepath.Base(r.dir), why), false); err != nil {
				return err
			}
		}
		if create {
			return r.createRepoForExisting()
		}
	}
	if r.e.UI.Interactive() {
		answer, err := r.e.UI.Input("GitHub repository, for the links in the readme and release notes (owner/name, or blank to fill in later)?", "")
		if err != nil {
			return err
		}
		r.repo = strings.TrimSpace(answer)
	}
	return r.nameExisting()
}

// nameExisting settles the name of a project that isn't getting a new
// repository.
func (r *run) nameExisting() error {
	r.name = r.o.Name
	if r.name == "" && r.repo != "" {
		_, r.name, _ = strings.Cut(r.repo, "/")
	}
	if r.name == "" {
		r.name = filepath.Base(r.dir)
	}
	if err := ValidateApp(r.name); err != nil {
		return fmt.Errorf("%w (give it one with --name)", err)
	}
	return nil
}

func (r *run) createRepoForExisting() error {
	name, err := r.projectName()
	if err != nil {
		return err
	}
	r.name = name
	owner, private, err := r.repoDetails()
	if err != nil {
		return err
	}
	slug := owner + "/" + r.name
	if !r.hasGit {
		r.say("making %s a git repository", r.dir)
		if err := gitx.Init(r.e.Sh, r.dir); err != nil {
			return err
		}
		r.hasGit = true
	}
	r.say("creating %s on GitHub, as this folder's origin", slug)
	if err := gh.CreateRepoFromSource(r.e.Sh, r.dir, slug, private); err != nil {
		return fmt.Errorf("creating %s: %w", slug, err)
	}
	r.repo = slug
	return nil
}

// scaffold writes the project's files.
func (r *run) scaffold() error {
	p := Params{App: r.name, Repo: r.repo, Branch: gitx.FallbackBranch, Pkg: "./src"}
	if p.Repo == "" {
		p.Repo = RepoPlaceholder
	}
	if r.hasGit {
		p.Branch = gitx.DefaultBranch(r.e.Sh, r.dir)
	}

	goSources := HasGoSources(r.dir)
	plan := Plan{Main: !goSources, Readme: true, Gitignore: true}
	if !r.fresh {
		_, err := os.Stat(filepath.Join(r.dir, "README.md"))
		plan.Readme = err != nil
		_, err = os.Stat(filepath.Join(r.dir, ".gitignore"))
		plan.Gitignore = err != nil
	}
	if goSources {
		mains := MainPackages(r.dir)
		switch len(mains) {
		case 0:
			r.warn("no main package found, so make.sh, make.ps1 and the Makefile build %s: change PKG in each if your program lives elsewhere", p.Pkg)
		case 1:
			p.Pkg = mains[0]
		default:
			i, err := r.e.UI.Select("Which main package is the program?", mains, 0)
			if err != nil {
				return err
			}
			p.Pkg = mains[i]
			if !r.e.UI.Interactive() {
				r.warn("%d main packages found (%s): using %s. Change PKG in make.sh, make.ps1 and the Makefile to use another", len(mains), strings.Join(mains, ", "), p.Pkg)
			}
		}
	}

	if err := r.ensureModule(&p); err != nil {
		return err
	}

	files, err := Files(r.e.Assets, p, plan)
	if err != nil {
		return err
	}
	r.say("writing files into %s", r.dir)
	outcomes, err := Write(r.dir, files, r.e.UI, r.o.Force, r.e.Out)
	if err != nil {
		return err
	}
	if outcomes["internal/appcli/config.go"] == Kept || outcomes["internal/appcli/handle.go"] == Kept {
		r.warn("you kept your own internal/appcli: the rest of it, and the build scripts, expect the version of it goscaffold writes")
	}

	r.fetchDependencies()
	if !plan.Readme && !r.fresh {
		if err := r.offerReadmeInstallSection(p); err != nil {
			return err
		}
	}
	if !plan.Gitignore {
		if b, err := os.ReadFile(filepath.Join(r.dir, ".gitignore")); err == nil {
			if missing := MissingIgnores(string(b), p.App); len(missing) > 0 {
				r.warn("your .gitignore doesn't cover what the build leaves behind: add %s", strings.Join(missing, " and "))
			}
		}
	}
	if p.Repo == RepoPlaceholder {
		r.warn("no GitHub repository is known, so links say %s: replace it in README.md, install.sh, install.ps1, .github/workflows/build.yml and internal/appcli/config.go once there is one", RepoPlaceholder)
	}
	if goSources && !UsesAppcli(r.dir) {
		r.say("")
		r.say("NOTE: this project already has Go code, so main was left alone. To get --help, --version,")
		r.say("--install and --upgrade, call the generated package first thing in your main():")
		r.say("")
		r.say("    import \"%s/internal/appcli\"", p.Module)
		r.say("")
		r.say("    if handled, exitCode := appcli.Handle(os.Args[1:]); handled {")
		r.say("        os.Exit(exitCode)")
		r.say("    }")
		r.say("")
		r.say("Document your own options in appcli.ExtraHelp so --help shows them too.")
	}
	r.summary(p)
	return nil
}

// ensureModule makes sure there is a go.mod, and records its path in p.
func (r *run) ensureModule(p *Params) error {
	if mod := ModulePath(r.dir); mod != "" {
		p.Module = mod
		return nil
	}
	mod := r.o.Module
	switch {
	case mod != "":
	case r.repo != "":
		mod = "github.com/" + r.repo
	default:
		mod = r.name
	}
	p.Module = mod
	if !r.e.Sh.Has("go") {
		r.warn("go isn't on your PATH, so no go.mod was made: run `go mod init %s` and `go mod tidy`", mod)
		return nil
	}
	r.say("go mod init %s", mod)
	if _, err := r.e.Sh.Exec(r.dir, "go", "mod", "init", mod); err != nil {
		return err
	}
	r.createdModule = true
	return nil
}

// fetchDependencies records the one thing the generated code needs from
// outside the standard library (golang.org/x/sys, for adding to the user's
// PATH on Windows) in go.mod and go.sum. A module goscaffold has just made is
// tidied; one that was already there is only added to, since tidying it could
// change what the user didn't ask to have changed.
func (r *run) fetchDependencies() {
	if !r.e.Sh.Has("go") || ModulePath(r.dir) == "" {
		return
	}
	args := []string{"get", "golang.org/x/sys"}
	if r.createdModule {
		args = []string{"mod", "tidy"}
	}
	r.say("go %s", strings.Join(args, " "))
	if _, err := r.e.Sh.Exec(r.dir, "go", args...); err != nil {
		r.warn("couldn't fetch the dependencies (is there network access?): run `go %s` yourself.\n%v", strings.Join(args, " "), err)
	}
}

func (r *run) offerReadmeInstallSection(p Params) error {
	path := filepath.Join(r.dir, "README.md")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	if strings.Contains(string(b), "install.sh") {
		return nil
	}
	add, err := r.e.UI.Confirm("README.md already exists - add install instructions to the end of it?", false)
	if err != nil || !add {
		return err
	}
	section, err := InstallSection(r.e.Assets, p)
	if err != nil {
		return err
	}
	text := strings.TrimRight(string(b), "\n") + "\n\n" + section
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return err
	}
	r.say("  added install instructions to README.md")
	return nil
}

func (r *run) summary(p Params) {
	r.say("")
	r.say("%s is scaffolded in %s. Nothing has been committed: have a look, then commit and push when you're ready.", p.App, r.dir)
	r.say("Build it with `make`, `./make.sh` or `./make.ps1`; try `%s --help`.", "./"+p.App)
}
