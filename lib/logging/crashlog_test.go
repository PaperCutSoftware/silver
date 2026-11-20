// SILVER - Service Wrapper
//
// Copyright (c) 2014-2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.

package logging

// Do not use t.Parallel() here;

import (
	"bytes"
	"errors"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Size()
}

func stubCrashOutput(t *testing.T) **os.File {
	t.Helper()
	var got *os.File
	orig := setCrashOutput
	setCrashOutput = func(f *os.File, _ debug.CrashOptions) error {
		got = f
		return nil
	}
	t.Cleanup(func() { setCrashOutput = orig })
	return &got
}

func TestSetupCrashOutput(t *testing.T) {
	full := strings.Repeat("x", crashLogMaxSize)

	tests := []struct {
		name        string
		owner       string
		setup       func(t *testing.T, path string) string
		wantEnabled bool
		wantLog     string
		check       func(t *testing.T, path string)
	}{
		{
			name:        "new file",
			wantEnabled: true,
			check: func(t *testing.T, path string) {
				if got := fileSize(t, path); got != 0 {
					t.Errorf("crash log size = %d, want 0", got)
				}
			},
		},
		{
			name:  "chown fails",
			owner: "someone",
			setup: func(t *testing.T, path string) string {
				orig := changeOwnerOfFileFunc
				changeOwnerOfFileFunc = func(_, owner string) error {
					if owner != "someone" {
						t.Errorf("chown owner = %q, want %q", owner, "someone")
					}
					return errors.New("chown failed")
				}
				t.Cleanup(func() { changeOwnerOfFileFunc = orig })
				return path
			},
			wantEnabled: true,
			wantLog:     "WARNING: Unable to set owner of crash log",
		},
		{
			name: "unwritable path",
			setup: func(t *testing.T, path string) string {
				return filepath.Join(filepath.Dir(path), "missing-dir", filepath.Base(path))
			},
			wantEnabled: false,
			wantLog:     "WARNING: Unable to open crash log",
		},
		{
			name: "full log rotated",
			setup: func(t *testing.T, path string) string {
				writeTestFile(t, path, full)
				return path
			},
			wantEnabled: true,
			check: func(t *testing.T, path string) {
				if got := fileSize(t, path+".1"); got != crashLogMaxSize {
					t.Errorf(".1 size = %d, want %d", got, crashLogMaxSize)
				}
				if got := fileSize(t, path); got != 0 {
					t.Errorf("crash log size = %d, want 0", got)
				}
			},
		},
		{
			name: "rotation fails",
			setup: func(t *testing.T, path string) string {
				writeTestFile(t, path, full)
				writeTestFile(t, path+".1", "older")
				// A non-empty directory at .2 makes the .1 -> .2 rename fail on every OS.
				if err := os.MkdirAll(filepath.Join(path+".2", "block"), 0o755); err != nil {
					t.Fatal(err)
				}
				return path
			},
			wantEnabled: true,
			wantLog:     "WARNING: Unable to rotate crash log",
			check: func(t *testing.T, path string) {
				if got := fileSize(t, path+".1"); got != int64(len("older")) {
					t.Errorf(".1 was overwritten (size %d)", got)
				}
				if got := fileSize(t, path); got != crashLogMaxSize {
					t.Errorf("crash log size = %d, want %d", got, crashLogMaxSize)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := stubCrashOutput(t)
			path := filepath.Join(t.TempDir(), "x-silver-crash.log")
			if tt.setup != nil {
				path = tt.setup(t, path)
			}
			var buf bytes.Buffer

			SetupCrashOutput(path, tt.owner, log.New(&buf, "", 0))

			if enabled := *called != nil; enabled != tt.wantEnabled {
				t.Errorf("crash output enabled = %v, want %v", enabled, tt.wantEnabled)
			}
			if tt.wantEnabled && !strings.Contains(buf.String(), "Crash output enabled: "+path) {
				t.Errorf("log %q missing confirmation for %s", buf.String(), path)
			}
			if !strings.Contains(buf.String(), tt.wantLog) {
				t.Errorf("log %q does not contain %q", buf.String(), tt.wantLog)
			}
			if tt.check != nil {
				tt.check(t, path)
			}
		})
	}
}

func TestRotateCrashLog(t *testing.T) {
	tests := []struct {
		name   string
		before map[string]string // file suffix ("", ".1", ".2") -> contents
		want   map[string]string // expected files after rotation; others must not exist
	}{
		{
			name: "missing file",
		},
		{
			name:   "below limit",
			before: map[string]string{"": "short"},
			want:   map[string]string{"": "short"},
		},
		{
			name:   "at limit shifts and drops oldest",
			before: map[string]string{"": "0123456789", ".1": "older", ".2": "oldest"},
			want:   map[string]string{".1": "0123456789", ".2": "older"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "x-silver-crash.log")
			for suffix, content := range tt.before {
				writeTestFile(t, path+suffix, content)
			}

			if err := rotateCrashLog(path, 10, 2); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for _, suffix := range []string{"", ".1", ".2", ".3"} {
				data, err := os.ReadFile(path + suffix)
				want, ok := tt.want[suffix]
				switch {
				case !ok && err == nil:
					t.Errorf("unexpected file %q", "x-silver-crash.log"+suffix)
				case ok && err != nil:
					t.Errorf("missing file %q: %v", "x-silver-crash.log"+suffix, err)
				case ok && string(data) != want:
					t.Errorf("%q = %q, want %q", "x-silver-crash.log"+suffix, data, want)
				}
			}
		})
	}
}
