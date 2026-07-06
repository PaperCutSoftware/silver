// SILVER - Service Wrapper
// Auto Updater
//
// Copyright (c) 2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

//go:build darwin

package update

import (
	"runtime"
	"syscall"
)

// nativeArch returns the host CPU architecture, which differs from
// runtime.GOARCH when an x64 binary runs under Rosetta 2 translation.
// sysctl.proc_translated is 1 for translated processes; the sysctl does
// not exist on Intel Macs (Sysctl returns an error).
func nativeArch() string {
	if translated, err := syscall.SysctlUint32("sysctl.proc_translated"); err == nil && translated == 1 {
		return "arm64"
	}
	return runtime.GOARCH
}
