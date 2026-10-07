// Package scaffold works out what a project needs to be released as a
// command line tool, and writes it.
package scaffold

import (
	"bytes"
	"fmt"
	"io/fs"
	"regexp"
	"strings"
	"unicode"
)

// RepoPlaceholder stands in for the GitHub repository in links when it isn't
// known, so the generated files say plainly what is left to fill in.
const RepoPlaceholder = "OWNER/REPO"

// Params is what goes into the templates.
type Params struct {
	App    string // the program's name: binary, zips, messages
	Module string // the Go module path
	Repo   string // owner/name on GitHub, or RepoPlaceholder
	Branch string // the branch releases are published from
	Pkg    string // the folder holding the main package, as ./src
}

var validApp = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// ValidateApp checks a program name is one that is safe in file names, shell
// scripts, makefiles and PowerShell, which every template uses it in.
func ValidateApp(name string) error {
	if !validApp.MatchString(name) {
		return fmt.Errorf("%q isn't a usable project name: use letters, digits, '.', '_' and '-', starting with a letter or digit", name)
	}
	return nil
}

var validOwner = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*$`)

// ValidateOwner checks a name is one a GitHub user or organisation can have:
// letters, digits and hyphens. It is checked before anything is created, so a
// typo in --org is an error rather than a half-made project.
func ValidateOwner(name string) error {
	if !validOwner.MatchString(name) {
		return fmt.Errorf("%q isn't a GitHub user or organisation name: use letters, digits and '-'", name)
	}
	return nil
}

// envName is app as an environment variable prefix: my-tool becomes MY_TOOL.
func envName(app string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(app) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

// psName is app as a PowerShell function name suffix: my-tool becomes MyTool.
func psName(app string) string {
	var b strings.Builder
	for _, part := range strings.FieldsFunc(app, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		b.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return b.String()
}

// lf makes line endings LF. A checkout on Windows may have turned the
// embedded templates into CRLF, which would break the shell scripts and
// make every generated file differ from what a Linux checkout writes.
func lf(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
}

// Render reads a template from assets and fills in p. Templates mark their
// holes with @@NAME@@ rather than {{ }} or ${ }, since every file type
// involved (GitHub workflows, PowerShell, shell, make) has a use for those.
func Render(assets fs.FS, name string, p Params) ([]byte, error) {
	raw, err := fs.ReadFile(assets, name)
	if err != nil {
		return nil, err
	}
	raw = lf(raw)
	repl := func(extra ...string) *strings.Replacer {
		return strings.NewReplacer(append([]string{
			"@@APP@@", p.App,
			"@@MODULE@@", p.Module,
			"@@REPO@@", p.Repo,
			"@@BRANCH@@", p.Branch,
			"@@PKG@@", p.Pkg,
			"@@APPENV@@", envName(p.App),
			"@@APPPS@@", psName(p.App),
		}, extra...)...)
	}
	if strings.Contains(string(raw), "@@INSTALL@@") {
		section, err := fs.ReadFile(assets, "templates/install-section.md.tmpl")
		if err != nil {
			return nil, err
		}
		section = lf(section)
		return []byte(repl("@@INSTALL@@", strings.TrimRight(repl().Replace(string(section)), "\n")).Replace(string(raw))), nil
	}
	return []byte(repl().Replace(string(raw))), nil
}

// InstallSection is the "Install" part of a readme, for projects whose
// readme already exists.
func InstallSection(assets fs.FS, p Params) (string, error) {
	b, err := Render(assets, "templates/install-section.md.tmpl", p)
	return string(b), err
}
