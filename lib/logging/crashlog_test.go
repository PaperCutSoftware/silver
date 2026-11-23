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

func TestSetupCrashOutput(t *testing.T) {
	tests := []struct {
		name         string
		owner        string
		existing     string // content written before setup; "" means no file
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
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			if string(got) != tc.existing {
				t.Errorf("expected crash log content %q, got %q", tc.existing, string(got))
			}
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
