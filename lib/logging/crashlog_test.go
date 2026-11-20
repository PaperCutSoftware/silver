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
	"regexp"
	"runtime/debug"
	"strings"
	"testing"
	"time"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

var endsWithRestartMarker = regexp.MustCompile(`\n` + regexp.QuoteMeta(restartMarkerPrefix) + `\S+` + regexp.QuoteMeta(restartMarkerSuffix) + `\n$`)

func assertEndsWithRestartMarker(t *testing.T, path string) {
	t.Helper()
	if got := readTestFile(t, path); !endsWithRestartMarker.MatchString(got) {
		t.Errorf("%s does not end with a restart marker: %q", filepath.Base(path), got[max(0, len(got)-80):])
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
	marker := restartMarker(time.Now())
	full := strings.Repeat("x", crashLogMaxSize)
	fullClean := strings.Repeat("x", crashLogMaxSize-len(marker)) + marker

	tests := []struct {
		name             string
		owner            string
		setup            func(t *testing.T, path string) string
		wantEnabled      bool
		wantCrashWarning bool
		wantLog          string
		check            func(t *testing.T, path string)
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
				writeTestFile(t, path, fullClean)
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
				writeTestFile(t, path, fullClean)
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
		{
			name: "previous crash",
			setup: func(t *testing.T, path string) string {
				writeTestFile(t, path, "panic: boom\n")
				return path
			},
			wantEnabled:      true,
			wantCrashWarning: true,
			wantLog:          "See crash log {path} (last written",
			check: func(t *testing.T, path string) {
				assertEndsWithRestartMarker(t, path)
				if got := readTestFile(t, path); !strings.HasPrefix(got, "panic: boom\n") {
					t.Errorf("crash trace not kept, got %q", got)
				}
			},
		},
		{
			name: "clean restart after crash",
			setup: func(t *testing.T, path string) string {
				writeTestFile(t, path, "panic: boom\n"+marker)
				return path
			},
			wantEnabled: true,
			check: func(t *testing.T, path string) {
				if got := readTestFile(t, path); got != "panic: boom\n"+marker {
					t.Errorf("crash log changed on a clean restart, got %q", got)
				}
			},
		},
		{
			name: "previous crash in full log",
			setup: func(t *testing.T, path string) string {
				writeTestFile(t, path, full)
				return path
			},
			wantEnabled:      true,
			wantCrashWarning: true,
			wantLog:          "See crash log {path}.1 (last written",
			check: func(t *testing.T, path string) {
				assertEndsWithRestartMarker(t, path+".1")
				if got := fileSize(t, path); got != 0 {
					t.Errorf("crash log size = %d, want 0", got)
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
			if got := strings.Contains(buf.String(), "Silver crashed during a previous run"); got != tt.wantCrashWarning {
				t.Errorf("crash warning logged = %v, want %v; log %q", got, tt.wantCrashWarning, buf.String())
			}
			if want := strings.ReplaceAll(tt.wantLog, "{path}", path); !strings.Contains(buf.String(), want) {
				t.Errorf("log %q does not contain %q", buf.String(), want)
			}
			if tt.check != nil {
				tt.check(t, path)
			}
		})
	}
}

func TestRotateCrashLog(t *testing.T) {
	tests := []struct {
		name        string
		before      map[string]string // file suffix ("", ".1", ".2") -> contents
		want        map[string]string // expected files after rotation; others must not exist
		wantRotated bool
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
			name:        "at limit shifts and drops oldest",
			before:      map[string]string{"": "0123456789", ".1": "older", ".2": "oldest"},
			want:        map[string]string{".1": "0123456789", ".2": "older"},
			wantRotated: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "x-silver-crash.log")
			for suffix, content := range tt.before {
				writeTestFile(t, path+suffix, content)
			}

			rotated, err := rotateCrashLog(path, 10, 2)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rotated != tt.wantRotated {
				t.Errorf("rotated = %v, want %v", rotated, tt.wantRotated)
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

func TestPreviousRunCrashed(t *testing.T) {
	marker := restartMarkerPrefix + "2026-01-01T00:00:00Z" + restartMarkerSuffix

	tests := []struct {
		name    string
		content *string // nil means the file doesn't exist
		want    bool
	}{
		{name: "missing file", content: nil, want: false},
		{name: "empty file", content: ptr(""), want: false},
		{name: "ends with restart marker", content: ptr("panic: boom\n\n" + marker + "\n"), want: false},
		{name: "ends with restart marker, CRLF", content: ptr("panic: boom\r\n\r\n" + marker + "\r\n"), want: false},
		{name: "ends with restart marker, no newline", content: ptr("panic: boom\n" + marker), want: false},
		{name: "trace only", content: ptr("panic: boom\n"), want: true},
		{name: "trace after restart marker", content: ptr(marker + "\npanic: boom\n"), want: true},
		{name: "trace longer than tail", content: ptr(strings.Repeat("goroutine 1 [running]:\n", 20)), want: true},
		{name: "longer than tail, ends with restart marker", content: ptr(strings.Repeat("goroutine 1 [running]:\n", 20) + "\n" + marker + "\n"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "x-silver-crash.log")
			if tt.content != nil {
				writeTestFile(t, path, *tt.content)
			}

			crashed, modTime, err := previousRunCrashed(path)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if crashed != tt.want {
				t.Errorf("crashed = %v, want %v", crashed, tt.want)
			}
			if crashed {
				info, _ := os.Stat(path)
				if !modTime.Equal(info.ModTime()) {
					t.Errorf("modTime = %v, want %v", modTime, info.ModTime())
				}
			}
		})
	}
}

func ptr(s string) *string { return &s }
