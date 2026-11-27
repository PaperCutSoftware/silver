// SILVER - Service Wrapper
//
// Copyright (c) 2014-2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
package main

import "os"

// reloadSignal watches for the .reload file that asks the service to restart.
// In active mode the file is consumed (removed). In passive mode the file is
// never touched; a reload fires when it appears or its modification time changes.
type reloadSignal struct {
	path     string
	passive  bool
	baseline fileState
}

// fileState is comparable with ==, so mtime is stored as UnixNano rather than time.Time.
type fileState struct {
	exists bool
	mtime  int64 // zero when absent
}

// statFile returns the file's state. ok is false when it can't be determined.
func statFile(path string) (s fileState, ok bool) {
	info, err := os.Stat(path)
	switch {
	case err == nil:
		return fileState{exists: true, mtime: info.ModTime().UnixNano()}, true
	case os.IsNotExist(err):
		return fileState{}, true
	default:
		return fileState{}, false
	}
}

// newReloadSignal returns a watcher for the reload file at path. In passive
// mode the file's current state is taken as the baseline, so a file that is
// already present does not trigger a reload.
func newReloadSignal(path string, passive bool) *reloadSignal {
	r := &reloadSignal{path: path, passive: passive}
	if passive {
		r.baseline, _ = statFile(path)
	}
	return r
}

// Poll reports whether a reload was requested.
func (r *reloadSignal) Poll() bool {
	if r.passive {
		return r.pollPassive()
	}
	return r.pollActive()
}

// pollActive consumes the reload file; it fires only if the removal succeeds.
func (r *reloadSignal) pollActive() bool {
	if _, err := os.Stat(r.path); err != nil {
		return false
	}
	return os.Remove(r.path) == nil
}

// pollPassive fires when the file is present and differs from the last state seen.
func (r *reloadSignal) pollPassive() bool {
	current, ok := statFile(r.path)
	if !ok {
		return false // unknown state: keep the baseline
	}
	previous := r.baseline
	r.baseline = current
	return current.exists && current != previous
}
