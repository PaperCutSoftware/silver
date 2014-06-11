// SILVER - Service Wrapper
//
// Copyright (c) 2014 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//
package run

import (
	"errors"
	"time"

	// FIXME: Remove this dependency and replace with a message channel
	"github.com/papercutsoftware/silver/lib/logging"
)

type ServiceConfig struct {
	RunConfig
	MaxCrashCount    int
	RestartDelaySecs int
	StartupDelaySecs int
	MonitorPing      *PingConfig
}

type PingConfig struct {
	URL                   string
	StartupDelaySecs      int
	IntervalSecs          int
	TimeoutSecs           int
	RestartOnFailureCount int
}

func RunService(c *ServiceConfig, terminate chan struct{}) (err error) {

	setupServiceConfigDefaults(c)

	sleep(c.StartupDelaySecs, terminate)

	c.Logger.Printf("%s: Starting service", c.name())

	crashCnt := 0
	crashPeriodStart := time.Now()
	for {
		if c.MonitorPing != nil && c.MonitorPing.URL != "" {
			t := wrapWithMonitor(terminate, c)
			_, err = RunWithMonitor(&c.RunConfig, t)
		} else {
			_, err = RunWithMonitor(&c.RunConfig, terminate)
		}
		if isTerminated(terminate) {
			break
		}

		// Raise an error if our crash count is exceeded in any given hour
		crashCnt++
		if time.Now().After(crashPeriodStart.Add(time.Hour)) {
			crashCnt = 0
			crashPeriodStart = time.Now()
		}
		if c.MaxCrashCount > 0 && crashCnt >= c.MaxCrashCount {
			err = errors.New("Service exceeded MaxCrashCount in last hour")
			break
		}

		c.Logger.Printf("%s: Restarting in %d seconds.", c.name(), c.RestartDelaySecs)
		sleep(c.RestartDelaySecs, terminate)
		if isTerminated(terminate) {
			break
		}
	}

	return err
}

func wrapWithMonitor(terminate chan struct{}, c *ServiceConfig) (trigger chan struct{}) {

	const DEFAULT_INTERVAL = 30
	const DEFAULT_TIMEOUT = 30

	trigger = make(chan struct{})
	go func() {
		// Setup monitor reasonable defaults
		monitor := c.MonitorPing
		intervalSecs := monitor.IntervalSecs
		if intervalSecs == 0 {
			intervalSecs = DEFAULT_INTERVAL
		}
		var timeout time.Duration
		if monitor.TimeoutSecs == 0 {
			timeout = DEFAULT_TIMEOUT * time.Second
		} else {
			timeout = time.Duration(monitor.TimeoutSecs) * time.Second
		}

		c.Logger.Printf("%s: Starting monitor on %s", c.name(), monitor.URL)

		sleep(monitor.StartupDelaySecs, terminate)
		failureCount := 0
		for {
			sleep(intervalSecs, terminate)
			if isTerminated(terminate) {
				break
			}
			if ok, err := pingURL(monitor.URL, timeout); !ok {
				failureCount++
				c.Logger.Printf("%s: Monitor detected error - '%v'", c.name(), err)
			} else {
				// Did the monitor report another error?
				if err != nil {
					c.Logger.Printf("%s: Monitor ping error '%v'", c.name(), err)
				}
				failureCount = 0
			}
			if failureCount > monitor.RestartOnFailureCount {
				c.Logger.Printf("%s: Service not responding. Forcing shutdown. (failures: %d)",
					c.name(), failureCount)
				break
			}
		}
		close(trigger)
	}()
	return trigger
}

func setupServiceConfigDefaults(c *ServiceConfig) {
	if c.RestartDelaySecs == 0 {
		c.RestartDelaySecs = 1
	}
	if c.Logger == nil {
		c.Logger = logging.NewNilLogger()
	}
}
