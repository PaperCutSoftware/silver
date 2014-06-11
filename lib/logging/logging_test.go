package logging

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"
)

func TestStandardLogging(t *testing.T) {
	lname := fmt.Sprintf("test-standard-loggin-%d.log", time.Now().Unix())
	logger := NewFileLogger(lname)
	defer func() {
		CloseAllOpenFileLoggers()
		os.Remove(lname)
	}()

	msg := "TestStandardLogging"
	logger.Printf(msg)

	output, err := ioutil.ReadFile(lname)
	if err != nil {
		t.Errorf("Unable to read file: %v", err)
	}

	if !strings.Contains(string(output), msg) {
		t.Errorf("Expected '%s', got '%s'", msg, output)
	}
}

func TestRollingLog(t *testing.T) {
	lname := fmt.Sprintf("test-rolling-log-%d.log", time.Now().Unix())
	rname := lname + ".1"

	logger := NewFileLoggerWithMaxSize(lname, 1024)
	defer func() {
		CloseAllOpenFileLoggers()
		os.Remove(lname)
		os.Remove(rname)
	}()

	msg := "TestRollingLog"
	for i := 0; i < 100; i++ {
		logger.Printf("%s-%d", msg, i)
	}

	// Test main log file
	output, err := ioutil.ReadFile(lname)
	if err != nil {
		t.Errorf("Unable to read file: %v", err)
	}
	if !strings.Contains(string(output), msg) {
		t.Errorf("Expected '%s', got '%s'", msg, output)
	}

	// Tested the older rolled file
	rolledOutput, err := ioutil.ReadFile(rname)
	if err != nil {
		t.Errorf("Unable to read rolled file: %v", err)
	}
	if !strings.Contains(string(rolledOutput), msg) {
		t.Errorf("Expected '%s', got '%s'", msg, rolledOutput)
	}
}
