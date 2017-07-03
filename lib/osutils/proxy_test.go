// Copyright (c) 2016 PaperCut Software International Pty. Ltd.

package osutils_test

import (
	"testing"

	"github.com/papercutsoftware/silver/lib/osutils"
)

// Just some simple tests for now that will work cross-platform

func TestGetHTTPProxiesServer_ExpectNoError(t *testing.T) {
	// Act
	proxies, err := osutils.GetHTTPProxies()

	// Assert
	if err != nil {
		t.Errorf("Got error from GetHTTPProxy: %v", err)
	}
	t.Logf("Proxies is set to: '%#v'", proxies)
}

func TestGetHTTPProxyServer_ExpectNoError(t *testing.T) {
	// Act
	proxy, err := osutils.GetHTTPProxy()

	// Assert
	if err != nil {
		t.Errorf("Got error from GetHTTPProxy: %v", err)
	}
	t.Logf("Proxy is set to: '%s'", proxy)
}
