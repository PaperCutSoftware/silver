package main

import (
	"bytes"
	"log"
	"path/filepath"
	"testing"

	"github.com/papercutsoftware/silver/service/config"
)

type crashCall struct {
	path   string
	owner  string
	logger *log.Logger
}

// stubSetupCrashOutput replaces setupCrashOutput with a recorder for the
// duration of the test.
func stubSetupCrashOutput(t *testing.T) *[]crashCall {
	t.Helper()
	calls := &[]crashCall{}
	orig := setupCrashOutput
	setupCrashOutput = func(path, owner string, logger *log.Logger) {
		*calls = append(*calls, crashCall{path, owner, logger})
	}
	t.Cleanup(func() { setupCrashOutput = orig })
	return calls
}

func newCrashTestContext(logFile, crashLogFile, userName string) *context {
	return &context{
		conf: &config.Config{
			ServiceConfig: config.ServiceConfig{
				LogFile:      logFile,
				CrashLogFile: crashLogFile,
				UserName:     userName,
			},
		},
		logger: log.New(&bytes.Buffer{}, "", 0),
	}
}

func TestStartCrashCapture(t *testing.T) {
	crashPath := filepath.Join(t.TempDir(), "crash.log")
	cases := []struct {
		name         string
		logFile      string
		crashLogFile string
		userName     string
	}{
		{"set with file log", "service.log", crashPath, "svcuser"},
		{"set with stdout log", "os.stdout", crashPath, "svcuser"},
		{"set with empty log file and user", "", crashPath, ""},
		{"empty with file log", "service.log", "", "svcuser"},
		{"empty with stdout log", "os.stdout", "", "svcuser"},
		{"empty with empty log file", "", "", "svcuser"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := stubSetupCrashOutput(t)
			ctx := newCrashTestContext(tc.logFile, tc.crashLogFile, tc.userName)

			startCrashCapture(ctx)

			if tc.crashLogFile == "" {
				if len(*calls) != 0 {
					t.Errorf("setupCrashOutput called %d times, want 0", len(*calls))
				}
				return
			}
			if len(*calls) != 1 {
				t.Fatalf("setupCrashOutput called %d times, want 1", len(*calls))
			}
			want := crashCall{tc.crashLogFile, tc.userName, ctx.logger}
			if got := (*calls)[0]; got != want {
				t.Errorf("setupCrashOutput(%q, %q, %p), want (%q, %q, %p)", got.path, got.owner, got.logger, want.path, want.owner, want.logger)
			}
		})
	}
}
