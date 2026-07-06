// SILVER - Service Wrapper
// Auto Updater
//
// Copyright (c) 2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package update

import (
	"errors"
	"syscall"
)

// osVersion returns the macOS product version (e.g. "14.5").
func osVersion() (string, error) {
	version, err := syscall.Sysctl("kern.osproductversion")
	if err != nil {
		return "", err
	}
	if version == "" {
		return "", errors.New("kern.osproductversion is empty")
	}
	return version, nil
}
