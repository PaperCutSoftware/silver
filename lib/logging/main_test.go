package logging

import (
	"errors"
	"os"
	"testing"
)

// crashHelperEnv names the environment variable that switches the test binary
// into crash-helper mode (see TestHelperCrash).
const crashHelperEnv = "SILVER_CRASH_HELPER"

// errRedirectNotStubbed is returned by the default test stub for redirectStderr.
var errRedirectNotStubbed = errors.New("redirectStderr not stubbed in test")

func TestMain(m *testing.M) {
	// Never let an in-process test redirect the go test process's own stderr.
	// Crash-helper subprocesses need the real redirect.
	if os.Getenv(crashHelperEnv) == "" {
		redirectStderr = func(f *os.File) error { return errRedirectNotStubbed }
	}
	os.Exit(m.Run())
}
