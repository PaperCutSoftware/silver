// SILVER - Service Wrapper
//
// Copyright (c) 2014-2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.

package config

import "path/filepath"

func userDataBaseDir() (string, error) {
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Application Support"), nil
}
