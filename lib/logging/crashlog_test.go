package logging

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// crashTempDir returns a temporary directory removed on a best-effort basis.
// SetupCrashOutput may keep the crash log open, which would make t.TempDir's
// cleanup fail on Windows.
func crashTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "silver-crashlog-test-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

type redirectCall struct {
	called bool
	name   string
	file   *os.File
}

// stubRedirect replaces redirectStderr for the duration of the test.
func stubRedirect(t *testing.T, ret error) *redirectCall {
	t.Helper()
	rc := &redirectCall{}
	old := redirectStderr
	redirectStderr = func(f *os.File) error {
		rc.called = true
		rc.file = f
		if f != nil {
			rc.name = f.Name()
		}
		return ret
	}
	t.Cleanup(func() { redirectStderr = old })
	return rc
}

type ownerCall struct {
	called bool
	name   string
	owner  string
}

// stubOwner replaces changeOwnerOfFileFunc for the duration of the test.
func stubOwner(t *testing.T, ret error) *ownerCall {
	t.Helper()
	oc := &ownerCall{}
	old := changeOwnerOfFileFunc
	changeOwnerOfFileFunc = func(name, owner string) error {
		oc.called = true
		oc.name = name
		oc.owner = owner
		return ret
	}
	t.Cleanup(func() { changeOwnerOfFileFunc = old })
	return oc
}

// absent marks a file expected not to exist in checkCrashFiles.
const absent = "\x00absent"

// blockRotation makes renaming path.1 onto path.2 fail by placing a non-empty
// directory at path.2, alongside an existing path.1.
func blockRotation(t *testing.T, path, backup1 string) {
	t.Helper()
	if err := os.WriteFile(path+".1", []byte(backup1), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(path+".2", "sub"), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path+".2", "sub", "f"), []byte("x"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// checkCrashFiles compares path+suffix contents with want; absent means the
// file must not exist.
func checkCrashFiles(t *testing.T, path string, want map[string]string) {
	t.Helper()
	for suffix, w := range want {
		got, err := os.ReadFile(path + suffix)
		if w == absent {
			if err == nil || !os.IsNotExist(err) {
				t.Errorf("expected %s not to exist, got err %v", path+suffix, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("ReadFile %s: %v", path+suffix, err)
			continue
		}
		if string(got) != w {
			t.Errorf("%s: got %d bytes %.40q, want %d bytes %.40q", path+suffix, len(got), got, len(w), w)
		}
	}
}

func TestSetupCrashOutput(t *testing.T) {
	full := strings.Repeat("x", crashLogMaxSize)
	tests := []struct {
		name         string
		owner        string
		existing     string // content written before setup; "" means no file
		setup        func(t *testing.T, path string)
		wantFiles    map[string]string // suffix -> content after setup; nil means {"": existing}
		missingDir   bool
		ownerErr     error
		redirectErr  error
		defaultStub  bool // rely on TestMain's stub instead of stubbing here
		wantRedirect bool
		wantOwner    bool
		wantLog      []string // "<path>" is replaced with the crash log path
		notLog       []string
	}{
		{
			name:         "new file",
			wantRedirect: true,
			wantLog:      []string{"Crash output enabled: <path>"},
			notLog:       []string{"WARNING"},
		},
		{
			name:         "existing file appended",
			existing:     "previous crash output\n",
			wantRedirect: true,
			wantLog:      []string{"Crash output enabled: <path>"},
		},
		{
			name:         "sets owner",
			owner:        "svc_user",
			wantRedirect: true,
			wantOwner:    true,
			wantLog:      []string{"Crash output enabled: <path>"},
		},
		{
			name:         "owner change fails",
			owner:        "svc_user",
			ownerErr:     errors.New("no such user"),
			wantRedirect: true,
			wantOwner:    true,
			wantLog:      []string{"WARNING: Unable to set owner of crash log <path>: ", "Crash output enabled: <path>"},
		},
		{
			name:       "missing directory",
			missingDir: true,
			wantLog:    []string{"WARNING: Unable to open crash log <path>: "},
			notLog:     []string{"Crash output enabled"},
		},
		{
			name:         "redirect fails",
			redirectErr:  errors.New("dup failed"),
			wantRedirect: true,
			wantLog:      []string{"WARNING: Unable to enable crash output to <path>: "},
			notLog:       []string{"Crash output enabled"},
		},
		{
			name:        "TestMain stub refuses redirect",
			defaultStub: true,
			wantLog:     []string{"WARNING: Unable to enable crash output to <path>: ", errRedirectNotStubbed.Error()},
			notLog:      []string{"Crash output enabled"},
		},
		{
			name:         "full log rotated",
			existing:     full,
			wantFiles:    map[string]string{"": "", ".1": full, ".2": absent},
			wantRedirect: true,
			wantLog:      []string{"Crash output enabled: <path>"},
			notLog:       []string{"WARNING"},
		},
		{
			name:         "log below limit not rotated",
			existing:     full[1:],
			wantFiles:    map[string]string{"": full[1:], ".1": absent},
			wantRedirect: true,
			wantLog:      []string{"Crash output enabled: <path>"},
			notLog:       []string{"WARNING"},
		},
		{
			name:         "rotation fails",
			existing:     full,
			setup:        func(t *testing.T, path string) { blockRotation(t, path, "backup-one") },
			wantFiles:    map[string]string{"": full, ".1": "backup-one"},
			wantRedirect: true,
			wantLog:      []string{"WARNING: Unable to rotate crash log <path>: ", "Crash output enabled: <path>"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var rc *redirectCall
			if !tc.defaultStub {
				rc = stubRedirect(t, tc.redirectErr)
			}
			oc := stubOwner(t, tc.ownerErr)
			dir := crashTempDir(t)
			if tc.missingDir {
				dir = filepath.Join(dir, "does-not-exist")
			}
			path := filepath.Join(dir, "crash.log")
			if tc.existing != "" {
				if err := os.WriteFile(path, []byte(tc.existing), 0644); err != nil {
					t.Fatalf("WriteFile: %v", err)
				}
			}
			if tc.setup != nil {
				tc.setup(t, path)
			}

			var buf bytes.Buffer
			SetupCrashOutput(path, tc.owner, log.New(&buf, "", 0))
			out := buf.String()

			for _, w := range tc.wantLog {
				if w = strings.ReplaceAll(w, "<path>", path); !strings.Contains(out, w) {
					t.Errorf("expected %q in log, got %q", w, out)
				}
			}
			for _, n := range tc.notLog {
				if strings.Contains(out, n) {
					t.Errorf("expected no %q in log, got %q", n, out)
				}
			}

			if rc != nil {
				if rc.called != tc.wantRedirect {
					t.Errorf("redirectStderr called = %v, want %v", rc.called, tc.wantRedirect)
				}
				if rc.called && (rc.file == nil || filepath.Clean(rc.name) != filepath.Clean(path)) {
					t.Errorf("expected redirectStderr to receive open file %q, got %q", path, rc.name)
				}
			}

			if oc.called != tc.wantOwner {
				t.Errorf("owner change called = %v, want %v", oc.called, tc.wantOwner)
			}
			if oc.called && (filepath.Clean(oc.name) != filepath.Clean(path) || oc.owner != tc.owner) {
				t.Errorf("expected owner change (%q, %q), got (%q, %q)", path, tc.owner, oc.name, oc.owner)
			}

			if tc.missingDir {
				if _, err := os.Stat(path); err == nil {
					t.Errorf("expected crash log %s not to exist", path)
				}
				return
			}
			wantFiles := tc.wantFiles
			if wantFiles == nil {
				wantFiles = map[string]string{"": tc.existing}
			}
			checkCrashFiles(t, path, wantFiles)
			if tc.existing == "" && runtime.GOOS != "windows" {
				fi, err := os.Stat(path)
				if err != nil {
					t.Fatalf("Stat: %v", err)
				}
				if perm := fi.Mode().Perm(); perm&^0644 != 0 || perm&0600 != 0600 {
					t.Errorf("expected crash log mode 0644 (subject to umask), got %#o", perm)
				}
			}
		})
	}
}

func TestRotateCrashLog(t *testing.T) {
	if crashLogMaxSize != 1<<20 || crashLogMaxBackups != 2 {
		t.Errorf("crashLogMaxSize = %d, crashLogMaxBackups = %d; want %d, 2", crashLogMaxSize, crashLogMaxBackups, 1<<20)
	}

	const maxSize = 10
	tests := []struct {
		name        string
		files       map[string]string // suffix -> content written before rotation
		block       bool              // blockRotation with path.1 = "backup-one"
		wantRotated bool
		wantErr     bool
		want        map[string]string
	}{
		{
			name: "missing file",
			want: map[string]string{"": absent, ".1": absent, ".2": absent},
		},
		{
			name:  "empty file",
			files: map[string]string{"": ""},
			want:  map[string]string{"": "", ".1": absent},
		},
		{
			name:  "smaller than limit",
			files: map[string]string{"": "123456789", ".1": "backup-one"},
			want:  map[string]string{"": "123456789", ".1": "backup-one", ".2": absent},
		},
		{
			name:        "exactly at limit",
			files:       map[string]string{"": "0123456789"},
			wantRotated: true,
			want:        map[string]string{"": absent, ".1": "0123456789", ".2": absent},
		},
		{
			name:        "above limit",
			files:       map[string]string{"": "current-log-full"},
			wantRotated: true,
			want:        map[string]string{"": absent, ".1": "current-log-full", ".2": absent},
		},
		{
			name:        "backups shift and oldest dropped",
			files:       map[string]string{"": "current-log-full", ".1": "backup-one", ".2": "backup-two"},
			wantRotated: true,
			want:        map[string]string{"": absent, ".1": "current-log-full", ".2": "backup-one", ".3": absent},
		},
		{
			name:        "existing .1 shifts to missing .2",
			files:       map[string]string{"": "current-log-full", ".1": "backup-one"},
			wantRotated: true,
			want:        map[string]string{"": absent, ".1": "current-log-full", ".2": "backup-one"},
		},
		{
			name:        "missing .1 skipped",
			files:       map[string]string{"": "current-log-full", ".2": "backup-two"},
			wantRotated: true,
			want:        map[string]string{"": absent, ".1": "current-log-full", ".2": "backup-two"},
		},
		{
			name:    "rename fails stops rotation",
			files:   map[string]string{"": "current-log-full"},
			block:   true,
			wantErr: true,
			want:    map[string]string{"": "current-log-full", ".1": "backup-one"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "crash.log")
			for suffix, content := range tc.files {
				if err := os.WriteFile(path+suffix, []byte(content), 0644); err != nil {
					t.Fatalf("WriteFile: %v", err)
				}
			}
			if tc.block {
				blockRotation(t, path, "backup-one")
			}

			rotated, err := rotateCrashLog(path, maxSize, crashLogMaxBackups)

			if tc.wantErr {
				var linkErr *os.LinkError
				if !errors.As(err, &linkErr) || os.IsNotExist(err) {
					t.Errorf("expected rename error, got %v", err)
				}
				if fi, statErr := os.Stat(path + ".2"); statErr != nil || !fi.IsDir() {
					t.Errorf("expected %s.2 directory left in place, got %v", path, statErr)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if rotated != tc.wantRotated {
					t.Errorf("rotated = %v, want %v", rotated, tc.wantRotated)
				}
			}
			checkCrashFiles(t, path, tc.want)
		})
	}
}

func TestSetupCrashOutput_FlushesFileLoggers(t *testing.T) {
	stubRedirect(t, nil)
	dir := crashTempDir(t)
	path := filepath.Join(dir, "crash.log")
	logPath := filepath.Join(dir, "service.log")

	logger := NewFileLogger(logPath, "", "")
	t.Cleanup(CloseAllOpenFileLoggers)

	SetupCrashOutput(path, "", logger)

	// Read immediately, without closing the logger or waiting for a timed flush.
	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(got), "Crash output enabled: "+path) {
		t.Errorf("expected enabled message flushed to %s, got %q", logPath, string(got))
	}
}

func TestFlushAllOpenFileLoggers(t *testing.T) {
	dir := crashTempDir(t)
	paths := []string{
		filepath.Join(dir, "a.log"),
		filepath.Join(dir, "b.log"),
	}
	loggers := []*log.Logger{
		NewFileLogger(paths[0], "", ""),
		NewFileLoggerWithMaxSize(paths[1], "", 1024*1024, 2, ""),
	}
	t.Cleanup(CloseAllOpenFileLoggers)

	// The second round checks the loggers are still usable after a flush.
	for _, round := range []string{"first", "second"} {
		for i, l := range loggers {
			l.Printf("%s-message-%d", round, i)
		}
		FlushAllOpenFileLoggers()

		for i, p := range paths {
			got, err := os.ReadFile(p)
			if err != nil {
				t.Fatalf("ReadFile %s: %v", p, err)
			}
			if want := fmt.Sprintf("%s-message-%d", round, i); !strings.Contains(string(got), want) {
				t.Errorf("expected %q flushed to %s, got %q", want, p, string(got))
			}
		}
	}
}
