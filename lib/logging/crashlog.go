// SILVER - Service Wrapper
//
// Copyright (c) 2014-2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.

package logging

import (
	"log"
	"runtime/debug"
)

// mockable for tests
var setCrashOutput = debug.SetCrashOutput

func SetupCrashOutput(path string, owner string, logger *log.Logger) {
	f, err := openLogFile(path, owner)
	if f == nil {
		logger.Printf("WARNING: Unable to open crash log %s: %v", path, err)
		return
	}
	defer f.Close()
	if err != nil {
		logger.Printf("WARNING: Unable to set owner of crash log %s: %v", path, err)
	}

	if err := setCrashOutput(f, debug.CrashOptions{}); err != nil {
		logger.Printf("WARNING: Unable to enable crash output to %s: %v", path, err)
		return
	}
	debug.SetTraceback("all")
	logger.Printf("Crash output enabled: %s", path)
}
