// SILVER - Service Wrapper
//
// Copyright (c) 2016 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//
// +build !nohttp

package svcutil

import (
	"errors"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

func pingHTTP(pingURL string, timeout time.Duration) (ok bool, err error) {
	client := httpClientWithTimeout(timeout)
	resp, err := client.Get(pingURL)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return false, errors.New("The HTTP status was not 200 OK")
	}
	if _, err := io.Copy(ioutil.Discard, resp.Body); err != nil {
		return false, err
	}
	return true, nil // OK
}

func httpClientWithTimeout(timeout time.Duration) *http.Client {
	tdial := func(network, addr string) (conn net.Conn, err error) {
		conn, err = net.DialTimeout(network, addr, timeout)
		if err != nil {
			return nil, err
		}
		conn.SetDeadline(time.Now().Add(timeout))
		return conn, err
	}

	return &http.Client{
		Transport: &http.Transport{
			Dial: tdial,
		},
	}
}
