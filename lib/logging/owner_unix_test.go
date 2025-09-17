// +build !windows

package logging

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestLogIsOwnedByCorrectUser(t *testing.T) {
	userName := "correct_user"

	functionCalled := false
	// Mock the function
	changeOwnerOfFileFunc = func(name string, owner string) error {
		if owner == userName {
			functionCalled = true
		}
		return nil
	}

	lname := fmt.Sprintf("%s/test-standard-log-%d.log", os.TempDir(), time.Now().Unix())

	logger := NewFileLogger(lname, userName, "2006-01-02 15:04:05")
	defer func() {
		os.Remove(lname)
	}()

	msg := "TestStandardLogging"
	logger.Printf(msg)
	CloseAllOpenFileLoggers()

	if !functionCalled {
		t.Errorf("Expected function changeOwnerOfFile to be called, but it wasn't")
	}
}
