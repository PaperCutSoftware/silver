// SILVER - Service Wrapper
//
// Copyright (c) 2014-2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.

package config_test

import (
	"os/user"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/papercutsoftware/silver/service/config"
)

func TestUserDataDir_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows only")
	}
	tests := []struct {
		name, localAppData, appData, want string
		wantErr                           bool
	}{
		{"LOCALAPPDATA", `C:\Users\bob\AppData\Local`, `C:\Users\bob\AppData\Roaming`, `C:\Users\bob\AppData\Local\MyServiceName`, false},
		{"APPDATA fallback", "", `C:\Users\bob\AppData\Roaming`, `C:\Users\bob\AppData\Roaming\MyServiceName`, false},
		{"neither set", "", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("LOCALAPPDATA", tt.localAppData)
			t.Setenv("APPDATA", tt.appData)

			got, err := config.UserDataDir("MyServiceName")

			if (err != nil) != tt.wantErr {
				t.Fatalf("UserDataDir error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("UserDataDir = %q, want %q", got, tt.want)
			}
		})
	}
}

// user.Current() is cached per process, so ignoring $HOME under sudo is checked manually, not here.
func TestUserDataDir_Unix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("not used on Windows")
	}
	// Arrange
	u, err := user.Current()
	if err != nil {
		t.Fatalf("user.Current: %v", err)
	}
	t.Setenv("XDG_DATA_HOME", "")
	want := filepath.Join(u.HomeDir, ".local", "share", "MyServiceName")
	if runtime.GOOS == "darwin" {
		want = filepath.Join(u.HomeDir, "Library", "Application Support", "MyServiceName")
	}

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

func TestUserDataDir_XDGDataHome(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		t.Skip("XDG_DATA_HOME is only used on Linux/Unix")
	}
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg-data")

	got, err := config.UserDataDir("MyServiceName")

	if err != nil {
		t.Fatalf("UserDataDir: %v", err)
	}
	if want := "/tmp/xdg-data/MyServiceName"; got != want {
		t.Errorf("UserDataDir = %q, want %q", got, want)
	}
}

func TestUserDataDir_RelativeXDGDataHomeIgnored(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		t.Skip("XDG_DATA_HOME is only used on Linux/Unix")
	}
	t.Setenv("XDG_DATA_HOME", "relative/dir")
	u, err := user.Current()
	if err != nil {
		t.Fatalf("user.Current: %v", err)
	}

	got, err := config.UserDataDir("MyServiceName")

	if err != nil {
		t.Fatalf("UserDataDir: %v", err)
	}
	if want := filepath.Join(u.HomeDir, ".local", "share", "MyServiceName"); got != want {
		t.Errorf("UserDataDir = %q, want %q", got, want)
	}
}
