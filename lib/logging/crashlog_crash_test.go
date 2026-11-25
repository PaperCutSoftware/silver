package logging

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// crashLogPathEnv carries the crash log path into the helper subprocess.
const crashLogPathEnv = "SILVER_CRASH_LOG"

// helperPreSetup maps a helper mode to an action run before SetupCrashOutput.
// Platform-specific test files register extra modes in init().
var helperPreSetup = map[string]func(){}

// TestHelperCrash is not a real test: it is the body of the crash subprocess.
func TestHelperCrash(t *testing.T) {
	mode := os.Getenv(crashHelperEnv)
	if mode == "" {
		return
	}
	path := os.Getenv(crashLogPathEnv)

	if pre, ok := helperPreSetup[mode]; ok {
		pre()
	}

	// Log to stdout: after the redirect, os.Stderr writes land in the crash log on Unix.
	logger := log.New(os.Stdout, "", 0)
	SetupCrashOutput(path, "", testVersion, logger)

	switch mode {
	case "panic", "closed-stderr":
		panic("silver-crash-test")
	case "mutex":
		var m sync.Mutex
		m.Unlock()
	case "clean":
		return
	default:
		logger.Printf("unknown helper mode %q", mode)
		os.Exit(3)
	}
}

type crashResult struct {
	crashLog string
	stdout   string
	stderr   string
	err      error
}

func runCrashHelper(t *testing.T, mode string) crashResult {
	t.Helper()
	return runCrashHelperAt(t, mode, filepath.Join(crashTempDir(t), "crash.log"))
}

// runCrashHelperAt runs the crash helper in the given mode against path.
func runCrashHelperAt(t *testing.T, mode, path string) crashResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHelperCrash$")
	env := make([]string, 0, len(os.Environ())+3)
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "GOTRACEBACK=") ||
			strings.HasPrefix(kv, crashHelperEnv+"=") ||
			strings.HasPrefix(kv, crashLogPathEnv+"=") {
			continue
		}
		env = append(env, kv)
	}
	env = append(env,
		"GOTRACEBACK=single",
		crashHelperEnv+"="+mode,
		crashLogPathEnv+"="+path,
	)
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("crash helper %q timed out; stdout=%q stderr=%q", mode, stdout.String(), stderr.String())
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading crash log %s: %v; stdout=%q stderr=%q", path, err, stdout.String(), stderr.String())
	}
	return crashResult{crashLog: string(data), stdout: stdout.String(), stderr: stderr.String(), err: runErr}
}

func requireNonZeroExit(t *testing.T, r crashResult) {
	t.Helper()
	var ee *exec.ExitError
	if !errors.As(r.err, &ee) {
		t.Fatalf("expected child to exit non-zero, got err=%v; stdout=%q", r.err, r.stdout)
	}
	if ee.ExitCode() == 0 {
		t.Fatalf("expected non-zero exit code, got 0")
	}
}

var goroutineHeader = regexp.MustCompile(`(?m)^goroutine \d+ \[`)

func TestCrashOutput_PanicCapturedWithAllGoroutines(t *testing.T) {
	r := runCrashHelper(t, "panic")
	requireNonZeroExit(t, r)

	if !strings.Contains(r.crashLog, "panic: silver-crash-test") {
		t.Errorf("expected panic message in crash log, got %q; stdout=%q stderr=%q", r.crashLog, r.stdout, r.stderr)
	}
	if n := len(goroutineHeader.FindAllString(r.crashLog, -1)); n < 2 {
		t.Errorf("expected at least 2 goroutine headers (traceback=all), got %d in %q", n, r.crashLog)
	}
	if !strings.Contains(r.stdout, "Crash output enabled: ") {
		t.Errorf("expected enabled message on child stdout, got %q", r.stdout)
	}
}

func TestCrashOutput_FatalErrorCaptured(t *testing.T) {
	r := runCrashHelper(t, "mutex")
	requireNonZeroExit(t, r)

	for _, want := range []string{"fatal error: sync: unlock of unlocked mutex", "sync.(*Mutex).Unlock"} {
		if !strings.Contains(r.crashLog, want) {
			t.Errorf("expected %q in crash log, got %q; stdout=%q stderr=%q", want, r.crashLog, r.stdout, r.stderr)
		}
	}
}

func TestCrashOutput_CleanExitWritesOnlyStartHeader(t *testing.T) {
	start := time.Now()
	r := runCrashHelper(t, "clean")
	end := time.Now()
	if r.err != nil {
		t.Fatalf("expected clean exit, got %v; stdout=%q stderr=%q", r.err, r.stdout, r.stderr)
	}
	checkHeaderAppended(t, "crash log", r.crashLog, "", start, end, 0, testVersion)
	if !strings.Contains(r.stdout, "Crash output enabled: ") {
		t.Errorf("expected enabled message on child stdout, got %q", r.stdout)
	}
}

// Three runs against one crash log: a panic, then two clean starts. Only the
// first clean start reports the early termination, and each start appends
// exactly one start header.
func TestCrashOutput_EarlyTerminationReportedOnce(t *testing.T) {
	path := filepath.Join(crashTempDir(t), "crash.log")
	const info = "INFO: Silver terminated early during a previous run."

	start := time.Now()
	crashed := runCrashHelperAt(t, "panic", path)
	end := time.Now()
	requireNonZeroExit(t, crashed)
	i := strings.Index(crashed.crashLog, "panic: silver-crash-test")
	if i < 0 {
		t.Fatalf("expected panic message in crash log, got %q; stdout=%q", crashed.crashLog, crashed.stdout)
	}
	if nl := strings.Index(crashed.crashLog[1:], "\n"); nl < 0 || nl+2 > i {
		t.Errorf("expected a start header before the panic, got %.200q", crashed.crashLog)
	} else {
		checkHeaderAppended(t, "panic run header", crashed.crashLog[:nl+2], "", start, end, 0, testVersion)
	}
	if strings.Contains(crashed.stdout, info) {
		t.Errorf("expected no early-termination message on first run, got %q", crashed.stdout)
	}

	prev := crashed.crashLog
	for _, run := range []struct {
		name     string
		wantInfo bool
	}{
		{"first clean run", true},
		{"second clean run", false},
	} {
		start := time.Now()
		r := runCrashHelperAt(t, "clean", path)
		end := time.Now()
		if r.err != nil {
			t.Fatalf("%s: expected clean exit, got %v; stdout=%q", run.name, r.err, r.stdout)
		}
		want := info + " See " + path + " (last written "
		if got := strings.Contains(r.stdout, want); got != run.wantInfo {
			t.Errorf("%s: %q on stdout = %v, want %v; stdout=%q", run.name, want, got, run.wantInfo, r.stdout)
		}
		if !run.wantInfo && strings.Contains(r.stdout, "INFO") {
			t.Errorf("%s: expected no INFO line, got %q", run.name, r.stdout)
		}
		checkHeaderAppended(t, run.name, r.crashLog, prev, start, end, 0, testVersion)
		prev = r.crashLog
	}
}
