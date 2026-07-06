// SILVER - Service Wrapper
// Auto Updater
//
// Copyright (c) 2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

//go:build !windows && !darwin && !linux

package update

// osVersion is unknown on unsupported platforms; the X-OS-Version header is
// simply omitted.
func osVersion() (string, error) {
	return "", nil
}
