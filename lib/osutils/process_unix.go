// SILVER - Service Wrapper
//
// Copyright (c) 2016 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//
// +build darwin dragonfly freebsd linux nacl netbsd openbsd solaris

package osutils

import (
	"os"
	"syscall"
)

func processIsRunning(pid int) (bool, error) {
	// Send zero signal to test
	err := sendSignal(pid, syscall.Signal(0))
	exists := err == nil
	return exists, nil
}

func processKillHard(pid int) error {
	return sendSignal(pid, os.Kill)
}

func processSysProcAttrForQuit() *syscall.SysProcAttr {
	// noop for Unix - only required for Windows
	return nil
}

func processSignalQuit(pid int) error {
	err1 := sendSignal(pid, os.Interrupt)
	err2 := sendSignal(pid, syscall.SIGTERM)
	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	return nil
}

func sendSignal(pid int, sig os.Signal) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	err = process.Signal(sig)
	if err != nil {
		return err
	}
	return nil
}
