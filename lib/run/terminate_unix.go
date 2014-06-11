// SILVER - Service Wrapper
//
// Copyright (c) 2014 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.

// +build !windows

package run

import (
	"os"
	"time"
)

func terminateProcess(process *os.Process, gracefulTimeoutSecs int) bool {
	pid := process.Pid

	process.Signal(os.Interrupt)

	stillRunning := true
	for i := 0; i < gracefulTimeoutSecs; i++ {
		if p, _ := os.FindProcess(pid); p == nil {
			stillRunning = false
			break
		}
		time.Sleep(1 * time.Second)
	}
	// Done our best... hard kill
	if stillRunning {
		process.Kill()
		return false
	}
	return true
}
