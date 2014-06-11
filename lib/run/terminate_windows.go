// SILVER - Service Wrapper
//
// Copyright (c) 2014 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.

// +build windows

package run

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func terminateProcess(process *os.Process, gracefulTimeoutSecs int) bool {
	pid := process.Pid

	// In Windows we'll send a Ctrl Break and also use taskkill.exe just in case it's a
	// wmain Windows program.
	sendCtrlBreak(pid)
	sendTaskKill(pid)

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

func sendCtrlBreak(pid int) {
	d, e := syscall.LoadDLL("kernel32.dll")
	if e != nil {
		return
	}
	p, e := d.FindProc("GenerateConsoleCtrlEvent")
	if e != nil {
		return
	}
	p.Call(syscall.CTRL_BREAK_EVENT, uintptr(pid))
}

func sendTaskKill(pid int) {
	p := fmt.Sprintf("%d", pid)
	cmd := exec.Command("taskkill.exe", "/pid", p)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	cmd.Run()

	// I've seen a few zombie cases so we'll make sure we explicitly kill
	sendCtrlBreak(cmd.Process.Pid)
	cmd.Process.Kill()
}
