//go:build windows
// +build windows

package logging

import (
	"os"

	"golang.org/x/sys/windows"
)

func redirectStderrImpl(f *os.File) error {
	proc := windows.CurrentProcess()
	var dup windows.Handle
	if err := windows.DuplicateHandle(proc, windows.Handle(f.Fd()), proc, &dup, 0, false, windows.DUPLICATE_SAME_ACCESS); err != nil {
		return err
	}
	if err := windows.SetStdHandle(windows.STD_ERROR_HANDLE, dup); err != nil {
		windows.CloseHandle(dup)
		return err
	}
	return nil
}
