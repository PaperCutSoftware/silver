// SILVER - Service Wrapper
//
// Copyright (c) 2014-2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
package main

import "os"

// reloadSignal watches for the .reload file that asks the service to restart.
type reloadSignal struct {
	path string
}

func newReloadSignal(path string) *reloadSignal {
	return &reloadSignal{path: path}
}

// Poll reports whether a reload was requested, consuming the signal file.
func (r *reloadSignal) Poll() bool {
	if _, err := os.Stat(r.path); err != nil {
		return false
	}
	return os.Remove(r.path) == nil
}
