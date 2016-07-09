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

func terminateProcess(process *os.Process, gracefulTimeoutSecs int) error {
	const checkPeriod = 500 * time.Millisecond
	graceful := time.Now().Add(time.Second * time.Duration(gracefulTimeoutSecs))

	pid := process.Pid

	// In Windows we'll send a Ctrl Break and also use taskkill.exe just in case it's a
	// wmain Windows program.
	if err := sendCtrlBreak(pid); err != nil {
		return err
	}
	// TODO - error?
	sendTaskKill(pid)

	stillRunning := true
	for {
		time.Sleep(checkPeriod)
		if p, _ := os.FindProcess(pid); p == nil {
			stillRunning = false
			break
		}
		if time.Now().After(graceful) {
			break
		}
		fmt.Printf("Here! %v\n", time.Now())
	}
	// Done our best... hard kill
	if stillRunning {
		process.Kill()
		for {
			time.Sleep(checkPeriod)
			fmt.Printf("TEMP checking...")
			if p, _ := os.FindProcess(pid); p != nil {
				fmt.Printf("Ending Unable to kill %v\n", p)
			}
		}
		time.Sleep(checkPeriod)
		if p, _ := os.FindProcess(pid); p != nil {
			return fmt.Errorf("Unable to kill process. PID: %v", pid)
		}
	}
	fmt.Printf("Ending! %v\n", time.Now())
	return nil
}

func sendCtrlBreak(pid int) error {
	d, err := syscall.LoadDLL("kernel32.dll")
	if err != nil {
		return err
	}
	p, err := d.FindProc("GenerateConsoleCtrlEvent")
	if err != nil {
		return err
	}
	p.Call(syscall.CTRL_BREAK_EVENT, uintptr(pid))
	return nil
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
