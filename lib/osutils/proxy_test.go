// Copyright (c) 2016 PaperCut Software International Pty. Ltd.

package osutils_test

import (
	"robusta/utils/osutils"
	"testing"
)

// Just some simple tests for now that will work cross-platform

func TestGetHTTPProxyServer_ExpectNoError(t *testing.T) {
	// Act
	proxy, err := osutils.GetHTTPProxy()

	// Assert
	if err != nil {
		t.Errorf("Got error from GetHTTPProxy: %v", err)
	}
	t.Logf("Proxy is set to: '%s'", proxy)
}
