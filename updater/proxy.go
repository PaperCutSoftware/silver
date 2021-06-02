// SILVER - Service Wrapper
// Auto Updater
//
// Copyright (c) 2014-2021 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package main

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func setupHTTPProxy() {
	// Force if set via flag
	if len(*httpProxy) > 0 {
		_ = os.Setenv("HTTP_PROXY", *httpProxy)
		return
	}
	// Check Silver Environment
	proxy := os.Getenv("SILVER_HTTP_PROXY")
	if proxy != "" {
		_ = os.Setenv("HTTP_PROXY", proxy)
		return
	}
	// Proxy conf file
	if dat, err := ioutil.ReadFile("http-proxy.conf"); err == nil {
		proxy = strings.TrimSpace(string(dat))
	}
	if proxy != "" {
		_ = os.Setenv("HTTP_PROXY", proxy)
		return
	}
}

func turnOffHTTPProxy() {
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		t.Proxy = func(req *http.Request) (*url.URL, error) {
			return nil, nil
		}
	}
}
