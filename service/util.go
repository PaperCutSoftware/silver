// SILVER - Service Wrapper
//
// Copyright (c) 2014 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//
package main

import (
	"path/filepath"
	"strings"

	"bitbucket.org/kardianos/osext"
)

func exePath() string {
	exePath, err := osext.Executable()
	if err != nil {
		panic(err)
	}
	return exePath
}

func exeName() (exeName string) {
	return filepath.Base(exePath())
}

func exeFolder() string {
	exeFolder, err := osext.ExecutableFolder()
	if err != nil {
		panic(err)
	}
	return exeFolder
}

func serviceName() (name string) {
	name = exeName()
	// Strip of ".exe" portion if found
	extension := filepath.Ext(name)
	if strings.ToLower(extension) == ".exe" {
		return name[0 : len(name)-4]
	}
	return name
}
