// Copyright (c) 2021 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.

// +build darwin linux

package svcutil

// SetServiceToRestart does nothing on macOS and Linux.
func SetServiceToRestart(conf RestartConfig) error {
	return nil
}
