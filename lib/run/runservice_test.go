// SILVER - Service Wrapper
//
// Copyright (c) 2014 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//
package run

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/papercutsoftware/silver/lib/logging"
)

func TestServiceNormalShutdown(t *testing.T) {

	const timeout = 2
	const maxRun = 10

	c := new(ServiceConfig)
	c.Path = os.Args[0]
	c.Args = helperArgs("sleep-for", strconv.Itoa(maxRun))

	terminate := make(chan struct{})
	go func() {
		time.Sleep(timeout * time.Second)
		close(terminate)
	}()

	startingTime := time.Now().UTC()
	err := RunService(c, terminate)
	if err != nil {
		t.Errorf("Service did not exit cleanly: %v", err)
	}
	endingTime := time.Now().UTC()

	duration := endingTime.Sub(startingTime)

	if duration >= time.Duration(maxRun*time.Second) {
		t.Error("Did not time out")
	}
}

func TestServiceCrashCount(t *testing.T) {
	const timeout = 20

	c := new(ServiceConfig)
	c.Path = os.Args[0]
	c.Args = helperArgs("crash-in", "1")
	c.MaxCrashCount = 2

	terminate := make(chan struct{})
	go func() {
		time.Sleep(timeout * time.Second)
		close(terminate)
	}()

	startingTime := time.Now().UTC()
	err := RunService(c, terminate)
	if err == nil {
		t.Errorf("Error expected from crashing service")
	}
	if !strings.Contains(err.Error(), "MaxCrashCount") {
		t.Errorf("Expected error with MaxCrashCount exceeded. Got: %s", err.Error())
	}
	endingTime := time.Now().UTC()

	duration := endingTime.Sub(startingTime)

	if duration >= time.Duration(timeout*time.Second) {
		t.Error("Did not exit due to persistent crashes")
	}
}

func TestServiceGracefulShutdown(t *testing.T) {
	const timeout = 1
	const shutdownTime = 5

	c := new(ServiceConfig)
	c.Path = os.Args[0]
	c.Args = helperArgs("work-before-exit", strconv.Itoa(shutdownTime))

	terminate := make(chan struct{})
	go func() {
		time.Sleep(timeout * time.Second)
		close(terminate)
	}()

	startingTime := time.Now().UTC()
	err := RunService(c, terminate)
	if err != nil {
		t.Errorf("Error reported from service: %v", err)
	}
	endingTime := time.Now().UTC()

	duration := endingTime.Sub(startingTime)

	if duration <= time.Duration(shutdownTime*time.Second) {
		t.Error("Expected graceful shutdown to take longer")
	}
	if duration > time.Duration((shutdownTime+2)*time.Second) {
		t.Error("Looks like graceful shutdown did not complete in expected time? Check.")
	}
}

func TestServiceUngracefulShutdown(t *testing.T) {
	const timeout = 1
	const shutdownTimeout = 5
	const shutdownTime = 20

	c := new(ServiceConfig)
	c.Path = os.Args[0]
	c.Args = helperArgs("work-before-exit", strconv.Itoa(shutdownTime))
	c.GracefulShutdownTimeoutSecs = shutdownTimeout

	terminate := make(chan struct{})
	go func() {
		time.Sleep(timeout * time.Second)
		close(terminate)
	}()

	startingTime := time.Now().UTC()
	err := RunService(c, terminate)
	if err != nil {
		t.Errorf("Error reported from service: %v", err)
	}
	endingTime := time.Now().UTC()

	duration := endingTime.Sub(startingTime)

	if duration <= time.Duration((timeout+shutdownTimeout)*time.Second) {
		t.Error("Expected shutdown to take longer")
	}
	if duration > time.Duration((timeout+shutdownTimeout+2)*time.Second) {
		t.Error("Looks like shutdown did not complete in expected time? Check.")
	}
}

func TestServiceEchoMonitorFailure(t *testing.T) {

	const timeout = 15
	const failTime = 5

	c := new(ServiceConfig)
	c.Path = os.Args[0]
	c.Args = helperArgs("echo-ping-fail-after", strconv.Itoa(failTime))
	c.Logger = logging.NewConsoleLogger()
	c.MaxCrashCount = 1
	c.MonitorPing = &PingConfig{
		URL:                   "echo://127.0.0.1:4300",
		IntervalSecs:          1,
		TimeoutSecs:           3,
		RestartOnFailureCount: 0,
	}

	terminate := make(chan struct{})
	go func() {
		time.Sleep(timeout * time.Second)
		close(terminate)
	}()

	startingTime := time.Now().UTC()
	err := RunService(c, terminate)
	if err == nil {
		t.Errorf("Service should have raised MaxCrashCount error")
	}
	endingTime := time.Now().UTC()

	duration := endingTime.Sub(startingTime)

	if duration < time.Duration(failTime*time.Second) {
		t.Error("Expected test to take longer")
	}
	if duration >= time.Duration(timeout*time.Second) {
		t.Error("Looks like failure detection did not work. Check.")
	}
}

func TestServiceTCPMonitorFailure(t *testing.T) {

	const timeout = 15
	const failTime = 5

	c := new(ServiceConfig)
	c.Path = os.Args[0]
	c.Args = helperArgs("tcp-ping-fail-after", strconv.Itoa(failTime))
	c.Logger = logging.NewConsoleLogger()
	c.MaxCrashCount = 1
	c.MonitorPing = &PingConfig{
		URL:                   "tcp://127.0.0.1:4300",
		IntervalSecs:          1,
		TimeoutSecs:           3,
		RestartOnFailureCount: 0,
	}

	terminate := make(chan struct{})
	go func() {
		time.Sleep(timeout * time.Second)
		close(terminate)
	}()

	startingTime := time.Now().UTC()
	err := RunService(c, terminate)
	if err == nil {
		t.Errorf("Service should have raised MaxCrashCount error")
	}
	endingTime := time.Now().UTC()

	duration := endingTime.Sub(startingTime)

	if duration < time.Duration(failTime*time.Second) {
		t.Error("Expected test to take longer. Check.")
	}
	if duration >= time.Duration(timeout*time.Second) {
		t.Error("Looks like failure detection did not work. Check.")
	}
}

func TestServiceFileMonitorFailure(t *testing.T) {

	const timeout = 15
	const failTime = 5
	const statusFile = "status.file"

	defer os.Remove(statusFile)

	c := new(ServiceConfig)
	c.Path = os.Args[0]
	c.Args = helperArgs("file-ping-fail-after", strconv.Itoa(failTime))
	c.Logger = logging.NewConsoleLogger()
	c.MaxCrashCount = 1
	c.MonitorPing = &PingConfig{
		URL:                   "file://" + statusFile,
		IntervalSecs:          1,
		TimeoutSecs:           3,
		RestartOnFailureCount: 0,
	}

	terminate := make(chan struct{})
	go func() {
		time.Sleep(timeout * time.Second)
		close(terminate)
	}()

	startingTime := time.Now().UTC()
	err := RunService(c, terminate)
	if err == nil {
		t.Errorf("Service should have raised MaxCrashCount error")
	}
	endingTime := time.Now().UTC()

	duration := endingTime.Sub(startingTime)

	if duration < time.Duration(failTime*time.Second) {
		t.Error("Expected test to take longer. Check.")
	}
	if duration >= time.Duration(timeout*time.Second) {
		t.Error("Looks like failure detection did not work. Check.")
	}
}
