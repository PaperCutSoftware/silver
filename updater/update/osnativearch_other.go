// SILVER - Service Wrapper
// Auto Updater
//
// Copyright (c) 2014-2021 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

//go:build !windows && !darwin

package update

import "runtime"

// nativeArch assumes the host architecture matches the binary on platforms
// without a reliable emulation signal (user-mode emulation on Linux is rare
// and deliberately hides itself from uname).
func nativeArch() string {
	return runtime.GOARCH
}
