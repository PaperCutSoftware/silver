package logging

import (
    "fmt"
    "os"
    "regexp"
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

	output, err := os.ReadFile(lname)
	if err != nil {
		t.Errorf("Unable to read file: %v", err)
	}

	if !strings.Contains(string(output), msg) {
		t.Errorf("Expected '%s', got '%s'", msg, output)
	}
}

func TestLogging_TimestampsOnByDefault(t *testing.T) {
    lname := fmt.Sprintf("%s/test-timestamps-on-%d.log", os.TempDir(), time.Now().UnixNano())

    logger := NewFileLogger(lname, "")
    defer func() {
        os.Remove(lname)
    }()

    msg := "TimestampsOnByDefault"
    logger.Printf(msg)
    CloseAllOpenFileLoggers()

    data, err := os.ReadFile(lname)
    if err != nil {
        t.Fatalf("Unable to read file: %v", err)
    }
    line := strings.SplitN(string(data), "\n", 2)[0]
    // Default flags include date and time. Format: YYYY/MM/DD HH:MM:SS ...
    re := regexp.MustCompile(`^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2} `)
    if !re.MatchString(line) {
        t.Fatalf("expected timestamp prefix, got: %q", line)
    }
    if !strings.Contains(line, msg) {
        t.Fatalf("expected message %q in line %q", msg, line)
    }
}

func TestLogging_TimestampsDisabledWithZeroFlags(t *testing.T) {
    lname := fmt.Sprintf("%s/test-timestamps-off-%d.log", os.TempDir(), time.Now().UnixNano())

    logger := NewFileLogger(lname, "")
    // Simulate disabling timestamps (as service/main.go does)
    logger.SetFlags(0)
    defer func() {
        os.Remove(lname)
    }()

    msg := "TimestampsOff"
    logger.Printf(msg)
    CloseAllOpenFileLoggers()

    data, err := os.ReadFile(lname)
    if err != nil {
        t.Fatalf("Unable to read file: %v", err)
    }
    line := strings.SplitN(string(data), "\n", 2)[0]
    // With flags=0 we expect no date/time prefix; line should start with msg
    if !strings.HasPrefix(line, msg) {
        t.Fatalf("expected line to start with message %q, got %q", msg, line)
    }
}

func TestRollingLog(t *testing.T) {
	lname := fmt.Sprintf("%s/test-rolling-log-%d.log", os.TempDir(), time.Now().Unix())

	// Create the logger with log rotation for max 5 backup files.
	logger := NewFileLoggerWithMaxSize(lname, "", 1024, 5)
	defer func() {
		// Clean up all the log files after the test.
		for i := 0; i <= 5; i++ { // Remove the main log file and the 5 backups.
			os.Remove(fmt.Sprintf("%s.%d", lname, i))
		}
		os.Remove(lname) // Also remove the main log.
	}()

	msg := "TestRollingLog"
	for i := 0; i < 2000; i++ {
		logger.Printf("%s-%d", msg, i)
	}
	CloseAllOpenFileLoggers()

	rolledFileName := lname
	// Check that exactly 5 log files are present
	for i := 0; i <= 5; i++ {
		if _, err := os.Stat(rolledFileName); os.IsNotExist(err) {
			t.Errorf("Expected log file '%s' to exist, but it does not.", rolledFileName)
		}
		output, err := os.ReadFile(rolledFileName)
		if err != nil {
			t.Errorf("Unable to read file: %v", err)
		}
		if !strings.Contains(string(output), msg) {
			t.Errorf("Expected '%s', got '%s'", msg, output)
		}
		rolledFileName = fmt.Sprintf("%s.%d", lname, i+1) // lname.1, lname.2, etc.
	}

	// Make sure no extra log files exist (like lname.6 or higher)
	extraFileName := fmt.Sprintf("%s.%d", lname, 6)
	if _, err := os.Stat(extraFileName); err == nil {
		t.Errorf("Unexpected log file '%s' found. Only 5 backup files should be present.", extraFileName)
	}
}

func TestRollingLogFlush_IsFlushed(t *testing.T) {
	// Arrange
	//lname := fmt.Sprintf("%s/test-flushed-log-%d.log", os.TempDir(), time.Now().Unix())
	lname := fmt.Sprintf("test-flushed-log-%d.log", time.Now().Unix())
	logger := NewFileLoggerWithMaxSize(lname, "", 10024, 5)
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
	output, err := os.ReadFile(lname)
	if err != nil {
		t.Fatalf("Error reading log: %v", err)
	}
	if !strings.Contains(string(output), "x") {
		t.Errorf("Expected 'x' in file. It did not flush in time")
	}
}
