// SILVER - Service Wrapper
//
// Copyright (c) 2014 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//
package run

import (
	"os"
	"testing"
	"time"
)

func Test_RunCommand_WithGUIProgram(t *testing.T) {

	// Arrange
	const terminateAfter = 2 * time.Second

	c := &CommandConfig{
		RunConfig: RunConfig{
			Path: "c:\\Windows\\notepad.exe",
			GracefulShutdownTimeoutSecs: 2,
		},
	}

	// Skip if we can't find notepad.
	if _, err := os.Stat(c.Path); err != nil {
		t.Skip("Skipping RunCommand_WithGUIProgram because notepad.exe does not exist")
	}
	startingTime := time.Now().UTC()

	terminate := make(chan struct{})
	go func() {
		time.Sleep(terminateAfter)
		close(terminate)
	}()

	// Act
	_, err := RunCommand(c, terminate)

	// Assert no error
	if err != nil {
		t.Errorf("Notepad did not exit cleanly: %v", err)
	}

	// Assert time
	endingTime := time.Now().UTC()
	duration := endingTime.Sub(startingTime)

	if duration < terminateAfter {
		t.Error("Command run quicker than expected!")
	}
	const threadhold = 500 * time.Millisecond
	if duration > terminateAfter+threadhold {
		t.Error("Terminate took longer than expected!")
	}
}

func TestRunCommandSimpleWithGracefulShutdown(t *testing.T) {
	// Arrange
	const timeout = 1 * time.Second

	c := new(CommandConfig)
	c.Path = os.Args[0]
	c.Args = helperArgs("work-before-exit", "1")

	terminate := make(chan struct{})
	go func() {
		time.Sleep(timeout)
		close(terminate)
	}()

	startingTime := time.Now()
	exitCode, err := RunCommand(c, terminate)
	if err != nil {
		t.Errorf("Command did not exit cleanly: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Unexpected exit code: %d", exitCode)
	}
	endingTime := time.Now()

	duration := endingTime.Sub(startingTime)

	maxExpected := 3 * time.Second
	if duration >= maxExpected {
		t.Error("Expected command to shut down quicker! Took: %v", duration)
	}
}

func TestRunCommandSimple(t *testing.T) {
	const timeout = 10 * time.Second

	c := new(CommandConfig)
	c.Path = os.Args[0]
	c.Args = helperArgs("sleep-for", "0")

	terminate := make(chan struct{})
	go func() {
		time.Sleep(timeout)
		close(terminate)
	}()

	startingTime := time.Now().UTC()
	exitCode, err := RunCommand(c, terminate)
	if err != nil {
		t.Errorf("Command did not exit cleanly: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Unexpected exit code: %d", exitCode)
	}
	endingTime := time.Now().UTC()

	duration := endingTime.Sub(startingTime)

	if duration >= timeout {
		t.Error("Command timed out!")
	}
}

func TestRunCommandProcessDoesNotExist(t *testing.T) {

	c := new(CommandConfig)
	c.Path = "invalid-exe-name"
	_, err := RunCommand(c, nil)
	if err == nil {
		t.Errorf("Invalid file expected to raise error.")
	}
}

func TestRunCommandStartupDelay(t *testing.T) {

	const delay = 3

	c := new(CommandConfig)
	c.Path = os.Args[0]
	c.Args = helperArgs("sleep-for", "0")
	c.StartupDelaySecs = delay

	startingTime := time.Now().UTC()
	_, err := RunCommand(c, nil)
	if err != nil {
		t.Errorf("Echo did not exit cleanly: %v", err)
	}
	endingTime := time.Now().UTC()

	duration := endingTime.Sub(startingTime)

	if duration < time.Duration(delay*time.Second) {
		t.Error("Startup delay not enforced")
	}
}

func TestRunCommandTerminateDuringDelay(t *testing.T) {

	const startDelay = 5
	const terminateDelay = 1

	c := new(CommandConfig)
	c.Path = os.Args[0]
	c.Args = helperArgs("sleep-for", "0")
	c.StartupDelaySecs = startDelay

	terminate := make(chan struct{})
	go func() {
		time.Sleep(terminateDelay * time.Second)
		close(terminate)
	}()

	startingTime := time.Now().UTC()
	_, err := RunCommand(c, terminate)
	if err != nil {
		t.Errorf("Echo did not exit cleanly: %v", err)
	}
	endingTime := time.Now().UTC()

	duration := endingTime.Sub(startingTime)

	if duration >= time.Duration(startDelay*time.Second) {
		t.Error("Did not terminate during startup delay")
	}
}
