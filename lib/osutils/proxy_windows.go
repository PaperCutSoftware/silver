// Copyright (c) 2016 PaperCut Software International Pty. Ltd.

package osutils

import (
	"fmt"
	"net/url"
	"strings"
	"syscall"
	"unsafe"
)

var (
	winhttp                               = syscall.NewLazyDLL("winhttp.dll")
	winHttpGetIEProxyConfigForCurrentUser = winhttp.NewProc("WinHttpGetIEProxyConfigForCurrentUser")
	winHttpGetProxyForUrl                 = winhttp.NewProc("WinHttpGetProxyForUrl")
	winHttpOpen                           = winhttp.NewProc("WinHttpOpen")
	winHttpSetTimeouts                    = winhttp.NewProc("WinHttpSetTimeouts")
	winHttpCloseHandle                    = winhttp.NewProc("WinHttpCloseHandle")
)

type winHttpCurrentUserIEProxyConfig struct {
	fAuthDetect       uint32
	lpszAutoConfigUrl *uint16
	lpszProxy         *uint16
	lpszProxyBypass   *uint16
}

type winHttpProxyInfo struct {
	dwAccessType    uint32
	lpszProxy       *uint16
	lpszProxyBypass *uint16
}

type winHttpAutoProxyOptions struct {
	dwFlags                uint32
	dwAutoDetectFlags      uint32
	lpszAutoConfigUrl      *uint16
	lpvReserved            *uint32
	dwReserved             uint32
	fAutoLogonIfChallenged uint32
}

const defaultCheckURL = "https://example.com"

func getHTTPProxies() ([]string, error) {
	none := []string{}
	proxyCfg, err := getProxyConfigForCurrentUser()
	if err != nil {
		return none, err
	}

	if proxyCfg.proxy != "" {
		return parseProxyList(proxyCfg.proxy), nil
	}

	// Have we got auto detect url?  Of not, return
	if proxyCfg.autoConfigUrl == "" {
		return none, nil
	}

	// FUTURE: Make this configurable if we have a need
	checkURL := defaultCheckURL
	if _, err := url.Parse(checkURL); err != nil {
		return none, fmt.Errorf("The supplied check URL is invalid: %v", err)
	}

	proxy, err := getProxyConfigFromURL(proxyCfg.autoConfigUrl, checkURL)
	if err != nil {
		return none, err
	}
	return parseProxyList(proxy), nil
}

func parseProxyList(list string) []string {

	if list == "" {
		return []string{""}
	}
	all := strings.Split(list, ";")
	allClean := make([]string, 0, len(all))
	for _, p := range all {
		allClean = append(allClean, validateProxy(p))
	}
	return allClean

}

// FUTURE: Inspect proxies and return a struct that differentiates between proxies intended for specific protocols
// We currently don't differentiate between proxies set for specific protocols.
func validateProxy (proxy string) string {
	advancedPrefixes := []string{"http=", "https=", "ftp=", "socks="}
	proxy = strings.TrimSpace(proxy)
	for _, prefix := range advancedPrefixes {
		if strings.HasPrefix(proxy,prefix) {
			return strings.TrimPrefix(proxy,prefix)
		}
	}
	return proxy
}

type proxyConfig struct {
	autoConfigUrl string
	proxy         string
	proxyBypass   string
}

func getProxyConfigForCurrentUser() (*proxyConfig, error) {
	pConfig := winHttpCurrentUserIEProxyConfig{}
	if r, _, e1 := winHttpGetIEProxyConfigForCurrentUser.Call(uintptr(unsafe.Pointer(&pConfig))); r == 0 {
		var err error
		if e1 != nil {
			err = e1
		} else {
			err = syscall.GetLastError()
		}
		return nil, err
	}
	c := &proxyConfig{
		proxy:         ptrToString(pConfig.lpszProxy, 1024),
		proxyBypass:   ptrToString(pConfig.lpszProxyBypass, 1024),
		autoConfigUrl: ptrToString(pConfig.lpszAutoConfigUrl, 1024),
	}
	return c, nil
}

func getProxyConfigFromURL(autoConfigURL string, checkURL string) (string, error) {
	const WINHTTP_AUTOPROXY_CONFIG_URL = 2

	hSession, err := openWinHttpSession()
	if err != nil {
		return "", err
	}
	// Best effort cleanup
	defer winHttpCloseHandle.Call(uintptr(hSession))

	lpcwszUrl, err := syscall.UTF16PtrFromString(checkURL)
	if err != nil {
		return "", err
	}

	lpszAutoConfigUrl, err := syscall.UTF16PtrFromString(autoConfigURL)
	if err != nil {
		return "", err
	}

	autoProxyOptions := winHttpAutoProxyOptions{
		dwFlags:                WINHTTP_AUTOPROXY_CONFIG_URL,
		lpszAutoConfigUrl:      lpszAutoConfigUrl,
		dwAutoDetectFlags:      0,
		fAutoLogonIfChallenged: 0,
	}

	proxyInfo := winHttpProxyInfo{}
	if r, _, e1 := winHttpGetProxyForUrl.Call(uintptr(hSession),
		uintptr(unsafe.Pointer(lpcwszUrl)),
		uintptr(unsafe.Pointer(&autoProxyOptions)),
		uintptr(unsafe.Pointer(&proxyInfo))); r == 0 {
		var err error
		if e1 != nil {
			err = e1
		} else {
			err = syscall.GetLastError()
		}
		return "", err
	}
	return ptrToString(proxyInfo.lpszProxy, 1024), nil
}

func openWinHttpSession() (syscall.Handle, error) {
	const WINHTTP_ACCESS_TYPE_NO_PROXY = 1

	returnHandle, _, e1 := winHttpOpen.Call(uintptr(0), WINHTTP_ACCESS_TYPE_NO_PROXY, uintptr(0), uintptr(0), 0)
	hInternet := syscall.Handle(returnHandle)
	if returnHandle == 0 {
		var err error
		if e1 != nil {
			err = e1
		} else {
			err = syscall.GetLastError()
		}
		return syscall.InvalidHandle, err
	}

	const tenSecs = 10000
	if r, _, e1 := winHttpSetTimeouts.Call(uintptr(hInternet), tenSecs, tenSecs, tenSecs, tenSecs); r == 0 {
		var err error
		if e1 != nil {
			err = e1
		} else {
			err = syscall.GetLastError()
		}
		return syscall.InvalidHandle, err
	}
	return hInternet, nil
}

func ptrToString(p *uint16, maxLength int) string {
	if p == nil {
		return ""
	}
	// Hack! Cast to a large array pointer, then "pretend" to slice to maxLength
	slice := (*[0xffff]uint16)(unsafe.Pointer(p))[:maxLength]
	ret := syscall.UTF16ToString(slice)
	return ret
}
