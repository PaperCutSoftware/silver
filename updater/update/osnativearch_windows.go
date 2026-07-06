// SILVER - Service Wrapper
// Auto Updater
//
// Copyright (c) 2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package update

import (
	"runtime"

	"golang.org/x/sys/windows"
)

// nativeArch returns the host CPU architecture, which differs from
// runtime.GOARCH when the binary runs under emulation (e.g. x64 on
// Windows-on-ARM). IsWow64Process2 reports the native machine regardless
// of emulation; it is unavailable before Windows 10 1511, but no such
// host is an ARM64 machine, so falling back to GOARCH is correct there.
func nativeArch() string {
	var processMachine, nativeMachine uint16
	if err := windows.IsWow64Process2(windows.CurrentProcess(), &processMachine, &nativeMachine); err != nil {
		return runtime.GOARCH
	}
	switch nativeMachine {
	case 0xAA64: // IMAGE_FILE_MACHINE_ARM64
		return "arm64"
	case 0x8664: // IMAGE_FILE_MACHINE_AMD64
		return "amd64"
	case 0x014C: // IMAGE_FILE_MACHINE_I386
		return "386"
	case 0x01C4: // IMAGE_FILE_MACHINE_ARMNT
		return "arm"
	}
	return runtime.GOARCH
}
