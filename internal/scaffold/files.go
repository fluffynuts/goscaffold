package scaffold

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"goscaffold/internal/ui"
)

// File is a file to write into the project.
type File struct {
	Path    string // slash-separated, relative to the project
	Content []byte
	Mode    fs.FileMode
}

// What to write besides the files every project gets.
type Plan struct {
	// Main: write src/main.go, which a project with Go code of its own
	// doesn't want.
	Main bool
	// Readme: write README.md, with an install section.
	Readme bool
	// Gitignore: write .gitignore.
	Gitignore bool
}

// tmpl is where a template lives in the assets.
func tmpl(name string) string { return "templates/" + name + ".tmpl" }

// Files renders everything the plan calls for.
func Files(assets fs.FS, p Params, plan Plan) ([]File, error) {
	type spec struct {
		path, template string
		mode           fs.FileMode
	}
	specs := []spec{
		{"VERSION", "VERSION", 0o644},
		{"Makefile", "Makefile", 0o644},
		{"make.sh", "make.sh", 0o755},
		{"make.ps1", "make.ps1", 0o755},
		{"install.sh", "install.sh", 0o755},
		{"install.ps1", "install.ps1", 0o644},
		{".github/workflows/build.yml", ".github/workflows/build.yml", 0o644},
	}
	for _, name := range []string{"config.go", "version.go", "handle.go", "install.go", "upgrade.go", "path_other.go", "path_windows.go", "appcli_test.go"} {
		specs = append(specs, spec{"internal/appcli/" + name, "internal/appcli/" + name, 0o644})
	}
	if plan.Main {
		specs = append(specs, spec{"src/main.go", "src/main.go", 0o644})
	}
	if plan.Readme {
		specs = append(specs, spec{"README.md", "README.md", 0o644})
	}

	var files []File
	for _, s := range specs {
		content, err := Render(assets, tmpl(s.template), p)
		if err != nil {
			return nil, fmt.Errorf("rendering %s: %w", s.path, err)
		}
		files = append(files, File{Path: s.path, Content: content, Mode: s.mode})
	}
	if plan.Gitignore {
		base, err := fs.ReadFile(assets, ".gitignore")
		if err != nil {
			return nil, err
		}
		extra, err := Render(assets, tmpl("gitignore-additions"), p)
		if err != nil {
			return nil, err
		}
		content := append(bytes.TrimRight(lf(base), "\n"), '\n')
		files = append(files, File{Path: ".gitignore", Content: append(content, extra...), Mode: 0o644})
	}
	return files, nil
}

// Outcome is what happened to a file.
type Outcome int

const (
	Created Outcome = iota
	Replaced
	Kept      // already there, and the user wants it left alone
	Unchanged // already there, and already what would have been written
)

// Write puts files under dir. A file that already exists is only replaced
// when force is set or the user says yes to "<file> already exists - replace
// it?" — which defaults to no, so pressing enter (or having no one to ask)
// leaves their files alone.
func Write(dir string, files []File, u ui.UI, force bool, out io.Writer) (map[string]Outcome, error) {
	outcomes := map[string]Outcome{}
	for _, f := range files {
		target := filepath.Join(dir, filepath.FromSlash(f.Path))
		existing, err := os.ReadFile(target)
		exists := err == nil
		if err != nil && !os.IsNotExist(err) {
			return outcomes, err
		}
		switch {
		case exists && bytes.Equal(existing, f.Content):
			fmt.Fprintf(out, "  %s is already up to date\n", f.Path)
			outcomes[f.Path] = Unchanged
			continue
		case exists && !force:
			replace, err := u.Confirm(f.Path+" already exists - replace it?", false)
			if err != nil {
				return outcomes, err
			}
			if !replace {
				fmt.Fprintf(out, "  kept your %s\n", f.Path)
				outcomes[f.Path] = Kept
				continue
			}
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return outcomes, err
		}
		mode := f.Mode
		if exists {
			if info, err := os.Stat(target); err == nil {
				mode = info.Mode().Perm() | (f.Mode & 0o111) // keep theirs, but a script stays runnable
			}
		}
		if err := os.WriteFile(target, f.Content, mode); err != nil {
			return outcomes, err
		}
		if err := os.Chmod(target, mode); err != nil { // past the umask
			return outcomes, err
		}
		if exists {
			fmt.Fprintf(out, "  replaced %s\n", f.Path)
			outcomes[f.Path] = Replaced
		} else {
			fmt.Fprintf(out, "  created %s\n", f.Path)
			outcomes[f.Path] = Created
		}
	}
	return outcomes, nil
}

// MissingIgnores lists which of the lines the build leaves behind an existing
// .gitignore doesn't already cover, so the user can be told to add them.
func MissingIgnores(gitignore string, app string) []string {
	have := map[string]bool{}
	for _, l := range strings.Split(gitignore, "\n") {
		have[strings.TrimSpace(l)] = true
	}
	covered := func(options ...string) bool {
		for _, o := range options {
			if have[o] {
				return true
			}
		}
		return false
	}
	var missing []string
	if !covered("/"+app, app, "/"+app+"/", app+"/") {
		missing = append(missing, "/"+app)
	}
	if !covered("/dist/", "/dist", "dist/", "dist") {
		missing = append(missing, "/dist/")
	}
	return missing
}
