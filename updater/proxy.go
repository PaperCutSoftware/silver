// SILVER - Service Wrapper
// Auto Updater
//
// Copyright (c) 2014-2025 PaperCut Software http://www.papercut.com/
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

// setupHTTPProxy attempts to set the HTTP(S)_PROXY vars using
// the SILVER_HTTP_PROXY or http-proxy.conf file.
// Return an error if we attempted and failed to do so.
func setupHTTPProxy(proxy string) error {
	// the proxy could be from a command line argument
	if proxy != "" {
		return setProxyEnv(proxy)
	}

	proxy = os.Getenv("SILVER_HTTP_PROXY")
	if proxy != "" {
		return setProxyEnv(proxy)
	}

	dat, err := os.ReadFile("http-proxy.conf")
	if err != nil {
		return err
	}
	proxy = strings.TrimSpace(string(dat))
	if proxy != "" {
		return setProxyEnv(proxy)
	}

	return nil
}

func setProxyEnv(proxy string) error {
	if err := os.Setenv("HTTP_PROXY", proxy); err != nil {
		return err
	}
	if err := os.Setenv("HTTPS_PROXY", proxy); err != nil {
		return err
	}
	fmt.Printf("Using HTTP/HTTPS proxy: %s\n", proxy)
	return nil
}

func turnOffHTTPProxy() {
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		t.Proxy = func(req *http.Request) (*url.URL, error) {
			return nil, nil
		}
	}
}
