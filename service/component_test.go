package main

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// chdirTemp changes the working directory to a new temp folder holding dirs (slash-separated)
// and returns that folder symlink-resolved. Tests using it must not run in parallel.
func chdirTemp(t *testing.T, dirs ...string) string {
	t.Helper()
	root := t.TempDir()
	makeTree(t, root, nil, dirs)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Errorf("restoring working directory: %v", err)
		}
	})
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	cwd, err := filepath.EvalSymlinks(wd)
	if err != nil {
		t.Fatal(err)
	}
	return cwd
}

func TestComponentOf(t *testing.T) {
	cwd := chdirTemp(t, "beta", "alpha", "a/b")
	tmp := t.TempDir() // not symlink-resolved: on macOS it is under /var, a symlink to /private/var
	makeTree(t, tmp, nil, []string{"beta"})
	tmpBeta, err := filepath.EvalSymlinks(filepath.Join(tmp, "beta"))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		path        string
		windowsOnly bool // '\' is only a separator on Windows
		wantDir     string
		wantGlob    string
		wantOK      bool
	}{
		{path: "v*/x", wantDir: cwd, wantGlob: "v*", wantOK: true},
		{path: "v?/x.exe", wantDir: cwd, wantGlob: "v?", wantOK: true},
		{path: "v[12]/x.exe", wantDir: cwd, wantGlob: "v[12]", wantOK: true},
		{path: "beta/v*/x", wantDir: filepath.Join(cwd, "beta"), wantGlob: "v*", wantOK: true},
		{path: `beta\v*\x`, windowsOnly: true, wantDir: filepath.Join(cwd, "beta"), wantGlob: "v*", wantOK: true},
		{path: "a/b/v*/c/x.exe", wantDir: filepath.Join(cwd, "a", "b"), wantGlob: "v*", wantOK: true},
		{path: "beta/v*/sub/v*/x", wantDir: filepath.Join(cwd, "beta"), wantGlob: "v*", wantOK: true},
		{path: "./beta/../alpha//v*/x", wantDir: filepath.Join(cwd, "alpha"), wantGlob: "v*", wantOK: true},
		{path: filepath.Join(tmp, "beta", "v*", "x.exe"), wantDir: tmpBeta, wantGlob: "v*", wantOK: true},
		{path: tmp + "/beta/./x/..//v*/x.exe", wantDir: tmpBeta, wantGlob: "v*", wantOK: true},
		{path: "bin/tool.exe", wantOK: false},
		{path: "powershell.exe", wantOK: false},
		{path: "foo*", wantOK: false},
		{path: "", wantOK: false},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			if tc.windowsOnly && runtime.GOOS != "windows" {
				t.Skip("backslash is not a path separator on this OS")
			}
			c, glob, ok := componentOf(tc.path)
			if ok != tc.wantOK {
				t.Fatalf("componentOf(%q) ok = %v, want %v", tc.path, ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if c.dir != tc.wantDir || glob != tc.wantGlob {
				t.Errorf("componentOf(%q) = (%q, %q), want (%q, %q)", tc.path, c.dir, glob, tc.wantDir, tc.wantGlob)
			}
		})
	}
}

func TestComponentsOf(t *testing.T) {
	root := chdirTemp(t, "alpha", "beta", "gamma")
	unresolved, err := os.Getwd() // may differ from root when the temp folder is behind a symlink
	if err != nil {
		t.Fatal(err)
	}
	at := func(parts ...string) string { return filepath.Join(append([]string{root}, parts...)...) }
	tests := []struct {
		name        string
		include     []string
		services    []string
		startup     []string
		asyncStart  []string
		scheduled   []string
		commands    []string
		windowsOnly bool     // '\' is only a separator on Windows
		want        []string // component dirs
	}{
		{name: "empty config"},
		{name: "root component", services: []string{"v*/pc-user-client.exe"}, want: []string{at()}},
		{name: "subfolder component", services: []string{"beta/v*/pc-user-client.exe"}, want: []string{at("beta")}},
		{name: "include and service in two components", include: []string{"alpha/v*/x.conf"}, services: []string{"beta/v*/y.exe"},
			want: []string{at("alpha"), at("beta")}},
		{name: "service root already replaced", services: []string{root + "/beta/v*/y.exe"}, want: []string{at("beta")}},
		{name: "no wildcard", services: []string{"bin/tool.exe"}, include: []string{"conf/extra.conf"}},
		{name: "startup and scheduled tasks", startup: []string{"alpha/v*/s.exe"}, scheduled: []string{"gamma/v*/t.exe"},
			want: []string{at("alpha"), at("gamma")}},
		{name: "async startup task", asyncStart: []string{"beta/v*/s.exe"}, want: []string{at("beta")}},
		{name: "duplicates across lists", services: []string{"beta/v*/a.exe", "beta/v*/b.exe"}, startup: []string{"beta/v*/c.exe"},
			scheduled: []string{"beta/v*/d.exe"}, include: []string{"beta/v*/e.conf"}, want: []string{at("beta")}},
		{name: "absolute and relative to the same folder", include: []string{unresolved + "/v*/x.conf"}, services: []string{"v*/y.exe"},
			want: []string{at()}},
		{name: "sorted", services: []string{"gamma/v*/g.exe", "alpha/v*/a.exe", "beta/v*/b.exe"},
			want: []string{at("alpha"), at("beta"), at("gamma")}},
		{name: "commands excluded", services: []string{"beta/v*/y.exe"}, commands: []string{"alpha/v*/c.exe", "v*/d.exe"},
			want: []string{at("beta")}},
		{name: "only commands", commands: []string{"alpha/v*/c.exe"}},
		{name: "mixed separators", windowsOnly: true, services: []string{`alpha\v*\x.exe`, "alpha/v*/y.exe"},
			want: []string{at("alpha")}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.windowsOnly && runtime.GOOS != "windows" {
				t.Skip("backslash is not a path separator on this OS")
			}
			conf := emptyConf()
			conf.Include = tc.include
			for _, p := range tc.services {
				conf.Services = append(conf.Services, svcAt(p))
			}
			for _, p := range tc.startup {
				conf.StartupTasks = append(conf.StartupTasks, startupAt(p, false))
			}
			for _, p := range tc.asyncStart {
				conf.StartupTasks = append(conf.StartupTasks, startupAt(p, true))
			}
			for _, p := range tc.scheduled {
				conf.ScheduledTasks = append(conf.ScheduledTasks, schedAt(p))
			}
			for _, p := range tc.commands {
				conf.Commands = append(conf.Commands, cmdAt(p))
			}
			got := componentsOf(conf)
			var dirs []string
			for _, c := range got {
				dirs = append(dirs, c.dir)
				if want := filepath.Join(c.dir, ".version"); c.versionFile() != want {
					t.Errorf("component %q versionFile() = %q, want %q", c.dir, c.versionFile(), want)
				}
			}
			if len(tc.want) == 0 {
				if len(dirs) != 0 {
					t.Errorf("componentsOf = %q, want none", dirs)
				}
				return
			}
			if !reflect.DeepEqual(dirs, tc.want) {
				t.Errorf("componentsOf = %q, want %q", dirs, tc.want)
			}
		})
	}
}

func TestComponentReadVersion(t *testing.T) {
	tests := []struct {
		name    string
		content *string // nil: no .version file
		want    string
	}{
		{name: "trimmed", content: strPtr(" 1.2.3\r\n"), want: "1.2.3"},
		{name: "whitespace only", content: strPtr(" \n\t"), want: ""},
		{name: "missing file", content: nil, want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			c := component{dir: dir}
			if tc.content != nil {
				if err := os.WriteFile(filepath.Join(dir, ".version"), []byte(*tc.content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if got := c.readVersion(); got != tc.want {
				t.Errorf("readVersion() = %q, want %q", got, tc.want)
			}
		})
	}
}
