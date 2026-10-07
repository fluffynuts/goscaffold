package sh

import "strings"

// Fake is a Runner for tests: it records every command and lets a handler
// answer them, with no real process involved.
type Fake struct {
	// Handler answers a command. A nil Handler (or a nil result from it, see
	// below) answers with empty output and no error.
	Handler func(dir, name string, args []string) (string, error)
	// Missing names the programs Has should report as not installed.
	Missing map[string]bool
	// Calls is every command run so far, as "name arg arg".
	Calls []string
}

func (f *Fake) Exec(dir, name string, args ...string) (string, error) {
	f.Calls = append(f.Calls, strings.TrimSpace(name+" "+strings.Join(args, " ")))
	if f.Handler == nil {
		return "", nil
	}
	return f.Handler(dir, name, args)
}

func (f *Fake) Has(name string) bool { return !f.Missing[name] }

// Ran reports whether a recorded command starts with prefix.
func (f *Fake) Ran(prefix string) bool {
	for _, c := range f.Calls {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}
