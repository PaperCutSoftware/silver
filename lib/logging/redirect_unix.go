//go:build !windows
// +build !windows

package logging

import (
	"os"

	"golang.org/x/sys/unix"
)

func redirectStderrImpl(f *os.File) error {
	fd := int(f.Fd())
	// Already stderr. Dup2(2, 2) is avoided as Linux arm64 implements it via dup3, which rejects equal fds.
	if fd == 2 {
		return nil
	}
	return unix.Dup2(fd, 2)
}
