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

func TestLogging_SampleConfigWithDefaultDateFormat(t *testing.T) {
	cfg := loadTestConfig(t, "logging-defaultdateformat.conf")
	logPath := filepath.Join(t.TempDir(), "no-micro.log")
	cfg.ServiceConfig.LogFile = logPath

	logger := logging.NewFileLogger(cfg.ServiceConfig.LogFile, cfg.ServiceConfig.UserName, cfg.ServiceConfig.LogFileTimestampFormat)
	t.Cleanup(logging.CloseAllOpenFileLoggers)

	testmessage := "LoggingWithDefaultDateFormat"
	logger.Printf("%s %s", testmessage, cfg.ServiceConfig.LogFileTimestampFormat)
	logging.CloseAllOpenFileLoggers()

	data, err := os.ReadFile(cfg.ServiceConfig.LogFile)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	line := string(data)

	reDate := regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} `)
	if !reDate.MatchString(line) {
		t.Fatalf("expected timestamp without microseconds, got %q", line)
	}

	reMicro := regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{6} `)
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

	logger := logging.NewFileLogger(cfg.ServiceConfig.LogFile, cfg.ServiceConfig.UserName, cfg.ServiceConfig.LogFileTimestampFormat)
	t.Cleanup(logging.CloseAllOpenFileLoggers)

	testmessage := "LoggingWithMicroseconds"
	msg := "%s %s"
	logger.Printf(msg, testmessage, cfg.ServiceConfig.LogFileTimestampFormat)
	logging.CloseAllOpenFileLoggers()

	data, err := os.ReadFile(cfg.ServiceConfig.LogFile)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	line := string(data)

	re := regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{6} `)
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
