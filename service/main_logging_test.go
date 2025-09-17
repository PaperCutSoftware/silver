package main

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/papercutsoftware/silver/lib/logging"
	"github.com/papercutsoftware/silver/service/config"
)

func TestLogging_SampleConfigWithoutMicroseconds(t *testing.T) {
	cfg := loadTestConfig(t, "logging-no-microseconds.conf")
	logPath := filepath.Join(t.TempDir(), "no-micro.log")
	cfg.ServiceConfig.LogFile = logPath

	logger := logging.NewFileLogger(cfg.ServiceConfig.LogFile, cfg.ServiceConfig.UserName)
	t.Cleanup(logging.CloseAllOpenFileLoggers)

	ctx := &context{
		conf:        cfg,
		logger:      logger,
		errorLogger: logger,
	}
	// this is usually done in main(), but we need to do it here for the test
	applyMicrosecondsLoggingConfiguration(ctx)

	testmessage := "LoggingWithoutMicroseconds"
	msg := "%s %t"
	logger.Printf(msg, testmessage, ctx.conf.ServiceConfig.LogFileTimestampMicroseconds)
	logging.CloseAllOpenFileLoggers()

	data, err := os.ReadFile(cfg.ServiceConfig.LogFile)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	line := strings.SplitN(string(data), "\n", 2)[0]

	reDate := regexp.MustCompile(`^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2} `)
	if !reDate.MatchString(line) {
		t.Fatalf("expected timestamp without microseconds, got %q", line)
	}

	reMicro := regexp.MustCompile(`^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\.\d{6} `)
	if reMicro.MatchString(line) {
		t.Fatalf("did not expect microsecond precision, got %q", line)
	}

	if !strings.Contains(line, testmessage) {
		t.Fatalf("expected log line to contain %q, got %q", testmessage, line)
	}
}

func TestLogging_SampleConfigWithMicroseconds(t *testing.T) {
	cfg := loadTestConfig(t, "logging-with-microseconds.conf")
	logPath := filepath.Join(t.TempDir(), "with-micro.log")
	cfg.ServiceConfig.LogFile = logPath

	logger := logging.NewFileLogger(cfg.ServiceConfig.LogFile, cfg.ServiceConfig.UserName)
	t.Cleanup(logging.CloseAllOpenFileLoggers)

	ctx := &context{
		conf:        cfg,
		logger:      logger,
		errorLogger: logger,
	}
	// this is usually done in main(), but we need to do it here for the test
	applyMicrosecondsLoggingConfiguration(ctx)

	testmessage := "LoggingWithMicroseconds"
	msg := "%s %t"
	logger.Printf(msg, testmessage, ctx.conf.ServiceConfig.LogFileTimestampMicroseconds)
	logging.CloseAllOpenFileLoggers()

	data, err := os.ReadFile(cfg.ServiceConfig.LogFile)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	line := strings.SplitN(string(data), "\n", 2)[0]

	re := regexp.MustCompile(`^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\.\d{6} `)
	if !re.MatchString(line) {
		t.Fatalf("expected microsecond precision, got %q", line)
	}

	if !strings.Contains(line, testmessage) {
		t.Fatalf("expected log line to contain %q, got %q", testmessage, line)
	}
}

func loadTestConfig(t *testing.T, filename string) *config.Config {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to resolve current file path")
	}
	baseDir := filepath.Dir(currentFile)
	path := filepath.Join(baseDir, "testdata", filename)

	vars := config.ReplacementVars{
		ServiceName: "silver-test",
		ServiceRoot: t.TempDir(),
	}

	cfg, err := config.LoadConfig(path, vars)
	if err != nil {
		t.Fatalf("loading config %s: %v", path, err)
	}

	return cfg
}
