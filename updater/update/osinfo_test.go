// SILVER - Service Wrapper
// Auto Updater
//
// Copyright (c) 2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package update

import (
	"net/http"
	"regexp"
	"runtime"
	"testing"
)

func TestTrimToNumericVersion(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"vanilla kernel", "6.17.24", "6.17.24"},
		{"ubuntu", "5.15.0-91-generic", "5.15.0"},
		{"debian", "6.1.0-13-amd64", "6.1.0"},
		{"rhel", "4.18.0-477.10.1.el8_8.x86_64", "4.18.0"},
		{"amazon linux", "6.1.66-91.160.amzn2023.x86_64", "6.1.66"},
		{"no trailing dot kept", "5.15.", "5.15"},
		{"non-numeric prefix", "generic-5.15", ""},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := trimToNumericVersion(tt.input); got != tt.want {
				t.Errorf("trimToNumericVersion(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestAddOSContextToRequestHeader(t *testing.T) {
	req, err := http.NewRequest("GET", "https://example.com/check-update/test", nil)
	if err != nil {
		t.Fatal(err)
	}

	addOSContextToRequestHeader(req)

	if got := req.Header.Get(headerOSTypeKey); got != runtime.GOOS {
		t.Errorf("%s = %q, want %q", headerOSTypeKey, got, runtime.GOOS)
	}
	if got := req.Header.Get(headerOSArchKey); got != runtime.GOARCH {
		t.Errorf("%s = %q, want %q", headerOSArchKey, got, runtime.GOARCH)
	}

	// Native arch is the binary arch except under emulation, where it must
	// be arm64 (Rosetta 2 and Windows-on-ARM both emulate on arm64 hosts).
	nativeGot := req.Header.Get(headerOSNativeArchKey)
	if nativeGot != runtime.GOARCH && nativeGot != "arm64" {
		t.Errorf("%s = %q, want %q or \"arm64\"", headerOSNativeArchKey, nativeGot, runtime.GOARCH)
	}

	// windows, darwin and linux all report a dot-separated numeric version.
	switch runtime.GOOS {
	case "windows", "darwin", "linux":
		got := req.Header.Get(headerOSVersionKey)
		if matched := regexp.MustCompile(`^\d+(\.\d+)+`).MatchString(got); !matched {
			t.Errorf("%s = %q, want a dot-separated numeric version", headerOSVersionKey, got)
		}
	}
}
