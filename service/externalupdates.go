// SILVER - Service Wrapper
//
// Copyright (c) 2014-2025 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
package main

import (
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/papercutsoftware/silver/service/config"
)

const (
	versionFileName       = ".version"
	defaultMaxUpdateDelay = 30 * time.Second
)

// externalUpdateWatcher restarts services when an external updater edits the
// version file
type externalUpdateWatcher struct {
	VersionFile    string
	Baseline       string
	Poll           time.Duration
	MaxRandomDelay time.Duration
	Load           func() (*config.Config, error)
	Restart        func(*config.Config) bool
	Done           <-chan struct{}
	Logger         *log.Logger
}

func (w *externalUpdateWatcher) Run() {
	baseline := w.Baseline
	var pending, loggedErr string
	for w.wait(w.Poll) {
		v := readVersionFile(w.VersionFile)
		if v == "" || v == baseline {
			pending, loggedErr = "", ""
			continue
		}
		if v != pending {
			pending, loggedErr = v, ""
			continue
		}

		conf, err := w.Load()
		if err == nil {
			err = checkConfRunnable(conf)
		}
		if err != nil {
			if msg := err.Error(); msg != loggedErr {
				w.Logger.Printf("ERROR: Unable to load config for version '%s', keeping current services: %v", v, err)
				loggedErr = msg
			}
			continue
		}
		loggedErr = ""

		if !w.wait(w.randomDelay()) {
			return
		}
		w.Logger.Printf("Version changed from '%s' to '%s'. Services will now restart.", baseline, v)
		if w.Restart(conf) {
			baseline, pending = v, ""
		}
	}
}

// jitter, as multiple instances can be updated at the same time on a given
// machine
func (w *externalUpdateWatcher) randomDelay() time.Duration {
	if w.MaxRandomDelay <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(int64(w.MaxRandomDelay)))
}

func (w *externalUpdateWatcher) wait(d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-w.Done:
		return false
	case <-t.C:
		return !isClosed(w.Done)
	}
}

func readVersionFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
