// SILVER - Service Wrapper
// Auto Updater
//
// Copyright (c) 2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package update

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"
)

const (
	headerOSTypeKey     string = "X-OS-Type"
	headerOSVersionKey  string = "X-OS-Version"
	headerOSArchKey     string = "X-OS-Arch"
	headerBinaryArchKey string = "X-Binary-Arch"
)

// trimToNumericVersion trims a version string to its leading dot-separated
// numeric part, dropping suffixes like the distro patch, kernel flavor and
// architecture in a Linux kernel release ("5.15.0-91-generic" → "5.15.0").
// The update server compares versions segment-by-segment numerically, so
// only digits and dots may be sent.
func trimToNumericVersion(version string) string {
	end := 0
	for end < len(version) && (version[end] == '.' || ('0' <= version[end] && version[end] <= '9')) {
		end++
	}
	return strings.TrimRight(version[:end], ".")
}

// setOSHeaders sets the OS identity headers used by the update server to
// select a build compatible with this host. The server keys build selection
// on X-OS-Type + X-OS-Arch; X-Binary-Arch reports what the running updater
// was compiled for, which differs from X-OS-Arch under emulation (Rosetta 2,
// Windows-on-ARM) so those installs migrate to native builds.
func setOSHeaders(req *http.Request) {
	req.Header.Set(headerOSTypeKey, runtime.GOOS)
	// Best-effort host architecture: detects Rosetta 2 and Windows-on-ARM
	// emulation; on other platforms falls back to the binary's arch.
	req.Header.Set(headerOSArchKey, nativeArch())
	req.Header.Set(headerBinaryArchKey, runtime.GOARCH)
	// Best effort: an unknown OS version is better than a failed update check.
	if version, err := osVersion(); err == nil {
		req.Header.Set(headerOSVersionKey, version)
	} else {
		fmt.Printf("Couldn't detect OS version: %v.\n", err)
	}
}
