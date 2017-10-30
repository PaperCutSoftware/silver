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
	"os/user"
	"runtime"
	"strconv"
	"sync"
	"time"
)

const (
	defaultMaxSize       = 10 * 1024 * 1024 // 10 MB
	defaultFlushInterval = 5 * time.Second
)

var (
	openRollingFiles = []*rollingFile{}
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
	bufWriter           *bufio.Writer
	bytesSinceLastFlush int64
	currentSize         int64
	flusher             *flusher
}

type flusher struct {
	interval time.Duration
	stop     chan struct{}
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

func newRollingFile(name string, owner string, maxSize int64) (rf *rollingFile, err error) {
	if maxSize <= 0 {
		maxSize = defaultMaxSize
	}
	rf = &rollingFile{
		name:    name,
		owner:   owner,
		maxSize: maxSize,
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
	// FUTURE: Support more than one roll.
	rf.bufWriter.Flush()
	rf.file.Close()
	archivedFile := rf.file.Name() + ".1"
	// Remove old archive and copy over existing
	os.Remove(archivedFile)
	os.Rename(rf.file.Name(), archivedFile)
	return rf.open()
}

func openLogFile(name string, owner string) (f *os.File, err error) {
	f, err = os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	// If owner is defined, change the owner of the log file to this user
	if owner != "" {
		ownerUser, err := user.Lookup(owner)
		if err != nil {
			return f, err
		}
		uid, err := strconv.Atoi(ownerUser.Uid)
		if err != nil {
			return f, err
		}
		gid, err := strconv.Atoi(ownerUser.Gid)
		if err != nil {
			return f, err
		}
		os.Chown(name, uid, gid)
	}
	return
}

// NewFileLogger implements a rolling logger with default maximum size (50Mb)
func NewFileLogger(file string, owner string) (logger *log.Logger) {
	return NewFileLoggerWithMaxSize(file, owner, defaultMaxSize)
}

// NewFileLoggerWithMaxSize implements a rolling logger with a set size
func NewFileLoggerWithMaxSize(file string, owner string, maxSize int64) (logger *log.Logger) {
	rf, err := newRollingFile(file, owner, maxSize)
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
