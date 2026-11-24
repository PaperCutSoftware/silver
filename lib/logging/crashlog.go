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
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

const (
	crashLogMaxSize    = 1 << 20
	crashLogMaxBackups = 2
	startHeaderPrefix  = "--- Silver started at "
	startHeaderSuffix  = " ---"
)

var goroutineHeaderRe = regexp.MustCompile(`^goroutine \d+ \[`)

// redirectStderr points the process's stderr at the crash log; mockable for tests.
var redirectStderr = redirectStderrImpl

// stderrFile keeps the crash log alive when it was opened as fd 2, so its
// finalizer never closes stderr.
var stderrFile *os.File

// SetupCrashOutput rotates path, reports a previous run that ended early, writes
// a start header, then sends unhandled panics and fatal runtime errors to path.
func SetupCrashOutput(path, owner, version string, logger *log.Logger) {
	defer FlushAllOpenFileLoggers()
	previousLog := path
	if rotated, err := rotateCrashLog(path, crashLogMaxSize, crashLogMaxBackups); err != nil {
		logger.Printf("WARNING: Unable to rotate crash log %s: %v", path, err)
	} else if rotated {
		previousLog = path + ".1"
	}
	reportEarlyTermination(previousLog, logger)
	enableCrashOutput(path, owner, version, logger)
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

func enableCrashOutput(path, owner, version string, logger *log.Logger) {
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
	if _, err := f.WriteString(startMarker(time.Now(), os.Getpid(), version)); err != nil {
		logger.Printf("WARNING: Unable to write start marker to crash log %s: %v", path, err)
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

// reportEarlyTermination logs a notice if the run that last wrote path ended early.
func reportEarlyTermination(path string, logger *log.Logger) {
	crashed, crashTime, err := previousRunCrashed(path)
	if err != nil {
		logger.Printf("WARNING: Unable to read crash log %s: %v", path, err)
		return
	}
	if !crashed {
		return
	}
	logger.Printf("INFO: Silver terminated early during a previous run. See %s (last written %s).", path, crashTime.Format(time.RFC3339))
}

// previousRunCrashed reports whether a crash line follows the last start
// header within the final crashLogMaxSize bytes of path.
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
	if err != nil {
		return false, time.Time{}, err
	}
	size := info.Size()
	if size == 0 {
		return false, time.Time{}, nil
	}

	var window int64 = crashLogMaxSize
	if size < window {
		window = size
	}
	offset := size - window
	buf := make([]byte, window)
	n, err := f.ReadAt(buf, offset)
	if err != nil && err != io.EOF {
		return false, time.Time{}, err
	}

	lines := strings.Split(string(buf[:n]), "\n")
	if offset > 0 {
		// The window starts mid-file, so its first line is partial.
		lines = lines[1:]
	}
	if !crashAfterLastStartHeader(lines) {
		return false, time.Time{}, nil
	}
	return true, info.ModTime(), nil
}

// crashAfterLastStartHeader reports whether any crash line follows the last
// start header in lines, or appears anywhere when there is no header.
func crashAfterLastStartHeader(lines []string) bool {
	crashed := false
	for _, line := range lines {
		line = strings.TrimSuffix(line, "\r")
		switch {
		case isStartHeader(line):
			crashed = false
		case isCrashLine(line):
			crashed = true
		}
	}
	return crashed
}

func isStartHeader(line string) bool {
	return strings.HasPrefix(line, startHeaderPrefix) && strings.HasSuffix(line, startHeaderSuffix)
}

func isCrashLine(line string) bool {
	return strings.HasPrefix(line, "panic: ") ||
		strings.HasPrefix(line, "fatal error: ") ||
		goroutineHeaderRe.MatchString(line)
}

// startMarker returns the header appended to the crash log on every start.
func startMarker(now time.Time, pid int, version string) string {
	return "\n" + startHeaderPrefix + now.Format(time.RFC3339) + " (pid " + strconv.Itoa(pid) + ", version " + version + ")" + startHeaderSuffix + "\n"
}
