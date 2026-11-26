//go:build !windows

package logging

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestFileLoggerWithDirPerm_CreatedDirPerms(t *testing.T) {
	// Clear the umask so the created dirs get exactly perm; otherwise e.g. umask 077 turns 0755
	// into 0700. Umask is process-wide, so this test must not run in parallel.
	orig := syscall.Umask(0)
	defer syscall.Umask(orig)

	for _, perm := range []os.FileMode{0o700, 0o755} {
		t.Run(perm.String(), func(t *testing.T) {
			root := t.TempDir()
			lname := filepath.Join(root, "a", "b", "test.log")

			NewFileLoggerWithDirPerm(lname, "", 1024, 1, "", perm)
			CloseAllOpenFileLoggers()

			for _, dir := range []string{filepath.Join(root, "a"), filepath.Join(root, "a", "b")} {
				info, err := os.Stat(dir)
				if err != nil {
					t.Fatalf("Stat %s: %v", dir, err)
				}
				if got := info.Mode().Perm(); got != perm {
					t.Errorf("%s perm = %v, want %v", dir, got, perm)
				}
			}
		})
	}
}

func TestFileLoggerWithDirPerm_ExistingDirUntouched(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "existing")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	// Chmod isn't affected by the umask, so the dir is 0755 whatever it is.
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("Chmod: %v", err)
	}

	NewFileLoggerWithDirPerm(filepath.Join(dir, "test.log"), "", 1024, 1, "", 0o700)
	CloseAllOpenFileLoggers()

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Errorf("existing dir perm = %v, want 0755", got)
	}
}
