// SILVER - Service Wrapper
//
// Copyright (c) 2014-2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.

package config

import (
	"errors"
	"os"
	"path/filepath"
)

func userDataBaseDir() (string, error) {
	for _, env := range []string{"LOCALAPPDATA", "APPDATA"} {
		if dir := os.Getenv(env); filepath.IsAbs(dir) {
			return dir, nil
		}
	}
	return "", errors.New("neither LOCALAPPDATA nor APPDATA is set to an absolute path")
}
