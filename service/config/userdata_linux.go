// SILVER - Service Wrapper
//
// Copyright (c) 2014-2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.

package config

import (
	"os"
	"path/filepath"
)

// userDataBaseDir follows the XDG Base Directory spec, which says a relative XDG_DATA_HOME is
// invalid and must be ignored.
func userDataBaseDir() (string, error) {
	if dir := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(dir) {
		return dir, nil
	}
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share"), nil
}
