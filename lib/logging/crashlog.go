// SILVER - Service Wrapper
//
// Copyright (c) 2014-2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.

package logging

import (
	"errors"
	"fmt"
	"log"
	"os"
	"runtime/debug"
)

const (
	crashLogMaxSize    = 1 << 20
	crashLogMaxBackups = 2
)

// redirectStderr points the process's stderr at the crash log; mockable for tests.
var redirectStderr = redirectStderrImpl

// stderrFile keeps the crash log alive when it was opened as fd 2, so its
// finalizer never closes stderr.
var stderrFile *os.File

// SetupCrashOutput rotates path, then sends unhandled panics and fatal runtime
// errors to it.
func SetupCrashOutput(path, owner string, logger *log.Logger) {
	defer FlushAllOpenFileLoggers()
	if _, err := rotateCrashLog(path, crashLogMaxSize, crashLogMaxBackups); err != nil {
		logger.Printf("WARNING: Unable to rotate crash log %s: %v", path, err)
	}
	enableCrashOutput(path, owner, logger)
}

// rotateCrashLog shifts path to path.1, path.1 to path.2 and so on up to
// maxBackups once path reaches maxSize bytes.
func rotateCrashLog(path string, maxSize int64, maxBackups int) (rotated bool, err error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.Size() < maxSize {
		return false, nil
	}

	for i := maxBackups; i > 0; i-- {
		from := path
		if i > 1 {
			from = fmt.Sprintf("%s.%d", path, i-1)
		}
		to := fmt.Sprintf("%s.%d", path, i)
		// Stop at the first failure so a later rename can't overwrite a backup
		if err := os.Rename(from, to); err != nil && !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
	}
	return true, nil
}

func enableCrashOutput(path, owner string, logger *log.Logger) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		logger.Printf("WARNING: Unable to open crash log %s: %v", path, err)
		return
	}
	// The redirect duplicates the descriptor, unless f already is stderr.
	if f.Fd() == 2 {
		stderrFile = f
	} else {
		defer f.Close()
	}
	if owner != "" {
		if err := changeOwnerOfFileFunc(path, owner); err != nil {
			logger.Printf("WARNING: Unable to set owner of crash log %s: %v", path, err)
		}
	}

	if err := redirectStderr(f); err != nil {
		logger.Printf("WARNING: Unable to enable crash output to %s: %v", path, err)
		return
	}
	debug.SetTraceback("all")
	logger.Printf("Crash output enabled: %s", path)
}
