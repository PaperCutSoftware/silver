//go:build !windows
// +build !windows

package logging

import (
	"strings"
	"syscall"
	"testing"
)

func init() {
	// Close fd 2 so the crash log is itself opened as fd 2.
	helperPreSetup["closed-stderr"] = func() { syscall.Close(2) }
}

func TestCrashOutput_ClosedStderrBeforeSetup(t *testing.T) {
	r := runCrashHelper(t, "closed-stderr")
	requireNonZeroExit(t, r)

	if !strings.Contains(r.crashLog, "panic: silver-crash-test") {
		t.Errorf("expected panic message in crash log, got %q; stdout=%q", r.crashLog, r.stdout)
	}
	if !strings.Contains(r.stdout, "Crash output enabled: ") {
		t.Errorf("expected enabled message on child stdout, got %q", r.stdout)
	}
	if strings.Contains(r.stdout, "Unable to enable crash output") {
		t.Errorf("expected no redirect warning, got %q", r.stdout)
	}
}
