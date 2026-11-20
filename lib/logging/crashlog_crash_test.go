// SILVER - Service Wrapper
//
// Copyright (c) 2014-2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.

package logging

// Re-execs this test binary (-test.run=^TestHelperCrash$) to crash a child
// process for real and check the resulting crash log.

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
)

const (
	crashHelperModeEnv = "SILVER_CRASH_HELPER"
	crashHelperPathEnv = "SILVER_CRASH_PATH"
	crashTestPanicMsg  = "silver-crash-test"
)

// Matches stack headers like "goroutine 7 [running]:" while ignoring
// "created by ... in goroutine 1" lines.
var goroutineHeader = regexp.MustCompile(`(?m)^goroutine \d+ \[`)

// TestHelperCrash is a subprocess entry point spawned by runCrashHelper.
func TestHelperCrash(t *testing.T) {
	mode := os.Getenv(crashHelperModeEnv)
	if mode == "" {
		t.Skip("helper process for crash tests")
	}

	SetupCrashOutput(os.Getenv(crashHelperPathEnv), "", log.New(os.Stderr, "", 0))

	switch mode {
	case "panic":
		panic(crashTestPanicMsg)
	case "fatal":
		var m sync.Mutex
		m.Unlock() // triggers "fatal error: sync: unlock of unlocked mutex"
	}
}

func runCrashHelper(t *testing.T, mode, path string) (crashLog string, output string, err error) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperCrash$")
	// Force GOTRACEBACK=single so we know SetupCrashOutput has set it properly
	cmd.Env = append(os.Environ(), "GOTRACEBACK=single", crashHelperModeEnv+"="+mode, crashHelperPathEnv+"="+path)
	out, err := cmd.CombinedOutput()

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("reading crash log: %v\nchild output:\n%s", readErr, out)
	}
	return string(data), string(out), err
}

func TestCrashOutput_PanicIsWrittenToCrashLog(t *testing.T) {
	crashLog, output, err := runCrashHelper(t, "panic", filepath.Join(t.TempDir(), "x-silver-crash.log"))

	if err == nil {
		t.Fatal("expected child process to exit with an error")
	}
	if !strings.Contains(crashLog, "panic: "+crashTestPanicMsg) {
		t.Errorf("crash log missing panic message:\n%s", crashLog)
	}
	if n := len(goroutineHeader.FindAllString(crashLog, -1)); n < 2 {
		t.Errorf("expected all-goroutine traceback (>= 2 goroutines), got %d:\n%s", n, crashLog)
	}
	// Stderr should still get the panic alongside the crash log.
	if !strings.Contains(output, "panic: "+crashTestPanicMsg) {
		t.Errorf("expected panic on stderr too, got:\n%s", output)
	}
}

func TestCrashOutput_FatalErrorIsWrittenToCrashLog(t *testing.T) {
	crashLog, output, err := runCrashHelper(t, "fatal", filepath.Join(t.TempDir(), "x-silver-crash.log"))

	if err == nil {
		t.Fatal("expected child process to exit with an error")
	}
	if !strings.Contains(crashLog, "sync.(*Mutex).Unlock") {
		t.Errorf("crash log missing faulting stack frame:\n%s", crashLog)
	}
	if !strings.Contains(output, "fatal error: sync: unlock of unlocked mutex") {
		t.Errorf("expected fatal error message on stderr, got:\n%s", output)
	}
}

func TestCrashOutput_RestartAfterCrashWarnsOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x-silver-crash.log")
	const warning = "Silver crashed during a previous run"

	if _, output, err := runCrashHelper(t, "panic", path); err == nil {
		t.Fatalf("expected panic run to fail:\n%s", output)
	}

	afterFirst, output, err := runCrashHelper(t, "clean", path)
	if err != nil {
		t.Fatalf("clean run failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, warning) {
		t.Errorf("expected crash warning on first restart, got:\n%s", output)
	}
	if !strings.Contains(afterFirst, "panic: "+crashTestPanicMsg) {
		t.Errorf("crash trace lost:\n%s", afterFirst)
	}
	assertEndsWithRestartMarker(t, path)

	afterSecond, output, err := runCrashHelper(t, "clean", path)
	if err != nil {
		t.Fatalf("second clean run failed: %v\n%s", err, output)
	}
	if strings.Contains(output, warning) {
		t.Errorf("expected no crash warning on a clean restart, got:\n%s", output)
	}
	if afterSecond != afterFirst {
		t.Error("crash log changed on a clean restart")
	}
}
