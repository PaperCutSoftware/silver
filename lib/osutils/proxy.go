// Copyright (c) 2016 PaperCut Software International Pty. Ltd.

package osutils

// GetHTTPProxy returns the system/current-user prefered HTTP proxy
// settings, or "" if none.
func GetHTTPProxy() (string, error) {
	proxies, err := getHTTPProxies()
	if err != nil {
		return "", err
	}
	// Return prefered (first)
	if len(proxies) == 0 {
		return "", nil
	}
	return proxies[0], nil
}

// GetHTTPProxies returns the system/current-user HTTP proxies as a list
// in system prefered order, or zero-length slice if none.
func GetHTTPProxies() ([]string, error) {
	return getHTTPProxies()
}
