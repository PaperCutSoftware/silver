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

// parseProxy parses the proxy String and returns an error if it fails.
// Copied from net/httpproxy/proxy.go
func parseProxy(proxy string) (*url.URL, error) {
	if proxy == "" {
		return nil, nil
	}

	proxyURL, err := url.Parse(proxy)
	if err != nil ||
		(proxyURL.Scheme != "http" &&
			proxyURL.Scheme != "https" &&
			proxyURL.Scheme != "socks5") {
		// proxy was bogus. Try prepending "http://" to it and
		// see if that parses correctly. If not, we fall
		// through and complain about the original one.
		if proxyURL, err := url.Parse("http://" + proxy); err == nil {
			return proxyURL, nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("invalid proxy address %q: %v", proxy, err)
	}
	return proxyURL, nil
}

// setupHTTPProxy attempts to set the HTTP(S)_PROXY vars using
// the SILVER_HTTP_PROXY or http-proxy.conf file.
// Return an error if we attempted and failed to do so or the proxy
// string was not valid.
func setupHTTPProxy(httpProxyArg string) error {
	// Force if set via flag
	proxy := httpProxyArg

	if proxy == "" {
		// Else check Silver Environment
		proxy = os.Getenv("SILVER_HTTP_PROXY")
		if proxy == "" {
			// Else check proxy conf file
			// If conf file is empty of data then there is no proxy set
			if dat, err := os.ReadFile("http-proxy.conf"); err == nil {
				proxy = strings.TrimSpace(string(dat))
			} else {
				return err
			}
		}
	}

	if proxy != "" {
		if _, err := parseProxy(proxy); err != nil {
			return err
		}
		if err := os.Setenv("HTTP_PROXY", proxy); err != nil {
			return err
		}
		if err := os.Setenv("HTTPS_PROXY", proxy); err != nil {
			return err
		}
		fmt.Printf("Using proxy: %s\n", proxy)
	}
	return nil
}

func turnOffHTTPProxy() {
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		t.Proxy = func(req *http.Request) (*url.URL, error) {
			return nil, nil
		}
	}
}
