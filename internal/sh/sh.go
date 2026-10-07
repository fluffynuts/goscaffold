// Package sh runs external commands (git, gh, go) behind an interface, so
// the code that drives them can be tested with a fake.
package sh

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Runner runs external commands.
type Runner interface {
	// Exec runs name with args in dir (the current directory when empty)
	// and returns its standard output. A failure's error carries the
	// command's standard error, which is what says what went wrong.
	Exec(dir, name string, args ...string) (string, error)
	// Has reports whether name can be found on the PATH.
	Has(name string) bool
}

// OS is the Runner that really runs things.
type OS struct{}

func (OS) Exec(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg != "" {
			return stdout.String(), fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, msg)
		}
		return stdout.String(), fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return stdout.String(), nil
}

func (OS) Has(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// Lines splits command output into its non-empty, trimmed lines.
func Lines(out string) []string {
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}
