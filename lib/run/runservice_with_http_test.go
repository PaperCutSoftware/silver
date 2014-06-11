// SILVER - Service Wrapper
//
// Copyright (c) 2014 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//
// +build !nohttp

package run

import (
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/papercutsoftware/silver/lib/logging"
)

func TestServiceHttpMonitorFailure(t *testing.T) {

	const timeout = 10
	const failTime = 5

	c := new(ServiceConfig)
	c.Path = os.Args[0]
	c.Args = helperArgs("http-ping-fail-after", strconv.Itoa(failTime))
	c.Logger = logging.NewConsoleLogger()
	c.MaxCrashCount = 1
	c.MonitorPing = &PingConfig{
		URL:                   "http://127.0.0.1:4300/test",
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
