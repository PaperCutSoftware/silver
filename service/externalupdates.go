// SILVER - Service Wrapper
//
// Copyright (c) 2014-2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
package main

import (
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/papercutsoftware/silver/service/config"
)

const defaultMaxUpdateDelay = 30 * time.Second

// externalUpdateWatcher restarts services when an external updater edits the
// version file of a component in use
type externalUpdateWatcher struct {
	Components      func(*config.Config) []component
	RunningVersions map[component]string
	Poll            time.Duration
	MaxRandomDelay  time.Duration
	Load            func() (*config.Config, error)
	Restart         func(*config.Config) bool
	Done            <-chan struct{}
	Logger          *log.Logger
}

func (w *externalUpdateWatcher) Run() {
	t := newVersionTracker(w.RunningVersions)
	components := sortedComponents(t.running)
	w.logIfNothingToWatch(components)
	for w.wait(w.Poll) {
		if !t.poll(components) {
			continue
		}
		conf, ok := w.loadRunnable(t)
		if !ok {
			continue
		}
		if !w.wait(w.randomDelay()) {
			return
		}
		if !w.restartOn(conf, t) {
			continue
		}
		components = w.Components(conf)
		t.advance(components)
		w.logIfNothingToWatch(components)
	}
}

// versionTracker follows the version of each watched component between polls
type versionTracker struct {
	running   map[component]string // version each watched component's running services started on
	changed   map[component]string // changed versions not yet restarted on
	loggedErr string               // last load error logged, so each is logged once while the versions stay
}

func newVersionTracker(running map[component]string) *versionTracker {
	t := &versionTracker{running: map[component]string{}, changed: map[component]string{}}
	for c, v := range running {
		t.running[c] = v
	}
	return t
}

// poll reads every component's version and reports whether there are changed
// versions and all of them read the same as on the previous poll
func (t *versionTracker) poll(components []component) (settled bool) {
	settled = true
	for _, c := range components {
		v := c.readVersion()
		switch {
		case v == "" || v == t.running[c]:
			if _, ok := t.changed[c]; ok {
				delete(t.changed, c)
				t.loggedErr = ""
			}
		case v != t.changed[c]:
			t.changed[c], t.loggedErr = v, ""
			settled = false
		}
	}
	return settled && len(t.changed) > 0
}

// advance makes the changed versions the running versions, then watches
// components: kept components keep their running versions and new ones start
// with none
func (t *versionTracker) advance(components []component) {
	for c, v := range t.changed {
		t.running[c] = v
	}
	kept := map[component]string{}
	for _, c := range components {
		if v, ok := t.running[c]; ok {
			kept[c] = v
		}
	}
	t.running, t.changed = kept, map[component]string{}
}

// loadRunnable loads the config to restart on and checks it can run, logging
// each new failure once
func (w *externalUpdateWatcher) loadRunnable(t *versionTracker) (*config.Config, bool) {
	conf, err := w.Load()
	if err == nil {
		err = checkConfRunnable(conf)
	}
	if err != nil {
		if msg := err.Error(); msg != t.loggedErr {
			w.Logger.Printf("ERROR: Unable to load config for new versions %s, keeping current services: %v", describeVersions(t.changed), err)
			t.loggedErr = msg
		}
		return nil, false
	}
	t.loggedErr = ""
	return conf, true
}

func (w *externalUpdateWatcher) restartOn(conf *config.Config, t *versionTracker) bool {
	for _, c := range sortedComponents(t.changed) {
		w.Logger.Printf("Component '%s' changed from running version '%s' to '%s'.", c.dir, t.running[c], t.changed[c])
	}
	w.Logger.Print("Services will now restart.")
	return w.Restart(conf)
}

func (w *externalUpdateWatcher) logIfNothingToWatch(components []component) {
	if len(components) == 0 {
		w.Logger.Print("No component '.version' files to watch: external updates will not restart services.")
	}
}

func describeVersions(versions map[component]string) string {
	var parts []string
	for _, c := range sortedComponents(versions) {
		parts = append(parts, fmt.Sprintf("'%s' in '%s'", versions[c], c.dir))
	}
	return strings.Join(parts, ", ")
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
