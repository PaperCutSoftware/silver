// SILVER - Service Wrapper
// Auto Updater
//
// Copyright (c) 2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// pushdir and popdir for the cwd
// cannot run in parallel
func chdirForTest(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %q: %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Errorf("restoring working directory: %v", err)
		}
	})
}

// realPath resolves symlinks (e.g. macOS /var -> /private/var) so paths compare cleanly.
func realPath(t *testing.T, p string) string {
	t.Helper()
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatalf("abs %q: %v", p, err)
	}
	r, err := filepath.EvalSymlinks(abs)
	if err != nil {
		t.Fatalf("eval symlinks %q: %v", abs, err)
	}
	return r
}

func cwd(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return realPath(t, wd)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", path, err)
	}
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected %q to exist: %v", path, err)
	}
}

func assertNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Errorf("expected %q not to exist, but it does", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("stat %q: unexpected error %v", path, err)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("reading %q: %v", path, err)
		return
	}
	if got := strings.TrimSpace(string(data)); got != want {
		t.Errorf("%q content = %q, want %q", path, got, want)
	}
}

// buildUpdateZip builds a zip holding an explicit temp-v2/ directory entry
// and temp-v2/a.txt.
func buildUpdateZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if _, err := zw.CreateHeader(&zip.FileHeader{Name: "temp-v2/", Method: zip.Store}); err != nil {
		t.Fatalf("zip dir entry: %v", err)
	}
	w, err := zw.Create("temp-v2/a.txt")
	if err != nil {
		t.Fatalf("zip file entry: %v", err)
	}
	if _, err := w.Write([]byte("hello v2\n")); err != nil {
		t.Fatalf("zip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

type updateServer struct {
	*httptest.Server
	hits int64
}

// newUpdateServer serves a version-2 manifest at /check and its zip at /v2.zip.
func newUpdateServer(t *testing.T) *updateServer {
	t.Helper()
	zipBytes := buildUpdateZip(t)
	sum := sha256.Sum256(zipBytes)

	us := &updateServer{}
	mux := http.NewServeMux()
	mux.HandleFunc("/check", func(w http.ResponseWriter, r *http.Request) {
		manifest := map[string]interface{}{
			"url":     us.URL + "/v2.zip",
			"version": "2",
			"sha256":  hex.EncodeToString(sum[:]),
			"operations": []map[string]interface{}{
				{"action": "move", "args": []string{"temp-v2", "v2"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(manifest)
	})
	mux.HandleFunc("/v2.zip", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(zipBytes)
	})
	us.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&us.hits, 1)
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(us.Close)
	return us
}

func (us *updateServer) checkURL() string { return us.URL + "/check" }
func (us *updateServer) hitCount() int64  { return atomic.LoadInt64(&us.hits) }

// newInstallRoot creates <root>/component1 with a .version of "1".
func newInstallRoot(t *testing.T) string {
	t.Helper()
	root := realPath(t, t.TempDir())
	mustMkdir(t, filepath.Join(root, "component1"))
	writeFile(t, filepath.Join(root, "component1", ".version"), "1")
	return root
}

func assertUpgradedInto(t *testing.T, target string) {
	t.Helper()
	assertExists(t, filepath.Join(target, "v2", "a.txt"))
	assertFileContent(t, filepath.Join(target, ".version"), "2")
	assertExists(t, filepath.Join(target, ".reload"))
	assertNotExists(t, filepath.Join(target, "temp-v2"))
}

// assertNothingWrittenTo checks that no update artefacts landed in dir.
func assertNothingWrittenTo(t *testing.T, dir string) {
	t.Helper()
	for _, p := range []string{".reload", ".version", "v2", "temp-v2"} {
		assertNotExists(t, filepath.Join(dir, p))
	}
}

// buildUpdater compiles the updater binary into a temp dir.
func buildUpdater(t *testing.T) string {
	t.Helper()
	srcDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	name := "updater"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	bin := filepath.Join(t.TempDir(), name)
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = srcDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build updater: %v\n%s", err, out)
	}
	return bin
}

type runResult struct {
	stdout, stderr string
	exitCode       int
}

func runUpdater(t *testing.T, bin, dir string, args ...string) runResult {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		} else {
			t.Fatalf("running updater: %v", err)
		}
	}
	return runResult{stdout: stdout.String(), stderr: stderr.String(), exitCode: code}
}

// requireExitOK fails the test unless the updater exited 0, dumping its output.
func requireExitOK(t *testing.T, res runResult) {
	t.Helper()
	if res.exitCode != 0 {
		t.Fatalf("exit code = %d, want 0\nstdout: %s\nstderr: %s", res.exitCode, res.stdout, res.stderr)
	}
}

// ---------------------------------------------------------------------------
// enterRootDir
// ---------------------------------------------------------------------------

func TestEnterRootDir_ChangesDir(t *testing.T) {
	cases := []struct {
		name string
		// setup prepares root (the starting cwd) and returns the
		// enterRootDir argument and the expected resulting cwd.
		setup func(t *testing.T, root string) (arg, want string)
	}{
		{
			name: "empty is no-op",
			setup: func(t *testing.T, root string) (string, string) {
				return "", root
			},
		},
		{
			name: "relative",
			setup: func(t *testing.T, root string) (string, string) {
				mustMkdir(t, filepath.Join(root, "component1"))
				return "component1", filepath.Join(root, "component1")
			},
		},
		{
			name: "nested relative",
			setup: func(t *testing.T, root string) (string, string) {
				mustMkdir(t, filepath.Join(root, "a", "b"))
				return filepath.Join("a", "b"), filepath.Join(root, "a", "b")
			},
		},
		{
			name: "absolute",
			setup: func(t *testing.T, root string) (string, string) {
				target := realPath(t, t.TempDir())
				return target, target
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := realPath(t, t.TempDir())
			chdirForTest(t, root)
			arg, want := tc.setup(t, root)

			if err := enterRootDir(arg); err != nil {
				t.Fatalf("enterRootDir(%q) = %v, want nil", arg, err)
			}
			if got := cwd(t); got != want {
				t.Errorf("working directory = %q, want %q", got, want)
			}
		})
	}
}

func TestEnterRootDir_Errors(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, root string) string
	}{
		{
			name: "missing relative path",
			setup: func(t *testing.T, root string) string {
				return "does-not-exist"
			},
		},
		{
			name: "missing absolute path",
			setup: func(t *testing.T, root string) string {
				return filepath.Join(root, "does-not-exist")
			},
		},
		{
			name: "regular file",
			setup: func(t *testing.T, root string) string {
				writeFile(t, filepath.Join(root, "afile"), "x")
				return "afile"
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := realPath(t, t.TempDir())
			chdirForTest(t, root)
			arg := tc.setup(t, root)

			if err := enterRootDir(arg); err == nil {
				t.Fatalf("enterRootDir(%q) = nil, want error", arg)
			}
			// Must not create the folder.
			if tc.name != "regular file" {
				p := arg
				if !filepath.IsAbs(p) {
					p = filepath.Join(root, p)
				}
				assertNotExists(t, p)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// End-to-end via upgradeIfRequired
// ---------------------------------------------------------------------------

// enterAndUpgrade runs enterRootDir(rootDir) then upgradeIfRequired against
// srv in-process, failing unless both succeed and an upgrade happened.
func enterAndUpgrade(t *testing.T, srv *updateServer, rootDir string) {
	t.Helper()
	if err := enterRootDir(rootDir); err != nil {
		t.Fatalf("enterRootDir(%q): %v", rootDir, err)
	}
	upgraded, err := upgradeIfRequired(srv.checkURL(), "")
	if err != nil {
		t.Fatalf("upgradeIfRequired: %v", err)
	}
	if !upgraded {
		t.Errorf("upgradeIfRequired upgraded = false, want true")
	}
}

func TestUpgrade_WithRootDir_WritesIntoRootDir(t *testing.T) {
	root := newInstallRoot(t)
	srv := newUpdateServer(t)
	chdirForTest(t, root)

	enterAndUpgrade(t, srv, "component1")

	assertUpgradedInto(t, filepath.Join(root, "component1"))
	assertNothingWrittenTo(t, root)
}

func TestUpgrade_WithoutRootDir_WritesIntoWorkingDir(t *testing.T) {
	root := realPath(t, t.TempDir())
	writeFile(t, filepath.Join(root, ".version"), "1")
	srv := newUpdateServer(t)
	chdirForTest(t, root)

	enterAndUpgrade(t, srv, "")

	assertUpgradedInto(t, root)
}

// ---------------------------------------------------------------------------
// CLI-level behaviour
// ---------------------------------------------------------------------------

func TestCLI_RootDir_EndToEnd(t *testing.T) {
	bin := buildUpdater(t)
	srv := newUpdateServer(t)

	t.Run("relative rootDir", func(t *testing.T) {
		root := newInstallRoot(t)
		res := runUpdater(t, bin, root, "-http", "-rootDir", "component1", srv.checkURL())
		requireExitOK(t, res)
		assertUpgradedInto(t, filepath.Join(root, "component1"))
		assertNothingWrittenTo(t, root)
	})

	t.Run("absolute rootDir", func(t *testing.T) {
		root := newInstallRoot(t)
		elsewhere := realPath(t, t.TempDir())
		res := runUpdater(t, bin, elsewhere, "-http", "-rootDir", filepath.Join(root, "component1"), srv.checkURL())
		requireExitOK(t, res)
		assertUpgradedInto(t, filepath.Join(root, "component1"))
		assertNothingWrittenTo(t, elsewhere)
		assertNothingWrittenTo(t, root)
	})

	t.Run("custom -f stays relative to rootDir", func(t *testing.T) {
		root := realPath(t, t.TempDir())
		mustMkdir(t, filepath.Join(root, "component1"))
		writeFile(t, filepath.Join(root, "component1", "my.version"), "1")
		res := runUpdater(t, bin, root, "-http", "-rootDir", "component1", "-f", "my.version", srv.checkURL())
		requireExitOK(t, res)
		assertFileContent(t, filepath.Join(root, "component1", "my.version"), "2")
		assertExists(t, filepath.Join(root, "component1", "v2", "a.txt"))
		assertNotExists(t, filepath.Join(root, "my.version"))
	})

	t.Run("no rootDir writes into working dir", func(t *testing.T) {
		root := realPath(t, t.TempDir())
		writeFile(t, filepath.Join(root, ".version"), "1")
		res := runUpdater(t, bin, root, "-http", srv.checkURL())
		requireExitOK(t, res)
		assertUpgradedInto(t, root)
	})
}

func TestCLI_ShowVersion_RespectsRootDir(t *testing.T) {
	bin := buildUpdater(t)
	root := realPath(t, t.TempDir())
	mustMkdir(t, filepath.Join(root, "component1"))
	writeFile(t, filepath.Join(root, "component1", ".version"), "42.7.3")
	writeFile(t, filepath.Join(root, ".version"), "9.9.9")

	t.Run("relative rootDir", func(t *testing.T) {
		res := runUpdater(t, bin, root, "-rootDir", "component1", "-v")
		requireExitOK(t, res)
		if !strings.Contains(res.stdout, "42.7.3") {
			t.Errorf("stdout = %q, want it to contain component1 version 42.7.3", res.stdout)
		}
		if strings.Contains(res.stdout, "9.9.9") {
			t.Errorf("stdout = %q, must not show the root .version 9.9.9", res.stdout)
		}
	})

	t.Run("absolute rootDir", func(t *testing.T) {
		elsewhere := realPath(t, t.TempDir())
		res := runUpdater(t, bin, elsewhere, "-rootDir", filepath.Join(root, "component1"), "-v")
		requireExitOK(t, res)
		if !strings.Contains(res.stdout, "42.7.3") {
			t.Errorf("stdout = %q, want it to contain 42.7.3", res.stdout)
		}
	})

	t.Run("no rootDir reads working dir", func(t *testing.T) {
		res := runUpdater(t, bin, root, "-v")
		requireExitOK(t, res)
		if !strings.Contains(res.stdout, "9.9.9") {
			t.Errorf("stdout = %q, want it to contain 9.9.9", res.stdout)
		}
	})
}

func TestCLI_InvalidRootDir_FailsBeforeNetwork(t *testing.T) {
	bin := buildUpdater(t)

	cases := []struct {
		name   string
		setup  func(t *testing.T, root string)
		folder string
		isFile bool
	}{
		{name: "missing folder", folder: "nope-missing"},
		{
			name:   "not a directory",
			folder: "plainfile",
			isFile: true,
			setup: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, "plainfile"), "x")
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := realPath(t, t.TempDir())
			if tc.setup != nil {
				tc.setup(t, root)
			}
			srv := newUpdateServer(t)

			res := runUpdater(t, bin, root, "-http", "-rootDir", tc.folder, srv.checkURL())

			if res.exitCode != 1 {
				t.Errorf("exit code = %d, want 1\nstderr: %s", res.exitCode, res.stderr)
			}
			if !strings.Contains(res.stderr, "ERROR:") {
				t.Errorf("stderr = %q, want it to contain \"ERROR:\"", res.stderr)
			}
			if !strings.Contains(res.stderr, tc.folder) {
				t.Errorf("stderr = %q, want it to name the folder %q", res.stderr, tc.folder)
			}
			if n := srv.hitCount(); n != 0 {
				t.Errorf("update server received %d requests, want 0", n)
			}
			info, err := os.Stat(filepath.Join(root, tc.folder))
			if tc.isFile {
				if err != nil || info.IsDir() {
					t.Errorf("expected %q to remain a regular file", tc.folder)
				}
			} else {
				assertNotExists(t, filepath.Join(root, tc.folder))
			}
			assertNotExists(t, filepath.Join(root, ".reload"))
			assertNotExists(t, filepath.Join(root, ".version"))
		})
	}
}
