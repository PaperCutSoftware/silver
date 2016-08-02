// Copyright (c) 2016 PaperCut Software International Pty. Ltd.

package osutils

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	winhttp                               = syscall.NewLazyDLL("winhttp.dll")
	winHttpGetIEProxyConfigForCurrentUser = winhttp.NewProc("WinHttpGetIEProxyConfigForCurrentUser")
)

type winHttpCurrentUserIEProxyConfig struct {
	fAuthDetect       uint32
	lpszAutoConfigUrl *uint16
	lpszProxy         *uint16
	lpszProxyBypass   *uint16
}

func getHTTPProxy() (string, error) {
	proxyConfig := winHttpCurrentUserIEProxyConfig{}
	if r, _, e1 := winHttpGetIEProxyConfigForCurrentUser.Call(uintptr(unsafe.Pointer(&proxyConfig))); r == 0 {
		fmt.Printf("r %v, e1 %v\n", r, e1)
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

func ptrToString(p *uint16, maxLength int) string {
	if p == nil {
		return ""
	}
	// Hack! Cast to a large array pointer, then "pretend" to slice to maxLength
	slice := (*[0xffff]uint16)(unsafe.Pointer(p))[:maxLength]
	ret := syscall.UTF16ToString(slice)
	return ret
}
