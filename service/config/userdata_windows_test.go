// SILVER - Service Wrapper
//
// Copyright (c) 2014-2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.

package config_test

import (
	"testing"

	"github.com/papercutsoftware/silver/service/config"
)

func TestUserDataDir_Windows(t *testing.T) {
	tests := []struct {
		name, localAppData, appData, want string
		wantErr                           bool
	}{
		{"LOCALAPPDATA", `C:\Users\bob\AppData\Local`, `C:\Users\bob\AppData\Roaming`, `C:\Users\bob\AppData\Local\MyServiceName`, false},
		{"APPDATA fallback", "", `C:\Users\bob\AppData\Roaming`, `C:\Users\bob\AppData\Roaming\MyServiceName`, false},
		{"relative LOCALAPPDATA ignored", `AppData\Local`, `C:\Users\bob\AppData\Roaming`, `C:\Users\bob\AppData\Roaming\MyServiceName`, false},
		{"neither set", "", "", "", true},
		{"both relative", `AppData\Local`, `AppData\Roaming`, "", true},
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
