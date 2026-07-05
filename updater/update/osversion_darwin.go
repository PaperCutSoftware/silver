// SILVER - Service Wrapper
// Auto Updater
//
// Copyright (c) 2014-2021 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

//go:build darwin

package update

import "syscall"

// osVersion returns the macOS product version (e.g. "14.5").
func osVersion() (string, error) {
	return syscall.Sysctl("kern.osproductversion")
}
