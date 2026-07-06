// SILVER - Service Wrapper
// Auto Updater
//
// Copyright (c) 2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package update

import "golang.org/x/sys/unix"

// osVersion returns the Linux kernel version (e.g. "5.15.0") via uname,
// trimming distro suffixes from the release string ("5.15.0-91-generic").
func osVersion() (string, error) {
	var uts unix.Utsname
	if err := unix.Uname(&uts); err != nil {
		return "", err
	}
	return trimToNumericVersion(unix.ByteSliceToString(uts.Release[:])), nil
}
