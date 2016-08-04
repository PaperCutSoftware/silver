// Copyright (c) 2016 PaperCut Software International Pty. Ltd.

package osutils

import (
	"fmt"
	"net/url"
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

func getHTTPProxy() (string, error) {
	proxyCfg, err := getProxyConfigForCurrentUser()
	if err != nil {
		return "", err
	} else if proxyCfg.proxy != "" {
		return proxyCfg.proxy, nil
	}

	// Have we got auto detect url?  Of not, return
	if proxyCfg.autoConfigUrl == "" {
		return "", nil
	}

	// FUTURE: Make this configurable if we have a need
	checkURL := defaultCheckURL
	if _, err := url.Parse(checkURL); err != nil {
		return "", fmt.Errorf("The supplied check URL is invalid: %v", err)
	}

	proxy, err := getProxyConfigFromURL(proxyCfg.autoConfigUrl, checkURL)
	if err != nil {
		return "", err
	}
	fmt.Printf("proxy config from url %v\n", proxy)
	return proxy, nil
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
