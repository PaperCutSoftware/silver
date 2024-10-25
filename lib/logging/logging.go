// SILVER - Service Wrapper
//
// Copyright (c) 2014 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//
// Contributors:  chris.dance@papercut.com

// Package logging implements a Go-compatible loggers such as console and
// basic file rotation.
//
// Silver's logging requirements are very basic.  We'll roll our own rather than
// bring in a fatter dependency like Seelog. All we require on top of Go's basic
// logging is some very basic file rotation (at the moment only one level).
package logging

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"runtime"
	"sync"
	"time"
)

const (
	defaultMaxSize        = 10 * 1024 * 1024 // 10 MB
	defaultMaxBackupFiles = 1
	defaultFlushInterval  = 5 * time.Second
)

var (
	openRollingFiles      = []*rollingFile{}
	changeOwnerOfFileFunc func(name, owner string) error
)

// Why a wrapper - see finalizer comment below.
type rollingFileWrapper struct {
	*rollingFile
}

type rollingFile struct {
	sync.Mutex
	name                string
	owner               string
	file                *os.File
	maxSize             int64
	maxBackupFiles      int
	bufWriter           *bufio.Writer
	bytesSinceLastFlush int64
	currentSize         int64
	flusher             *flusher
}

type flusher struct {
	interval time.Duration
	stop     chan struct{}
}

func init() {
	changeOwnerOfFileFunc = changeOwnerOfFile
}

func (f *flusher) run(rf *rollingFile) {
	tick := time.Tick(f.interval)
	for {
		select {
		case <-tick:
			rf.flush()
		case <-f.stop:
			return
		}
	}
}

func stopFlusher(rfw *rollingFileWrapper) {
	close(rfw.flusher.stop)
}

func newRollingFile(name string, owner string, maxSize int64, maxFiles int) (rf *rollingFile, err error) {
	if maxSize <= 0 {
		maxSize = defaultMaxSize
	}
	rf = &rollingFile{
		name:           name,
		owner:          owner,
		maxSize:        maxSize,
		maxBackupFiles: maxFiles,
		flusher: &flusher{
			interval: defaultFlushInterval,
			stop:     make(chan struct{}),
		},
	}
	err = rf.open()
	go rf.flusher.run(rf)
	return
}

func (rf *rollingFile) Write(p []byte) (n int, err error) {
	rf.Lock()
	defer rf.Unlock()

	if rf.currentSize+int64(len(p)) >= rf.maxSize {
		rf.roll()
	}
	n, err = rf.bufWriter.Write(p)
	rf.currentSize += int64(n)
	rf.bytesSinceLastFlush += int64(n)
	return
}

func (rf *rollingFile) flush() {
	rf.Lock()
	if rf.bytesSinceLastFlush > 0 {
		rf.bufWriter.Flush()
		rf.bytesSinceLastFlush = 0
	}
	rf.Unlock()
}

func (rf *rollingFile) open() error {
	var err error
	rf.file, err = openLogFile(rf.name, rf.owner)
	if err != nil {
		return err
	}
	rf.bufWriter = bufio.NewWriter(rf.file)
	finfo, err := rf.file.Stat()
	if err != nil {
		return err
	}
	rf.currentSize = finfo.Size()
	openRollingFiles = append(openRollingFiles, rf)
	return nil
}

func (rf *rollingFile) roll() error {
	// Flush the current buffer
	rf.bufWriter.Flush()
	rf.file.Close()

	// Start from the last backup file and move everything back by 1 step
	for i := rf.maxBackupFiles; i > 0; i-- {
		var olderFile, newerFile string

		if i == 1 {
			olderFile = rf.file.Name() // Original file
		} else {
			olderFile = fmt.Sprintf("%s.%d", rf.file.Name(), i-1)
		}
		newerFile = fmt.Sprintf("%s.%d", rf.file.Name(), i)

		// If the older file exists, rename it to the next higher numbered file
		if _, err := os.Stat(olderFile); err == nil {
			os.Rename(olderFile, newerFile)
		}
	}

	// Reopen a new log file for writing
	return rf.open()
}

func openLogFile(name string, owner string) (f *os.File, err error) {
	f, err = os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	err = changeOwnerOfFileFunc(name, owner)
	return
}

// NewFileLogger implements a rolling logger with default maximum size (50Mb)
func NewFileLogger(file string, owner string) (logger *log.Logger) {
	return NewFileLoggerWithMaxSize(file, owner, defaultMaxSize, defaultMaxBackupFiles)
}

// NewFileLoggerWithMaxSize implements a rolling logger with a set size
func NewFileLoggerWithMaxSize(file string, owner string, maxSize int64, maxBackupFiles int) (logger *log.Logger) {
	rf, err := newRollingFile(file, owner, maxSize, maxBackupFiles)
	// This trick ensures that the flusher goroutine does not keep
	// the returned wrapper object from being garbage collected. When it is
	// garbage collected, the finalizer stops the janitor goroutine, after
	// which rw can be collected.
	rfWrapper := &rollingFileWrapper{rf}
	runtime.SetFinalizer(rfWrapper, stopFlusher)
	if err == nil {
		logger = log.New(rfWrapper, "", log.Ldate|log.Ltime)
	} else {
		fmt.Fprintf(os.Stderr, "WARNING: Unable to set up log file: %v\n", err)
		logger = NewNilLogger()
	}
	return logger
}

// CloseAllOpenFileLoggers is a convenience method for tests
func CloseAllOpenFileLoggers() {
	for _, rf := range openRollingFiles {
		rf.bufWriter.Flush()
		rf.file.Close()
	}
	openRollingFiles = []*rollingFile{}
}

// NewNilLogger is a logger noop/discade implementation
func NewNilLogger() *log.Logger {
	return log.New(ioutil.Discard, "", 0)
}

// NewConsoleLogger is a basic logger to Stderr
func NewConsoleLogger() (logger *log.Logger) {
	logger = log.New(os.Stderr, "", log.Ldate|log.Ltime)
	return logger
}
