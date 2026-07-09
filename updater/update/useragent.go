// SILVER - Service Wrapper
// Auto Updater
//
// Copyright (c) 2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package update

import "runtime/debug"

// userAgent returns the User-Agent for update check requests, e.g.
// "silver-updater/v1.6.1". The version is stamped automatically by the Go
// toolchain from the module's VCS tag; builds without VCS information
// (or from an untagged tree reporting "(devel)") omit it.
func userAgent() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return "silver-updater/" + v
		}
	}
	return "silver-updater"
}
