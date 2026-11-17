// SILVER - Service Wrapper
//
// Copyright (c) 2014-2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.

package config

import (
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
)

// UserDataDir returns the current user's data directory for appName, used for ${UserData}:
//
//	Windows: %LOCALAPPDATA%\<appName> (fallback %APPDATA%)
//	macOS:   <home>/Library/Application Support/<appName>
//	Linux:   ${XDG_DATA_HOME:-<home>/.local/share}/<appName>
func UserDataDir(appName string) (string, error) {
	base, err := userDataBaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appName), nil
}

func userDataBaseDir() (string, error) {
	switch runtime.GOOS {
	case "windows":
		for _, env := range []string{"LOCALAPPDATA", "APPDATA"} {
			if dir := os.Getenv(env); dir != "" {
				return dir, nil
			}
		}
		return "", errors.New("neither LOCALAPPDATA nor APPDATA is set")
	case "darwin":
		home, err := homeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support"), nil
	default:
		if dir := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(dir) {
			return dir, nil
		}
		home, err := homeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "share"), nil
	}
}

// homeDir looks up home from the uid, not $HOME: sudo can keep the invoking user's $HOME.
func homeDir() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	if u.HomeDir == "" {
		return "", errors.New("current user has no home directory")
	}
	return u.HomeDir, nil
}
