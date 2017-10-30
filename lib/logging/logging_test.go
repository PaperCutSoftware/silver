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
	lname := fmt.Sprintf("%s/test-standard-log-%d.log", os.TempDir(), time.Now().Unix())

	logger := NewFileLogger(lname, "")
	defer func() {
		os.Remove(lname)
	}()

	msg := "TestStandardLogging"
	logger.Printf(msg)
	CloseAllOpenFileLoggers()

	output, err := ioutil.ReadFile(lname)
	if err != nil {
		t.Errorf("Unable to read file: %v", err)
	}

	if !strings.Contains(string(output), msg) {
		t.Errorf("Expected '%s', got '%s'", msg, output)
	}
}

func TestRollingLog(t *testing.T) {
	lname := fmt.Sprintf("%s/test-rolling-log-%d.log", os.TempDir(), time.Now().Unix())
	rname := lname + ".1"

	logger := NewFileLoggerWithMaxSize(lname, "", 1024)
	defer func() {
		os.Remove(lname)
		os.Remove(rname)
	}()

	msg := "TestRollingLog"
	for i := 0; i < 100; i++ {
		logger.Printf("%s-%d", msg, i)
	}
	CloseAllOpenFileLoggers()

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

func TestRollingLogFlush_IsFlushed(t *testing.T) {
	// Arrange
	//lname := fmt.Sprintf("%s/test-flushed-log-%d.log", os.TempDir(), time.Now().Unix())
	lname := fmt.Sprintf("test-flushed-log-%d.log", time.Now().Unix())
	logger := NewFileLoggerWithMaxSize(lname, "", 10024)
	defer func() {
		CloseAllOpenFileLoggers()
		os.Remove(lname)
	}()

	// Act
	msg := "TestRollingLog"
	for i := 0; i < 100; i++ {
		logger.Printf("%s-%d", msg, i)
	}
	logger.Printf("x")
	// Log should flush after 5 seconds!
	time.Sleep(5*time.Second + 500*time.Millisecond)

	// Assert
	output, err := ioutil.ReadFile(lname)
	if err != nil {
		t.Fatalf("Error reading log: %v", err)
	}
	if !strings.Contains(string(output), "x") {
		t.Errorf("Expected 'x' in file. It did not flush in time")
	}
}
