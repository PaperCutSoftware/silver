// SILVER - Service Wrapper
//
// Copyright (c) 2014 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//
// +build immortal

package main

import (
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"
)

func handlePanic(ctx *context) {
	err := recover()
	if err == nil {
		// did not crash. return without doing anything
		return
	}

	logger := panicLogger(ctx)

	abort := debounce()
	if abort {
		logger.Printf("service crashed too many times. bailing...")
		os.Exit(2)
	}

	logger.Println("service is crashing; waiting for all sub-services to terminate")
	logger.Printf("stack: %v", err)
	doStop(ctx)

	cmd := exec.Command(exePath())
	log.Printf("starting new instance of %s; got err %v", exeName(), cmd.Start())
}

func panicLogger(ctx *context) *log.Logger {
	crashlog := ctx.conf.ServiceConfig.CrashLogFile
	if crashlog == "" {
		crashlog = "crashlog.log"
	}

	f, err := os.OpenFile(crashlog, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		f = os.Stderr
	}

	crashLogger := log.New(f, "", log.Ldate|log.Ltime)

	return crashLogger
}

type LastCrash struct {
	Timestamp  time.Time
	CrashCount int
}

const (
	lastPanicFile  = "silver.lastcrash"
	maxCrashCount  = 5
	debounceFactor = 1 * time.Second
)

func debounce() bool {
	f, err := os.OpenFile(lastPanicFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Printf("failed to open %s", lastPanicFile)
		// die if we cant debounce
		return true
	}
	defer f.Close()

	var m sync.Mutex
	now := time.Now()

	lc := readLastPanic(f, &m)
	if lc == nil {
		lc = &LastCrash{
			Timestamp:  now,
			CrashCount: 1,
		}

		_ = writeLastPanic(f, &m, lc)
		return false
	}

	if time.Since(lc.Timestamp) > time.Hour {
		lc.Timestamp = now
		_ = writeLastPanic(f, &m, lc)
		return false
	}

	if lc.CrashCount < maxCrashCount {
		lc.Timestamp = now
		lc.CrashCount = lc.CrashCount + 1

		time.Sleep(debounceFactor * time.Duration(lc.CrashCount))
		_ = writeLastPanic(f, &m, lc)
		return false
	}

	return true
}

func readLastPanic(f *os.File, m *sync.Mutex) *LastCrash {
	m.Lock()
	defer m.Unlock()

	// Seek to front
	f.Seek(0, 0)
	l := &LastCrash{}
	dec := json.NewDecoder(f)
	err := dec.Decode(l)
	if err != nil {
		log.Printf("failed to read lastcrash: %v", err)
		return nil
	}

	return l
}

func writeLastPanic(f *os.File, m *sync.Mutex, lc *LastCrash) error {
	m.Lock()
	defer m.Unlock()

	// Seek to front
	f.Seek(0, 0)
	enc := json.NewEncoder(f)
	return enc.Encode(lc)
}
