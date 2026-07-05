// SILVER - Service Wrapper
// Auto Updater
//
// Copyright (c) 2014-2021 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

//go:build windows

package update

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// osVersion returns the Windows version as major.minor.build (e.g.
// "10.0.19041"). RtlGetVersion reports the true OS version, unaffected by
// the compatibility shims that skew GetVersionEx.
func osVersion() (string, error) {
	info := windows.RtlGetVersion()
	return fmt.Sprintf("%d.%d.%d", info.MajorVersion, info.MinorVersion, info.BuildNumber), nil
}
