// SILVER - Service Wrapper
//
// Copyright (c) 2014 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package main

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"os/exec"
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

	// run silver in the same mode (run, install)
	cmd := exec.Command(exePath(), os.Args...)
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
	now := time.Now()

	lc := readLastPanic()
	if lc == nil {
		lc = &LastCrash{
			Timestamp:  now,
			CrashCount: 1,
		}

		_ = writeLastPanic(lc)
		return false
	}

	if time.Since(lc.Timestamp) > time.Hour {
		lc.Timestamp = now
		_ = writeLastPanic(lc)
		return false
	}

	if lc.CrashCount < maxCrashCount {
		lc.Timestamp = now
		lc.CrashCount = lc.CrashCount + 1

		time.Sleep(debounceFactor * time.Duration(lc.CrashCount))
		_ = writeLastPanic(lc)
		return false
	}

	return true
}

func readLastPanic() *LastCrash {
	f, err := os.OpenFile(lastPanicFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Printf("failed to open %s", lastPanicFile)
		return nil
	}
	defer f.Close()

	l := &LastCrash{}
	dec := json.NewDecoder(f)
	err = dec.Decode(l)
	if err != nil {
		log.Printf("failed to read lastcrash: %v", err)
		return nil
	}

	return l
}

func writeLastPanic(lc *LastCrash) error {
	f, err := os.OpenFile(lastPanicFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return errors.New("failed to write lastcrash file")
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	return enc.Encode(lc)
}
