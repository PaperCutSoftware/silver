// SILVER - Service Wrapper
//
// Copyright (c) 2016 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package osutils

import "time"

// ProcessKillGracefully kills a process gracefully allowing maxTime before
// a hard kill is issued.
func ProcessKillGracefully(pid int, maxTime time.Duration) error {
	const checkPeriod = 500 * time.Millisecond
	end := time.Now().Add(maxTime)

	if err := ProcessSignalQuit(pid); err != nil {
		return err
	}

	for {
		if time.Now().After(end) {
			break
		}
		sleep := checkPeriod
		if time.Now().Add(sleep).After(end) {
			sleep = end.Sub(time.Now())
		}
		time.Sleep(sleep)
		running, err := ProcessIsRunning(pid)
		if err != nil {
			break
		}
		if !running {
			// done!
			return nil
		}
	}
	// Oh well... hard kill
	return ProcessKillHard(pid)
}

// ProcessIsRunning tests to see if a process with PID is running.
func ProcessIsRunning(pid int) (bool, error) {
	return processIsRunning(pid)
}

// ProcessKillHard Hard kills a process (SIGKILL on Unix and TerminateProcess on Windows)
func ProcessKillHard(pid int) error {
	return processKillHard(pid)
}

// ProcessSignalQuit instructs a process to cleanly exist (SIGTERM/SIGINT on Unix
// and Control-C or WM_QUIT on Windows)
func ProcessSignalQuit(pid int) error {
	return processSignalQuit(pid)
}
