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
)

type winHttpCurrentUserIEProxyConfig struct {
	fAuthDetect       uint32
	lpszAutoConfigUrl *uint16
	lpszProxy         *uint16
	lpszProxyBypass   *uint16
}

/* assumptions:
DWORD = unint32
LPVOID= *uint32
LPCWSTR = *uint32

*/
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

func getHTTPProxy() (string, error) {
	fmt.Printf("TEMP: testing\n")
	// TODO: config file proxyurl overrides this default url
	url, err := url.Parse("https://example.com")
	if err != nil {
		return "", err
	}

	proxy, err := getProxyConfigFromURL(url)
	if err != nil {
		return "", err
	} else if proxy != "" {
		fmt.Printf("proxy config from url %v\n", proxy)
		return proxy, nil
	}

	// TODO we should test the connection here.
	proxy, err = getProxyConfigForCurrentUser()
	if err != nil {
		return "", err
	} else if proxy != "" {
		fmt.Printf("proxy config from url %v\n", proxy)
		return proxy, nil
	}
	return proxy, nil
}

func getProxyConfigForCurrentUser() (string, error) {
	proxyConfig := winHttpCurrentUserIEProxyConfig{}
	if r, _, e1 := winHttpGetIEProxyConfigForCurrentUser.Call(uintptr(unsafe.Pointer(&proxyConfig))); r == 0 {
		var err error
		if e1 != nil {
			err = e1
		} else {
			err = syscall.GetLastError()
		}
		return "", err
	}

	return ptrToString(proxyConfig.lpszProxy, 1024), nil
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

	//should we continue on error? only setting the defaults a bit lower from 1 min each stage
	if r, _, e1 := winHttpSetTimeouts.Call(uintptr(hInternet), 10000, 10000, 10000, 10000); r == 0 {

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

// TODO add more logging?

func getProxyConfigFromURL(url *url.URL) (string, error) {
	const WINHTTP_AUTOPROXY_AUTO_DETECT = 1
	const WINHTTP_AUTO_DETECT_TYPE_DHCP = 1
	const WINHTTP_AUTO_DETECT_TYPE_DNS_A = 2

	hSession, err := openWinHttpSession()
	if err != nil {
		return "", err
	}
	fmt.Println("querieng api")
	// FIXME need 32 bit but not there?
	//
	lpcwszUrl, err := syscall.UTF16PtrFromString(url.String())
	if err != nil {
		return "", err
	}

	autoProxyOptions := winHttpAutoProxyOptions{
		dwFlags:                WINHTTP_AUTOPROXY_AUTO_DETECT,
		dwAutoDetectFlags:      WINHTTP_AUTO_DETECT_TYPE_DHCP | WINHTTP_AUTO_DETECT_TYPE_DNS_A,
		fAutoLogonIfChallenged: 0,
	}

	proxyInfo := winHttpProxyInfo{}
	if r, _, e1 := winHttpGetProxyForUrl.Call(uintptr(hSession), uintptr(unsafe.Pointer(lpcwszUrl)), uintptr(unsafe.Pointer(&autoProxyOptions)), uintptr(unsafe.Pointer(&proxyInfo))); r == 0 {
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

func ptrToString(p *uint16, maxLength int) string {
	if p == nil {
		return ""
	}
	// Hack! Cast to a large array pointer, then "pretend" to slice to maxLength
	slice := (*[0xffff]uint16)(unsafe.Pointer(p))[:maxLength]
	ret := syscall.UTF16ToString(slice)
	return ret
}
