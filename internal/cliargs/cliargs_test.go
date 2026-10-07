package cliargs

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		want Args
	}{
		{"just a path", []string{"some/dir"}, Args{Path: "some/dir"}},
		{"long options", []string{"--name", "foo", "--org", "bar", "--private", "dir"}, Args{Path: "dir", Name: "foo", Org: "bar", Visibility: "private"}},
		{"short options", []string{"-n", "foo", "-o", "bar", "-P", "-g", "-c", "-f", "-y", "-r", "o/r", "-m", "mod", "dir"},
			Args{Path: "dir", Name: "foo", Org: "bar", Visibility: "public", NoRepo: true, CreateRepo: true, Force: true, Yes: true, Repo: "o/r", Module: "mod"}},
		{"equals form", []string{"--name=foo", "--module=github.com/o/foo", "dir"}, Args{Path: "dir", Name: "foo", Module: "github.com/o/foo"}},
		{"options after the path", []string{"dir", "-p"}, Args{Path: "dir", Visibility: "private"}},
		{"help needs no path", []string{"--help"}, Args{Help: true}},
	}
	for _, c := range cases {
		got, err := Parse(c.argv)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	for name, argv := range map[string][]string{
		"no path":           {},
		"two paths":         {"a", "b"},
		"unknown option":    {"--frobnicate", "dir"},
		"missing value":     {"dir", "--name"},
		"value on a switch": {"--force=yes", "dir"},
	} {
		if _, err := Parse(argv); err == nil {
			t.Errorf("%s: no error for %v", name, argv)
		}
	}
}
