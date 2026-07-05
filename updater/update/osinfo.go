// SILVER - Service Wrapper
// Auto Updater
//
// Copyright (c) 2014-2021 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package update

import (
	"net/http"
	"runtime"
)

const (
	headerOSTypeKey    string = "X-OS-Type"
	headerOSVersionKey string = "X-OS-Version"
	headerOSArchKey    string = "X-OS-Arch"
)

// addOSContextToRequestHeader adds OS identity headers used by the update
// server to select a build compatible with this host. GOOS and GOARCH are
// sent verbatim so no OS/arch mapping code ships in the client.
func addOSContextToRequestHeader(req *http.Request) {
	req.Header.Set(headerOSTypeKey, runtime.GOOS)
	req.Header.Set(headerOSArchKey, runtime.GOARCH)
	// Best effort: an unknown OS version is better than a failed update check.
	if version, err := osVersion(); err == nil && version != "" {
		req.Header.Set(headerOSVersionKey, version)
	}
}
