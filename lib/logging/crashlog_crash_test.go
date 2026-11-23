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
	SetupCrashOutput(path, "", logger)

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
	path := filepath.Join(crashTempDir(t), "crash.log")

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

func TestCrashOutput_CleanExitLeavesLogEmpty(t *testing.T) {
	r := runCrashHelper(t, "clean")
	if r.err != nil {
		t.Fatalf("expected clean exit, got %v; stdout=%q stderr=%q", r.err, r.stdout, r.stderr)
	}
	if r.crashLog != "" {
		t.Errorf("expected empty crash log, got %q", r.crashLog)
	}
	if !strings.Contains(r.stdout, "Crash output enabled: ") {
		t.Errorf("expected enabled message on child stdout, got %q", r.stdout)
	}
}
