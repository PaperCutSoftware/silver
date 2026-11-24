package logging

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
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

// crashMTime is a fixed modification time given to crash log fixtures.
var crashMTime = time.Date(2024, time.March, 4, 5, 6, 7, 0, time.Local)

// setCrashMTime sets the modification time of name to crashMTime.
func setCrashMTime(t *testing.T, name string) {
	t.Helper()
	if err := os.Chtimes(name, crashMTime, crashMTime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}
}

// skipIfPermsIgnored skips tests that rely on file permission bits.
func skipIfPermsIgnored(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("file permission bits are not enforced here")
	}
}

// testVersion is the build version passed to SetupCrashOutput by these tests.
const testVersion = "1.2.3-test"

var startHeaderRe = regexp.MustCompile(`^\n--- Silver started at ([^\n]+) \(pid (\d+), version ([^\n]*)\) ---\n$`)

// checkHeaderAppended checks got is prefix followed by exactly one start header
// whose time lies within [after, before], with the given pid and version.
// A pid of 0 accepts any pid.
func checkHeaderAppended(t *testing.T, name, got, prefix string, after, before time.Time, pid int, version string) {
	t.Helper()
	if !strings.HasPrefix(got, prefix) {
		t.Errorf("%s: expected previous content (%d bytes) kept, got %d bytes %.60q", name, len(prefix), len(got), got)
		return
	}
	rest := got[len(prefix):]
	m := startHeaderRe.FindStringSubmatch(rest)
	if m == nil {
		t.Errorf("%s: expected exactly one start header appended, got %q", name, rest)
		return
	}
	ts, err := time.Parse(time.RFC3339, m[1])
	if err != nil {
		t.Errorf("%s: header time %q is not RFC3339: %v", name, m[1], err)
		return
	}
	if ts.Before(after.Truncate(time.Second)) || ts.After(before) {
		t.Errorf("%s: header time %v not within [%v, %v]", name, ts, after, before)
	}
	if pid != 0 && m[2] != strconv.Itoa(pid) {
		t.Errorf("%s: header pid %s, want %d", name, m[2], pid)
	}
	if m[3] != version {
		t.Errorf("%s: header version %q, want %q", name, m[3], version)
	}
}

func TestSetupCrashOutput(t *testing.T) {
	full := strings.Repeat("x", crashLogMaxSize)
	crash := "panic: boom\n\ngoroutine 1 [running]:\nmain.main()\n"
	fullCrash := "panic: boom\n" + strings.Repeat("x", crashLogMaxSize-len("panic: boom\n")-1) + "\n"
	setMTime := func(t *testing.T, path string) { setCrashMTime(t, path) }
	const enabled = "Crash output enabled: <path>"
	tests := []struct {
		name         string
		owner        string
		existing     string // content written before setup; "" means no file
		setup        func(t *testing.T, path string)
		wantFiles    map[string]string // suffix -> content; for "" the content before the start header. nil means {"": existing}
		missingDir   bool
		ownerErr     error
		redirectErr  error
		defaultStub  bool // rely on TestMain's stub instead of stubbing here
		wantRedirect bool
		wantOwner    bool
		wantLog      []string // in order; "<path>" is replaced with the crash log path, "<mtime>" with crashMTime in RFC3339
		notLog       []string
	}{
		{
			name:         "new file",
			wantRedirect: true,
			wantLog:      []string{enabled},
			notLog:       []string{"WARNING", "INFO"},
		},
		{
			name:         "existing file appended",
			existing:     "previous crash output\n",
			wantRedirect: true,
			wantLog:      []string{enabled},
		},
		{
			name:         "sets owner",
			owner:        "svc_user",
			wantRedirect: true,
			wantOwner:    true,
			wantLog:      []string{enabled},
		},
		{
			name:         "owner change fails",
			owner:        "svc_user",
			ownerErr:     errors.New("no such user"),
			wantRedirect: true,
			wantOwner:    true,
			wantLog:      []string{"WARNING: Unable to set owner of crash log <path>: ", enabled},
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
			wantLog:      []string{enabled},
			notLog:       []string{"WARNING", "INFO"},
		},
		{
			name:         "log below limit not rotated",
			existing:     full[1:],
			wantFiles:    map[string]string{"": full[1:], ".1": absent},
			wantRedirect: true,
			wantLog:      []string{enabled},
			notLog:       []string{"WARNING", "INFO"},
		},
		{
			name:         "rotation fails",
			existing:     full,
			setup:        func(t *testing.T, path string) { blockRotation(t, path, "backup-one") },
			wantFiles:    map[string]string{"": full, ".1": "backup-one"},
			wantRedirect: true,
			wantLog:      []string{"WARNING: Unable to rotate crash log <path>: ", enabled},
		},
		{
			name:         "previous crash",
			existing:     crash,
			setup:        setMTime,
			wantRedirect: true,
			wantLog:      []string{"INFO: Silver terminated early during a previous run. See <path> (last written <mtime>).", enabled},
			notLog:       []string{"WARNING"},
		},
		{
			name:         "start header after crash, then stray stderr",
			existing:     crash + "\n--- Silver started at 2024-01-02T03:04:05Z (pid 42) ---\nERROR: Invalid config\n",
			wantRedirect: true,
			wantLog:      []string{enabled},
			notLog:       []string{"WARNING", "INFO", "terminated early"},
		},
		{
			name:         "previous crash in full log",
			existing:     fullCrash,
			setup:        setMTime,
			wantFiles:    map[string]string{"": "", ".1": fullCrash, ".2": absent},
			wantRedirect: true,
			wantLog:      []string{"INFO: Silver terminated early during a previous run. See <path>.1 (last written <mtime>).", enabled},
			notLog:       []string{"See <path> (", "WARNING"},
		},
		{
			name:     "crash log unreadable",
			existing: crash,
			setup: func(t *testing.T, path string) {
				skipIfPermsIgnored(t)
				if err := os.Chmod(path, 0200); err != nil {
					t.Fatalf("Chmod: %v", err)
				}
			},
			wantRedirect: true,
			wantLog:      []string{"WARNING: Unable to read crash log <path>: ", enabled},
			notLog:       []string{"terminated early"},
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
			start := time.Now()
			SetupCrashOutput(path, tc.owner, testVersion, log.New(&buf, "", 0))
			end := time.Now()
			out := buf.String()

			expand := strings.NewReplacer("<path>", path, "<mtime>", crashMTime.Format(time.RFC3339))
			pos := 0
			for _, w := range tc.wantLog {
				w = expand.Replace(w)
				i := strings.Index(out[pos:], w)
				if i < 0 {
					t.Errorf("expected %q in log in order, got %q", w, out)
					continue
				}
				pos += i + len(w)
			}
			for _, n := range append(tc.notLog, "Unable to write start marker") {
				if n = expand.Replace(n); strings.Contains(out, n) {
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
			if tc.existing == "" && runtime.GOOS != "windows" {
				fi, err := os.Stat(path)
				if err != nil {
					t.Fatalf("Stat: %v", err)
				}
				if perm := fi.Mode().Perm(); perm&^0644 != 0 || perm&0600 != 0600 {
					t.Errorf("expected crash log mode 0644 (subject to umask), got %#o", perm)
				}
			}
			os.Chmod(path, 0644) // make an unreadable fixture readable for the checks below

			wantFiles := map[string]string{"": tc.existing}
			if tc.wantFiles != nil {
				wantFiles = map[string]string{}
				for k, v := range tc.wantFiles {
					wantFiles[k] = v
				}
			}
			prefix := wantFiles[""]
			delete(wantFiles, "")
			if got, err := os.ReadFile(path); err != nil {
				t.Errorf("ReadFile %s: %v", path, err)
			} else {
				checkHeaderAppended(t, path, string(got), prefix, start, end, os.Getpid(), testVersion)
			}
			checkCrashFiles(t, path, wantFiles)
		})
	}
}

func TestPreviousRunCrashed(t *testing.T) {
	fixed := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)
	if got, want := startMarker(fixed, 1234, "1.2.3"), "\n--- Silver started at 2024-01-02T03:04:05Z (pid 1234, version 1.2.3) ---\n"; got != want {
		t.Errorf("startMarker = %q, want %q", got, want)
	}
	marker := startMarker(fixed, 1234, "1.2.3")
	const header = "--- Silver started at 2024-01-02T03:04:05Z (pid 1) ---"

	const win = crashLogMaxSize
	// The window (last win bytes) starts at the "p" of "panic: foo", in the
	// middle of the line "xyz panic: foo".
	tail := "panic: foo\n" + strings.Repeat("a", win-len("panic: foo\n")-1) + "\n"
	cutMidLine := "header\nxyz " + tail
	// A traceback longer than the window: its panic line falls outside.
	longTrace := marker + "panic: boom\n\n" + strings.Repeat("\tmain.frame()\n", win/len("\tmain.frame()\n")+10) +
		"goroutine 7 [running]:\n\tmain.g()\n"
	// A header outside the window, then a crash inside it.
	oldMarkerThenCrash := marker + strings.Repeat("b", win) + "\npanic: late\n"

	tests := []struct {
		name    string
		content string
		noFile  bool
		dir     bool // a directory at path: a read error other than not-exist
		want    bool
		wantErr bool
	}{
		{name: "missing file", noFile: true},
		{name: "empty file", content: ""},
		{name: "header only", content: marker},
		{name: "stray stderr only", content: "ERROR: Invalid config\nsomething else\n"},
		{name: "crash with no header", content: "panic: boom\n\ngoroutine 1 [running]:\nmain.main()\n", want: true},
		{name: "crash followed by header", content: "panic: boom\n\ngoroutine 1 [running]:\n" + marker},
		{name: "header with unvalidated contents", content: "panic: boom\n--- Silver started at not-a-time ---\n"},
		{name: "header followed by stray stderr", content: "panic: boom\n" + marker + "ERROR: Invalid config\n"},
		{name: "header followed by crash", content: marker + "panic: boom\n", want: true},
		{name: "crash after the last of several headers", content: marker + "panic: a\n" + marker + "panic: b\n" + marker + "fatal error: c\n", want: true},
		{name: "crash before the last of several headers", content: marker + "panic: a\n" + marker + "ERROR: x\n"},
		{name: "fatal error line", content: "fatal error: sync: unlock of unlocked mutex\n", want: true},
		{name: "goroutine header only", content: "goroutine 1 [running]:\n", want: true},
		{name: "multi-digit goroutine header", content: "goroutine 1234 [chan receive]:\n", want: true},
		{name: "goroutine without number", content: "goroutine x [running]:\ngoroutine [running]:\n"},
		{name: "crash text not at line start", content: "log: panic: boom\n  fatal error: x\n goroutine 1 [running]:\n"},
		{name: "panic without colon space", content: "panicking\npanic:boom\n"},
		{name: "indented header does not count", content: "panic: boom\n " + header + "\n", want: true},
		{name: "old restart marker does not count", content: "panic: boom\n--- Silver restarted at 2024-01-02T03:04:05Z ---\n", want: true},
		{name: "CRLF crash", content: header + "\r\npanic: boom\r\n\r\ngoroutine 1 [running]:\r\n", want: true},
		{name: "CRLF crash followed by header", content: "panic: boom\r\n" + header + "\r\n"},
		{name: "crash without trailing newline", content: marker + "panic: boom", want: true},
		{name: "header without trailing newline", content: "panic: boom\n" + header},
		{name: "window cut mid-line before panic", content: cutMidLine},
		{name: "window exactly whole file starting with panic", content: tail, want: true},
		{name: "traceback longer than window", content: longTrace, want: true},
		{name: "header outside window then crash", content: oldMarkerThenCrash, want: true},
		{name: "read error", dir: true, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "crash.log")
			switch {
			case tc.dir:
				if err := os.Mkdir(path, 0755); err != nil {
					t.Fatalf("Mkdir: %v", err)
				}
			case !tc.noFile:
				if err := os.WriteFile(path, []byte(tc.content), 0644); err != nil {
					t.Fatalf("WriteFile: %v", err)
				}
				setCrashMTime(t, path)
			}

			crashed, crashTime, err := previousRunCrashed(path)

			if tc.wantErr {
				if err == nil || os.IsNotExist(err) {
					t.Errorf("expected a read error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if crashed != tc.want {
				t.Errorf("crashed = %v, want %v", crashed, tc.want)
			}
			if crashed && !crashTime.Equal(crashMTime) {
				t.Errorf("crashTime = %v, want file mod time %v", crashTime, crashMTime)
			}
			if (tc.noFile || tc.content == "") && !crashTime.IsZero() {
				t.Errorf("crashTime = %v, want zero", crashTime)
			}
			if !tc.noFile {
				checkCrashFiles(t, path, map[string]string{"": tc.content})
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

	SetupCrashOutput(path, "", testVersion, logger)

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
