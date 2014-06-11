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

func TestExecGUIProgram(t *testing.T) {

	const timeout = 2

	c := new(CommandConfig)
	c.Path = "c:\\Windows\\notepad.exe"

	// Skip if we can't find notepad.
	if _, err := os.Stat(c.Path); err == nil {
		terminate := make(chan struct{})
		go func() {
			time.Sleep(timeout * time.Second)
			close(terminate)
		}()

		startingTime := time.Now().UTC()
		_, err := RunCommand(c, terminate)
		endingTime := time.Now().UTC()

		if err != nil {
			t.Errorf("Notepad did not exit cleanly: %v", err)
		}

		duration := endingTime.Sub(startingTime)

		if duration < time.Duration(timeout*time.Second) {
			t.Error("Command did not timeout!")
		}
	}
}

func TestRunCommandSimple(t *testing.T) {

	const timeout = 10

	c := new(CommandConfig)
	c.Path = os.Args[0]
	c.Args = helperArgs("sleep-for", "0")

	terminate := make(chan struct{})
	go func() {
		time.Sleep(timeout * time.Second)
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

	if duration >= time.Duration(timeout*time.Second) {
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
