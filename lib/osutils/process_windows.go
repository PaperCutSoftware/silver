// SILVER - Service Wrapper
//
// Copyright (c) 2016 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//
// FUTURE: Maybe on windows we should use job control to also cover children?
//         i.e. https://blogs.msdn.microsoft.com/oldnewthing/20131209-00/?p=2433

package osutils

import (
	"fmt"
	"syscall"
	"unsafe"
)

func processIsRunning(pid int) (bool, error) {
	const STILL_ACTIVE = uint32(259)

	handle, err := openProcessHandle(pid)
	if err != nil {
		// Assume process does not exist so not running
		return false, nil
	}
	defer syscall.CloseHandle(handle)

	var ec uint32
	err = syscall.GetExitCodeProcess(syscall.Handle(handle), &ec)
	if err != nil {
		return false, fmt.Errorf("GetExitCodeProcess Error: %v", err)
	}
	return ec == STILL_ACTIVE, nil
}

func processKillHard(pid int) error {
	h, err := openProcessHandle(pid)
	if err != nil {
		return fmt.Errorf("OpenProcess Error: %v", err)
	}
	defer syscall.CloseHandle(h)
	const exitCode = 1
	return syscall.TerminateProcess(h, uint32(exitCode))
}

func processSignalQuit(pid int) error {
	err1 := sendCtrlBreak(pid)
	err2 := sendWMQuit(pid)
	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	return nil
}

func processSysProcAttrForQuit() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

func openProcessHandle(pid int) (syscall.Handle, error) {
	const da = syscall.STANDARD_RIGHTS_READ |
		syscall.PROCESS_QUERY_INFORMATION |
		syscall.SYNCHRONIZE |
		syscall.PROCESS_TERMINATE
	return syscall.OpenProcess(da, false, uint32(pid))
}

// Used to nicely quit console applications
func sendCtrlBreak(pid int) error {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	procGenerateConsoleCtrlEvent := kernel32.NewProc("GenerateConsoleCtrlEvent")
	r, _, _ := procGenerateConsoleCtrlEvent.Call(syscall.CTRL_BREAK_EVENT, uintptr(pid))
	if r == 0 {
		return fmt.Errorf("Error calling GenerateConsoleCtrlEvent")
	}
	return nil
}

// Used to nicely quit gui applications
func sendWMQuit(pid int) error {
	user32 := syscall.NewLazyDLL("user32.dll")
	procEnumWindows := user32.NewProc("EnumWindows")
	procGetWindowThreadProcessId := user32.NewProc("GetWindowThreadProcessId")
	procPostMessage := user32.NewProc("PostMessageW")

	quitCallback := syscall.NewCallback(func(hwnd syscall.Handle, lparam uintptr) uintptr {
		pid := int(lparam)
		// Does the window belong to our PID?
		var windowPID int
		procGetWindowThreadProcessId.Call(uintptr(hwnd),
			uintptr(unsafe.Pointer(&windowPID)))
		if windowPID == pid {
			const WM_CLOSE = 16
			procPostMessage.Call(uintptr(hwnd), uintptr(WM_CLOSE), 0, 0)
		}
		return 1 // continue enumeration
	})
	ret, _, _ := procEnumWindows.Call(quitCallback, uintptr(pid))
	if ret == 0 {
		return fmt.Errorf("Error called EnumWindows")
	}
	return nil
}
