// Copyright (c) 2016 PaperCut Software International Pty. Ltd.

package osutils

// GetHTTPProxy returns the system/current-user HTTP proxy settings, or "" if none.
func GetHTTPProxy() (string, error) {
	return getHTTPProxy()
}
