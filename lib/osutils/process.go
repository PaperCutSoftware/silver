// SILVER - Service Wrapper
//
// Copyright (c) 2016 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package osutils

import (
	"syscall"
	"time"
)

// ProcessKillGracefully kills a process gracefully allowing maxTime before
// a hard kill is issued.
//
// IMPORTANT: On Windows processes started form Go must be in their own process group.
// cmd.SysProcAttr = &syscall.SysProcAttr{
//	CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
// }
func ProcessKillGracefully(pid int, maxTime time.Duration) error {
	const checkPeriod = 500 * time.Millisecond
	end := time.Now().Add(maxTime)

	ProcessSignalQuit(pid)
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

// ProcessSysProcAttrForQuit returns a SysProcAttr suitable to set either
// cmd.SysProcAttr or os.StartProcess. At the current time is only is required
// for Windows to ensure a new process group is created.  It's nil on Unix.
func ProcessSysProcAttrForQuit() *syscall.SysProcAttr {
	return processSysProcAttrForQuit()
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
//
// IMPORTANT: On Windows processes started from Go must be in their own process group.
func ProcessSignalQuit(pid int) error {
	return processSignalQuit(pid)
}
