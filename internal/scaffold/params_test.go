package scaffold

import (
	"testing"
	"testing/fstest"
)

func TestValidateApp(t *testing.T) {
	for _, ok := range []string{"goscaffold", "my-tool", "my_tool", "tool2", "a.b", "7zip"} {
		if err := ValidateApp(ok); err != nil {
			t.Errorf("%q refused: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "-tool", ".hidden", "my tool", "tool;rm", "a/b", `a"b`, "$(x)"} {
		if ValidateApp(bad) == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestEnvAndPowerShellNames(t *testing.T) {
	cases := []struct{ app, env, ps string }{
		{"goscaffold", "GOSCAFFOLD", "Goscaffold"},
		{"my-tool", "MY_TOOL", "MyTool"},
		{"foo.bar_baz", "FOO_BAR_BAZ", "FooBarBaz"},
	}
	for _, c := range cases {
		if got := envName(c.app); got != c.env {
			t.Errorf("envName(%q) = %q, want %q", c.app, got, c.env)
		}
		if got := psName(c.app); got != c.ps {
			t.Errorf("psName(%q) = %q, want %q", c.app, got, c.ps)
		}
	}
}

func TestRenderFillsEveryHole(t *testing.T) {
	assets := fstest.MapFS{
		"templates/a.tmpl": {Data: []byte("@@APP@@ @@MODULE@@ @@REPO@@ @@BRANCH@@ @@PKG@@ @@APPENV@@ @@APPPS@@ ${{ github.run_number }} {{x}}")},
	}
	got, err := Render(assets, "templates/a.tmpl", Params{App: "my-tool", Module: "github.com/o/my-tool", Repo: "o/my-tool", Branch: "trunk", Pkg: "./src"})
	if err != nil {
		t.Fatal(err)
	}
	want := "my-tool github.com/o/my-tool o/my-tool trunk ./src MY_TOOL MyTool ${{ github.run_number }} {{x}}"
	if string(got) != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestRenderExpandsTheInstallSectionAndItsOwnHoles(t *testing.T) {
	assets := fstest.MapFS{
		"templates/README.md.tmpl":          {Data: []byte("@@APP@@\n---\n\n@@INSTALL@@\n")},
		"templates/install-section.md.tmpl": {Data: []byte("## Install\n\nrun @@APP@@ from @@REPO@@@@BRANCH@@\n")},
	}
	got, err := Render(assets, "templates/README.md.tmpl", Params{App: "x", Repo: "o/x", Branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "x\n---\n\n## Install\n\nrun x from o/xmain\n"; string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestValidateOwner(t *testing.T) {
	for _, ok := range []string{"bobsaget", "my-org", "Acme2", "a"} {
		if err := ValidateOwner(ok); err != nil {
			t.Errorf("%q refused: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "-org", "my org", "my_org", "a/b", "org;x"} {
		if ValidateOwner(bad) == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestRenderNormalisesLineEndings(t *testing.T) {
	assets := fstest.MapFS{
		"templates/a.tmpl": {Data: []byte("#!/bin/sh\r\necho @@APP@@\r\n")},
	}
	got, err := Render(assets, "templates/a.tmpl", Params{App: "x"})
	if err != nil || string(got) != "#!/bin/sh\necho x\n" {
		t.Errorf("got %q, %v", got, err)
	}
}
