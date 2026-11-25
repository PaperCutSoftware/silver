package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestUpdater_ReloadFile runs the real updater binary against a local update
// server and checks which files a successful upgrade leaves behind.
func TestUpdater_ReloadFile(t *testing.T) {
	exe := buildUpdater(t)
	srv := newUpdateServer(t)

	tests := []struct {
		name        string
		flags       []string
		versionFile string
		wantReload  bool
	}{
		{name: "default writes reload", versionFile: ".version", wantReload: true},
		{name: "no-reload skips reload", flags: []string{"-no-reload"}, versionFile: ".version", wantReload: false},
		{name: "no-reload=false writes reload", flags: []string{"-no-reload=false"}, versionFile: ".version", wantReload: true},
		{name: "no-reload with custom version file", flags: []string{"-no-reload", "-f", "custom.ver"}, versionFile: "custom.ver", wantReload: false},
		{name: "custom version file writes reload", flags: []string{"-f", "custom.ver"}, versionFile: "custom.ver", wantReload: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			versionPath := filepath.Join(dir, tc.versionFile)
			if err := os.WriteFile(versionPath, []byte("1"), 0o644); err != nil {
				t.Fatal(err)
			}

			args := append([]string{"-http"}, tc.flags...)
			args = append(args, srv.URL+"/version-check.json")
			cmd := exec.Command(exe, args...)
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("updater %v failed: %v\n%s", args, err, out)
			}

			got, err := os.ReadFile(versionPath)
			if err != nil {
				t.Fatalf("reading version file: %v\n%s", err, out)
			}
			if v := strings.TrimSpace(string(got)); v != "2" {
				t.Errorf("version file = %q, want %q\n%s", v, "2", out)
			}
			if _, err := os.Stat(filepath.Join(dir, "v-done-0002")); err != nil {
				t.Errorf("update was not applied: %v\n%s", err, out)
			}

			_, err = os.Stat(filepath.Join(dir, ".reload"))
			if gotReload := err == nil; gotReload != tc.wantReload {
				t.Errorf(".reload exists = %v, want %v (stat err: %v)\n%s", gotReload, tc.wantReload, err, out)
			}
		})
	}
}

// newUpdateServer serves the testdata manifest, pointed at its own copy of the zip.
func newUpdateServer(t *testing.T) *httptest.Server {
	t.Helper()
	td := filepath.Join(updaterDir(t), "testdata")
	raw, err := os.ReadFile(filepath.Join(td, "version-check.json"))
	if err != nil {
		t.Fatal(err)
	}
	zip, err := os.ReadFile(filepath.Join(td, "version-2.zip"))
	if err != nil {
		t.Fatal(err)
	}

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/version-check.json":
			var m map[string]interface{}
			if err := json.Unmarshal(raw, &m); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			m["url"] = srv.URL + "/version-2.zip"
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(m)
		case "/version-2.zip":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(zip)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func updaterDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	return filepath.Dir(thisFile)
}

func buildUpdater(t *testing.T) string {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "updater.exe")
	cmd := exec.Command("go", "build", "-o", exe, ".")
	cmd.Dir = updaterDir(t)
	if o, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building updater: %v\n%s", err, o)
	}
	return exe
}
