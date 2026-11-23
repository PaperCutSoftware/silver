package main

import (
	"bytes"
	stdcontext "context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/papercutsoftware/silver/lib/logging"
	"github.com/papercutsoftware/silver/service/config"
	"github.com/papercutsoftware/silver/service/svcutil"
)

type crashCall struct {
	path   string
	owner  string
	logger *log.Logger
}

func TestStartCrashCapture(t *testing.T) {
	const warning = `WARNING: CrashLogFile is ignored because LogFile is "os.stdout". The two settings can't be used together.` + "\n"
	crashPath := filepath.Join(t.TempDir(), "crash.log")
	cases := []struct {
		name         string
		logFile      string
		crashLogFile string
		userName     string
		wantCall     bool
		wantLog      string
	}{
		{"set with file log", "service.log", crashPath, "svcuser", true, ""},
		{"set with empty log file and user", "", crashPath, "", true, ""},
		{"set with stdout log", "os.stdout", crashPath, "svcuser", false, warning},
		{"empty with file log", "service.log", "", "svcuser", false, ""},
		{"empty with stdout log", "os.stdout", "", "svcuser", false, ""},
		{"empty with empty log file", "", "", "svcuser", false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls []crashCall
			orig := setupCrashOutput
			setupCrashOutput = func(path, owner string, logger *log.Logger) {
				calls = append(calls, crashCall{path, owner, logger})
			}
			t.Cleanup(func() { setupCrashOutput = orig })

			var logBuf bytes.Buffer
			ctx := &context{
				conf: &config.Config{ServiceConfig: config.ServiceConfig{
					LogFile:      tc.logFile,
					CrashLogFile: tc.crashLogFile,
					UserName:     tc.userName,
				}},
				logger: log.New(&logBuf, "", 0),
			}

			startCrashCapture(ctx)

			if got := logBuf.String(); got != tc.wantLog {
				t.Errorf("logged %q, want %q", got, tc.wantLog)
			}
			if !tc.wantCall {
				if len(calls) != 0 {
					t.Errorf("setupCrashOutput called %d times, want 0", len(calls))
				}
				return
			}
			want := crashCall{tc.crashLogFile, tc.userName, ctx.logger}
			if len(calls) != 1 || calls[0] != want {
				t.Errorf("setupCrashOutput calls = %+v, want exactly [%+v]", calls, want)
			}
		})
	}
}

const (
	crashHelperEnv       = "SILVER_CRASHCAPTURE_HELPER" // "runtask" or "child"
	crashHelperDirEnv    = "SILVER_CRASHCAPTURE_DIR"
	crashChildStdoutLine = "crashcapture-child-stdout-7f3a"
	crashChildStderrLine = "crashcapture-child-stderr-9b1c"
)

// With crash output really enabled, a task's stdout and stderr still reach
// LogFile and the crash log stays empty. It runs in a subprocess because
// SetupCrashOutput redirects the process's stderr.
func TestCrashCapture_ChildOutputStaysInLogFile(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := stdcontext.WithTimeout(stdcontext.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHelperCrashCapture$")
	cmd.Env = append(os.Environ(), crashHelperEnv+"=runtask", crashHelperDirEnv+"="+dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("helper failed: %v; output:\n%s", err, out)
	}

	logData, err := os.ReadFile(filepath.Join(dir, "service.log"))
	if err != nil {
		t.Fatalf("reading LogFile: %v", err)
	}
	for _, want := range []string{
		"Crash output enabled: " + filepath.Join(dir, "crash.log"), // proves the real redirect ran
		": STDOUT|" + crashChildStdoutLine,
		": STDERR|" + crashChildStderrLine,
	} {
		if !strings.Contains(string(logData), want) {
			t.Errorf("LogFile does not contain %q; LogFile:\n%s", want, logData)
		}
	}
	if crashData, _ := os.ReadFile(filepath.Join(dir, "crash.log")); len(crashData) != 0 {
		t.Errorf("crash log = %q, want empty", crashData)
	}
}

// TestHelperCrashCapture is not a real test: it is the subprocess body for
// TestCrashCapture_ChildOutputStaysInLogFile ("runtask") and the task that
// subprocess runs ("child").
func TestHelperCrashCapture(t *testing.T) {
	switch os.Getenv(crashHelperEnv) {
	case "runtask":
		dir := os.Getenv(crashHelperDirEnv)
		logger := logging.NewFileLogger(filepath.Join(dir, "service.log"), "", "")
		logging.SetupCrashOutput(filepath.Join(dir, "crash.log"), "", logger)

		os.Setenv(crashHelperEnv, "child") // inherited by the task
		exitCode, err := svcutil.ExecuteTask(nil, svcutil.TaskConfig{
			Path:        os.Args[0],
			Args:        []string{"-test.run=^TestHelperCrashCapture$"},
			Logger:      logger,
			ErrorLogger: logger,
		})
		logging.CloseAllOpenFileLoggers()
		if err != nil || exitCode != 0 {
			fmt.Printf("ExecuteTask: exitCode=%d err=%v\n", exitCode, err)
			os.Exit(3)
		}
		os.Exit(0)
	case "child":
		fmt.Fprintln(os.Stdout, crashChildStdoutLine)
		fmt.Fprintln(os.Stderr, crashChildStderrLine)
		os.Exit(0)
	}
}
