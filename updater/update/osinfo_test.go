// SILVER - Service Wrapper
// Auto Updater
//
// Copyright (c) 2014-2021 PaperCut Software http://www.papercut.com/
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

	// windows, darwin and linux all report a dot-separated numeric version.
	switch runtime.GOOS {
	case "windows", "darwin", "linux":
		got := req.Header.Get(headerOSVersionKey)
		if matched := regexp.MustCompile(`^\d+(\.\d+)+`).MatchString(got); !matched {
			t.Errorf("%s = %q, want a dot-separated numeric version", headerOSVersionKey, got)
		}
	}
}
