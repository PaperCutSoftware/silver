// SILVER - Service Wrapper
//
// Copyright (c) 2014 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//
package run

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"errors"
	"net"
	"net/url"
)

var pingFileCache = struct {
	sync.Mutex
	m map[string]string
}{m: make(map[string]string)}

func pingURL(pingUrl string, timeout time.Duration) (ok bool, err error) {
	u, err := url.Parse(pingUrl)
	if err != nil {
		return true, errors.New("Invalid Ping URL!") // Assume OK
	}
	switch strings.ToLower(u.Scheme) {
	case "tcp":
		return pingTCP(u.Host, timeout)
	case "echo":
		return pingTCPEcho(u.Host, timeout)
	case "http":
		fallthrough
	case "https":
		return pingHTTP(pingUrl, timeout)
	case "file":
		return pingFile(pingUrl)
	default:
		return true, errors.New("Unsupported URL Scheme") // Assume OK
	}
}

func pingTCP(host string, timeout time.Duration) (ok bool, err error) {
	conn, err := net.DialTimeout("tcp", host, timeout)
	if err != nil {
		return false, err
	}
	conn.Close()
	return true, nil
}

func pingTCPEcho(host string, timeout time.Duration) (ok bool, err error) {
	conn, err := net.DialTimeout("tcp", host, timeout)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(timeout))

	// Challenge the server with a unique ping
	ping := fmt.Sprintf("ping-%d", time.Now().UTC())
	if _, err := fmt.Fprintf(conn, ping); err != nil {
		return false, err
	}
	buf := make([]byte, 1024)
	if _, err := conn.Read(buf); err != nil {
		return false, err
	}
	if !strings.Contains(string(buf), ping) {
		return false, errors.New("Server did not echo")
	}
	return true, nil
}

func pingFile(fileUrl string) (ok bool, err error) {
	file := strings.TrimPrefix(fileUrl, "file://")
	info, err := os.Stat(file)
	if err != nil {
		return true, err
	}
	stamp := fmt.Sprintf("%d%d", info.Size(), info.ModTime().UnixNano())
	pingFileCache.Lock()
	defer pingFileCache.Unlock()
	if v, ok := pingFileCache.m[file]; ok {
		if v == stamp {
			// No change!
			return false, errors.New(fmt.Sprintf("File %s did not change", file))
		}
	}
	pingFileCache.m[file] = stamp
	return true, nil
}
