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
	crashLogMaxSize    = 1 * 1024 * 1024
	crashLogMaxBackups = 2
)

// mockable for tests
var setCrashOutput = debug.SetCrashOutput

func SetupCrashOutput(path string, owner string, logger *log.Logger) {
	if err := rotateCrashLog(path, crashLogMaxSize, crashLogMaxBackups); err != nil {
		logger.Printf("WARNING: Unable to rotate crash log %s: %v", path, err)
	}

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

func rotateCrashLog(path string, maxSize int64, maxBackups int) error {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Size() < maxSize {
		return nil
	}

	for i := maxBackups; i > 0; i-- {
		from := path
		if i > 1 {
			from = fmt.Sprintf("%s.%d", path, i-1)
		}
		to := fmt.Sprintf("%s.%d", path, i)
		// Stop at the first failure so a later rename can't overwrite a backup
		if err := os.Rename(from, to); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}
