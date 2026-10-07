package scaffold

import (
	"bufio"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ModulePath reads the module path from the go.mod in dir, or "" when there
// is none.
func ModulePath(dir string) string {
	f, err := os.Open(filepath.Join(dir, "go.mod"))
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if rest, ok := strings.CutPrefix(line, "module"); ok && (rest == "" || rest[0] == ' ' || rest[0] == '\t') {
			if i := strings.Index(rest, "//"); i >= 0 {
				rest = rest[:i]
			}
			return strings.Trim(strings.TrimSpace(rest), `"`+"`")
		}
	}
	return ""
}

// skipDir is whether a folder is left out when looking through a project for
// Go sources: version control, dependencies, and hidden folders.
func skipDir(name string) bool {
	return name == "vendor" || name == "node_modules" || name == "testdata" || strings.HasPrefix(name, ".")
}

// goFiles lists the .go files under dir (slash-separated, relative to it),
// leaving out those in folders skipDir excludes, in nested modules (a folder
// with a go.mod of its own is a different project), and in the CLI-support
// package goscaffold generates, which is not the project's own code.
func goFiles(dir string) []string {
	var files []string
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel != "." && skipDir(d.Name()) {
				return filepath.SkipDir
			}
			if rel == "internal/appcli" {
				return filepath.SkipDir
			}
			if rel != "." {
				if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".go") {
			files = append(files, rel)
		}
		return nil
	})
	return files
}

// HasGoSources reports whether the project in dir has Go code of its own.
func HasGoSources(dir string) bool {
	return len(goFiles(dir)) > 0
}

// MainPackages lists the folders under dir holding a main package, as
// "./path" ("." for dir itself), shallowest first.
func MainPackages(dir string) []string {
	seen := map[string]bool{}
	for _, f := range goFiles(dir) {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, filepath.FromSlash(f)), nil, parser.PackageClauseOnly)
		if err != nil || parsed.Name.Name != "main" {
			continue
		}
		folder := "."
		if d := filepath.ToSlash(filepath.Dir(filepath.FromSlash(f))); d != "." {
			folder = "./" + d
		}
		seen[folder] = true
	}
	var folders []string
	for f := range seen {
		folders = append(folders, f)
	}
	sort.Slice(folders, func(i, j int) bool {
		di, dj := strings.Count(folders[i], "/"), strings.Count(folders[j], "/")
		if di != dj {
			return di < dj
		}
		return folders[i] < folders[j]
	})
	return folders
}

// IsEmptyDir reports whether dir exists, is a folder, and has nothing in it.
func IsEmptyDir(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) == 0
}

// UsesAppcli reports whether the project's own code already imports the
// generated CLI-support package, so there's no need to tell the user how to.
func UsesAppcli(dir string) bool {
	for _, f := range goFiles(dir) {
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(f)))
		if err == nil && strings.Contains(string(b), `/internal/appcli"`) {
			return true
		}
	}
	return false
}
