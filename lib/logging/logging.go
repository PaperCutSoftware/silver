// SILVER - Service Wrapper
//
// Copyright (c) 2014 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//
// Contributors:  chris.dance@papercut.com

//
// Silver's logging requirements are very basic.  We'll roll our own rather than
// bring in a fatter dependency like Seelog. All we require on top of Go's basic
// logging is some very basic file rotation (at the moment only one level).
//
package logging

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"runtime"
	"sync"
	"time"
)

const (
	defaultMaxSize       = 10 * 1024 * 1024 // 10 MB
	defaultFlushInterval = 5 * time.Second
)

var (
	openLogFiles = make(map[string]*os.File)
)

// Why a wrapper - see finalizer comment below.
type rollingFileWrapper struct {
	*rollingFile
}

type rollingFile struct {
	name                string
	maxSize             int64
	mu                  sync.Mutex
	currentFile         *os.File
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

func newRollingFile(name string, maxSize int64) (rf *rollingFile, err error) {
	if maxSize <= 0 {
		maxSize = defaultMaxSize
	}
	rf = &rollingFile{
		name:    name,
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
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if rf.currentSize+int64(len(p)) >= rf.maxSize {
		rf.roll()
	}
	n, err = rf.currentFile.Write(p)
	rf.currentSize += int64(n)
	rf.bytesSinceLastFlush += int64(n)
	return
}

func (rf *rollingFile) flush() {
	rf.mu.Lock()
	if rf.bytesSinceLastFlush > 0 {
		rf.currentFile.Sync()
		rf.bytesSinceLastFlush = 0
	}
	rf.mu.Unlock()
}

func (rf *rollingFile) open() error {
	var err error
	rf.currentFile, err = openLogFile(rf.name)
	if err != nil {
		return err
	}
	finfo, err := rf.currentFile.Stat()
	if err != nil {
		return err
	}
	rf.currentSize = finfo.Size()
	return nil
}

func (rf *rollingFile) roll() error {
	// FUTURE: Support more than one roll.
	rf.currentFile.Close()
	archivedFile := rf.currentFile.Name() + ".1"
	// Remove old archive and copy over existing
	os.Remove(archivedFile)
	os.Rename(rf.currentFile.Name(), archivedFile)
	return rf.open()
}

func openLogFile(name string) (f *os.File, err error) {
	f, err = os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err == nil {
		openLogFiles[name] = f
	}
	return
}

func NewFileLogger(file string) (logger *log.Logger) {
	return NewFileLoggerWithMaxSize(file, defaultMaxSize)
}

func NewFileLoggerWithMaxSize(file string, maxSize int64) (logger *log.Logger) {
	rf, err := newRollingFile(file, maxSize)
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

// Convenience method - really to just help with testing
func CloseAllOpenFileLoggers() {
	for name, file := range openLogFiles {
		file.Close()
		delete(openLogFiles, name)
	}
}

func NewNilLogger() *log.Logger {
	return log.New(ioutil.Discard, "", 0)
}

func NewConsoleLogger() (logger *log.Logger) {
	logger = log.New(os.Stderr, "", log.Ldate|log.Ltime)
	return logger
}
