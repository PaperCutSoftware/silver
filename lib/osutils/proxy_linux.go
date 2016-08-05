// Copyright (c) 2016 PaperCut Software International Pty. Ltd.

package osutils

import "os"

func getHTTPProxies() ([]string, error) {
	// FUTURE: Also use gconftool -R /system/http_proxy  ?
	proxy := os.Getenv("HTTP_PROXY")
	if proxy == "" {
		proxy = os.Getenv("http_proxy")
	}
	return []string{proxy}, nil
}
