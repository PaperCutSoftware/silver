// SILVER - Service Wrapper
//
// Copyright (c) 2014-2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.

package config_test

import (
	"os/user"
	"path/filepath"
	"testing"

	"github.com/papercutsoftware/silver/service/config"
)

// user.Current() is cached per process, so ignoring $HOME under sudo is checked manually, not here.
func TestUserDataDir_Darwin(t *testing.T) {
	// Arrange
	u, err := user.Current()
	if err != nil {
		t.Fatalf("user.Current: %v", err)
	}
	// XDG_DATA_HOME is a Linux convention and must not affect macOS.
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg-data")
	want := filepath.Join(u.HomeDir, "Library", "Application Support", "MyServiceName")

	// Act
	got, err := config.UserDataDir("MyServiceName")

	// Assert
	if err != nil {
		t.Fatalf("UserDataDir: %v", err)
	}
	if got != want {
		t.Errorf("UserDataDir = %q, want %q", got, want)
	}
}
