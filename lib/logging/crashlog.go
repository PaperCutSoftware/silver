// SILVER - Service Wrapper
//
// Copyright (c) 2014-2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.

package logging

import (
	"log"
	"os"
	"runtime/debug"
)

// redirectStderr points the process's stderr at the crash log; mockable for tests.
var redirectStderr = redirectStderrImpl

// stderrFile keeps the crash log alive when it was opened as fd 2, so its
// finalizer never closes stderr.
var stderrFile *os.File

// SetupCrashOutput sends unhandled panics and fatal runtime errors to path.
func SetupCrashOutput(path, owner string, logger *log.Logger) {
	defer FlushAllOpenFileLoggers()
	enableCrashOutput(path, owner, logger)
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
