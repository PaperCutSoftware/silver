// SILVER - Service Wrapper
//
// Copyright (c) 2014 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//
package run

import (
	"time"
)

type CommandConfig struct {
	RunConfig
	StartupDelaySecs       int
	StartupRandomDelaySecs int
	TimeoutSecs            int
}

func RunCommand(c *CommandConfig, terminate chan struct{}) (exitCode int, err error) {

	sleep(c.StartupDelaySecs, terminate)
	sleepRandom(c.StartupRandomDelaySecs, terminate)
	if isTerminated(terminate) {
		return 0, nil
	}

	// If we have a timeout, wrap our terminate channel
	t := terminate
	if c.TimeoutSecs > 0 {
		t = make(chan struct{})
		go func() {
			select {
			case <-terminate:
				close(t)
			case <-time.After(time.Duration(c.TimeoutSecs) * time.Second):
				close(t)
			}
		}()
	}

	return RunWithMonitor(&c.RunConfig, t)
}
