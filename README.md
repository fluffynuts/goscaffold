goscaffold
---

Scaffolds a Go project so it can be built, tested and released as a command line tool: a GitHub
repository, build scripts, a release workflow, and `--help`, `--version`, `--install` and `--upgrade`
support to extend later.

## Usage

```sh
goscaffold <path to folder>
```

The path can be relative or absolute. What happens next depends on the folder.

**A new project** (the folder doesn't exist, or is empty): goscaffold asks for the project's name
(defaulting to the folder's), who should own the GitHub repository (you, or one of your
organisations, as `gh` knows them) and whether it's public (the default) or private. It creates the
repository with `gh`, checks it out into the folder, and writes:

- `README.md`, `.gitignore` and a `VERSION` file (the `major.minor` part of the version, bumped by hand)
- `src/main.go`, which prints "scaffolded with goscaffold"
- `internal/appcli`, the command line support, as ordinary source that is yours to edit:
  - `--help` shows every option, with examples
  - `--version` prints `my-tool 0.1.57 (5da629aa28c2, built 2026-10-07T06:38:08Z)`
  - `--install` copies the binary to `~/.local/bin` and checks that folder is on your `PATH`: on Windows
    it adds it to your user `PATH` (open a new terminal to pick it up); elsewhere it only warns
  - `--upgrade` downloads the latest release for this machine, checks it against the release's
    `SHA256SUMS`, unpacks it in a temporary folder and runs its `--install`
- `Makefile`, `make.sh` and `make.ps1`: build, test, vet, check, clean and dist. None of them lists
  source files, so they keep working as the project grows and files move
- `install.sh` and `install.ps1`, for installing from a release without cloning anything
- `.github/workflows/build.yml`, which vets and tests on Linux, macOS and Windows, packages a zip per
  platform and architecture, and publishes a release for every push to the repository's default branch

Nothing is committed or pushed: that's yours to decide.

**An existing project:** the same, minus what the project has already:

- a project that is already under git with a GitHub remote isn't given another repository. One
  without a remote (or not under git at all) is offered one
- a project with Go sources keeps its own `main`. `internal/appcli` is still written, and goscaffold
  prints the lines to add to your `main()` to use it
- the main package is found for the build scripts (you're asked which, when there are several)
- an existing `README.md` or `.gitignore` is left alone (goscaffold offers to add the install
  instructions to the readme, and says what the `.gitignore` is missing)

Whenever a file it would write already exists, it asks `<file> already exists - replace it? [y/N]`.
The default is no, so pressing enter leaves your files alone.

### Options

```text
  -n, --name <name>      the project's name (default: asked for, offering the folder's name)
  -o, --org <owner>      create the GitHub repository under this user or organisation
  -P, --public           make the new GitHub repository public (the default)
  -p, --private          make the new GitHub repository private
  -r, --repo <o/name>    the project's GitHub repository, if it isn't a git remote yet
  -m, --module <path>    the Go module path, if there is no go.mod (default: github.com/<owner>/<name>)
  -g, --no-repo          don't create a GitHub repository
  -c, --create-repo      for an existing project with no GitHub remote, create one without asking
  -f, --force            replace files that already exist without asking
  -y, --yes              never ask: take the default answer to every question
```

Without a terminal to ask on (or with `--yes`) every question takes its default: your own account,
public, the folder's name, and "no" to replacing a file. So
`goscaffold -y -o my-org -p ~/code/my-tool` needs no one at the keyboard.

The branch releases are published from is the one the repository already uses: the remote's default
branch for an existing project (else the one checked out), and for a new one the branch the fresh
clone of it lands on. When neither says, it falls back to your `init.defaultBranch`, then `main`.

### Releases

Every release has a zip per platform (`my-tool-linux-amd64.zip`, `my-tool-macos-arm64.zip`,
`my-tool-windows-amd64.zip` and so on, each for x86-64 and ARM) holding the binary and `README.md`,
plus a `SHA256SUMS`. To ship other files or folders in the zips, list them in `BUNDLE` in `make.sh`
and `make.ps1`; the workflow needs no change.

## Developing goscaffold

`make check` vets and tests. The files it writes are the templates under `templates/`, embedded in
the binary, with `@@APP@@`, `@@MODULE@@`, `@@REPO@@`, `@@BRANCH@@` and `@@PKG@@` where a project's own
values go. goscaffold is scaffolded by itself.


## Install

**Linux and macOS:**

```sh
curl -fsSL https://raw.githubusercontent.com/fluffynuts/goscaffold/master/install.sh | sh
```

**Windows** (PowerShell):

```powershell
irm https://raw.githubusercontent.com/fluffynuts/goscaffold/master/install.ps1 | iex
```

Either one downloads the latest release for your machine, checks it against the release's
checksums, and runs `goscaffold --install` from it. That puts the `goscaffold` binary in `~/.local/bin`. On
Windows it adds `~\.local\bin` to your user `PATH` if it isn't there already (open a new terminal to
pick it up); on Linux and macOS, where that depends on your shell's profile, it tells you what to add
if the folder isn't on your `PATH`. `install.sh` needs `curl` or `wget`, and `unzip` (or `python3`).

Run the same command again to upgrade, or, once goscaffold is installed:

```sh
goscaffold --upgrade
```

You can also download the zip for your platform from the
[latest release](https://github.com/fluffynuts/goscaffold/releases/latest) (`goscaffold-linux-amd64.zip`,
`goscaffold-macos-arm64.zip`, `goscaffold-windows-amd64.zip` and so on, each for x86-64 and ARM, plus a
`SHA256SUMS`), unzip it anywhere, and run `goscaffold --install` from the folder it makes.
