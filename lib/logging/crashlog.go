// SILVER - Service Wrapper
//
// Copyright (c) 2014-2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.

package logging

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/debug"
	"strings"
	"time"
)

const (
	crashLogMaxSize     = 1 * 1024 * 1024 // 10mb
	crashLogMaxBackups  = 2
	restartMarkerPrefix = "--- Silver restarted at "
	restartMarkerSuffix = " ---"
	crashTailBytes      = 128
)

// mockable for tests
var setCrashOutput = debug.SetCrashOutput

// SetupCrashOutput rotates path, warns on a prior crash, and sends unhandled
// panics and fatal runtime errors to path and stderr
func SetupCrashOutput(path, owner string, logger *log.Logger) {
	defer FlushAllOpenFileLoggers()

	previousLog := path
	if rotated, err := rotateCrashLog(path, crashLogMaxSize, crashLogMaxBackups); err != nil {
		logger.Printf("WARNING: Unable to rotate crash log %s: %v", path, err)
	} else if rotated {
		previousLog = path + ".1"
	}

	reportPreviousCrash(previousLog, logger)
	enableCrashOutput(path, owner, logger)
}

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

func reportPreviousCrash(path string, logger *log.Logger) {
	crashed, crashTime, err := previousRunCrashed(path)
	if err != nil {
		logger.Printf("WARNING: Unable to read crash log %s: %v", path, err)
		return
	}
	if !crashed {
		return
	}
	logger.Printf("WARNING: Silver crashed during a previous run. See crash log %s (last written %s).", path, crashTime.Format(time.RFC3339))
	if err := appendRestartMarker(path); err != nil {
		logger.Printf("WARNING: Unable to mark crash log %s: %v", path, err)
	}
}

func enableCrashOutput(path, owner string, logger *log.Logger) {
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

func previousRunCrashed(path string) (crashed bool, crashTime time.Time, err error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, time.Time{}, nil
	}
	if err != nil {
		return false, time.Time{}, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || info.Size() == 0 {
		return false, time.Time{}, err
	}

	last, err := lastLine(f, info.Size())
	if err != nil {
		return false, time.Time{}, err
	}
	if strings.HasPrefix(last, restartMarkerPrefix) && strings.HasSuffix(last, restartMarkerSuffix) {
		return false, time.Time{}, nil
	}
	return true, info.ModTime(), nil
}

func lastLine(f *os.File, size int64) (string, error) {
	tail := make([]byte, min(size, crashTailBytes))
	n, err := f.ReadAt(tail, size-int64(len(tail)))
	if err != nil && err != io.EOF {
		return "", err
	}
	text := strings.TrimRight(string(tail[:n]), "\r\n")
	return text[strings.LastIndex(text, "\n")+1:], nil
}

func restartMarker(now time.Time) string {
	return "\n" + restartMarkerPrefix + now.Format(time.RFC3339) + restartMarkerSuffix + "\n"
}

func appendRestartMarker(path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(restartMarker(time.Now()))
	return err
}
