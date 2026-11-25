package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/papercutsoftware/silver/service/config"
)

func svcAt(p string) config.Service {
	s := config.Service{}
	s.Path = p
	return s
}

func schedAt(p string) config.ScheduledTask {
	s := config.ScheduledTask{}
	s.Path = p
	return s
}

func startupAt(p string, async bool) config.StartupTask {
	s := config.StartupTask{}
	s.Path = p
	s.Async = async
	return s
}

func cmdAt(p string) config.Command {
	c := config.Command{}
	c.Path = p
	return c
}

// makeTree creates files (empty) and directories under root; names use '/'.
func makeTree(t *testing.T, root string, files, dirs []string) {
	t.Helper()
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range files {
		p := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

const dummyTool = "silver-checkconf-dummy-tool"

// putDummyOnPath sets PATH to a temp folder holding an executable named dummyTool.
func putDummyOnPath(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	name := dummyTool
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

func TestCheckConfRunnable(t *testing.T) {
	// j joins root with a slash-separated relative path, keeping glob characters.
	j := func(root, rel string) string { return filepath.Join(root, filepath.FromSlash(rel)) }

	tests := []struct {
		name    string
		files   []string
		dirs    []string
		onPath  bool // put dummyTool on PATH
		conf    func(root string) *config.Config
		wantErr bool
		// errPath, when set, is a relative config path (joined with root unless bare) that must appear in the error
		errPath string
		errBare bool
		// errFolders must also appear in the error (resolved folder and current version folder)
		errFolders []string
	}{
		{
			name:  "all present",
			files: []string{"v1/sub.conf", "v1/a.exe", "v1/b.exe", "v1/c.exe"},
			conf: func(r string) *config.Config {
				c := emptyConf()
				c.Include = []string{j(r, "v*/sub.conf")}
				c.Services = []config.Service{svcAt(j(r, "v*/a.exe"))}
				c.ScheduledTasks = []config.ScheduledTask{schedAt(j(r, "v*/b.exe"))}
				c.StartupTasks = []config.StartupTask{startupAt(j(r, "v*/c.exe"), false)}
				return c
			},
		},
		{
			name:  "empty config",
			files: nil,
			conf:  func(r string) *config.Config { return emptyConf() },
		},
		{
			name:  "glob matching nothing",
			files: []string{"v1/sub.conf"},
			conf: func(r string) *config.Config {
				c := emptyConf()
				c.Include = []string{j(r, "v*/sub.conf")}
				c.Services = []config.Service{svcAt(j(r, "v*/missing.exe"))}
				return c
			},
			wantErr: true, errPath: "v*/missing.exe",
		},
		{
			name:  "glob matching only a directory",
			files: []string{"v1/sub.conf"},
			dirs:  []string{"v1/x.exe"},
			conf: func(r string) *config.Config {
				c := emptyConf()
				c.Include = []string{j(r, "v*/sub.conf")}
				c.Services = []config.Service{svcAt(j(r, "v*/x.exe"))}
				return c
			},
			wantErr: true, errPath: "v*/x.exe",
		},
		{
			name:  "missing scheduled task",
			files: []string{"v1/sub.conf", "v1/a.exe"},
			conf: func(r string) *config.Config {
				c := emptyConf()
				c.Include = []string{j(r, "v*/sub.conf")}
				c.Services = []config.Service{svcAt(j(r, "v*/a.exe"))}
				c.ScheduledTasks = []config.ScheduledTask{schedAt(j(r, "v*/sched.exe"))}
				return c
			},
			wantErr: true, errPath: "v*/sched.exe",
		},
		{
			name:  "missing sync startup task",
			files: []string{"v1/sub.conf"},
			conf: func(r string) *config.Config {
				c := emptyConf()
				c.Include = []string{j(r, "v*/sub.conf")}
				c.StartupTasks = []config.StartupTask{startupAt(j(r, "v*/init.exe"), false)}
				return c
			},
			wantErr: true, errPath: "v*/init.exe",
		},
		{
			name:  "missing async startup task is ignored",
			files: []string{"v1/sub.conf", "v1/a.exe"},
			conf: func(r string) *config.Config {
				c := emptyConf()
				c.Include = []string{j(r, "v*/sub.conf")}
				c.Services = []config.Service{svcAt(j(r, "v*/a.exe"))}
				c.StartupTasks = []config.StartupTask{startupAt(j(r, "v*/async.exe"), true)}
				return c
			},
		},
		{
			name:  "commands are ignored",
			files: []string{"v1/sub.conf", "v1/a.exe"},
			conf: func(r string) *config.Config {
				c := emptyConf()
				c.Include = []string{j(r, "v*/sub.conf")}
				c.Services = []config.Service{svcAt(j(r, "v*/a.exe"))}
				c.Commands = []config.Command{cmdAt(j(r, "v*/nope.exe")), cmdAt(j(r, "bin/nope.exe"))}
				return c
			},
		},
		{
			name:   "bare name found on PATH",
			onPath: true,
			conf: func(r string) *config.Config {
				c := emptyConf()
				c.Services = []config.Service{svcAt(dummyTool)}
				return c
			},
		},
		{
			name:   "unknown bare name",
			onPath: true,
			conf: func(r string) *config.Config {
				c := emptyConf()
				c.Services = []config.Service{svcAt("silver-checkconf-no-such-tool")}
				return c
			},
			wantErr: true, errPath: "silver-checkconf-no-such-tool", errBare: true,
		},
		{
			name:  "exe missing from include folder but present in older one",
			files: []string{"v123/sub.conf", "v123/x.exe", "v124/sub.conf"},
			conf: func(r string) *config.Config {
				c := emptyConf()
				c.Include = []string{j(r, "v*/sub.conf")}
				c.Services = []config.Service{svcAt(j(r, "v*/x.exe"))}
				return c
			},
			wantErr: true, errPath: "v*/x.exe", errFolders: []string{"v123", "v124"},
		},
		{
			name:  "stray version.txt and vendor folder in root",
			files: []string{"v123/sub.conf", "v123/x.exe", "version.txt"},
			dirs:  []string{"vendor"},
			conf: func(r string) *config.Config {
				c := emptyConf()
				c.Include = []string{j(r, "v*/sub.conf")}
				c.Services = []config.Service{svcAt(j(r, "v*/x.exe"))}
				return c
			},
		},
		{
			name:  "no include uses newest folder, skipping files",
			files: []string{"v123/x.exe", "v124/x.exe", "vz.txt"},
			conf: func(r string) *config.Config {
				c := emptyConf()
				c.Services = []config.Service{svcAt(j(r, "v*/x.exe"))}
				return c
			},
		},
		{
			name:  "no include and exe missing from newest folder",
			files: []string{"v123/x.exe"},
			dirs:  []string{"v124"},
			conf: func(r string) *config.Config {
				c := emptyConf()
				c.Services = []config.Service{svcAt(j(r, "v*/x.exe"))}
				return c
			},
			wantErr: true, errPath: "v*/x.exe", errFolders: []string{"v123", "v124"},
		},
		{
			name: "beta component anchored to its own include",
			files: []string{
				"v1/sub.conf", "v1/a.exe",
				"beta/v1/x.exe", "beta/v2/sub.conf", "beta/v2/x.exe",
			},
			conf: func(r string) *config.Config {
				c := emptyConf()
				c.Include = []string{j(r, "v*/sub.conf"), j(r, "beta/v*/sub.conf")}
				c.Services = []config.Service{svcAt(j(r, "v*/a.exe")), svcAt(j(r, "beta/v*/x.exe"))}
				return c
			},
		},
		{
			name:  "beta component exe falls back to older beta folder",
			files: []string{"v1/sub.conf", "v1/a.exe", "beta/v1/x.exe", "beta/v2/sub.conf"},
			conf: func(r string) *config.Config {
				c := emptyConf()
				c.Include = []string{j(r, "v*/sub.conf"), j(r, "beta/v*/sub.conf")}
				c.Services = []config.Service{svcAt(j(r, "v*/a.exe")), svcAt(j(r, "beta/v*/x.exe"))}
				return c
			},
			wantErr: true, errPath: "beta/v*/x.exe",
		},
		{
			name:  "two includes in one component resolve to different folders",
			files: []string{"v123/a.conf", "v124/b.conf", "v124/x.exe"},
			conf: func(r string) *config.Config {
				c := emptyConf()
				c.Include = []string{j(r, "v*/a.conf"), j(r, "v*/b.conf")}
				c.Services = []config.Service{svcAt(j(r, "v*/x.exe"))}
				return c
			},
			wantErr: true, errPath: "v*/b.conf", errFolders: []string{"v123", "v124"},
		},
		{
			name:  "path with no wildcard only has to exist",
			files: []string{"v2/sub.conf", "bin/tool.exe"},
			conf: func(r string) *config.Config {
				c := emptyConf()
				c.Include = []string{j(r, "v*/sub.conf")}
				c.Services = []config.Service{svcAt(j(r, "bin/tool.exe"))}
				return c
			},
		},
		{
			name:  "path with no wildcard missing",
			files: []string{"v2/sub.conf"},
			conf: func(r string) *config.Config {
				c := emptyConf()
				c.Include = []string{j(r, "v*/sub.conf")}
				c.Services = []config.Service{svcAt(j(r, "bin/tool.exe"))}
				return c
			},
			wantErr: true, errPath: "bin/tool.exe",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			makeTree(t, root, tc.files, tc.dirs)
			if tc.onPath {
				putDummyOnPath(t)
			}
			conf := tc.conf(root)

			err := checkConfRunnable(conf)
			if !tc.wantErr {
				if err != nil {
					t.Fatalf("checkConfRunnable() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatal("checkConfRunnable() = nil, want an error")
			}
			if tc.errPath != "" {
				want := tc.errPath
				if !tc.errBare {
					want = j(root, tc.errPath)
				}
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not name the failing path %q", err.Error(), want)
				}
			}
			for _, f := range tc.errFolders {
				if !strings.Contains(err.Error(), f) {
					t.Errorf("error %q does not name folder %q", err.Error(), f)
				}
			}
		})
	}
}

// TestCheckConfRunnable_RelativeIncludeAbsoluteService changes the working directory,
// so it cannot run in parallel and is kept out of the table above.
func TestCheckConfRunnable_RelativeIncludeAbsoluteService(t *testing.T) {
	// Not symlink-resolved: on macOS t.TempDir() is under /var while os.Getwd()
	// after a chdir returns the /private/var form.
	root := t.TempDir()
	makeTree(t, root, []string{"v123/sub.conf", "v123/x.exe", "v124/sub.conf", "vendor/readme.txt"}, nil)

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

	conf := emptyConf()
	conf.Include = []string{"v*/sub.conf"}
	conf.Services = []config.Service{svcAt(filepath.Join(root, "v*", "x.exe"))}

	err = checkConfRunnable(conf)
	if err == nil {
		t.Fatal("checkConfRunnable() = nil, want an error")
	}
	for _, f := range []string{"v123", "v124"} {
		if !strings.Contains(err.Error(), f) {
			t.Errorf("error %q does not name folder %q", err.Error(), f)
		}
	}

	makeTree(t, root, []string{"v124/x.exe"}, nil)
	if err := checkConfRunnable(conf); err != nil {
		t.Fatalf("checkConfRunnable() after adding v124/x.exe = %v, want nil", err)
	}
}
