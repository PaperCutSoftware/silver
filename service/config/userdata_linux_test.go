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
func TestUserDataDir_Unix(t *testing.T) {
	u, err := user.Current()
	if err != nil {
		t.Fatalf("user.Current: %v", err)
	}
	homeDefault := filepath.Join(u.HomeDir, ".local", "share", "MyServiceName")
	tests := []struct {
		name, xdgDataHome, want string
	}{
		{"default", "", homeDefault},
		{"XDG_DATA_HOME", "/tmp/xdg-data", "/tmp/xdg-data/MyServiceName"},
		{"relative XDG_DATA_HOME ignored", "relative/dir", homeDefault},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XDG_DATA_HOME", tt.xdgDataHome)

			got, err := config.UserDataDir("MyServiceName")

			if err != nil {
				t.Fatalf("UserDataDir: %v", err)
			}
			if got != tt.want {
				t.Errorf("UserDataDir = %q, want %q", got, tt.want)
			}
		})
	}
}
