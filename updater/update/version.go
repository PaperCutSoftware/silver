// SILVER - Service Wrapper
// Auto Updater
//
// Copyright (c) 2014-2021 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package update

import (
	"github.com/papercutsoftware/silver/lib/osutils"
)

// ReadCurrentVersion returns current version as read from versionFile.
func ReadCurrentVersion(versionFile string) string {
	return osutils.ReadStringFromFile(versionFile, "1")
}
