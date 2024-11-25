// SILVER - Service Wrapper
// Auto Updater
//
// Copyright (c) 2014-2021 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func setupHTTPProxy() {
	// Force if set via flag
	proxy := *httpProxy

	if proxy == "" {
		// Else check Silver Environment
		proxy = os.Getenv("SILVER_HTTP_PROXY")
		if proxy == "" {
			// Else check proxy conf file
			if dat, err := os.ReadFile("http-proxy.conf"); err == nil {
				proxy = strings.TrimSpace(string(dat))
			}
		}
	}

	if proxy != "" {
		fmt.Printf("Using proxy: %s\n", proxy)
		_ = os.Setenv("HTTP_PROXY", proxy)
		_ = os.Setenv("HTTPS_PROXY", proxy)
	}
}

func turnOffHTTPProxy() {
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		t.Proxy = func(req *http.Request) (*url.URL, error) {
			return nil, nil
		}
	}
}
