// SILVER - Service Wrapper
// Auto Updater
//
// Copyright (c) 2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

//go:build !windows && !darwin && !linux

package update

import (
	"fmt"
	"runtime"
)

// osVersion has no detection on this platform; the caller omits the
// X-OS-Version header on error.
func osVersion() (string, error) {
	return "", fmt.Errorf("no OS version detection support for %s", runtime.GOOS)
}
