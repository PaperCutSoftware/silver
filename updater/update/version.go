/*
 * Copyright © 2021 PaperCut Software International Pty. Ltd.
 */

package update

import (
	"github.com/papercutsoftware/silver/lib/osutils"
)

func ReadCurrentVersion(versionFile string) string {
	return osutils.ReadStringFromFile(versionFile, "1")
}
